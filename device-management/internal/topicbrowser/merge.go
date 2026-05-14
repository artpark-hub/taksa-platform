package topicbrowser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"

	unsv1 "github.com/artpark-hub/taksa-platform/device-management/api/uns/v1"
)

// UpsertRow is one materialized UNS topic row for persistence.
type UpsertRow struct {
	UnsTreeID      string
	CanonicalTopic string
	Level0         string
	LocationLevels []string
	DataContract   string
	VirtualPath    string
	Name           string
	Metadata       map[string]string
	LastEvent      *unsv1.EventTableEntry
}

// MergeResult is the outcome of merging topicBrowser from one status message.
type MergeResult struct {
	Rows                 []UpsertRow
	FullCatalogReplace   bool // when true, DM should replace all device_topics for this device with Rows (edge sent a full UNS map in one bundle, or empty catalog)
	ReportedTopicCount   int  // core.topicBrowser.topicCount from status (0 = valid)
}

// MergeFromStatusMessageContent extracts core.topicBrowser from a status message body,
// decodes UnsBundle protobuf frames in key order, and returns merged topic rows
// (latest event per uns_tree_id wins by produced_at_ms).
//
// FullCatalogReplace is derived only from fields already present on the wire:
//   - topicCount == 0  → treat as authoritative empty catalog (replace with no rows).
//   - some decoded bundle's uns_map has len(entries) == topicCount and topicCount > 0
//     → that bundle is a full UNS snapshot (same shape as umh-core cache bundle), safe to replace DB rows.
// Otherwise only incremental topic info is present → callers should upsert without deleting stale keys.
func MergeFromStatusMessageContent(messageContent string) (MergeResult, error) {
	var out MergeResult
	payload, err := DecodeStatusPayload(messageContent)
	if err != nil {
		return out, err
	}
	core := jsonMap(payload, "core", "Core")
	if core == nil {
		return out, nil
	}
	tb := jsonMap(core, "topicBrowser", "TopicBrowser")
	if tb == nil {
		return out, nil
	}
	topicCount := parseTopicCount(tb)
	out.ReportedTopicCount = topicCount

	rawBundles, ok := tb["unsBundles"]
	if !ok {
		rawBundles = tb["UnsBundles"]
	}
	if rawBundles == nil {
		// No bundles; still honor empty catalog if edge says so.
		out.FullCatalogReplace = topicCount == 0
		return out, nil
	}
	bundleMap, ok := rawBundles.(map[string]interface{})
	if !ok || len(bundleMap) == 0 {
		out.FullCatalogReplace = topicCount == 0
		return out, nil
	}
	indices := make([]int, 0, len(bundleMap))
	for k := range bundleMap {
		i, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		indices = append(indices, i)
	}
	sort.Ints(indices)

	topics := make(map[string]*unsv1.TopicInfo)
	events := make(map[string]*unsv1.EventTableEntry)
	fullCatalogReplace := false

	if topicCount == 0 {
		fullCatalogReplace = true
	}

	for _, idx := range indices {
		key := strconv.Itoa(idx)
		val := bundleMap[key]
		b, err := bundleBytesFromJSONValue(val)
		if err != nil || len(b) == 0 {
			continue
		}
		var ub unsv1.UnsBundle
		if err := proto.Unmarshal(b, &ub); err != nil {
			continue
		}
		if um := ub.GetUnsMap(); um != nil && um.GetEntries() != nil {
			n := len(um.GetEntries())
			if topicCount > 0 && n == topicCount {
				fullCatalogReplace = true
			}
			for id, info := range um.GetEntries() {
				if info == nil {
					continue
				}
				topics[id] = info
			}
		}
		if et := ub.GetEvents(); et != nil {
			for _, ent := range et.GetEntries() {
				if ent == nil || ent.GetUnsTreeId() == "" {
					continue
				}
				id := ent.GetUnsTreeId()
				prev, ok := events[id]
				if !ok || ent.GetProducedAtMs() > prev.GetProducedAtMs() {
					events[id] = ent
				}
			}
		}
	}

	rows := make([]UpsertRow, 0, len(topics))
	for id, info := range topics {
		hash := HashTopicInfo(info)
		if hash == "" {
			hash = id
		}
		canonical := BuildCanonicalTopic(info)
		if canonical == "" {
			continue
		}
		vpath := ""
		if info.VirtualPath != nil {
			vpath = info.GetVirtualPath()
		}
		md := info.GetMetadata()
		if md == nil {
			md = map[string]string{}
		}
		loc := append([]string(nil), info.GetLocationSublevels()...)
		row := UpsertRow{
			UnsTreeID:      hash,
			CanonicalTopic: canonical,
			Level0:         info.GetLevel0(),
			LocationLevels: loc,
			DataContract:   info.GetDataContract(),
			VirtualPath:    vpath,
			Name:           info.GetName(),
			Metadata:       md,
			LastEvent:      events[hash],
		}
		if row.LastEvent == nil {
			if e, ok := events[id]; ok {
				row.LastEvent = e
			}
		}
		rows = append(rows, row)
	}

	out.Rows = rows
	out.FullCatalogReplace = fullCatalogReplace
	return out, nil
}

func parseTopicCount(tb map[string]interface{}) int {
	if tb == nil {
		return -1
	}
	var raw interface{}
	for _, k := range []string{"topicCount", "TopicCount"} {
		if v, ok := tb[k]; ok {
			raw = v
			break
		}
	}
	if raw == nil {
		return -1
	}
	switch v := raw.(type) {
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return -1
		}
		return int(i)
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return -1
		}
		return i
	case float64:
		return int(v)
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	default:
		return -1
	}
}

func bundleBytesFromJSONValue(val interface{}) ([]byte, error) {
	switch t := val.(type) {
	case string:
		return base64.StdEncoding.DecodeString(t)
	case []byte:
		return t, nil
	case []interface{}:
		return nil, fmt.Errorf("unexpected bundle type slice")
	case json.Number:
		return nil, fmt.Errorf("unexpected bundle type number")
	default:
		if s, ok := t.(string); ok {
			return base64.StdEncoding.DecodeString(s)
		}
		return nil, fmt.Errorf("unexpected bundle type %T", val)
	}
}

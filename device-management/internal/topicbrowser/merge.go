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

// CatalogSyncMode describes how DM should persist this merge.
type CatalogSyncMode string

const (
	CatalogSyncIncremental CatalogSyncMode = "INCREMENTAL"
	CatalogSyncFullReplace CatalogSyncMode = "FULL_REPLACE"
	CatalogSyncEmpty       CatalogSyncMode = "EMPTY"
)

// MergeResult is the outcome of merging topicBrowser from one status message.
type MergeResult struct {
	Rows               []UpsertRow
	FullCatalogReplace bool // when true, DM should replace all device_topics for this device with Rows
	ReportedTopicCount int  // core.topicBrowser.topicCount from status (-1 if missing)
	SyncMode           CatalogSyncMode
	HadBundleZero      bool // unsBundles contained key "0" with a decodable full snapshot
}

// MergeFromStatusMessageContent extracts core.topicBrowser from a status message body,
// decodes UnsBundle protobuf frames in key order, and returns merged topic rows
// (latest event per uns_tree_id wins by produced_at_ms).
//
// FullCatalogReplace is derived only from fields already present on the wire:
//   - topicCount == 0  → authoritative empty catalog (clear rows).
//   - unsBundles["0"] decodes to a bundle whose uns_map has len(entries) == topicCount and topicCount > 0
//     → bootstrap full snapshot (umh-core sends complete cache as bundle 0 for new subscribers).
// Otherwise incremental upsert only (no deletes of stale topics).
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
	authoritativeEmpty := topicCount == 0 && topicBrowserCatalogAuthoritative(tb)

	rawBundles, ok := tb["unsBundles"]
	if !ok {
		rawBundles = tb["UnsBundles"]
	}
	if rawBundles == nil {
		if authoritativeEmpty {
			out.FullCatalogReplace = true
			out.SyncMode = CatalogSyncEmpty
		} else {
			out.SyncMode = CatalogSyncIncremental
		}
		return out, nil
	}
	bundleMap, ok := rawBundles.(map[string]interface{})
	if !ok || len(bundleMap) == 0 {
		if authoritativeEmpty {
			out.FullCatalogReplace = true
			out.SyncMode = CatalogSyncEmpty
		} else {
			out.SyncMode = CatalogSyncIncremental
		}
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
	hadBundleZero := false

	if authoritativeEmpty {
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
			// Bundle 0 is the umh-core bootstrap full cache snapshot for new subscribers.
			if idx == 0 && topicCount > 0 && n == topicCount {
				fullCatalogReplace = true
				hadBundleZero = true
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
	out.HadBundleZero = hadBundleZero
	switch {
	case authoritativeEmpty:
		out.SyncMode = CatalogSyncEmpty
	case fullCatalogReplace:
		out.SyncMode = CatalogSyncFullReplace
	default:
		out.SyncMode = CatalogSyncIncremental
	}
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

// topicBrowserCatalogAuthoritative is false when umh-core reports topicCount:0 on a degraded/error
// topic browser (Go JSON always emits topicCount). Those heartbeats must not wipe DM's catalog.
func topicBrowserCatalogAuthoritative(tb map[string]interface{}) bool {
	if tb == nil {
		return false
	}
	h := jsonMap(tb, "health", "Health")
	if h == nil {
		return true
	}
	switch v := healthCategoryRaw(h); v {
	case "active", "neutral", "":
		return true
	case "degraded", "unknown":
		return false
	default:
		// umh-core HealthCategory is numeric in JSON: 0=Neutral, 1=Active, 2=Degraded
		switch v {
		case "0", "1":
			return true
		default:
			return false
		}
	}
}

func healthCategoryRaw(h map[string]interface{}) string {
	if h == nil {
		return ""
	}
	for _, k := range []string{"category", "Category"} {
		if v, ok := h[k]; ok {
			switch t := v.(type) {
			case string:
				return strings.ToLower(strings.TrimSpace(t))
			case float64:
				return fmt.Sprintf("%d", int(t))
			case json.Number:
				i, err := t.Int64()
				if err != nil {
					return ""
				}
				return fmt.Sprintf("%d", i)
			case int:
				return fmt.Sprintf("%d", t)
			case int32:
				return fmt.Sprintf("%d", t)
			case int64:
				return fmt.Sprintf("%d", t)
			}
		}
	}
	return ""
}

func jsonStringField(m map[string]interface{}, keys ...string) string {
	if m == nil {
		return ""
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		for alt, val := range m {
			if strings.EqualFold(alt, k) {
				if s, ok := val.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}

func bundleBytesFromJSONValue(val interface{}) ([]byte, error) {
	switch t := val.(type) {
	case string:
		return base64.StdEncoding.DecodeString(t)
	case []byte:
		return t, nil
	case []interface{}:
		b := make([]byte, 0, len(t))
		for _, elem := range t {
			switch n := elem.(type) {
			case float64:
				b = append(b, byte(n))
			case json.Number:
				i, err := n.Int64()
				if err != nil {
					return nil, fmt.Errorf("bundle byte: %w", err)
				}
				b = append(b, byte(i))
			default:
				return nil, fmt.Errorf("unexpected bundle slice element %T", elem)
			}
		}
		return b, nil
	case json.Number:
		return nil, fmt.Errorf("unexpected bundle type number")
	default:
		return nil, fmt.Errorf("unexpected bundle type %T", val)
	}
}

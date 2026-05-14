package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	unsv1 "github.com/artpark-hub/taksa-platform/device-management/api/uns/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/topicbrowser"
	"google.golang.org/protobuf/encoding/protojson"
)

// DeviceTopicRepo persists materialized UNS topics from edge status.
type DeviceTopicRepo struct {
	data *Data
}

// NewDeviceTopicRepo constructs a repo bound to the data layer.
func NewDeviceTopicRepo(d *Data) *DeviceTopicRepo {
	return &DeviceTopicRepo{data: d}
}

// ListDeviceTopicsQuery filters and pages topic rows.
type ListDeviceTopicsQuery struct {
	DeviceID string
	Text     string // substring on canonical path or metadata (ILIKE), GraphQL text filter semantics
	Meta     []TopicMetaEq
	Offset   int64
	Limit    int64 // page size, max 100
}

// TopicMetaEq is one metadata key = value constraint.
type TopicMetaEq struct {
	Key string
	Eq  string
}

// UpsertDeviceTopics merges topic rows from one status sync.
func (r *DeviceTopicRepo) UpsertDeviceTopics(ctx context.Context, tenantID, deviceID string, rows []topicbrowser.UpsertRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := r.data.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	for _, row := range rows {
		id := deviceID + ":" + row.UnsTreeID
		locJSON, err := json.Marshal(row.LocationLevels)
		if err != nil {
			return err
		}
		mdJSON, err := json.Marshal(row.Metadata)
		if err != nil {
			return err
		}
		var lastEv interface{}
		if row.LastEvent != nil {
			b, err := protojson.Marshal(row.LastEvent)
			if err != nil {
				return err
			}
			lastEv = string(b)
		}
		var lastAt interface{}
		if row.LastEvent != nil && row.LastEvent.GetProducedAtMs() > 0 {
			lastAt = time.UnixMilli(int64(row.LastEvent.GetProducedAtMs()))
		}
		var vpathArg interface{}
		if row.VirtualPath != "" {
			vpathArg = row.VirtualPath
		}
		q := `
INSERT INTO device_topics (
  id, tenant_id, device_id, uns_tree_id, canonical_topic,
  level0, location_sublevels, data_contract, virtual_path, name,
  metadata_json, last_event_json, last_event_at, last_synced, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?::jsonb, ?, ?, ?, ?::jsonb, ?::jsonb, ?, ?, ?, ?)
ON CONFLICT (tenant_id, device_id, uns_tree_id) DO UPDATE SET
  canonical_topic = EXCLUDED.canonical_topic,
  level0 = EXCLUDED.level0,
  location_sublevels = EXCLUDED.location_sublevels,
  data_contract = EXCLUDED.data_contract,
  virtual_path = EXCLUDED.virtual_path,
  name = EXCLUDED.name,
  metadata_json = EXCLUDED.metadata_json,
  last_event_json = EXCLUDED.last_event_json,
  last_event_at = EXCLUDED.last_event_at,
  last_synced = EXCLUDED.last_synced,
  updated_at = EXCLUDED.updated_at
`
		q = convertPlaceholders(q)
		_, err = tx.ExecContext(ctx, q,
			id, tenantID, deviceID, row.UnsTreeID, row.CanonicalTopic,
			row.Level0, string(locJSON), row.DataContract, vpathArg, row.Name,
			string(mdJSON), lastEv, lastAt, now, now, now,
		)
		if err != nil {
			return fmt.Errorf("upsert device topic %s: %w", row.UnsTreeID, err)
		}
	}
	return tx.Commit()
}

// ClearDeviceTopics removes all materialized topics for a device (authoritative empty catalog).
func (r *DeviceTopicRepo) ClearDeviceTopics(ctx context.Context, tenantID, deviceID string) error {
	q := convertPlaceholders(`DELETE FROM device_topics WHERE tenant_id = ? AND device_id = ?`)
	_, err := r.data.db.ExecContext(ctx, q, tenantID, deviceID)
	if err != nil {
		return fmt.Errorf("clear device topics: %w", err)
	}
	return nil
}

// ReplaceAllDeviceTopics replaces the entire topic set for a device in one transaction
// (used when status indicates a full UNS map snapshot — see topicbrowser.MergeResult).
func (r *DeviceTopicRepo) ReplaceAllDeviceTopics(ctx context.Context, tenantID, deviceID string, rows []topicbrowser.UpsertRow) error {
	tx, err := r.data.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	del := convertPlaceholders(`DELETE FROM device_topics WHERE tenant_id = ? AND device_id = ?`)
	if _, err := tx.ExecContext(ctx, del, tenantID, deviceID); err != nil {
		return fmt.Errorf("replace device topics delete: %w", err)
	}

	now := time.Now()
	for _, row := range rows {
		id := deviceID + ":" + row.UnsTreeID
		locJSON, err := json.Marshal(row.LocationLevels)
		if err != nil {
			return err
		}
		mdJSON, err := json.Marshal(row.Metadata)
		if err != nil {
			return err
		}
		var lastEv interface{}
		if row.LastEvent != nil {
			b, err := protojson.Marshal(row.LastEvent)
			if err != nil {
				return err
			}
			lastEv = string(b)
		}
		var lastAt interface{}
		if row.LastEvent != nil && row.LastEvent.GetProducedAtMs() > 0 {
			lastAt = time.UnixMilli(int64(row.LastEvent.GetProducedAtMs()))
		}
		var vpathArg interface{}
		if row.VirtualPath != "" {
			vpathArg = row.VirtualPath
		}
		q := `
INSERT INTO device_topics (
  id, tenant_id, device_id, uns_tree_id, canonical_topic,
  level0, location_sublevels, data_contract, virtual_path, name,
  metadata_json, last_event_json, last_event_at, last_synced, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?::jsonb, ?, ?, ?, ?::jsonb, ?::jsonb, ?, ?, ?, ?)
`
		q = convertPlaceholders(q)
		if _, err := tx.ExecContext(ctx, q,
			id, tenantID, deviceID, row.UnsTreeID, row.CanonicalTopic,
			row.Level0, string(locJSON), row.DataContract, vpathArg, row.Name,
			string(mdJSON), lastEv, lastAt, now, now, now,
		); err != nil {
			return fmt.Errorf("replace device topic insert %s: %w", row.UnsTreeID, err)
		}
	}
	return tx.Commit()
}

// DeviceTopicRow is a DB row for API mapping.
type DeviceTopicRow struct {
	UnsTreeID      string
	CanonicalTopic string
	Level0         string
	LocationJSON   string
	DataContract   string
	VirtualPath    sql.NullString
	Name           string
	MetadataJSON   string
	LastEventJSON  sql.NullString
	LastEventAt    sql.NullTime
	UpdatedAt      time.Time
}

// ListDeviceTopics returns topics ordered by canonical_topic.
func (r *DeviceTopicRepo) ListDeviceTopics(ctx context.Context, tenantID string, q ListDeviceTopicsQuery) ([]DeviceTopicRow, error) {
	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = 20
	}
	args := []interface{}{tenantID, q.DeviceID}
	sb := strings.Builder{}
	sb.WriteString(`SELECT uns_tree_id, canonical_topic, level0, location_sublevels::text, data_contract, virtual_path, name, metadata_json::text, last_event_json::text, last_event_at, updated_at
FROM device_topics WHERE tenant_id = ? AND device_id = ?`)
	if q.Text != "" {
		sb.WriteString(` AND (canonical_topic ILIKE ? OR metadata_json::text ILIKE ?)`)
		pat := "%" + q.Text + "%"
		args = append(args, pat, pat)
	}
	for _, m := range q.Meta {
		if m.Key == "" {
			continue
		}
		sb.WriteString(` AND EXISTS (SELECT 1 FROM jsonb_each_text(metadata_json) AS e WHERE e.key = ? AND e.value = ?)`)
		args = append(args, m.Key, m.Eq)
	}
	sb.WriteString(` ORDER BY canonical_topic ASC OFFSET ? LIMIT ?`)
	args = append(args, q.Offset, q.Limit+1)

	query := convertPlaceholders(sb.String())
	rows, err := r.data.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeviceTopicRow
	for rows.Next() {
		var tr DeviceTopicRow
		if err := rows.Scan(&tr.UnsTreeID, &tr.CanonicalTopic, &tr.Level0, &tr.LocationJSON, &tr.DataContract, &tr.VirtualPath, &tr.Name, &tr.MetadataJSON, &tr.LastEventJSON, &tr.LastEventAt, &tr.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}

// GetDeviceTopic returns one topic by tree id or exact canonical path.
func (r *DeviceTopicRepo) GetDeviceTopic(ctx context.Context, tenantID, deviceID, unsTreeID, canonicalTopic string) (*DeviceTopicRow, error) {
	if unsTreeID == "" && canonicalTopic == "" {
		return nil, fmt.Errorf("uns_tree_id or canonical_topic required")
	}
	var q string
	args := []interface{}{tenantID, deviceID}
	if unsTreeID != "" {
		q = `SELECT uns_tree_id, canonical_topic, level0, location_sublevels::text, data_contract, virtual_path, name, metadata_json::text, last_event_json::text, last_event_at, updated_at
FROM device_topics WHERE tenant_id = ? AND device_id = ? AND uns_tree_id = ? LIMIT 1`
		args = append(args, unsTreeID)
	} else {
		q = `SELECT uns_tree_id, canonical_topic, level0, location_sublevels::text, data_contract, virtual_path, name, metadata_json::text, last_event_json::text, last_event_at, updated_at
FROM device_topics WHERE tenant_id = ? AND device_id = ? AND canonical_topic = ? LIMIT 1`
		args = append(args, canonicalTopic)
	}
	q = convertPlaceholders(q)
	row := r.data.db.QueryRowContext(ctx, q, args...)
	var tr DeviceTopicRow
	err := row.Scan(&tr.UnsTreeID, &tr.CanonicalTopic, &tr.Level0, &tr.LocationJSON, &tr.DataContract, &tr.VirtualPath, &tr.Name, &tr.MetadataJSON, &tr.LastEventJSON, &tr.LastEventAt, &tr.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tr, nil
}

// ParseStoredEvent unmarshals last_event_json into protobuf.
func ParseStoredEvent(lastJSON string) (*unsv1.EventTableEntry, error) {
	if lastJSON == "" {
		return nil, nil
	}
	var ev unsv1.EventTableEntry
	if err := protojson.Unmarshal([]byte(lastJSON), &ev); err != nil {
		return nil, err
	}
	return &ev, nil
}

package data

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/topicbrowser"
)

// DeviceTopicCatalogRow is sync metadata for one device.
type DeviceTopicCatalogRow struct {
	DeviceID               string
	LastSyncedAt           time.Time
	ReportedTopicCount     int32
	MaterializedTopicCount int32
	LastSyncMode           string
	LastFullReplaceAt      sql.NullTime
	LastHadBundleZero      bool
}

// UpsertDeviceTopicCatalog records the outcome of a topic sync.
func (r *DeviceTopicRepo) UpsertDeviceTopicCatalog(
	ctx context.Context,
	tenantID, deviceID string,
	reportedCount int,
	syncMode topicbrowser.CatalogSyncMode,
	hadBundleZero bool,
) error {
	matCount, err := r.CountDeviceTopics(ctx, tenantID, deviceID)
	if err != nil {
		return err
	}
	now := time.Now()
	var fullReplaceAt interface{}
	if syncMode == topicbrowser.CatalogSyncFullReplace || syncMode == topicbrowser.CatalogSyncEmpty {
		fullReplaceAt = now
	}
	q := `
INSERT INTO device_topic_catalog (
  tenant_id, device_id, last_synced_at, reported_topic_count, materialized_topic_count,
  last_sync_mode, last_full_replace_at, last_had_bundle_zero
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (tenant_id, device_id) DO UPDATE SET
  last_synced_at = EXCLUDED.last_synced_at,
  reported_topic_count = EXCLUDED.reported_topic_count,
  materialized_topic_count = EXCLUDED.materialized_topic_count,
  last_sync_mode = EXCLUDED.last_sync_mode,
  last_full_replace_at = COALESCE(EXCLUDED.last_full_replace_at, device_topic_catalog.last_full_replace_at),
  last_had_bundle_zero = EXCLUDED.last_had_bundle_zero
`
	q = convertPlaceholders(q)
	_, err = r.data.db.ExecContext(ctx, q,
		tenantID, deviceID, now, reportedCount, matCount, string(syncMode), fullReplaceAt, hadBundleZero,
	)
	if err != nil {
		return fmt.Errorf("upsert device topic catalog: %w", err)
	}
	return nil
}

// GetDeviceTopicCatalog returns catalog metadata for a device.
func (r *DeviceTopicRepo) GetDeviceTopicCatalog(ctx context.Context, tenantID, deviceID string) (*DeviceTopicCatalogRow, error) {
	q := convertPlaceholders(`
SELECT device_id, last_synced_at, reported_topic_count, materialized_topic_count,
       last_sync_mode, last_full_replace_at, last_had_bundle_zero
FROM device_topic_catalog WHERE tenant_id = ? AND device_id = ?`)
	row := r.data.db.QueryRowContext(ctx, q, tenantID, deviceID)
	var out DeviceTopicCatalogRow
	if err := row.Scan(
		&out.DeviceID, &out.LastSyncedAt, &out.ReportedTopicCount, &out.MaterializedTopicCount,
		&out.LastSyncMode, &out.LastFullReplaceAt, &out.LastHadBundleZero,
	); err == sql.ErrNoRows {
		mat, err := r.CountDeviceTopics(ctx, tenantID, deviceID)
		if err != nil {
			return nil, err
		}
		if mat == 0 {
			return nil, nil
		}
		return &DeviceTopicCatalogRow{
			DeviceID:               deviceID,
			MaterializedTopicCount: int32(mat),
			LastSyncMode:           string(topicbrowser.CatalogSyncIncremental),
		}, nil
	} else if err != nil {
		return nil, err
	}
	return &out, nil
}

// CountDeviceTopics returns total materialized topics for a device.
func (r *DeviceTopicRepo) CountDeviceTopics(ctx context.Context, tenantID, deviceID string) (int32, error) {
	q := convertPlaceholders(`SELECT COUNT(*) FROM device_topics WHERE tenant_id = ? AND device_id = ?`)
	var n int32
	if err := r.data.db.QueryRowContext(ctx, q, tenantID, deviceID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (r *DeviceTopicRepo) appendTopicFilters(sb *strings.Builder, args *[]interface{}, tenantID, deviceID, text, pathPrefix string, meta []TopicMetaEq) {
	sb.WriteString(` FROM device_topics WHERE tenant_id = ? AND device_id = ?`)
	*args = append(*args, tenantID, deviceID)
	if text != "" {
		sb.WriteString(` AND (canonical_topic ILIKE ? OR metadata_json::text ILIKE ?)`)
		pat := "%" + text + "%"
		*args = append(*args, pat, pat)
	}
	if pathPrefix != "" {
		p := pathPrefix
		if !strings.HasSuffix(p, ".") {
			p += "."
		}
		sb.WriteString(` AND (canonical_topic = ? OR canonical_topic LIKE ?)`)
		trimmed := strings.TrimSuffix(p, ".")
		*args = append(*args, trimmed, p+"%")
	}
	for _, m := range meta {
		if m.Key == "" {
			continue
		}
		sb.WriteString(` AND EXISTS (SELECT 1 FROM jsonb_each_text(metadata_json) AS e WHERE e.key = ? AND e.value = ?)`)
		*args = append(*args, m.Key, m.Eq)
	}
}

// CountDeviceTopicsFiltered counts rows matching list filters.
func (r *DeviceTopicRepo) CountDeviceTopicsFiltered(ctx context.Context, tenantID, deviceID, text, pathPrefix string, meta []TopicMetaEq) (int32, error) {
	var sb strings.Builder
	args := make([]interface{}, 0, 8)
	sb.WriteString(`SELECT COUNT(*)`)
	r.appendTopicFilters(&sb, &args, tenantID, deviceID, text, pathPrefix, meta)
	q := convertPlaceholders(sb.String())
	var n int32
	if err := r.data.db.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// TopicTreeNodeRow is one child segment in the UNS tree.
type TopicTreeNodeRow struct {
	Segment               string
	IsLeaf                bool
	DescendantLeafCount   int32
	UnsTreeID             string
	CanonicalTopic        string
}

// ListTopicNodes returns distinct child path segments under path_prefix.
func (r *DeviceTopicRepo) ListTopicNodes(ctx context.Context, tenantID, deviceID string, pathPrefix []string, text string, meta []TopicMetaEq) ([]TopicTreeNodeRow, error) {
	prefix := strings.Join(pathPrefix, ".")
	depth := len(pathPrefix) + 1

	var sb strings.Builder
	args := make([]interface{}, 0, 12)
	sb.WriteString(`
SELECT segment, COUNT(*)::int AS descendant_leaf_count,
       BOOL_OR(canonical_topic = child_path) AS is_leaf,
       MAX(CASE WHEN canonical_topic = child_path THEN uns_tree_id END) AS uns_tree_id,
       MAX(CASE WHEN canonical_topic = child_path THEN canonical_topic END) AS canonical_topic
FROM (
  SELECT canonical_topic, uns_tree_id,
         split_part(canonical_topic, '.', ?) AS segment,
         CASE WHEN ? = '' THEN split_part(canonical_topic, '.', ?)
              ELSE ? || '.' || split_part(canonical_topic, '.', ?) END AS child_path`)
	args = append(args, depth, prefix, depth, prefix, depth)
	r.appendTopicFilters(&sb, &args, tenantID, deviceID, text, prefix, meta)
	if prefix != "" {
		sb.WriteString(` AND canonical_topic <> ?`)
		args = append(args, prefix)
	}
	sb.WriteString(`
) AS tagged
WHERE segment <> ''
GROUP BY segment
ORDER BY segment`)
	q := convertPlaceholders(sb.String())
	rows, err := r.data.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TopicTreeNodeRow, 0, 16)
	for rows.Next() {
		var row TopicTreeNodeRow
		var unsTreeID, canonical sql.NullString
		if err := rows.Scan(&row.Segment, &row.DescendantLeafCount, &row.IsLeaf, &unsTreeID, &canonical); err != nil {
			return nil, err
		}
		if unsTreeID.Valid {
			row.UnsTreeID = unsTreeID.String
		}
		if canonical.Valid {
			row.CanonicalTopic = canonical.String
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Segment < out[j].Segment })
	return out, nil
}

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	unsv1 "github.com/artpark-hub/taksa-platform/device-management/api/uns/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/data"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/topicbrowser"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultTopicPageSize = 100
	maxTopicPageSize     = 500
)

// ListDeviceTopics lists materialized UNS topics with GraphQL-equivalent text and metadata filters.
func (s *DeviceMgmtService) ListDeviceTopics(ctx context.Context, req *v1.ListDeviceTopicsRequest) (*v1.ListDeviceTopicsResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}
	if s.deviceTopicRepo == nil {
		return emptyListTopicsResponse(), nil
	}
	if _, err := s.deviceUc.GetDevice(ctx, req.DeviceId); err != nil {
		return nil, status.Error(codes.NotFound, "device not found")
	}

	pageSize := normalizeTopicPageSize(req.PageSize)
	offset := int32(0)
	if req.PageToken != "" {
		o, err := decodePageToken(req.PageToken)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		if o < 0 {
			return nil, status.Error(codes.InvalidArgument, "page_token must be non-negative")
		}
		offset = o
	}
	meta := toTopicMetaEq(req.Meta)

	total, err := s.deviceTopicRepo.CountDeviceTopics(ctx, tenantID, req.DeviceId)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("count topics: %v", err))
	}
	filtered, err := s.deviceTopicRepo.CountDeviceTopicsFiltered(ctx, tenantID, req.DeviceId, req.Text, req.PathPrefix, meta)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("count filtered topics: %v", err))
	}

	rows, err := s.deviceTopicRepo.ListDeviceTopics(ctx, tenantID, data.ListDeviceTopicsQuery{
		DeviceID:      req.DeviceId,
		Text:          req.Text,
		PathPrefix:    req.PathPrefix,
		Meta:          meta,
		Offset:        int64(offset),
		Limit:         int64(pageSize),
		OmitLastEvent: req.OmitLastEvent,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("list topics: %v", err))
	}
	next := ""
	if len(rows) > int(pageSize) {
		rows = rows[:pageSize]
		next = encodePageToken(offset + pageSize)
	}
	out := make([]*v1.DeviceTopic, 0, len(rows))
	for i := range rows {
		dt, err := mapDeviceTopicRowToProto(&rows[i], req.OmitLastEvent)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		out = append(out, dt)
	}
	return &v1.ListDeviceTopicsResponse{
		Topics:         out,
		NextPageToken:  next,
		TotalCount:     total,
		FilteredCount:  filtered,
	}, nil
}

// GetDeviceTopic returns a single topic by uns_tree_id or exact canonical_topic.
func (s *DeviceMgmtService) GetDeviceTopic(ctx context.Context, req *v1.GetDeviceTopicRequest) (*v1.DeviceTopic, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	if req.UnsTreeId == "" && req.CanonicalTopic == "" {
		return nil, status.Error(codes.InvalidArgument, "uns_tree_id or canonical_topic is required")
	}
	if req.UnsTreeId != "" && req.CanonicalTopic != "" {
		return nil, status.Error(codes.InvalidArgument, "only one of uns_tree_id or canonical_topic may be set")
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}
	if s.deviceTopicRepo == nil {
		return nil, status.Error(codes.NotFound, "topic not found")
	}
	if _, err := s.deviceUc.GetDevice(ctx, req.DeviceId); err != nil {
		return nil, status.Error(codes.NotFound, "device not found")
	}
	row, err := s.deviceTopicRepo.GetDeviceTopic(ctx, tenantID, req.DeviceId, req.UnsTreeId, req.CanonicalTopic)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return nil, status.Error(codes.NotFound, "topic not found")
	}
	dt, err := mapDeviceTopicRowToProto(row, false)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return dt, nil
}

// GetDeviceTopicCatalogStatus returns materialized catalog sync metadata for a device.
func (s *DeviceMgmtService) GetDeviceTopicCatalogStatus(ctx context.Context, req *v1.GetDeviceTopicCatalogStatusRequest) (*v1.DeviceTopicCatalogStatus, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}
	if _, err := s.deviceUc.GetDevice(ctx, req.DeviceId); err != nil {
		return nil, status.Error(codes.NotFound, "device not found")
	}
	if s.deviceTopicRepo == nil {
		return &v1.DeviceTopicCatalogStatus{DeviceId: req.DeviceId}, nil
	}
	row, err := s.deviceTopicRepo.GetDeviceTopicCatalog(ctx, tenantID, req.DeviceId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if row == nil {
		return &v1.DeviceTopicCatalogStatus{DeviceId: req.DeviceId}, nil
	}
	out := &v1.DeviceTopicCatalogStatus{
		DeviceId:               req.DeviceId,
		ReportedTopicCount:     row.ReportedTopicCount,
		MaterializedTopicCount: row.MaterializedTopicCount,
		LastSyncMode:           mapCatalogSyncMode(row.LastSyncMode),
		LastHadBundleZero:      row.LastHadBundleZero,
	}
	if !row.LastSyncedAt.IsZero() {
		out.CatalogLastSyncedAt = timestamppb.New(row.LastSyncedAt)
	}
	if row.LastFullReplaceAt.Valid {
		out.LastFullReplaceAt = timestamppb.New(row.LastFullReplaceAt.Time)
	}
	if row.ReportedTopicCount >= 0 && row.MaterializedTopicCount > row.ReportedTopicCount {
		out.CatalogStaleWarning = true
	}
	return out, nil
}

// EnsureDeviceStatusSubscription queues edge subscribe for one device (explicit; ignores auto-disable).
func (s *DeviceMgmtService) EnsureDeviceStatusSubscription(ctx context.Context, req *v1.EnsureDeviceStatusSubscriptionRequest) (*v1.EnsureDeviceStatusSubscriptionResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}
	if s.instanceUc == nil {
		return nil, status.Error(codes.Unimplemented, "instance usecase not configured")
	}
	if _, err := s.deviceUc.GetDevice(ctx, req.DeviceId); err != nil {
		return nil, status.Error(codes.NotFound, "device not found")
	}
	result, err := s.instanceUc.QueueStatusSubscription(ctx, req.DeviceId, req.Resubscribed)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := &v1.EnsureDeviceStatusSubscriptionResponse{AlreadyPending: result.AlreadyPending}
	if result.Action != nil {
		out.ActionId = result.Action.Id
		out.CreatedAt = timeToProto(result.Action.CreatedAt)
		out.ExpiresAt = timeToProto(result.Action.ExpiresAt)
	}
	return out, nil
}

// ListTopicNodes returns child segments for lazy tree expansion.
func (s *DeviceMgmtService) ListTopicNodes(ctx context.Context, req *v1.ListTopicNodesRequest) (*v1.ListTopicNodesResponse, error) {
	if req == nil || req.DeviceId == "" {
		return nil, status.Error(codes.InvalidArgument, "device_id is required")
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return nil, status.Error(codes.PermissionDenied, "tenant_id not found in context")
	}
	if s.deviceTopicRepo == nil {
		return &v1.ListTopicNodesResponse{PathPrefix: strings.Join(req.PathPrefix, ".")}, nil
	}
	if _, err := s.deviceUc.GetDevice(ctx, req.DeviceId); err != nil {
		return nil, status.Error(codes.NotFound, "device not found")
	}
	meta := toTopicMetaEq(req.Meta)
	rows, err := s.deviceTopicRepo.ListTopicNodes(ctx, tenantID, req.DeviceId, req.PathPrefix, req.Text, meta)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("list topic nodes: %v", err))
	}
	nodes := make([]*v1.TopicTreeNode, 0, len(rows))
	for i := range rows {
		n := &v1.TopicTreeNode{
			Segment:               rows[i].Segment,
			IsLeaf:                rows[i].IsLeaf,
			DescendantLeafCount:     rows[i].DescendantLeafCount,
		}
		if rows[i].IsLeaf {
			n.UnsTreeId = rows[i].UnsTreeID
			n.CanonicalTopic = rows[i].CanonicalTopic
		}
		nodes = append(nodes, n)
	}
	return &v1.ListTopicNodesResponse{
		PathPrefix: strings.Join(req.PathPrefix, "."),
		Nodes:      nodes,
	}, nil
}

func emptyListTopicsResponse() *v1.ListDeviceTopicsResponse {
	return &v1.ListDeviceTopicsResponse{Topics: []*v1.DeviceTopic{}}
}

func normalizeTopicPageSize(pageSize int32) int32 {
	if pageSize <= 0 {
		return defaultTopicPageSize
	}
	if pageSize > maxTopicPageSize {
		return maxTopicPageSize
	}
	return pageSize
}

func toTopicMetaEq(in []*v1.TopicMetaEq) []data.TopicMetaEq {
	meta := make([]data.TopicMetaEq, 0, len(in))
	for _, m := range in {
		if m == nil || m.Key == "" {
			continue
		}
		meta = append(meta, data.TopicMetaEq{Key: m.Key, Eq: m.Eq})
	}
	return meta
}

func mapCatalogSyncMode(mode string) v1.TopicCatalogSyncMode {
	switch topicbrowser.CatalogSyncMode(mode) {
	case topicbrowser.CatalogSyncFullReplace:
		return v1.TopicCatalogSyncMode_FULL_REPLACE
	case topicbrowser.CatalogSyncEmpty:
		return v1.TopicCatalogSyncMode_EMPTY
	case topicbrowser.CatalogSyncIncremental:
		return v1.TopicCatalogSyncMode_INCREMENTAL
	default:
		return v1.TopicCatalogSyncMode_TOPIC_CATALOG_SYNC_MODE_UNSPECIFIED
	}
}

func mapDeviceTopicRowToProto(row *data.DeviceTopicRow, omitLastEvent bool) (*v1.DeviceTopic, error) {
	var loc []string
	if row.LocationJSON != "" {
		if err := json.Unmarshal([]byte(row.LocationJSON), &loc); err != nil {
			return nil, fmt.Errorf("location json: %w", err)
		}
	}
	mdMap := map[string]string{}
	if row.MetadataJSON != "" {
		if err := json.Unmarshal([]byte(row.MetadataJSON), &mdMap); err != nil {
			return nil, fmt.Errorf("metadata json: %w", err)
		}
	}
	mdKeys := make([]string, 0, len(mdMap))
	for k := range mdMap {
		mdKeys = append(mdKeys, k)
	}
	sort.Strings(mdKeys)
	md := make([]*v1.TopicMetadataEntry, 0, len(mdKeys))
	for _, k := range mdKeys {
		md = append(md, &v1.TopicMetadataEntry{Key: k, Value: mdMap[k]})
	}
	dt := &v1.DeviceTopic{
		Topic:             row.CanonicalTopic,
		UnsTreeId:         row.UnsTreeID,
		Level_0:           row.Level0,
		LocationSublevels: loc,
		DataContract:      row.DataContract,
		Name:              row.Name,
		Metadata:          md,
		UpdatedAt:         timestamppb.New(row.UpdatedAt),
	}
	if row.VirtualPath.Valid && row.VirtualPath.String != "" {
		dt.VirtualPath = &row.VirtualPath.String
	}
	if !omitLastEvent && row.LastEventJSON.Valid && row.LastEventJSON.String != "" {
		ev, err := data.ParseStoredEvent(row.LastEventJSON.String)
		if err != nil {
			return nil, err
		}
		if ev != nil {
			mapLastEventIntoDeviceTopic(dt, ev)
		}
	}
	return dt, nil
}

func mapLastEventIntoDeviceTopic(dt *v1.DeviceTopic, entry *unsv1.EventTableEntry) {
	switch entry.GetPayloadFormat() {
	case unsv1.PayloadFormat_RELATIONAL:
		dt.LastEvent = &v1.DeviceTopic_Relational{Relational: mapRelationalEvent(entry)}
	default:
		dt.LastEvent = &v1.DeviceTopic_TimeSeries{TimeSeries: mapTimeSeriesEvent(entry)}
	}
}

func mapTimeSeriesEvent(entry *unsv1.EventTableEntry) *v1.TopicTimeSeriesEvent {
	ts := time.UnixMilli(int64(entry.GetProducedAtMs()))
	out := &v1.TopicTimeSeriesEvent{
		ProducedAt:   timestamppb.New(ts),
		KafkaHeaders: mapKafkaHeaders(entry.GetRawKafkaMsg()),
		ScalarType:   v1.TopicScalarType_TOPIC_SCALAR_TYPE_UNSPECIFIED,
	}
	tsPayload := entry.GetTs()
	if tsPayload == nil {
		out.SourceTs = timestamppb.New(ts)
		out.ScalarType = v1.TopicScalarType_STRING
		empty := ""
		out.StringValue = &empty
		return out
	}
	if tsPayload.GetTimestampMs() > 0 {
		out.SourceTs = timestamppb.New(time.UnixMilli(tsPayload.GetTimestampMs()))
	} else {
		out.SourceTs = timestamppb.New(ts)
	}
	switch tsPayload.GetScalarType() {
	case unsv1.ScalarType_BOOLEAN:
		out.ScalarType = v1.TopicScalarType_BOOLEAN
		if b := tsPayload.GetBooleanValue(); b != nil {
			v := b.GetValue()
			out.BooleanValue = &v
		}
	case unsv1.ScalarType_STRING:
		out.ScalarType = v1.TopicScalarType_STRING
		if str := tsPayload.GetStringValue(); str != nil {
			v := str.GetValue()
			out.StringValue = &v
		}
	case unsv1.ScalarType_NUMERIC:
		out.ScalarType = v1.TopicScalarType_NUMERIC
		if n := tsPayload.GetNumericValue(); n != nil {
			v := n.GetValue()
			out.NumericValue = &v
		}
	default:
		out.ScalarType = v1.TopicScalarType_STRING
		empty := ""
		out.StringValue = &empty
	}
	return out
}

func mapRelationalEvent(entry *unsv1.EventTableEntry) *v1.TopicRelationalEvent {
	ts := time.UnixMilli(int64(entry.GetProducedAtMs()))
	rel := entry.GetRel()
	st := &structpb.Struct{}
	if rel != nil && len(rel.GetJson()) > 0 {
		var raw map[string]interface{}
		if err := json.Unmarshal(rel.GetJson(), &raw); err != nil {
			raw = map[string]interface{}{
				"_parseError": err.Error(),
				"_rawSize":    len(rel.GetJson()),
			}
		}
		var err error
		st, err = structpb.NewStruct(raw)
		if err != nil {
			st, _ = structpb.NewStruct(map[string]interface{}{"_structError": err.Error()})
		}
	}
	return &v1.TopicRelationalEvent{
		ProducedAt:   timestamppb.New(ts),
		Json:         st,
		KafkaHeaders: mapKafkaHeaders(entry.GetRawKafkaMsg()),
	}
}

func mapKafkaHeaders(k *unsv1.EventKafka) []*v1.TopicMetadataEntry {
	if k == nil || k.GetHeaders() == nil {
		return nil
	}
	keys := make([]string, 0, len(k.GetHeaders()))
	for key := range k.GetHeaders() {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]*v1.TopicMetadataEntry, 0, len(keys))
	for _, key := range keys {
		out = append(out, &v1.TopicMetadataEntry{Key: key, Value: k.GetHeaders()[key]})
	}
	return out
}

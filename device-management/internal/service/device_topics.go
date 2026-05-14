package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	unsv1 "github.com/artpark-hub/taksa-platform/device-management/api/uns/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/data"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
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
		return &v1.ListDeviceTopicsResponse{Topics: []*v1.DeviceTopic{}}, nil
	}
	if _, err := s.deviceUc.GetDevice(ctx, req.DeviceId); err != nil {
		return nil, status.Error(codes.NotFound, "device not found")
	}
	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = 100 // align with umh-core GraphQL default max batch
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := int32(0)
	if req.PageToken != "" {
		o, err := decodePageToken(req.PageToken)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid page_token")
		}
		offset = o
	}
	meta := make([]data.TopicMetaEq, 0, len(req.Meta))
	for _, m := range req.Meta {
		if m == nil || m.Key == "" {
			continue
		}
		meta = append(meta, data.TopicMetaEq{Key: m.Key, Eq: m.Eq})
	}
	rows, err := s.deviceTopicRepo.ListDeviceTopics(ctx, tenantID, data.ListDeviceTopicsQuery{
		DeviceID: req.DeviceId,
		Text:     req.Text,
		Meta:     meta,
		Offset:   int64(offset),
		Limit:    int64(pageSize),
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
		dt, err := mapDeviceTopicRowToProto(&rows[i])
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		out = append(out, dt)
	}
	return &v1.ListDeviceTopicsResponse{Topics: out, NextPageToken: next}, nil
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
	dt, err := mapDeviceTopicRowToProto(row)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return dt, nil
}

func mapDeviceTopicRowToProto(row *data.DeviceTopicRow) (*v1.DeviceTopic, error) {
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
	md := make([]*v1.TopicMetadataEntry, 0, len(mdMap))
	for k, v := range mdMap {
		md = append(md, &v1.TopicMetadataEntry{Key: k, Value: v})
	}
	dt := &v1.DeviceTopic{
		Topic:             row.CanonicalTopic,
		UnsTreeId:         row.UnsTreeID,
		Level_0:           row.Level0,
		LocationSublevels:  loc,
		DataContract:       row.DataContract,
		Name:               row.Name,
		Metadata:           md,
		UpdatedAt:          timestamppb.New(row.UpdatedAt),
	}
	if row.VirtualPath.Valid && row.VirtualPath.String != "" {
		dt.VirtualPath = &row.VirtualPath.String
	}
	if row.LastEventJSON.Valid && row.LastEventJSON.String != "" {
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
		ProducedAt:    timestamppb.New(ts),
		KafkaHeaders:  mapKafkaHeaders(entry.GetRawKafkaMsg()),
		ScalarType:    v1.TopicScalarType_TOPIC_SCALAR_TYPE_UNSPECIFIED,
		NumericValue:  nil,
		StringValue:   nil,
		BooleanValue:  nil,
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
		ProducedAt:    timestamppb.New(ts),
		Json:          st,
		KafkaHeaders:  mapKafkaHeaders(entry.GetRawKafkaMsg()),
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

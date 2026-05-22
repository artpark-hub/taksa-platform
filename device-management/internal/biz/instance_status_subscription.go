package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"google.golang.org/protobuf/types/known/anypb"
)

// StatusSubscriptionQueueResult is the outcome of queueing a subscribe action.
type StatusSubscriptionQueueResult struct {
	Action         *models.Action
	AlreadyPending bool
}

func (uc *InstanceUsecase) statusHeartbeatIsStale(ctx context.Context, deviceID string) bool {
	msgs, err := uc.store.Messages().GetRecentByDevice(ctx, deviceID, 10)
	if err != nil || len(msgs) == 0 {
		return true
	}
	return !recentStatusHeartbeatAt(msgs, uc.statusSub.StatusHeartbeatStaleThreshold)
}

func deviceLastSeenFresh(device *v1.Device, maxAge time.Duration) bool {
	if device == nil || device.LastSeen == nil {
		return false
	}
	return time.Since(device.LastSeen.AsTime()) <= maxAge
}

// catalogSyncIsStale is true when the device is pulling (fresh last_seen) but topic catalog is not updating.
func (uc *InstanceUsecase) catalogSyncIsStale(ctx context.Context, tenantID, deviceID string) bool {
	if uc.deviceTopicRepo == nil {
		return false
	}
	device, err := uc.store.Devices().GetByID(ctx, deviceID)
	if err != nil || device == nil {
		return false
	}
	if !deviceLastSeenFresh(device, uc.statusSub.CatalogStaleThreshold) {
		return false
	}
	row, err := uc.deviceTopicRepo.GetDeviceTopicCatalog(ctx, tenantID, deviceID)
	if err != nil {
		return true
	}
	if row == nil {
		return true
	}
	if row.LastSyncedAt.IsZero() {
		return true
	}
	return time.Since(row.LastSyncedAt) > uc.statusSub.CatalogStaleThreshold
}

func (uc *InstanceUsecase) needsStatusSubscriptionRefresh(ctx context.Context, tenantID, deviceID string) bool {
	if uc.statusHeartbeatIsStale(ctx, deviceID) {
		return true
	}
	return uc.catalogSyncIsStale(ctx, tenantID, deviceID)
}

// MaybeEnsureStatusSubscription queues subscribe when auto resubscribe is enabled and catalog or heartbeat is stale.
func (uc *InstanceUsecase) MaybeEnsureStatusSubscription(ctx context.Context, deviceID string) {
	if deviceID == "" || !uc.statusSub.AutoResubscribeOnPull {
		return
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return
	}
	if !uc.needsStatusSubscriptionRefresh(ctx, tenantID, deviceID) {
		return
	}
	_, _ = uc.QueueStatusSubscription(ctx, deviceID, true)
}

// QueueStatusSubscription queues a subscribe action for the edge (always, for explicit API and login).
func (uc *InstanceUsecase) QueueStatusSubscription(ctx context.Context, deviceID string, resubscribed bool) (StatusSubscriptionQueueResult, error) {
	var out StatusSubscriptionQueueResult
	if deviceID == "" {
		return out, nil
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return out, nil
	}

	payloadJSON, err := jsonMarshalSubscribe(resubscribed)
	if err != nil {
		return out, err
	}

	now := time.Now()
	action := &models.Action{
		Id:         generateUUID(),
		DeviceId:   deviceID,
		Type:       subscribeActionType,
		Payload:    &anypb.Any{TypeUrl: subscribeActionPayloadTypeURL, Value: payloadJSON},
		MaxRetries: 0,
		RetryCount: 0,
		Status:     models.ActionStatusQueued,
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Duration(subscribeActionDefaultTTLSeconds) * time.Second),
	}
	if err := uc.store.Actions().Save(ctx, tenantID, action); err != nil {
		if err.Error() == "record already exists" {
			out.AlreadyPending = true
			return out, nil
		}
		fmt.Printf("Warning: failed to queue subscribe action for device %s: %v\n", deviceID, err)
		return out, err
	}
	out.Action = action
	return out, nil
}

func jsonMarshalSubscribe(resubscribed bool) ([]byte, error) {
	return json.Marshal(map[string]any{"resubscribed": resubscribed})
}

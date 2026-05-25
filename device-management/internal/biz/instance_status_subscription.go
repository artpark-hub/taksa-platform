package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/middleware"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
	"github.com/artpark-hub/taksa-platform/device-management/internal/storage/postgres"
	"github.com/artpark-hub/taksa-platform/device-management/internal/topicbrowser"
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

// catalogNeedsBootstrap is true when DM has no materialized topics or last sync was authoritative empty.
func (uc *InstanceUsecase) catalogNeedsBootstrap(ctx context.Context, tenantID, deviceID string) bool {
	if uc.deviceTopicRepo == nil {
		return false
	}
	row, err := uc.deviceTopicRepo.GetDeviceTopicCatalog(ctx, tenantID, deviceID)
	if err != nil || row == nil {
		return true
	}
	if row.MaterializedTopicCount == 0 || row.LastSyncMode == string(topicbrowser.CatalogSyncEmpty) {
		return true
	}
	return false
}

// resolveSubscribeResubscribed chooses subscribe payload resubscribed flag.
// After EMPTY / zero materialized count, edge must send bootstrap bundle 0 (resubscribed=false).
func (uc *InstanceUsecase) resolveSubscribeResubscribed(ctx context.Context, tenantID, deviceID string, requested *bool) bool {
	if requested != nil {
		return *requested
	}
	if uc.catalogNeedsBootstrap(ctx, tenantID, deviceID) {
		return false
	}
	return true
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
	since := time.Now().Add(-uc.statusSub.SubscribeQueueCooldown)
	if recent, err := uc.store.Actions().HasRecentSubscribeForDevice(ctx, tenantID, deviceID, since); err == nil && recent {
		return
	}
	_, _ = uc.QueueStatusSubscription(ctx, deviceID, nil)
}

// QueueStatusSubscription queues a subscribe action for the edge (explicit API, login, or auto resubscribe).
// requestedResubscribed nil => resolve from catalog state (bootstrap when empty).
func (uc *InstanceUsecase) QueueStatusSubscription(ctx context.Context, deviceID string, requestedResubscribed *bool) (StatusSubscriptionQueueResult, error) {
	var out StatusSubscriptionQueueResult
	if deviceID == "" {
		return out, nil
	}
	tenantID := middleware.GetTenantID(ctx)
	if tenantID == "" {
		return out, nil
	}

	resubscribed := uc.resolveSubscribeResubscribed(ctx, tenantID, deviceID, requestedResubscribed)
	needsBootstrap := uc.catalogNeedsBootstrap(ctx, tenantID, deviceID)
	if needsBootstrap {
		// Replace any stale QUEUED subscribe (e.g. resubscribed:true) so pull delivers bootstrap payload.
		_, _ = uc.store.Actions().DeleteQueuedSubscribe(ctx, tenantID, deviceID)
	}
	action, err := uc.saveSubscribeAction(ctx, tenantID, deviceID, resubscribed)
	if errors.Is(err, postgres.ErrAlreadyExists) {
		if needsBootstrap {
			_, _ = uc.store.Actions().DeleteQueuedSubscribe(ctx, tenantID, deviceID)
			action, err = uc.saveSubscribeAction(ctx, tenantID, deviceID, resubscribed)
		}
		if errors.Is(err, postgres.ErrAlreadyExists) {
			out.AlreadyPending = true
			return out, nil
		}
	}
	if err != nil {
		fmt.Printf("Warning: failed to queue subscribe action for device %s: %v\n", deviceID, err)
		return out, err
	}
	out.Action = action
	return out, nil
}

func (uc *InstanceUsecase) saveSubscribeAction(ctx context.Context, tenantID, deviceID string, resubscribed bool) (*models.Action, error) {
	payloadJSON, err := jsonMarshalSubscribe(resubscribed)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	return action, nil
}

func jsonMarshalSubscribe(resubscribed bool) ([]byte, error) {
	return json.Marshal(map[string]any{"resubscribed": resubscribed})
}

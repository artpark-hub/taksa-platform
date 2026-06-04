package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
)

const (
	defaultActionRetentionMinutes       = 60
	defaultActionCleanupIntervalMinutes = 10
	autoExpireQueuedMessage             = "Queued action auto-expired (device did not pull in time)"
)

// ActionCleanupSettings is resolved DM config for action retention and background cleanup.
type ActionCleanupSettings struct {
	RetentionMinutes       int
	CleanupIntervalMinutes int
	AutoExpireMinutes      int // 0 = disabled
}

// ResolveActionCleanupSettings applies defaults from config.yaml (env overrides applied before call).
func ResolveActionCleanupSettings(cfg *conf.ActionCleanup) ActionCleanupSettings {
	out := ActionCleanupSettings{
		RetentionMinutes:       defaultActionRetentionMinutes,
		CleanupIntervalMinutes: defaultActionCleanupIntervalMinutes,
	}
	if cfg == nil {
		return out
	}
	out.RetentionMinutes = int(cfg.RetentionMinutes)
	out.CleanupIntervalMinutes = int(cfg.CleanupIntervalMinutes)
	out.AutoExpireMinutes = int(cfg.AutoExpireMinutes)
	return out
}

// StartActionCleanupLoop starts a background loop that deletes terminal actions and old messages.
func (uc *InstanceUsecase) StartActionCleanupLoop(ctx context.Context) {
	if uc == nil || uc.store == nil || ctx == nil {
		return
	}

	retentionMinutes := uc.actionCleanup.RetentionMinutes
	intervalMinutes := uc.actionCleanup.CleanupIntervalMinutes
	autoExpireMinutes := uc.actionCleanup.AutoExpireMinutes

	if retentionMinutes <= 0 {
		fmt.Printf("WARNING: action_cleanup.retention_minutes=%d is invalid; clamping to default %d\n", retentionMinutes, defaultActionRetentionMinutes)
		retentionMinutes = defaultActionRetentionMinutes
	}
	if intervalMinutes <= 0 {
		fmt.Printf("WARNING: action_cleanup.cleanup_interval_minutes=%d disables cleanup loop\n", intervalMinutes)
		return
	}

	retention := time.Duration(retentionMinutes) * time.Minute
	interval := time.Duration(intervalMinutes) * time.Minute

	ctx, cancel := context.WithCancel(ctx)
	uc.actionCleanupCancel = cancel
	uc.actionCleanupWG.Add(1)
	go func() {
		defer uc.actionCleanupWG.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				before := time.Now().Add(-retention)
				cleanupCtx := context.Background()

				if err := uc.store.Actions().ExpireQueuedPastDeadline(cleanupCtx); err != nil {
					fmt.Printf("WARNING: per-action TTL expiry failed: %v\n", err)
				}

				if autoExpireMinutes > 0 {
					expireBefore := time.Now().Add(-time.Duration(autoExpireMinutes) * time.Minute)
					if err := uc.store.Actions().ExpireQueuedOlderThan(cleanupCtx, expireBefore, autoExpireQueuedMessage); err != nil {
						fmt.Printf("WARNING: queued action auto-expire failed: %v\n", err)
					}
				}

				if err := uc.store.Actions().CleanupTerminal(cleanupCtx, before); err != nil {
					fmt.Printf("WARNING: action cleanup failed: %v\n", err)
				}
				if err := uc.store.Messages().CleanupOld(cleanupCtx, before); err != nil {
					fmt.Printf("WARNING: message cleanup failed: %v\n", err)
				}
			}
		}
	}()
}

// StopActionCleanupLoop cancels the background cleanup loop (call from app shutdown).
func (uc *InstanceUsecase) StopActionCleanupLoop() {
	if uc == nil || uc.actionCleanupCancel == nil {
		return
	}
	uc.actionCleanupCancel()
	uc.actionCleanupWG.Wait()
	uc.actionCleanupCancel = nil
}

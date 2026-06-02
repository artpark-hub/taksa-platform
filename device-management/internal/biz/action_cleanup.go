package biz

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultActionRetentionMinutes       = 60
	defaultActionCleanupIntervalMinutes = 10
	autoExpireQueuedMessage             = "Queued action auto-expired (device did not pull in time)"
)

const (
	actionRetentionEnvVar      = "TAKSA_DM_ACTION_RETENTION_MINUTES"
	actionCleanupIntervalEnvVar = "TAKSA_DM_ACTION_CLEANUP_INTERVAL_MINUTES"
	actionAutoExpireEnvVar     = "TAKSA_DM_ACTION_AUTO_EXPIRE_MINUTES"
)

func envInt(name string, defaultValue int) int {
	v := os.Getenv(name)
	if v == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue
	}
	return n
}

func envIntWarn(name string, defaultValue int) (int, bool) {
	v := os.Getenv(name)
	if v == "" {
		return defaultValue, false
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fmt.Printf("WARNING: %s=%q is not an integer; using default %d\n", name, v, defaultValue)
		return defaultValue, true
	}
	return n, true
}

// envIntPositive returns (value, true) when name is set to a positive integer; otherwise (0, false).
func envIntPositive(name string) (int, bool) {
	v := os.Getenv(name)
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// StartActionCleanupLoop starts a background loop that deletes terminal actions and old messages.
//
// Configuration:
// - TAKSA_DM_ACTION_RETENTION_MINUTES: how long terminal actions/messages are retained (default: 60)
// - TAKSA_DM_ACTION_CLEANUP_INTERVAL_MINUTES: how often cleanup runs (default: 10). <= 0 disables loop.
// - TAKSA_DM_ACTION_AUTO_EXPIRE_MINUTES: optional; when set, QUEUED UI/async actions older than this are marked EXPIRED
//   (excludes subscribe and UNS→NATS mirror deploy/edit — see models.Action.ExcludedFromAutoExpire)
//
// Retention uses message.created_at and a derived action "terminal timestamp"
// (COALESCE(completed_at, delivered_at, created_at)).
func (uc *InstanceUsecase) StartActionCleanupLoop() {
	if uc == nil || uc.store == nil {
		return
	}

	retentionMinutes, _ := envIntWarn(actionRetentionEnvVar, defaultActionRetentionMinutes)
	intervalMinutes, _ := envIntWarn(actionCleanupIntervalEnvVar, defaultActionCleanupIntervalMinutes)

	if retentionMinutes <= 0 {
		fmt.Printf("WARNING: %s=%d is invalid; clamping to default %d\n", actionRetentionEnvVar, retentionMinutes, defaultActionRetentionMinutes)
		retentionMinutes = defaultActionRetentionMinutes
	}
	if intervalMinutes <= 0 {
		fmt.Printf("WARNING: %s=%d disables cleanup loop\n", actionCleanupIntervalEnvVar, intervalMinutes)
		return
	}

	retention := time.Duration(retentionMinutes) * time.Minute
	interval := time.Duration(intervalMinutes) * time.Minute

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			before := time.Now().Add(-retention)

			// Cleanup is cross-tenant by design. The SQL store methods will not apply tenant scoping
			// when tenant_id is missing from context.
			ctx := context.Background()

			if err := uc.store.Actions().ExpireQueuedPastDeadline(ctx); err != nil {
				fmt.Printf("WARNING: per-action TTL expiry failed: %v\n", err)
			}

			if autoExpireMinutes, ok := envIntPositive(actionAutoExpireEnvVar); ok {
				expireBefore := time.Now().Add(-time.Duration(autoExpireMinutes) * time.Minute)
				if err := uc.store.Actions().ExpireQueuedOlderThan(ctx, expireBefore, autoExpireQueuedMessage); err != nil {
					fmt.Printf("WARNING: queued action auto-expire failed: %v\n", err)
				}
			}

			if err := uc.store.Actions().CleanupTerminal(ctx, before); err != nil {
				fmt.Printf("WARNING: action cleanup failed: %v\n", err)
			}
			if err := uc.store.Messages().CleanupOld(ctx, before); err != nil {
				fmt.Printf("WARNING: message cleanup failed: %v\n", err)
			}
		}
	}()
}


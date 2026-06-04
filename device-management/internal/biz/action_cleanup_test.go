package biz

import (
	"testing"

	"github.com/artpark-hub/taksa-platform/device-management/internal/conf"
	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

func TestResolveActionCleanupSettings(t *testing.T) {
	def := ResolveActionCleanupSettings(nil)
	if def.RetentionMinutes != 60 || def.CleanupIntervalMinutes != 10 || def.AutoExpireMinutes != 0 {
		t.Fatalf("unexpected defaults: %+v", def)
	}

	cfg := &conf.ActionCleanup{
		RetentionMinutes:       2,
		CleanupIntervalMinutes: 1,
		AutoExpireMinutes:      30,
	}
	got := ResolveActionCleanupSettings(cfg)
	if got.RetentionMinutes != 2 || got.CleanupIntervalMinutes != 1 || got.AutoExpireMinutes != 30 {
		t.Fatalf("unexpected resolved: %+v", got)
	}

	off := ResolveActionCleanupSettings(&conf.ActionCleanup{AutoExpireMinutes: 0})
	if off.AutoExpireMinutes != 0 {
		t.Fatal("expected auto-expire disabled when zero")
	}
}

func TestIsActionTerminalForPushIgnore(t *testing.T) {
	if !isActionTerminalForPushIgnore(models.ActionStatusCancelled) {
		t.Fatal("CANCELLED should be ignored on push")
	}
	if !isActionTerminalForPushIgnore(models.ActionStatusExpired) {
		t.Fatal("EXPIRED should be ignored on push")
	}
	if isActionTerminalForPushIgnore(models.ActionStatusDelivered) {
		t.Fatal("DELIVERED should not be ignored on push")
	}
}

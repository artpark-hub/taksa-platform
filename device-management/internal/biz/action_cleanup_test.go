package biz

import (
	"testing"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

func TestEnvIntPositive(t *testing.T) {
	t.Setenv("DM_ACTION_AUTO_EXPIRE_MINUTES", "")
	if _, ok := envIntPositive("DM_ACTION_AUTO_EXPIRE_MINUTES"); ok {
		t.Fatal("expected unset env to be disabled")
	}

	t.Setenv("DM_ACTION_AUTO_EXPIRE_MINUTES", "30")
	v, ok := envIntPositive("DM_ACTION_AUTO_EXPIRE_MINUTES")
	if !ok || v != 30 {
		t.Fatalf("expected (30, true), got (%d, %v)", v, ok)
	}

	t.Setenv("DM_ACTION_AUTO_EXPIRE_MINUTES", "0")
	if _, ok := envIntPositive("DM_ACTION_AUTO_EXPIRE_MINUTES"); ok {
		t.Fatal("expected zero to be disabled")
	}

	t.Setenv("DM_ACTION_AUTO_EXPIRE_MINUTES", "bad")
	if _, ok := envIntPositive("DM_ACTION_AUTO_EXPIRE_MINUTES"); ok {
		t.Fatal("expected invalid value to be disabled")
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

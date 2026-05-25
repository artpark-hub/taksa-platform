package biz

import (
	"testing"
	"time"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

func TestRecentStatusHeartbeatAt(t *testing.T) {
	now := time.Now()
	threshold := 2 * time.Minute

	msgs := []*models.Message{
		{Type: "action-reply", CreatedAt: now},
		{Type: "status", CreatedAt: now.Add(-3 * time.Minute)},
	}
	if recentStatusHeartbeatAt(msgs, threshold) {
		t.Fatal("expected no recent status heartbeat")
	}

	msgs = []*models.Message{
		{Type: "status", CreatedAt: now.Add(-30 * time.Second)},
	}
	if !recentStatusHeartbeatAt(msgs, threshold) {
		t.Fatal("expected recent status heartbeat")
	}
}

func TestIsStatusMessageType(t *testing.T) {
	for _, typ := range []string{"status", "status-message", "StatusMessage"} {
		if !isStatusMessageType(typ) {
			t.Fatalf("expected %q to be a status type", typ)
		}
	}
	if isStatusMessageType("action-reply") {
		t.Fatal("action-reply should not be a status type")
	}
}

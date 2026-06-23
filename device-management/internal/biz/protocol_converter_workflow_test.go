package biz

import (
	"testing"

	"github.com/artpark-hub/taksa-platform/device-management/internal/models"
)

func TestIsWorkflowTerminal(t *testing.T) {
	terminal := []models.ActionStatus{
		models.ActionStatusCompleted,
		models.ActionStatusFailed,
		models.ActionStatusCancelled,
		models.ActionStatusExpired,
	}
	for _, s := range terminal {
		if !isWorkflowTerminal(s) {
			t.Fatalf("expected terminal: %v", s)
		}
	}
	nonTerminal := []models.ActionStatus{
		models.ActionStatusQueued,
		models.ActionStatusProcessing,
		models.ActionStatusDelivered,
	}
	for _, s := range nonTerminal {
		if isWorkflowTerminal(s) {
			t.Fatalf("expected non-terminal: %v", s)
		}
	}
}

package models

import (
	"testing"

	"google.golang.org/protobuf/types/known/anypb"
)

func TestActionExcludedFromAutoExpire(t *testing.T) {
	tests := []struct {
		name     string
		action   *Action
		excluded bool
	}{
		{
			name:     "get-config is not excluded",
			action:   &Action{Type: "get-config-file"},
			excluded: false,
		},
		{
			name:     "subscribe is excluded",
			action:   &Action{Type: ActionTypeSubscribe},
			excluded: true,
		},
		{
			name: "nats mirror deploy is excluded",
			action: &Action{
				Type:    "deploy-data-flow-component",
				Payload: &anypb.Any{Value: []byte(`{"name":"UNS-to-NATS-mirror"}`)},
			},
			excluded: true,
		},
		{
			name: "other deploy is not excluded",
			action: &Action{
				Type:    "deploy-data-flow-component",
				Payload: &anypb.Any{Value: []byte(`{"name":"some-other-dfc"}`)},
			},
			excluded: false,
		},
		{
			name: "deploy with mirror string only inside nested data is not excluded",
			action: &Action{
				Type:    "deploy-data-flow-component",
				Payload: &anypb.Any{Value: []byte(`{"name":"my-bridge","payload":{"customDataFlowComponent":{"outputs":{"data":"UNS-to-NATS-mirror"}}}}`)},
			},
			excluded: false,
		},
		{
			name: "nats mirror edit is excluded",
			action: &Action{
				Type:    "edit-data-flow-component",
				Payload: &anypb.Any{Value: []byte(`{"name":"UNS-to-NATS-mirror"}`)},
			},
			excluded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.action.ExcludedFromAutoExpire(); got != tt.excluded {
				t.Fatalf("ExcludedFromAutoExpire() = %v, want %v", got, tt.excluded)
			}
		})
	}
}

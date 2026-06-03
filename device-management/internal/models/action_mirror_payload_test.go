package models

import "testing"

func TestIsNATSMirrorDeployOrEditPayload(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{
			name:    "deploy mirror",
			payload: []byte(`{"name":"UNS-to-NATS-mirror","meta":{"type":"custom"}}`),
			want:    true,
		},
		{
			name:    "edit mirror",
			payload: []byte(`{"uuid":"x","name":"UNS-to-NATS-mirror","state":"active"}`),
			want:    true,
		},
		{
			name:    "wrong top-level name",
			payload: []byte(`{"name":"generic-opcua-bridge-1"}`),
			want:    false,
		},
		{
			name:    "marker only in nested yaml",
			payload: []byte(`{"name":"other","payload":{"customDataFlowComponent":{"outputs":{"data":"UNS-to-NATS-mirror"}}}}`),
			want:    false,
		},
		{
			name:    "invalid json",
			payload: []byte(`not json`),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNATSMirrorDeployOrEditPayload(tt.payload); got != tt.want {
				t.Fatalf("IsNATSMirrorDeployOrEditPayload() = %v, want %v", got, tt.want)
			}
		})
	}
}

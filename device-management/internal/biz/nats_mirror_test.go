package biz

import (
	"testing"
)

func TestBuildNATSMirrorDeployActionPayload_shape(t *testing.T) {
	m, err := buildNATSMirrorDeployActionPayload("tenant-a", "dev-1", []string{"nats://127.0.0.1:4222"})
	if err != nil {
		t.Fatal(err)
	}
	if m["name"] != natsMirrorDataFlowName {
		t.Fatalf("name: got %v", m["name"])
	}
	meta, ok := m["meta"].(map[string]interface{})
	if !ok || meta["type"] != "custom" {
		t.Fatalf("meta: %#v", m["meta"])
	}
	pl, ok := m["payload"].(map[string]interface{})
	if !ok {
		t.Fatal("missing payload")
	}
	cdfc, ok := pl["customDataFlowComponent"].(map[string]interface{})
	if !ok {
		t.Fatal("missing customDataFlowComponent")
	}
	for _, k := range []string{"inputs", "outputs", "pipeline", "rawYAML"} {
		if _, ok := cdfc[k]; !ok {
			t.Fatalf("missing cdfc key %s", k)
		}
	}
}

func TestBuildNATSMirrorEditActionPayload_hasUUID(t *testing.T) {
	m, err := buildNATSMirrorEditActionPayload("tenant-a", "dev-1", []string{"nats://127.0.0.1:4222"})
	if err != nil {
		t.Fatal(err)
	}
	if m["uuid"] != natsMirrorComponentUUID() {
		t.Fatalf("uuid: got %v want %s", m["uuid"], natsMirrorComponentUUID())
	}
	if m["name"] != natsMirrorDataFlowName {
		t.Fatalf("name: got %v", m["name"])
	}
}

func TestIsNATSMirrorEditNotFoundError(t *testing.T) {
	msg := `edit(UNS-to-NATS-mirror): failed to edit dataflow component: dataflow component with UUID 95d80fc0-d68b-5f99-865e-4de41b9ad51e not found`
	if !isNATSMirrorEditNotFoundError(msg) {
		t.Fatal("expected not found")
	}
	if isNATSMirrorEditNotFoundError("connection refused") {
		t.Fatal("expected false for unrelated error")
	}
}

func TestNATSMirrorConfigFingerprint_orderIndependent(t *testing.T) {
	a := NATSMirrorConfigFingerprint([]string{"nats://b:4222", "nats://a:4222"})
	b := NATSMirrorConfigFingerprint([]string{"nats://a:4222", "nats://b:4222"})
	if a == "" || a != b {
		t.Fatalf("fingerprints differ: %q vs %q", a, b)
	}
	c := NATSMirrorConfigFingerprint([]string{"nats://a:4222"})
	if a == c {
		t.Fatal("expected different fingerprint for different URL set")
	}
}

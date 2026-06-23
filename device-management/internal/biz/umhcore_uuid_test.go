package biz

import "testing"

func TestGenerateUUIDFromName_deterministic(t *testing.T) {
	a := GenerateUUIDFromName("line-1-opcua-bridge")
	b := GenerateUUIDFromName("line-1-opcua-bridge")
	if a != b || a == "" {
		t.Fatalf("uuid: %q %q", a, b)
	}
}

package protocolconverter

import "testing"

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"protocol converter with UUID x not found", true},
		{"Failed to delete protocol converter: protocol converter with UUID 4261275c-a06e-5eea-b173-a7fb83b103dd not found", true},
		{"connection refused", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsNotFoundError(tt.msg); got != tt.want {
			t.Errorf("IsNotFoundError(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

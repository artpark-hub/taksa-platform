package opcua

import (
	"testing"

	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
)

func TestIsMinimalProtocolConverterReply(t *testing.T) {
	minimal := &v2.ProtocolConverter{Uuid: "4261275c-a06e-5eea-b173-a7fb83b103dd"}
	if !IsMinimalProtocolConverterReply(minimal) {
		t.Fatal("expected minimal edit reply")
	}
	if IsOpcUaProtocolConverter(minimal) {
		t.Fatal("minimal reply is not opc-ua shaped")
	}

	full := &v2.ProtocolConverter{
		Uuid: "4261275c-a06e-5eea-b173-a7fb83b103dd",
		ReadDFC: &v2.ProtocolConverterDFC{
			Inputs: &v2.CommonDataFlowComponentInputConfig{Type: "opcua"},
		},
	}
	if IsMinimalProtocolConverterReply(full) {
		t.Fatal("full reply is not minimal")
	}
	if !IsOpcUaProtocolConverter(full) {
		t.Fatal("expected opc-ua kind")
	}
}

package opcua

import (
	"strings"

	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
)

// ProtocolMetaValue is stored in ProtocolConverter.meta.protocol.
const ProtocolMetaValue = "opcua"

// InputType is readDFC.inputs.type for OPC-UA bridges.
const InputType = "opcua"

// IsOpcUaProtocolConverter reports whether the umh-core payload is an OPC-UA bridge.
func IsOpcUaProtocolConverter(pc *v2.ProtocolConverter) bool {
	if pc == nil {
		return false
	}
	if pc.Meta != nil {
		p := strings.ToLower(strings.TrimSpace(pc.Meta.Protocol))
		if p == ProtocolMetaValue || p == "opc-ua" {
			return true
		}
	}
	if pc.ReadDFC != nil && pc.ReadDFC.Inputs != nil {
		t := strings.ToLower(strings.TrimSpace(pc.ReadDFC.Inputs.Type))
		if t == InputType {
			return true
		}
	}
	return false
}

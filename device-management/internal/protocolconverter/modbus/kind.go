package modbus

import (
	"strings"

	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
)

// ProtocolMetaValue is stored in ProtocolConverter.meta.protocol (matches umh-core / benthos "modbus").
const ProtocolMetaValue = "modbus"

// InputType is readDFC.inputs.type for Modbus bridges.
const InputType = "modbus"

// IsMinimalProtocolConverterReply reports umh-core edit success payloads with uuid only.
func IsMinimalProtocolConverterReply(pc *v2.ProtocolConverter) bool {
	if pc == nil || pc.GetUuid() == "" {
		return false
	}
	if pc.GetReadDFC() != nil {
		return false
	}
	if pc.Meta != nil && strings.TrimSpace(pc.Meta.GetProtocol()) != "" {
		return false
	}
	return true
}

// IsModbusProtocolConverter reports whether the umh-core payload is a Modbus bridge.
func IsModbusProtocolConverter(pc *v2.ProtocolConverter) bool {
	if pc == nil {
		return false
	}
	if pc.Meta != nil {
		p := strings.ToLower(strings.TrimSpace(pc.Meta.GetProtocol()))
		if p == ProtocolMetaValue || p == "modbus" || p == "modbus-tcp" {
			return true
		}
	}
	if pc.ReadDFC != nil && pc.ReadDFC.Inputs != nil {
		t := strings.ToLower(strings.TrimSpace(pc.ReadDFC.Inputs.Type))
		if t == InputType {
			return true
		}
		if strings.Contains(strings.ToLower(pc.ReadDFC.Inputs.GetData()), "modbus:") {
			return true
		}
	}
	return false
}

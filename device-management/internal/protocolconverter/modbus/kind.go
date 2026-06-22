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
		if isModbusWireLabel(pc.Meta.GetProtocol()) {
			return true
		}
	}
	if pc.ReadDFC != nil && pc.ReadDFC.Inputs != nil {
		in := pc.ReadDFC.Inputs
		if isModbusWireLabel(in.GetType()) {
			return true
		}
		if isModbusInputYAML(in.GetData()) {
			return true
		}
	}
	return false
}

func isModbusWireLabel(s string) bool {
	n := strings.ToLower(strings.TrimSpace(s))
	if n == "" {
		return false
	}
	return n == ProtocolMetaValue || n == "modbus_tcp" || n == "modbus-tcp" || strings.Contains(n, "modbus")
}

func isModbusInputYAML(data string) bool {
	d := strings.ToLower(data)
	return strings.Contains(d, "modbus:") || strings.Contains(d, "\nmodbus:")
}

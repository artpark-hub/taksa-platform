package protocolconverter

import "strings"

// IsGenericCatalogType reports whether the catalog type is the DFC kind, not a wire protocol.
// Heartbeat sync only knows dfcType "protocol-converter"; deploy/get responses carry opcua/modbus.
func IsGenericCatalogType(t string) bool {
	n := strings.ToLower(strings.TrimSpace(t))
	return n == "" || n == "protocol-converter"
}

// IsOpcUaCatalogType reports whether the catalog already records an OPC-UA wire protocol.
func IsOpcUaCatalogType(t string) bool {
	n := strings.ToLower(strings.TrimSpace(t))
	return n == "opcua" || n == "opc-ua" || strings.Contains(n, "opcua")
}

// IsKnownNonOpcUaCatalogType reports a catalog type that is definitely not OPC-UA.
func IsKnownNonOpcUaCatalogType(t string) bool {
	if IsGenericCatalogType(t) || IsOpcUaCatalogType(t) {
		return false
	}
	n := strings.ToLower(strings.TrimSpace(t))
	return strings.Contains(n, "modbus") || strings.Contains(n, "sparkplug")
}

// WireTypeFromMap extracts readDFC.inputs.type or meta.protocol from an umh-core payload map.
func WireTypeFromMap(pc map[string]interface{}) string {
	if pc == nil {
		return ""
	}
	if meta, ok := pc["meta"].(map[string]interface{}); ok {
		if protocol, ok := meta["protocol"].(string); ok && protocol != "" {
			return protocol
		}
	}
	for _, key := range []string{"readDFC", "read_dfc", "ReadDFC"} {
		readDFC, ok := pc[key].(map[string]interface{})
		if !ok {
			continue
		}
		inputs, ok := readDFC["inputs"].(map[string]interface{})
		if !ok {
			inputs, ok = readDFC["Inputs"].(map[string]interface{})
		}
		if !ok {
			continue
		}
		for _, inputKey := range []string{"type", "Type"} {
			if inputType, ok := inputs[inputKey].(string); ok && inputType != "" {
				return inputType
			}
		}
	}
	return ""
}

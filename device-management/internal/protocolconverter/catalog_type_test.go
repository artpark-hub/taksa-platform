package protocolconverter

import "testing"

func TestIsGenericCatalogType(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"protocol-converter", true},
		{"", true},
		{"opcua", false},
		{"modbus", false},
	} {
		if got := IsGenericCatalogType(tc.in); got != tc.want {
			t.Errorf("IsGenericCatalogType(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsKnownNonOpcUaCatalogType(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"protocol-converter", false},
		{"opcua", false},
		{"modbus", true},
		{"modbus_tcp", true},
	} {
		if got := IsKnownNonOpcUaCatalogType(tc.in); got != tc.want {
			t.Errorf("IsKnownNonOpcUaCatalogType(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsKnownNonModbusCatalogType(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"protocol-converter", false},
		{"modbus", false},
		{"modbus_tcp", false},
		{"opcua", true},
		{"opc-ua", true},
	} {
		if got := IsKnownNonModbusCatalogType(tc.in); got != tc.want {
			t.Errorf("IsKnownNonModbusCatalogType(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestWireTypeFromJSON(t *testing.T) {
	deployShell := []byte(`{"name":"bridge-1","meta":{"protocol":"opcua"}}`)
	if got := WireTypeFromJSON(deployShell); got != "opcua" {
		t.Fatalf("WireTypeFromJSON deploy shell = %q, want opcua", got)
	}

	configure := []byte(`{"uuid":"abc","readDFC":{"inputs":{"type":"opcua"}}}`)
	if got := WireTypeFromJSON(configure); got != "opcua" {
		t.Fatalf("WireTypeFromJSON configure = %q, want opcua", got)
	}
}

func TestWireTypeFromMap(t *testing.T) {
	pc := map[string]interface{}{
		"meta": map[string]interface{}{"protocol": "opcua"},
		"readDFC": map[string]interface{}{
			"inputs": map[string]interface{}{"type": "opcua"},
		},
	}
	if got := WireTypeFromMap(pc); got != "opcua" {
		t.Fatalf("WireTypeFromMap meta = %q, want opcua", got)
	}

	pc = map[string]interface{}{
		"readDFC": map[string]interface{}{
			"inputs": map[string]interface{}{"type": "modbus"},
		},
	}
	if got := WireTypeFromMap(pc); got != "modbus" {
		t.Fatalf("WireTypeFromMap inputs = %q, want modbus", got)
	}
}

func TestCatalogWireTypeFromProtocolKind(t *testing.T) {
	for _, tc := range []struct {
		in, want string
	}{
		{"opcua", "opcua"},
		{"modbus_tcp", "modbus"},
		{"", ""},
	} {
		if got := CatalogWireTypeFromProtocolKind(tc.in); got != tc.want {
			t.Errorf("CatalogWireTypeFromProtocolKind(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveCatalogWireType(t *testing.T) {
	if got := ResolveCatalogWireType("protocol-converter", "opcua"); got != "opcua" {
		t.Fatalf("preserve existing wire type: got %q", got)
	}
	if got := ResolveCatalogWireType("modbus", "protocol-converter"); got != "modbus" {
		t.Fatalf("prefer incoming wire type: got %q", got)
	}
	if got := ResolveCatalogWireType("protocol-converter", "protocol-converter"); got != "protocol-converter" {
		t.Fatalf("both generic: got %q", got)
	}
}

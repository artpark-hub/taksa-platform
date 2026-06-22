package modbus

import (
	"strings"
	"testing"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter/opcua"
)

const genericModbusInputYAML = `modbus:
    byteOrder: ABCD
    controller: tcp://{{ .IP }}:{{ .PORT }}
    slaveIDs:
        - 1
    timeBetweenReads: 1s
    timeout: 5s
    unifiedAddresses:
        - VoltageL1_L2.holding.3019.FLOAT32
        - VoltageL2_L3.holding.3021.FLOAT32
        - VoltageL3_L1.holding.3023.FLOAT32
        - Average_Voltage_L_N.holding.3035.FLOAT32
`

const genericModbusProcessorYAML = `tag_processor:
    advancedProcessing: |-
        return msg;
    conditions:
        - if: msg.meta.modbus_tag_name.includes("Phase_angle")
          then: |
            msg.payload = Number(msg.payload);
            return msg;
        - if: msg.meta.modbus_tag_name.includes("Voltage") && !msg.meta.modbus_tag_name.includes("Phase_angle")
          then: |
            msg.payload = Number(msg.payload);
            return msg;
    defaults: |-
        switch(msg.meta.modbus_tag_unified_address) {
        {{- range .AddressMappings }}
          case "{{ .Address }}":
            break;
        {{- end }}
          case "VoltageL1_L2.holding.3019.FLOAT32":
            break;
          case "VoltageL2_L3.holding.3021.FLOAT32":
            msg.meta.tag_name = "VoltageL2_L3";
            break;
          default:
            break;
        }
        return msg;
`

func TestParseInputStructured_deviceYAMLIndent(t *testing.T) {
	cfg, status := parseInputStructured(genericModbusInputYAML)
	if status != v1.SectionParseStatus_PARSE_OK {
		t.Fatalf("parse status: %v", status)
	}
	std := cfg.GetStandard()
	if len(std.GetSlaveIds()) != 1 || std.GetSlaveIds()[0] != 1 {
		t.Fatalf("slaveIds: %v", std.GetSlaveIds())
	}
	if len(std.GetUnifiedAddresses()) != 4 {
		t.Fatalf("unifiedAddresses: %d", len(std.GetUnifiedAddresses()))
	}
	if cfg.GetAdvanced().GetByteOrder() != "ABCD" {
		t.Fatalf("byteOrder: %q", cfg.GetAdvanced().GetByteOrder())
	}
}

func TestParseProcessorStructured_genericModbusBridge(t *testing.T) {
	templateVars := map[string]string{
		"AddressMappings": `[{"Address":"VoltageL1_L2.holding.3019.FLOAT32","TagName":"","DataContract":"_historian"},{"Address":"VoltageL2_L3.holding.3021.FLOAT32","TagName":"VoltageL2_L3","DataContract":"_historian"}]`,
	}
	readFlow, status := opcua.ParseReadFlowStructured(genericModbusProcessorYAML, "buffer:\n    none: {}\n", templateVars)
	if status != v1.SectionParseStatus_PARSE_OK {
		t.Fatalf("processor parse: %v", status)
	}
	if readFlow.GetProcessorMode() != v1.EditSectionMode_STRUCTURED {
		t.Fatalf("processor mode: %v", readFlow.GetProcessorMode())
	}
	for _, m := range readFlow.GetProcessor().GetTagMappings() {
		if strings.Contains(m.GetNodeId(), "{{") {
			t.Fatalf("template placeholder in tagMappings: %+v", m)
		}
	}
	conds := readFlow.GetProcessor().GetConditions()
	if len(conds) != 2 {
		t.Fatalf("conditions: %d", len(conds))
	}
	if conds[0].GetClauses()[0].GetField() != "Modbus Tag Name" {
		t.Fatalf("clause field: %+v", conds[0].GetClauses()[0])
	}
	if len(conds[1].GetClauses()) != 2 {
		t.Fatalf("expected 2 clauses for compound condition, got %d", len(conds[1].GetClauses()))
	}
	if readFlow.GetDataType() != v1.BridgeDataType_TIME_SERIES {
		t.Fatalf("readFlow dataType: %v", readFlow.GetDataType())
	}
}

func TestInferInputMode_structuredOK(t *testing.T) {
	cfg, status := parseInputStructured(genericModbusInputYAML)
	if inferInputMode(status, cfg) != v1.EditSectionMode_STRUCTURED {
		t.Fatal("expected STRUCTURED mode")
	}
}

func TestBridgeDataTypeFromReadFlow(t *testing.T) {
	rf := &v1.OpcUaReadFlowSection{DataType: v1.BridgeDataType_TIME_SERIES}
	if got := bridgeDataTypeFromReadFlow(rf); got != v1.BridgeDataType_TIME_SERIES {
		t.Fatalf("got %v", got)
	}
}

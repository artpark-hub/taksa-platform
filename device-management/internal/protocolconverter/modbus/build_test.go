package modbus_test

import (
	"strings"
	"testing"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter/modbus"
)

func TestBuildConfigurePayload_modbusInput(t *testing.T) {
	req := &v1.DeployModbusProtocolConverterRequest{
		Name: "line-1-modbus",
		Connection: &v1.ProtocolConverterConnection{
			Ip:   "192.168.1.10",
			Port: 502,
		},
		Input: &v1.ModbusInputSection{
			Mode: v1.EditSectionMode_STRUCTURED,
			Structured: &v1.ModbusInputStructuredConfig{
				Standard: &v1.ModbusStandardInputConfig{
					Controller: "tcp://{{ .IP }}:{{ .PORT }}",
					SlaveIds:   []uint32{1},
					UnifiedAddresses: []string{
						"temperature.holding.100.INT16",
					},
					TimeBetweenReads: "1s",
					Timeout:          "1s",
				},
			},
		},
		ReadFlow: &v1.OpcUaReadFlowSection{
			ProcessorMode: v1.EditSectionMode_STRUCTURED,
			Processor: &v1.OpcUaProcessorStructuredConfig{
				Defaults: &v1.OpcUaProcessorDefaults{
					LocationPath: "{{ .location_path }}",
				},
			},
		},
		State: "active",
	}

	pc, err := modbus.BuildConfigurePayload("test-uuid", req.Name, req.Connection, req.Location, req.Input, req.ReadFlow, nil, req.State)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pc.ReadDFC.Inputs.Data, "modbus:") {
		t.Fatalf("expected modbus input yaml, got %q", pc.ReadDFC.Inputs.Data)
	}
	if pc.ReadDFC.Inputs.Type != modbus.InputType {
		t.Fatalf("input type: %s", pc.ReadDFC.Inputs.Type)
	}
	if pc.Meta.Protocol != modbus.ProtocolMetaValue {
		t.Fatalf("meta protocol: %s", pc.Meta.Protocol)
	}
}

func TestIsModbusProtocolConverter(t *testing.T) {
	pc := modbus.BuildDeployShell("x", &v1.ProtocolConverterConnection{Ip: "1.2.3.4", Port: 502}, nil, "active")
	if !modbus.IsModbusProtocolConverter(pc) {
		t.Fatal("deploy shell should be modbus kind")
	}
}

func TestBuildConfigurePayload_legacyAddressBitZero(t *testing.T) {
	req := &v1.DeployModbusProtocolConverterRequest{
		Name: "bit-bridge",
		Connection: &v1.ProtocolConverterConnection{
			Ip:   "192.168.1.10",
			Port: 502,
		},
		Input: &v1.ModbusInputSection{
			Mode: v1.EditSectionMode_STRUCTURED,
			Structured: &v1.ModbusInputStructuredConfig{
				Standard: &v1.ModbusStandardInputConfig{
					SlaveIds: []uint32{1},
					Addresses: []*v1.ModbusLegacyAddress{{
						Name:    "coil_flag",
						Address: 10,
						Type:    "BIT",
						Bit:     0,
					}},
				},
			},
		},
		ReadFlow: &v1.OpcUaReadFlowSection{
			ProcessorMode: v1.EditSectionMode_RAW,
			RawProcessorYaml: "tag_processor:\n  defaults: return msg;\n",
		},
		State: "active",
	}

	pc, err := modbus.BuildConfigurePayload("test-uuid", req.Name, req.Connection, req.Location, req.Input, req.ReadFlow, nil, req.State)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pc.ReadDFC.Inputs.Data, "bit: 0") {
		t.Fatalf("expected bit: 0 in yaml, got %q", pc.ReadDFC.Inputs.Data)
	}
}

func TestBuildConfigurePayload_workarounds(t *testing.T) {
	req := &v1.DeployModbusProtocolConverterRequest{
		Name: "workaround-bridge",
		Connection: &v1.ProtocolConverterConnection{
			Ip:   "192.168.1.10",
			Port: 502,
		},
		Input: &v1.ModbusInputSection{
			Mode: v1.EditSectionMode_STRUCTURED,
			Structured: &v1.ModbusInputStructuredConfig{
				Standard: &v1.ModbusStandardInputConfig{
					SlaveIds:          []uint32{1},
					UnifiedAddresses:  []string{"temp.holding.1.INT16"},
				},
				Advanced: &v1.ModbusAdvancedInputConfig{
					Workarounds: &v1.ModbusWorkaroundsConfig{
						PauseAfterConnect:        "500ms",
						OneRequestPerField:       true,
						ReadCoilsStartingAtZero:  true,
						TimeBetweenRequests:      "100ms",
						StringRegisterLocation:   "high",
					},
				},
			},
		},
		ReadFlow: &v1.OpcUaReadFlowSection{
			ProcessorMode: v1.EditSectionMode_RAW,
			RawProcessorYaml: "tag_processor:\n  defaults: return msg;\n",
		},
		State: "active",
	}

	pc, err := modbus.BuildConfigurePayload("test-uuid", req.Name, req.Connection, req.Location, req.Input, req.ReadFlow, nil, req.State)
	if err != nil {
		t.Fatal(err)
	}
	yaml := pc.ReadDFC.Inputs.Data
	for _, want := range []string{
		"pauseAfterConnect: 500ms",
		"oneRequestPerField: true",
		"readCoilsStartingAtZero: true",
		"timeBetweenRequests: 100ms",
		"stringRegisterLocation: high",
	} {
		if !strings.Contains(yaml, want) {
			t.Fatalf("expected %q in yaml, got %q", want, yaml)
		}
	}
}

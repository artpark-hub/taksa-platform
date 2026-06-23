package modbus_test

import (
	"testing"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter/modbus"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestDeployModbusProtoJSON_camelCaseBinds(t *testing.T) {
	const body = `{
		"name": "bridge-1",
		"connection": {"ip": "10.0.0.1", "port": 502},
		"applyReadConfig": true,
		"input": {
			"structured": {
				"standard": {"unifiedAddresses": ["tag.holding.1.INT16"]}
			}
		},
		"readFlow": {"processor": {}}
	}`
	var req v1.DeployModbusProtocolConverterRequest
	if err := protojson.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := modbus.ValidateDeployRequest(&req); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestValidateDeployRequest_fullDeployOK(t *testing.T) {
	req := &v1.DeployModbusProtocolConverterRequest{
		Name:            "bridge-1",
		Connection:      &v1.ProtocolConverterConnection{Ip: "10.0.0.1", Port: 502},
		ApplyReadConfig: true,
		Input: &v1.ModbusInputSection{
			Structured: &v1.ModbusInputStructuredConfig{
				Standard: &v1.ModbusStandardInputConfig{
					UnifiedAddresses: []string{"tag.holding.1.INT16"},
				},
			},
		},
		ReadFlow: &v1.OpcUaReadFlowSection{
			Processor: &v1.OpcUaProcessorStructuredConfig{},
		},
	}
	if err := modbus.ValidateDeployRequest(req); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

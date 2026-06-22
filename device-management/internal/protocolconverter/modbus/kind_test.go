package modbus_test

import (
	"encoding/json"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter/modbus"
)

func TestIsModbusProtocolConverter_umhCoreGETShapes(t *testing.T) {
	samples := []struct {
		name string
		json string
		want bool
	}{
		{
			name: "full get with modbus readDFC",
			json: `{"uuid":"75f3193b-0472-5f82-ab2a-70aa14c9f208","name":"Generic-modbus-bridge-1","connection":{"ip":"172.25.66.181","port":502},"meta":{"processingMode":"custom","protocol":"modbus"},"readDFC":{"inputs":{"type":"modbus","data":"modbus:\n  controller: tcp://x:502\n"}}}`,
			want: true,
		},
		{
			name: "legacy meta modbus_tcp only",
			json: `{"uuid":"75f3193b-0472-5f82-ab2a-70aa14c9f208","name":"Generic-modbus-bridge-1","meta":{"protocol":"modbus_tcp"}}`,
			want: true,
		},
		{
			name: "input type label with modbus yaml",
			json: `{"uuid":"75f3193b-0472-5f82-ab2a-70aa14c9f208","name":"Generic-modbus-bridge-1","readDFC":{"inputs":{"type":"label","data":"label: bridge_input\nmodbus:\n  controller: tcp://x:502\n"}}}`,
			want: true,
		},
		{
			name: "shell only",
			json: `{"uuid":"75f3193b-0472-5f82-ab2a-70aa14c9f208","name":"Generic-modbus-bridge-1"}`,
			want: false,
		},
		{
			name: "generate bridge",
			json: `{"uuid":"f6699dda-0d2c-5586-a00b-1e5aaa59761e","name":"Generic-generate-bridge-2","readDFC":{"inputs":{"type":"generate","data":"generate:\n  interval: 1s\n"}},"meta":{"protocol":"generate"}}`,
			want: false,
		},
	}
	for _, tc := range samples {
		t.Run(tc.name, func(t *testing.T) {
			var pc v2.ProtocolConverter
			if err := protojson.Unmarshal([]byte(tc.json), &pc); err != nil {
				t.Fatalf("protojson: %v", err)
			}
			if got := modbus.IsModbusProtocolConverter(&pc); got != tc.want {
				t.Fatalf("protojson kind=%v want %v meta=%q type=%q", got, tc.want, pc.GetMeta().GetProtocol(), pc.GetReadDFC().GetInputs().GetType())
			}
			var pc2 v2.ProtocolConverter
			if err := json.Unmarshal([]byte(tc.json), &pc2); err != nil {
				t.Fatalf("json: %v", err)
			}
			if got := modbus.IsModbusProtocolConverter(&pc2); got != tc.want {
				t.Fatalf("encoding/json kind=%v want %v", got, tc.want)
			}
		})
	}
}

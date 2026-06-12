package opcua

import (
	"strings"
	"testing"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
)

const genericBridgeProcessorYAML = `tag_processor:
    advancedProcessing: return msg;
    conditions:
        - if: msg.meta.opcua_attr_nodeid === "ns=4;i=6211"
          then: |
            msg.payload = parseFloat(msg.payload) + 273.15;
            msg.meta.tag_name = "CurrentTemperatureKelvin";
            return msg;
    defaults: |-
        msg.meta.location_path = "{{ .location_path }}";
        msg.meta.data_contract = "_historian";
        switch(msg.meta.opcua_attr_nodeid) {
          case "ns=1;i=1":
            msg.meta.tag_name = "state";
            break;
          case "ns=1;i=2":
            msg.meta.tag_name = "cycle_count";
            break;
          case "ns=1;i=3":
            msg.meta.tag_name = "good_count";
            break;
          case "ns=1;i=9":
            msg.meta.tag_name = "die_temperature_c";
            break;
          case "ns=1;i=1000":
            msg.meta.tag_name = "MetalFormingPress";
            break;
          default:
            break;
        }
        return msg;
`

func TestMergeTagMappings_switchAuthoritative(t *testing.T) {
	template := []*v1.OpcUaTagMapping{
		{NodeId: "ns=1;i=1", TagName: "state", DataContract: "_historian"},
		{NodeId: "ns=1;i=2", TagName: "cycle_count", DataContract: "_historian"},
	}
	switchCases := parseTagMappingsFromDefaultCode(extractYAMLSection(genericBridgeProcessorYAML, "defaults", "conditions", "advancedProcessing"))

	merged := mergeTagMappings(template, switchCases)
	if len(merged) != 5 {
		t.Fatalf("expected 5 tag mappings, got %d", len(merged))
	}
	byNode := map[string]string{}
	for _, m := range merged {
		byNode[m.GetNodeId()] = m.GetTagName()
	}
	if byNode["ns=1;i=1000"] != "MetalFormingPress" {
		t.Fatalf("missing switch mapping: %+v", byNode)
	}
}

func TestParseProcessorStructured_conditionsListForm(t *testing.T) {
	templateVars := map[string]string{
		"AddressMappings": `[{"Address":"ns=1;i=1","TagName":"state","DataContract":"_historian"},{"Address":"ns=1;i=2","TagName":"cycle_count","DataContract":"_historian"}]`,
	}
	readFlow, status := parseProcessorStructured(genericBridgeProcessorYAML, "buffer:\n  none: {}\n", templateVars)
	if status != v1.SectionParseStatus_PARSE_OK {
		t.Fatalf("parse status: %v", status)
	}
	if len(readFlow.GetProcessor().GetTagMappings()) != 5 {
		t.Fatalf("tag mappings: %d", len(readFlow.GetProcessor().GetTagMappings()))
	}
	conds := readFlow.GetProcessor().GetConditions()
	if len(conds) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conds))
	}
	if !strings.Contains(conds[0].GetIfExpression(), "ns=4;i=6211") {
		t.Fatalf("unexpected if: %q", conds[0].GetIfExpression())
	}
	if !strings.Contains(conds[0].GetAction(), "273.15") {
		t.Fatalf("unexpected then: %q", conds[0].GetAction())
	}
	if readFlow.GetProcessor().GetConditionsYaml() == "[]" {
		t.Fatal("conditions_yaml should not be empty for list-form conditions")
	}
}

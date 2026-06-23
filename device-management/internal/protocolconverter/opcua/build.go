package opcua

import (
	"fmt"
	"strings"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
)

const defaultBufferYAML = "buffer:\n  none: {}\n"

const defaultDesiredState = "active"

// BuildDeployShell builds the umh-core deploy payload (no readDFC).
func BuildDeployShell(name string, conn *v1.ProtocolConverterConnection, location *v1.ProtocolConverterLocation, state string) *v2.ProtocolConverter {
	pc := &v2.ProtocolConverter{
		Name:  name,
		State: normalizeDesiredState(state),
		Connection: &v2.ProtocolConverterConnection{
			Ip:   conn.GetIp(),
			Port: conn.GetPort(),
		},
		Location: locationLevels(location),
		Meta: &v2.ProtocolConverterMeta{
			Protocol: ProtocolMetaValue,
		},
	}
	return pc
}

// BuildConfigurePayload builds the full edit payload including readDFC.
func BuildConfigurePayload(uuid, name string, conn *v1.ProtocolConverterConnection, location *v1.ProtocolConverterLocation,
	input *v1.OpcUaInputSection, readFlow *v1.OpcUaReadFlowSection, templateVars map[string]string, state string) (*v2.ProtocolConverter, error) {

	inputYAML, err := resolveInputYAML(input)
	if err != nil {
		return nil, err
	}
	processorYAML, bufferYAML, err := resolveReadFlowYAML(readFlow)
	if err != nil {
		return nil, err
	}

	pc := BuildDeployShell(name, conn, location, state)
	pc.Uuid = uuid
	pc.ReadDFC = &v2.ProtocolConverterDFC{
		IgnoreErrors: true,
		Inputs: &v2.CommonDataFlowComponentInputConfig{
			Type: InputType,
			Data: ensureTrailingNewline(inputYAML),
		},
		Pipeline: &v2.CommonDataFlowComponentPipelineConfig{
			Threads: -1,
			Processors: map[string]*v2.CommonDataFlowComponentPipelineConfigProcessor{
				"0": {
					Type: "tag_processor",
					Data: ensureTrailingNewline(processorYAML),
				},
			},
		},
		RawYAML: &v2.CommonDataFlowComponentRawYamlConfig{
			Data: ensureTrailingNewline(bufferYAML),
		},
	}
	pc.TemplateInfo = buildTemplateInfo(conn, templateVars, readFlow)
	return pc, nil
}

func normalizeDesiredState(state string) string {
	s := strings.TrimSpace(strings.ToLower(state))
	if s == "" || s == "active" {
		return defaultDesiredState
	}
	return s
}

func resolveInputYAML(section *v1.OpcUaInputSection) (string, error) {
	if section == nil {
		return "", fmt.Errorf("input section is required")
	}
	switch section.GetMode() {
	case v1.EditSectionMode_RAW:
		raw := strings.TrimSpace(section.GetRawYaml())
		if raw == "" {
			return "", fmt.Errorf("input.raw_yaml is required when mode is RAW")
		}
		return raw, nil
	case v1.EditSectionMode_STRUCTURED, v1.EditSectionMode_EDIT_SECTION_MODE_UNSPECIFIED:
		return buildInputYAMLFromStructured(section.GetStructured())
	default:
		return "", fmt.Errorf("invalid input mode")
	}
}

func resolveReadFlowYAML(section *v1.OpcUaReadFlowSection) (processorYAML, bufferYAML string, err error) {
	bufferYAML = defaultBufferYAML
	processorYAML = defaultProcessorYAML()

	if section == nil {
		return processorYAML, bufferYAML, nil
	}

	switch section.GetBufferMode() {
	case v1.EditSectionMode_RAW:
		if raw := strings.TrimSpace(section.GetRawBufferYaml()); raw != "" {
			bufferYAML = raw
		}
	case v1.EditSectionMode_STRUCTURED, v1.EditSectionMode_EDIT_SECTION_MODE_UNSPECIFIED:
		if inject := section.GetYamlInject(); inject != nil {
			if raw := strings.TrimSpace(inject.GetRawYaml()); raw != "" {
				bufferYAML = raw
			}
		}
	}

	switch section.GetProcessorMode() {
	case v1.EditSectionMode_RAW:
		raw := strings.TrimSpace(section.GetRawProcessorYaml())
		if raw == "" {
			return "", "", fmt.Errorf("read_flow.raw_processor_yaml is required when processor_mode is RAW")
		}
		processorYAML = raw
	case v1.EditSectionMode_STRUCTURED, v1.EditSectionMode_EDIT_SECTION_MODE_UNSPECIFIED:
		built, err := buildProcessorYAMLFromStructured(section.GetProcessor())
		if err != nil {
			return "", "", err
		}
		processorYAML = built
	default:
		return "", "", fmt.Errorf("invalid processor_mode")
	}

	return processorYAML, bufferYAML, nil
}

// ResolveReadFlowYAML builds tag_processor and buffer YAML (shared by OPC-UA and Modbus facades).
func ResolveReadFlowYAML(section *v1.OpcUaReadFlowSection) (string, string, error) {
	return resolveReadFlowYAML(section)
}

// ParseReadFlowStructured parses tag_processor YAML into structured read_flow fields.
func ParseReadFlowStructured(processorYAML, bufferYAML string, templateVars map[string]string) (*v1.OpcUaReadFlowSection, v1.SectionParseStatus) {
	return parseProcessorStructured(processorYAML, bufferYAML, templateVars)
}

// BuildTemplateInfo assembles template variables for protocol converter deploy/edit.
func BuildTemplateInfo(conn *v1.ProtocolConverterConnection, extra map[string]string, readFlow *v1.OpcUaReadFlowSection) *v2.ProtocolConverterTemplateInfo {
	return buildTemplateInfo(conn, extra, readFlow)
}

func buildTemplateInfo(conn *v1.ProtocolConverterConnection, extra map[string]string, readFlow *v1.OpcUaReadFlowSection) *v2.ProtocolConverterTemplateInfo {
	vars := []*v2.ProtocolConverterVariable{
		{Label: "IP", Value: conn.GetIp()},
		{Label: "PORT", Value: fmt.Sprintf("%d", conn.GetPort())},
	}
	merged := map[string]string{}
	for k, v := range extra {
		merged[k] = v
	}
	if readFlow != nil && readFlow.GetProcessor() != nil {
		if merged["AddressMappings"] == "" {
			if json := buildAddressMappingsJSON(readFlow.GetProcessor().GetTagMappings()); json != "" {
				merged["AddressMappings"] = json
			}
		}
	}
	for k, v := range merged {
		vars = append(vars, &v2.ProtocolConverterVariable{Label: k, Value: v})
	}
	return &v2.ProtocolConverterTemplateInfo{Variables: vars}
}

func locationLevels(loc *v1.ProtocolConverterLocation) map[int32]string {
	if loc == nil || len(loc.GetLevels()) == 0 {
		return nil
	}
	return loc.GetLevels()
}

func ensureTrailingNewline(s string) string {
	if s == "" {
		return s
	}
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

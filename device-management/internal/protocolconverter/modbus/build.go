package modbus

import (
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	v2 "github.com/artpark-hub/taksa-platform/device-management/api/umh-core/v2"
	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter/opcua"
)

const defaultBufferYAML = "buffer:\n  none: {}\n"
const defaultDesiredState = "active"

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

func BuildConfigurePayload(uuid, name string, conn *v1.ProtocolConverterConnection, location *v1.ProtocolConverterLocation,
	input *v1.ModbusInputSection, readFlow *v1.OpcUaReadFlowSection, templateVars map[string]string, state string) (*v2.ProtocolConverter, error) {

	inputYAML, err := resolveInputYAML(input)
	if err != nil {
		return nil, err
	}
	processorYAML, bufferYAML, err := opcua.ResolveReadFlowYAML(readFlow)
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
	pc.TemplateInfo = opcua.BuildTemplateInfo(conn, templateVars, readFlow)
	return pc, nil
}

func normalizeDesiredState(state string) string {
	s := strings.TrimSpace(strings.ToLower(state))
	if s == "" || s == "active" {
		return defaultDesiredState
	}
	return s
}

func resolveInputYAML(section *v1.ModbusInputSection) (string, error) {
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

func buildInputYAMLFromStructured(cfg *v1.ModbusInputStructuredConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("input.structured is required")
	}
	std := cfg.GetStandard()
	if std == nil {
		std = &v1.ModbusStandardInputConfig{}
	}
	adv := cfg.GetAdvanced()
	if adv == nil {
		adv = &v1.ModbusAdvancedInputConfig{}
	}

	controller := strings.TrimSpace(std.GetController())
	if controller == "" {
		controller = "tcp://{{ .IP }}:{{ .PORT }}"
	}
	if len(std.GetUnifiedAddresses()) == 0 && len(std.GetAddresses()) == 0 {
		return "", fmt.Errorf("input.structured.standard.unified_addresses or addresses must not be empty")
	}

	lines := []string{"modbus:"}
	lines = append(lines, fmt.Sprintf("  controller: %s", formatYAMLScalar(controller)))

	if len(std.GetSlaveIds()) > 0 {
		lines = append(lines, "  slaveIDs:")
		for _, id := range std.GetSlaveIds() {
			lines = append(lines, fmt.Sprintf("    - %d", id))
		}
	}

	if tbr := strings.TrimSpace(std.GetTimeBetweenReads()); tbr != "" {
		lines = append(lines, fmt.Sprintf("  timeBetweenReads: %s", formatYAMLScalar(tbr)))
	}
	if timeout := strings.TrimSpace(std.GetTimeout()); timeout != "" {
		lines = append(lines, fmt.Sprintf("  timeout: %s", formatYAMLScalar(timeout)))
	}

	if len(std.GetUnifiedAddresses()) > 0 {
		lines = append(lines, "  unifiedAddresses:")
		for _, addr := range std.GetUnifiedAddresses() {
			lines = append(lines, fmt.Sprintf("    - %s", formatYAMLScalar(strings.TrimSpace(addr))))
		}
	} else {
		lines = append(lines, "  addresses:")
		for _, a := range std.GetAddresses() {
			lines = append(lines, "    - name: "+formatYAMLScalar(a.GetName()))
			if r := strings.TrimSpace(a.GetRegister()); r != "" {
				lines = append(lines, "      register: "+formatYAMLScalar(r))
			}
			lines = append(lines, fmt.Sprintf("      address: %d", a.GetAddress()))
			if t := strings.TrimSpace(a.GetType()); t != "" {
				lines = append(lines, "      type: "+formatYAMLScalar(t))
			}
			if a.GetLength() > 0 {
				lines = append(lines, fmt.Sprintf("      length: %d", a.GetLength()))
			}
			if a.GetBit() > 0 {
				lines = append(lines, fmt.Sprintf("      bit: %d", a.GetBit()))
			}
			if a.GetScale() != 0 {
				lines = append(lines, fmt.Sprintf("      scale: %g", a.GetScale()))
			}
			if o := strings.TrimSpace(a.GetOutput()); o != "" {
				lines = append(lines, "      output: "+formatYAMLScalar(o))
			}
			if a.GetSlaveId() > 0 {
				lines = append(lines, fmt.Sprintf("      slaveID: %d", a.GetSlaveId()))
			}
		}
	}

	if tm := strings.TrimSpace(adv.GetTransmissionMode()); tm != "" {
		lines = append(lines, fmt.Sprintf("  transmissionMode: %s", formatYAMLScalar(tm)))
	}
	if opt := strings.TrimSpace(adv.GetOptimization()); opt != "" {
		lines = append(lines, fmt.Sprintf("  optimization: %s", formatYAMLScalar(opt)))
	}
	if adv.GetOptimizationMaxRegisterFill() > 0 {
		lines = append(lines, fmt.Sprintf("  optimizationMaxRegisterFill: %d", adv.GetOptimizationMaxRegisterFill()))
	}
	if bo := strings.TrimSpace(adv.GetByteOrder()); bo != "" {
		lines = append(lines, fmt.Sprintf("  byteOrder: %s", formatYAMLScalar(bo)))
	}
	if adv.GetBusyRetries() > 0 {
		lines = append(lines, fmt.Sprintf("  busyRetries: %d", adv.GetBusyRetries()))
	}
	if brw := strings.TrimSpace(adv.GetBusyRetriesWait()); brw != "" {
		lines = append(lines, fmt.Sprintf("  busyRetriesWait: %s", formatYAMLScalar(brw)))
	}
	if w := adv.GetWorkarounds(); w != nil {
		if pac := strings.TrimSpace(w.GetPauseAfterConnect()); pac != "" {
			lines = append(lines, "  workarounds:")
			lines = append(lines, fmt.Sprintf("    pauseAfterConnect: %s", formatYAMLScalar(pac)))
		}
	}

	for _, setting := range cfg.GetAdditionalSettings() {
		key := strings.TrimSpace(setting.GetKey())
		if key == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s: %s", key, formatYAMLScalar(setting.GetValue())))
	}

	return strings.Join(lines, "\n"), nil
}

func formatYAMLScalar(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":{}[]&*#?|-<>=!%@`") || strings.Contains(s, "\n") {
		return strconv.Quote(s)
	}
	return s
}

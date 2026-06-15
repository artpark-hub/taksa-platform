package opcua

import (
	"fmt"
	"strings"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
)

// ValidateDeployRequest validates facade deploy input before queuing workflow.
func ValidateDeployRequest(req *v1.DeployOpcUaProtocolConverterRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}
	if strings.TrimSpace(req.GetName()) == "" {
		return fmt.Errorf("name is required")
	}
	if err := validateConnection(req.GetConnection()); err != nil {
		return err
	}
	if req.GetApplyReadConfig() {
		if err := validateInputSection(req.GetInput()); err != nil {
			return err
		}
	}
	return nil
}

// ValidateEditRequest validates facade edit input.
func ValidateEditRequest(req *v1.EditOpcUaProtocolConverterRequest) error {
	if req == nil {
		return fmt.Errorf("request is required")
	}
	if strings.TrimSpace(req.GetUuid()) == "" {
		return fmt.Errorf("uuid is required")
	}
	if req.GetConnection() != nil {
		if err := validateConnection(req.GetConnection()); err != nil {
			return err
		}
	}
	if req.GetInput() == nil {
		return fmt.Errorf("input is required")
	}
	if req.GetReadFlow() == nil {
		return fmt.Errorf("read_flow is required")
	}
	if err := validateInputSection(req.GetInput()); err != nil {
		return err
	}
	return nil
}

func validateConnection(conn *v1.ProtocolConverterConnection) error {
	if conn == nil || strings.TrimSpace(conn.GetIp()) == "" || conn.GetPort() == 0 {
		return fmt.Errorf("connection ip and port are required")
	}
	return nil
}

func validateInputSection(section *v1.OpcUaInputSection) error {
	if section == nil {
		return fmt.Errorf("input section is required when applying read config")
	}
	if section.GetMode() == v1.EditSectionMode_RAW {
		if strings.TrimSpace(section.GetRawYaml()) == "" {
			return fmt.Errorf("input.raw_yaml is required when mode is RAW")
		}
		return nil
	}
	if section.GetMode() == v1.EditSectionMode_STRUCTURED || section.GetMode() == v1.EditSectionMode_EDIT_SECTION_MODE_UNSPECIFIED {
		if err := validateStructuredInput(section.GetStructured()); err != nil {
			return err
		}
		return nil
	}
	if section.GetStructured() == nil && strings.TrimSpace(section.GetRawYaml()) == "" {
		return fmt.Errorf("input: set mode explicitly when providing both structured and raw")
	}
	return nil
}

func validateStructuredInput(cfg *v1.OpcUaInputStructuredConfig) error {
	if cfg == nil || cfg.GetStandard() == nil {
		return fmt.Errorf("input.structured.standard is required")
	}
	if len(collectNodeIDs(cfg.GetStandard())) == 0 {
		return fmt.Errorf("input.structured.standard.subscribe_node_ids or node_ids_text must not be empty")
	}
	return nil
}

func hasStructuredInput(section *v1.OpcUaInputSection) bool {
	if section == nil || section.GetStructured() == nil {
		return false
	}
	return len(collectNodeIDs(section.GetStructured().GetStandard())) > 0 ||
		len(section.GetStructured().GetAdditionalSettings()) > 0 ||
		section.GetStructured().GetAdvanced() != nil
}

func hasStructuredProcessor(section *v1.OpcUaReadFlowSection) bool {
	if section == nil || section.GetProcessor() == nil {
		return false
	}
	p := section.GetProcessor()
	return p.GetDefaults() != nil ||
		len(p.GetTagMappings()) > 0 ||
		len(p.GetConditions()) > 0 ||
		strings.TrimSpace(p.GetDefaultsCode()) != "" ||
		strings.TrimSpace(p.GetConditionsYaml()) != "" ||
		strings.TrimSpace(p.GetAdvancedProcessing()) != ""
}

// ValidateReadFlowSectionModes validates dual-mode read_flow sections (shared with Modbus facade).
func ValidateReadFlowSectionModes(readFlow *v1.OpcUaReadFlowSection) error {
	if readFlow == nil {
		return nil
	}
	hasP := hasStructuredProcessor(readFlow) || readFlow.GetYamlInject() != nil
	hasR := strings.TrimSpace(readFlow.GetRawProcessorYaml()) != "" ||
		strings.TrimSpace(readFlow.GetRawBufferYaml()) != ""
	if hasP && hasR && (readFlow.GetProcessorMode() == v1.EditSectionMode_EDIT_SECTION_MODE_UNSPECIFIED ||
		readFlow.GetBufferMode() == v1.EditSectionMode_EDIT_SECTION_MODE_UNSPECIFIED) {
		return fmt.Errorf("read_flow: both structured and raw yaml set; set processor_mode and buffer_mode explicitly")
	}
	return nil
}

// ValidateSectionModes rejects ambiguous dual structured+raw without explicit mode.
func ValidateSectionModes(input *v1.OpcUaInputSection, readFlow *v1.OpcUaReadFlowSection) error {
	if input != nil {
		hasS := hasStructuredInput(input)
		hasR := strings.TrimSpace(input.GetRawYaml()) != ""
		if hasS && hasR && input.GetMode() == v1.EditSectionMode_EDIT_SECTION_MODE_UNSPECIFIED {
			return fmt.Errorf("input: both structured and raw_yaml set; set mode explicitly")
		}
	}
	return ValidateReadFlowSectionModes(readFlow)
}

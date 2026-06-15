package modbus

import (
	"fmt"
	"strings"

	v1 "github.com/artpark-hub/taksa-platform/device-management/api/devicemgmt/v1"
	"github.com/artpark-hub/taksa-platform/device-management/internal/protocolconverter/opcua"
)

func ValidateDeployRequest(req *v1.DeployModbusProtocolConverterRequest) error {
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

func ValidateEditRequest(req *v1.EditModbusProtocolConverterRequest) error {
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

func validateInputSection(section *v1.ModbusInputSection) error {
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
		return validateStructuredInput(section.GetStructured())
	}
	return nil
}

func validateStructuredInput(cfg *v1.ModbusInputStructuredConfig) error {
	if cfg == nil || cfg.GetStandard() == nil {
		return fmt.Errorf("input.structured.standard is required")
	}
	std := cfg.GetStandard()
	if len(std.GetUnifiedAddresses()) == 0 && len(std.GetAddresses()) == 0 {
		return fmt.Errorf("input.structured.standard.unified_addresses or addresses must not be empty")
	}
	return nil
}

func ValidateSectionModes(input *v1.ModbusInputSection, readFlow *v1.OpcUaReadFlowSection) error {
	if input != nil {
		hasS := hasStructuredModbusInput(input)
		hasR := strings.TrimSpace(input.GetRawYaml()) != ""
		if hasS && hasR && input.GetMode() == v1.EditSectionMode_EDIT_SECTION_MODE_UNSPECIFIED {
			return fmt.Errorf("input: both structured and raw_yaml set; set mode explicitly")
		}
	}
	return opcua.ValidateReadFlowSectionModes(readFlow)
}

func hasStructuredModbusInput(section *v1.ModbusInputSection) bool {
	if section == nil || section.GetStructured() == nil {
		return false
	}
	std := section.GetStructured().GetStandard()
	if std == nil {
		return false
	}
	return len(std.GetUnifiedAddresses()) > 0 || len(std.GetAddresses()) > 0 ||
		strings.TrimSpace(std.GetController()) != "" ||
		len(section.GetStructured().GetAdditionalSettings()) > 0 ||
		section.GetStructured().GetAdvanced() != nil
}

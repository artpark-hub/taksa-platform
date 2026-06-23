package models

import "time"

const (
	WorkflowTypeDeployOpcUa = "deploy-opcua-protocol-converter"
	WorkflowTypeDeployModbus = "deploy-modbus-protocol-converter"

	ProtocolKindOpcUa    = "opcua"
	ProtocolKindModbusTCP = "modbus_tcp"

	WorkflowStageDeploying    = "DEPLOYING"
	WorkflowStageConfiguring  = "CONFIGURING"
	WorkflowStageRollingBack  = "ROLLING_BACK"

	RollbackClean       = "CLEAN"
	RollbackOrphanShell = "ORPHAN_SHELL"
)

// ActionWorkflow orchestrates multi-step protocol converter operations.
// Status uses ActionStatus values; Stage is set only while Status == ActionStatusProcessing.
type ActionWorkflow struct {
	ID                string
	TenantID          string
	DeviceID          string
	WorkflowType      string
	ProtocolKind      string
	ConverterUUID     string
	ConverterName     string
	Status            ActionStatus
	Stage             string
	RollbackStatus    string
	DeployActionID    string
	ConfigureActionID string
	RollbackActionID       string
	PendingConfigureJSON   string
	ErrorMessage           string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ExpiresAt         time.Time
	CompletedAt       time.Time
}

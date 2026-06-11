-- Action workflows: composite orchestration (e.g. facade deploy + configure).
CREATE TABLE IF NOT EXISTS action_workflows (
  id TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,
  device_id TEXT NOT NULL,
  workflow_type TEXT NOT NULL,
  protocol_kind TEXT NOT NULL,
  converter_uuid TEXT,
  converter_name TEXT,
  status INTEGER NOT NULL DEFAULT 3,
  stage TEXT,
  rollback_status TEXT,
  deploy_action_id TEXT,
  configure_action_id TEXT,
  rollback_action_id TEXT,
  pending_configure_json TEXT,
  error_message TEXT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMP WITH TIME ZONE,
  completed_at TIMESTAMP WITH TIME ZONE,
  CONSTRAINT fk_action_workflows_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_action_workflows_tenant_device
  ON action_workflows(tenant_id, device_id);

CREATE INDEX IF NOT EXISTS idx_action_workflows_deploy_action
  ON action_workflows(deploy_action_id);

CREATE INDEX IF NOT EXISTS idx_action_workflows_configure_action
  ON action_workflows(configure_action_id);

CREATE INDEX IF NOT EXISTS idx_action_workflows_rollback_action
  ON action_workflows(rollback_action_id);

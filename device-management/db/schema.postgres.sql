-- Taksa Platform DM - PostgreSQL Database Schema
-- IDENTICAL structure to SQLite, only SQL syntax differs
-- Production-quality database design for device management system

-- Devices table: Stores all registered devices
CREATE TABLE IF NOT EXISTS devices (
  id TEXT PRIMARY KEY,
  uuid TEXT UNIQUE NOT NULL,
  tenant_id UUID NOT NULL,  -- Multi-tenancy: isolate devices by tenant
  created_by TEXT,  -- Owner UUID (user identifier within tenant)
  name TEXT NOT NULL,
  
  -- Hardware metadata
  hardware_version TEXT,
  operating_system TEXT,
  manufacturer TEXT,
  firmware_version TEXT,
  ip_address TEXT,
  mac_address TEXT,
  
  -- Location hierarchy (7 levels: company, plant, area, zone, line, workCell, workUnit)
  location_company TEXT,      -- Level 0
  location_plant TEXT,        -- Level 1
  location_area TEXT,         -- Level 2
  location_zone TEXT,         -- Level 3
  location_line TEXT,         -- Level 4
  location_work_cell TEXT,    -- Level 5
  location_work_unit TEXT,    -- Level 6
  
  -- Certificates
  certificate TEXT,  -- PEM format (X.509)
  encrypted_private_key TEXT,  -- PEM format (PKCS#8)
  
  -- Status: 0=UNSPECIFIED, 1=PENDING, 2=ACTIVE, 3=INACTIVE, 4=SUSPENDED, 5=DECOMMISSIONED
  status INTEGER DEFAULT 1,
  
  -- Timestamps
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  last_seen TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  last_login_at TIMESTAMP WITH TIME ZONE,
  auth_token_expires_at TIMESTAMP WITH TIME ZONE,
  
  UNIQUE(tenant_id, name),  -- Device names unique per tenant
  UNIQUE(uuid)
);

CREATE INDEX IF NOT EXISTS idx_devices_tenant_id ON devices(tenant_id);
CREATE INDEX IF NOT EXISTS idx_devices_created_by ON devices(created_by);
CREATE INDEX IF NOT EXISTS idx_devices_uuid ON devices(uuid);
CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status);
CREATE INDEX IF NOT EXISTS idx_devices_company ON devices(location_company);
CREATE INDEX IF NOT EXISTS idx_devices_created_at ON devices(created_at);
CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices(last_seen);
CREATE INDEX IF NOT EXISTS idx_devices_tenant_created_by ON devices(tenant_id, created_by);

-- Auth tokens table: Stores authentication tokens with raw token for hashing during validation
CREATE TABLE IF NOT EXISTS auth_tokens (
  id TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,  -- Multi-tenancy isolation
  token TEXT UNIQUE NOT NULL,  -- Raw token (64 hex chars, 32 bytes)
  device_id TEXT NOT NULL,
  
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
  
  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_auth_tokens_tenant_id ON auth_tokens(tenant_id);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_device_id ON auth_tokens(device_id);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_expires_at ON auth_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_token ON auth_tokens(token);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_tenant_device ON auth_tokens(tenant_id, device_id);

-- Actions table: Stores actions queued for devices
CREATE TABLE IF NOT EXISTS actions (
  id TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,  -- Multi-tenancy isolation
  device_id TEXT NOT NULL,
  action_type TEXT NOT NULL,  -- deploy-protocol-converter, restart, update, etc.

  -- Payload
  payload_type TEXT,  -- google.protobuf.Any @type
  payload_data TEXT,  -- JSON encoded

  -- Retry logic
  max_retries INTEGER DEFAULT 3,
  retry_count INTEGER DEFAULT 0,

  -- Status: 0=UNSPECIFIED, 1=QUEUED, 2=DELIVERED, 3=PROCESSING, 4=COMPLETED, 5=FAILED, 6=EXPIRED, 7=CANCELLED
  status INTEGER DEFAULT 1,

  -- Error details
  error_message TEXT,

  -- Timestamps
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMP WITH TIME ZONE,
  delivered_at TIMESTAMP WITH TIME ZONE,
  completed_at TIMESTAMP WITH TIME ZONE,

  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_actions_tenant_id ON actions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_actions_device_id ON actions(device_id);
CREATE INDEX IF NOT EXISTS idx_actions_status ON actions(status);
CREATE INDEX IF NOT EXISTS idx_actions_created_at ON actions(created_at);
CREATE INDEX IF NOT EXISTS idx_actions_device_status ON actions(device_id, status);
CREATE INDEX IF NOT EXISTS idx_actions_tenant_status ON actions(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_actions_tenant_device ON actions(tenant_id, device_id);

-- Idempotency for edge status subscription: at most one QUEUED subscribe action per device+tenant.
-- This supports running multiple device-management replicas without enqueueing duplicate subscribe actions.
CREATE UNIQUE INDEX IF NOT EXISTS uq_actions_subscribe_queued
  ON actions(tenant_id, device_id, action_type)
  WHERE action_type = 'subscribe' AND status = 1;

-- Messages table: Stores message history for auditing and debugging
CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,  -- Multi-tenancy isolation
  device_id TEXT NOT NULL,
  message_type TEXT NOT NULL,  -- status, telemetry, action, error
  
  -- Content
  content TEXT,  -- Base64 encoded
  
  -- Metadata
  trace_id TEXT,
  request_id TEXT,
  correlation_id TEXT,
  
  -- Direction: 0=UNSPECIFIED, 1=INBOUND (Device→Console), 2=OUTBOUND (Console→Device)
  direction INTEGER DEFAULT 0,
  
  -- Timestamps
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMP WITH TIME ZONE,
  
  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_tenant_id ON messages(tenant_id);
CREATE INDEX IF NOT EXISTS idx_messages_device_id ON messages(device_id);
CREATE INDEX IF NOT EXISTS idx_messages_message_type ON messages(message_type);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_direction ON messages(direction);
CREATE INDEX IF NOT EXISTS idx_messages_device_created ON messages(device_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_tenant_device ON messages(tenant_id, device_id);

-- Certificates table: Stores X.509 certificates for devices and users
CREATE TABLE IF NOT EXISTS certificates (
  id TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,  -- Multi-tenancy isolation
  device_id TEXT NOT NULL,
  user_email TEXT,  -- Optional: email of certificate owner
  
  -- Certificate data
  certificate TEXT NOT NULL,  -- PEM format
  private_key TEXT,  -- Encrypted PEM format
  
  -- Status
  is_active INTEGER DEFAULT 1,
  
  -- Timestamps
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMP WITH TIME ZONE,
  
  UNIQUE(tenant_id, device_id, user_email),
  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_certificates_tenant_id ON certificates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_certificates_device_id ON certificates(device_id);
CREATE INDEX IF NOT EXISTS idx_certificates_user_email ON certificates(user_email);
CREATE INDEX IF NOT EXISTS idx_certificates_device_email ON certificates(device_id, user_email);
CREATE INDEX IF NOT EXISTS idx_certificates_tenant_device ON certificates(tenant_id, device_id);

-- Device certificates: Stores device's own certificate for mTLS communication
-- One certificate per device, enforced by PRIMARY KEY
CREATE TABLE IF NOT EXISTS device_certificates (
  device_id TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,  -- Multi-tenancy isolation
  certificate TEXT NOT NULL,  -- PEM format X.509 certificate
  private_key TEXT,           -- Encrypted PEM format private key
  is_active INTEGER DEFAULT 1,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMP WITH TIME ZONE,
  
  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_device_certificates_tenant_id ON device_certificates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_device_certificates_expires_at ON device_certificates(expires_at);

-- User certificates: Stores user-specific certificates for individual users on a device
-- Multiple certificates per device, one per user email per tenant
CREATE TABLE IF NOT EXISTS user_certificates (
  id TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,  -- Multi-tenancy isolation
  device_id TEXT NOT NULL,
  user_email TEXT NOT NULL,   -- Email of certificate owner
  certificate TEXT NOT NULL,  -- PEM format X.509 certificate
  private_key TEXT,           -- Encrypted PEM format private key
  is_active INTEGER DEFAULT 1,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMP WITH TIME ZONE,
  
  UNIQUE(tenant_id, device_id, user_email),
  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_user_certificates_tenant_id ON user_certificates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_user_certificates_device_id ON user_certificates(device_id);
CREATE INDEX IF NOT EXISTS idx_user_certificates_user_email ON user_certificates(user_email);
CREATE INDEX IF NOT EXISTS idx_user_certificates_expires_at ON user_certificates(expires_at);
CREATE INDEX IF NOT EXISTS idx_user_certificates_tenant_device ON user_certificates(tenant_id, device_id);

-- Action Message Tracking: Correlates request and response messages for complete traceability
-- One entry per action sent to device - tracks the entire message flow lifecycle
--
-- LIFECYCLE:
--   1. Pull: Generate trace_id for this action→device send, store here
--   2. Device executes action
--   3. Push: Device sends response with response_trace_id (echo of trace_id)
--   4. Mark completed_at when response received
--   5. Audit: Complete request-response pair available for tracing
CREATE TABLE IF NOT EXISTS action_message_tracking (
  id TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,        -- Multi-tenancy isolation
  action_id TEXT NOT NULL,        -- FK to actions(id) - the action being sent
  device_id TEXT NOT NULL,        -- FK to devices(id) - for querying traces by device
  
  -- Request tracking (outbound: Pull request)
  trace_id TEXT NOT NULL,         -- Generated when pulling action, sent in message.metadata.traceId
  trace_generated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  
  -- Response tracking (inbound: Push response)
  response_trace_id TEXT,         -- Echo from device response (should match trace_id)
  response_message_id TEXT,       -- ID of the response message (direct pointer for O(1) lookup)
  response_received_at TIMESTAMP WITH TIME ZONE,      -- When response was received via Push
  
  -- Status: 1=PENDING, 2=IN_FLIGHT, 3=RESPONSE_RECEIVED, 4=COMPLETED
  correlation_status INTEGER DEFAULT 1,  -- Track correlation lifecycle
  
  -- Timestamps
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  completed_at TIMESTAMP WITH TIME ZONE,
  
  -- Indexes
  UNIQUE(action_id),              -- One tracking entry per action
  FOREIGN KEY(action_id) REFERENCES actions(id) ON DELETE CASCADE,
  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_action_message_tracking_tenant_id ON action_message_tracking(tenant_id);
CREATE INDEX IF NOT EXISTS idx_action_message_tracking_action_id ON action_message_tracking(action_id);
CREATE INDEX IF NOT EXISTS idx_action_message_tracking_device_id ON action_message_tracking(device_id);
CREATE INDEX IF NOT EXISTS idx_action_message_tracking_trace_id ON action_message_tracking(trace_id);
CREATE INDEX IF NOT EXISTS idx_action_message_tracking_response_trace_id ON action_message_tracking(response_trace_id);
CREATE INDEX IF NOT EXISTS idx_action_message_tracking_status ON action_message_tracking(correlation_status);
CREATE INDEX IF NOT EXISTS idx_action_message_tracking_created_at ON action_message_tracking(created_at);
CREATE INDEX IF NOT EXISTS idx_action_message_tracking_tenant_device ON action_message_tracking(tenant_id, device_id);

-- Settings table: Stores system configuration
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  description TEXT,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_settings_key ON settings(key);

-- Insert default settings (PostgreSQL syntax)
INSERT INTO settings (key, value, description) VALUES
  ('api_url', 'https://localhost:8080/api', 'Base URL for API calls'),
  ('jwt_secret', 'default-secret-change-in-production', 'Secret key for JWT signing'),
  ('token_expiry_hours', '1', 'JWT token expiry time in hours'),
  ('auth_token_expiry_days', '7', 'Auth token expiry time in days'),
  ('max_message_history', '1000', 'Maximum message history to retain per device'),
  ('polling_interval_ms', '10', 'Expected polling interval from devices in milliseconds')
ON CONFLICT (key) DO NOTHING;

-- Protocol Converters table: Stores protocol converter (bridge) configurations
-- Tracks both intended state (from deploy/edit/delete actions) and actual state (from device)
CREATE TABLE IF NOT EXISTS protocol_converters (
  id TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,  -- Multi-tenancy isolation
  device_id TEXT NOT NULL,
  uuid TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  connection_uuid TEXT,
  
  -- YAML configurations (all optional)
  input_yaml TEXT,
  processor_yaml TEXT,
  inject_yaml TEXT,
  
  -- Optional settings
  ignore_errors INTEGER DEFAULT 0,
  metadata TEXT,  -- JSON object stored as string
  
  -- Status tracking
  deployment_status TEXT DEFAULT 'PENDING',  -- PENDING, ACTIVE, FAILED
  health_status TEXT DEFAULT 'UNKNOWN',      -- ONLINE, OFFLINE, UNKNOWN
  error_message TEXT,
  
  -- Tracking
  last_synced TIMESTAMP WITH TIME ZONE,     -- From latest StatusMessage
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  
  -- Constraints
  CONSTRAINT unique_tenant_device_converter UNIQUE (tenant_id, device_id, uuid),
  CONSTRAINT fk_protocol_converters_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_protocol_converters_tenant_id 
  ON protocol_converters(tenant_id);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_device_id 
  ON protocol_converters(device_id);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_uuid 
  ON protocol_converters(uuid);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_tenant_device 
  ON protocol_converters(tenant_id, device_id);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_device_status 
  ON protocol_converters(device_id, deployment_status);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_type 
  ON protocol_converters(device_id, type);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_created_at 
  ON protocol_converters(created_at);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_last_synced 
  ON protocol_converters(last_synced);

-- Data Models table: Stores data model definitions with versioning
-- DataModels use Name+Version as compound key (unlike Protocol Converters which use UUID)
CREATE TABLE IF NOT EXISTS data_models (
  id SERIAL PRIMARY KEY,
  tenant_id UUID NOT NULL,  -- Multi-tenancy isolation
  device_id TEXT NOT NULL,
  name TEXT NOT NULL,
  version TEXT NOT NULL,  -- String representation of the integer version (e.g., "1", "2", "3")
  description TEXT,
  encoded_structure TEXT,  -- Base64 encoded YAML structure
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  
  -- Constraints
  CONSTRAINT unique_tenant_device_model_version UNIQUE (tenant_id, device_id, name, version),
  CONSTRAINT fk_data_models_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_data_models_tenant_id 
  ON data_models(tenant_id);

CREATE INDEX IF NOT EXISTS idx_data_models_device_id 
  ON data_models(device_id);

CREATE INDEX IF NOT EXISTS idx_data_models_tenant_device 
  ON data_models(tenant_id, device_id);

CREATE INDEX IF NOT EXISTS idx_data_models_name 
  ON data_models(device_id, name);

CREATE INDEX IF NOT EXISTS idx_data_models_version 
  ON data_models(device_id, name, version);

CREATE INDEX IF NOT EXISTS idx_data_models_created_at 
  ON data_models(created_at);

-- Stream Processors table: Stores stream processor configurations
-- StreamProcessors use UUID as primary identifier (similar to Protocol Converters)
CREATE TABLE IF NOT EXISTS stream_processors (
  id TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,  -- Multi-tenancy isolation
  device_id TEXT NOT NULL,
  uuid TEXT NOT NULL,
  name TEXT NOT NULL,
  model_name TEXT,              -- Data model name (from StreamProcessorModelRef)
  model_version TEXT,           -- Data model version
  encoded_config TEXT,          -- Base64-encoded YAML configuration
  location_json TEXT,           -- JSON map of location levels
  ignore_health_check BOOLEAN DEFAULT false,
  metadata_json TEXT,           -- JSON map of metadata
  
  -- Status tracking
  deployment_status TEXT DEFAULT 'PENDING',  -- PENDING, ACTIVE, FAILED
  health_status TEXT DEFAULT 'UNKNOWN',      -- ONLINE, OFFLINE, UNKNOWN
  error_message TEXT,
  last_synced TIMESTAMP WITH TIME ZONE,     -- From latest StatusMessage
  
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  
  -- Constraints
  CONSTRAINT unique_tenant_device_stream_processor UNIQUE (tenant_id, device_id, uuid),
  CONSTRAINT fk_stream_processors_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_stream_processors_tenant_id 
  ON stream_processors(tenant_id);

CREATE INDEX IF NOT EXISTS idx_stream_processors_device_id 
  ON stream_processors(device_id);

CREATE INDEX IF NOT EXISTS idx_stream_processors_uuid 
  ON stream_processors(uuid);

CREATE INDEX IF NOT EXISTS idx_stream_processors_tenant_device 
  ON stream_processors(tenant_id, device_id);

CREATE INDEX IF NOT EXISTS idx_stream_processors_device_created_at 
  ON stream_processors(device_id, created_at);

CREATE INDEX IF NOT EXISTS idx_stream_processors_deployment_status 
  ON stream_processors(device_id, deployment_status);

CREATE INDEX IF NOT EXISTS idx_stream_processors_last_synced 
  ON stream_processors(last_synced);

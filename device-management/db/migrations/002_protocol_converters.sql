-- Migration: Add protocol_converters table
-- Created: Phase 1 Implementation

CREATE TABLE IF NOT EXISTS protocol_converters (
  id TEXT PRIMARY KEY,
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
  last_synced TEXT,  -- From latest StatusMessage
  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
  
  -- Constraints
  UNIQUE(device_id, uuid),
  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_protocol_converters_device_id 
  ON protocol_converters(device_id);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_uuid 
  ON protocol_converters(uuid);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_device_status 
  ON protocol_converters(device_id, deployment_status);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_type 
  ON protocol_converters(device_id, type);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_created_at 
  ON protocol_converters(created_at);

CREATE INDEX IF NOT EXISTS idx_protocol_converters_last_synced 
  ON protocol_converters(last_synced);

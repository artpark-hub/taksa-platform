-- Add Stream Processors table for SQLite
-- StreamProcessors use UUID as primary identifier (similar to Protocol Converters)

CREATE TABLE IF NOT EXISTS stream_processors (
  id TEXT PRIMARY KEY,
  device_id TEXT NOT NULL,
  uuid TEXT NOT NULL,
  name TEXT NOT NULL,
  model_name TEXT,              -- Data model name (from StreamProcessorModelRef)
  model_version TEXT,           -- Data model version
  encoded_config TEXT,          -- Base64-encoded YAML configuration
  location_json TEXT,           -- JSON map of location levels
  ignore_health_check INTEGER DEFAULT 0,
  metadata_json TEXT,           -- JSON map of metadata
  created_at TEXT DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
  
  UNIQUE(device_id, uuid),
  FOREIGN KEY(device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_stream_processors_device_id 
  ON stream_processors(device_id);

CREATE INDEX IF NOT EXISTS idx_stream_processors_uuid 
  ON stream_processors(uuid);

CREATE INDEX IF NOT EXISTS idx_stream_processors_device_created_at 
  ON stream_processors(device_id, created_at);

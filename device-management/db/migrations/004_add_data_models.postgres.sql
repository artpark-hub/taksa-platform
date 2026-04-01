-- Add Data Models table for PostgreSQL
-- DataModels use Name+Version as compound key

CREATE TABLE IF NOT EXISTS data_models (
  id SERIAL PRIMARY KEY,
  device_id TEXT NOT NULL,
  name TEXT NOT NULL,
  version TEXT NOT NULL,
  description TEXT,
  encoded_structure TEXT,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  
  CONSTRAINT unique_device_model_version UNIQUE (device_id, name, version),
  CONSTRAINT fk_data_models_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_data_models_device_id ON data_models(device_id);
CREATE INDEX IF NOT EXISTS idx_data_models_name ON data_models(device_id, name);
CREATE INDEX IF NOT EXISTS idx_data_models_version ON data_models(device_id, name, version);
CREATE INDEX IF NOT EXISTS idx_data_models_created_at ON data_models(created_at);

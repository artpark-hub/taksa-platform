-- Migration 001: UNS topic browser materialization tables
-- Apply to existing databases created before device_topics were added to schema.postgres.sql
-- Idempotent: safe to run multiple times

CREATE TABLE IF NOT EXISTS device_topics (
  id TEXT PRIMARY KEY,
  tenant_id UUID NOT NULL,
  device_id TEXT NOT NULL,
  uns_tree_id TEXT NOT NULL,
  canonical_topic TEXT NOT NULL,
  level0 TEXT NOT NULL DEFAULT '',
  location_sublevels JSONB NOT NULL DEFAULT '[]'::jsonb,
  data_contract TEXT NOT NULL DEFAULT '',
  virtual_path TEXT,
  name TEXT NOT NULL DEFAULT '',
  metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  last_event_json JSONB,
  last_event_at TIMESTAMP WITH TIME ZONE,
  last_synced TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT unique_tenant_device_topic UNIQUE (tenant_id, device_id, uns_tree_id)
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'fk_device_topics_device'
  ) THEN
    ALTER TABLE device_topics
      ADD CONSTRAINT fk_device_topics_device
      FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_device_topics_tenant_id ON device_topics(tenant_id);
CREATE INDEX IF NOT EXISTS idx_device_topics_device_id ON device_topics(device_id);
CREATE INDEX IF NOT EXISTS idx_device_topics_tenant_device ON device_topics(tenant_id, device_id);
CREATE INDEX IF NOT EXISTS idx_device_topics_canonical ON device_topics(device_id, canonical_topic);
CREATE INDEX IF NOT EXISTS idx_device_topics_updated ON device_topics(device_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_device_topics_metadata_gin ON device_topics USING gin (metadata_json);
CREATE INDEX IF NOT EXISTS idx_device_topics_canonical_prefix
  ON device_topics (tenant_id, device_id, canonical_topic varchar_pattern_ops);

CREATE TABLE IF NOT EXISTS device_topic_catalog (
  tenant_id UUID NOT NULL,
  device_id TEXT NOT NULL,
  last_synced_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
  reported_topic_count INT NOT NULL DEFAULT -1,
  materialized_topic_count INT NOT NULL DEFAULT 0,
  last_sync_mode TEXT NOT NULL DEFAULT 'INCREMENTAL',
  last_full_replace_at TIMESTAMP WITH TIME ZONE,
  last_had_bundle_zero BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (tenant_id, device_id)
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'fk_device_topic_catalog_device'
  ) THEN
    ALTER TABLE device_topic_catalog
      ADD CONSTRAINT fk_device_topic_catalog_device
      FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_device_topic_catalog_device ON device_topic_catalog(device_id);

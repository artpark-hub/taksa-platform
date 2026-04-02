-- Migration: Add created_by field, remove serial_number, and ensure name is unique per tenant
-- Purpose: Track device ownership (tenant), enable multi-tenant device management

BEGIN TRANSACTION;

-- Add created_by column
ALTER TABLE devices ADD COLUMN created_by TEXT;

-- Add index for created_by lookups
CREATE INDEX IF NOT EXISTS idx_devices_created_by ON devices(created_by);

-- Drop serial_number index if it exists
DROP INDEX IF EXISTS idx_devices_serial_number;

-- Add unique constraint on (created_by, name) for per-tenant uniqueness
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_device_per_tenant ON devices(created_by, name);

COMMIT;

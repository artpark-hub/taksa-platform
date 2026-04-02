-- Migration: Add created_by field, remove serial_number, and ensure name is unique per tenant
-- Purpose: Track device ownership (tenant), enable multi-tenant device management

BEGIN TRANSACTION;

-- Add created_by column
ALTER TABLE devices ADD COLUMN created_by TEXT;

-- Add index for created_by lookups
CREATE INDEX IF NOT EXISTS idx_devices_created_by ON devices(created_by);

-- Drop serial_number index if it exists
DROP INDEX IF EXISTS idx_devices_serial_number;

-- Drop serial_number column
ALTER TABLE devices DROP COLUMN IF EXISTS serial_number;

-- Drop existing UNIQUE(serial_number) constraint if it exists
ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_serial_number_key;

-- Drop old global unique constraint on name
ALTER TABLE devices DROP CONSTRAINT IF EXISTS unique_device_name;

-- Add unique constraint on (created_by, name) for per-tenant uniqueness
ALTER TABLE devices ADD CONSTRAINT unique_device_per_tenant UNIQUE (created_by, name);

COMMIT;

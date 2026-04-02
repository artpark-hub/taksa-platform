-- Migration: Add created_by field, remove serial_number, and ensure name is globally unique
-- Purpose: Track device ownership (tenant), remove deprecated serial_number field, ensure name uniqueness globally

BEGIN TRANSACTION;

-- Add created_by column
ALTER TABLE devices ADD COLUMN created_by TEXT;

-- Add index for created_by lookups
CREATE INDEX IF NOT EXISTS idx_devices_created_by ON devices(created_by);

-- Drop serial_number index if it exists
DROP INDEX IF EXISTS idx_devices_serial_number;

-- Drop serial_number column
ALTER TABLE devices DROP COLUMN IF EXISTS serial_number;

-- Drop existing UNIQUE(serial_number) constraint if it exists by dropping and re-adding the constraint
ALTER TABLE devices DROP CONSTRAINT IF EXISTS devices_serial_number_key;

-- Add unique constraint on name (globally unique)
ALTER TABLE devices ADD CONSTRAINT unique_device_name UNIQUE (name);

COMMIT;

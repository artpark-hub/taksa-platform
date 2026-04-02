-- Migration: Add created_by field, remove serial_number, and ensure name is globally unique
-- Purpose: Track device ownership (tenant), remove deprecated serial_number field, ensure name uniqueness globally

BEGIN TRANSACTION;

-- Add created_by column
ALTER TABLE devices ADD COLUMN created_by TEXT;

-- Add index for created_by lookups
CREATE INDEX IF NOT EXISTS idx_devices_created_by ON devices(created_by);

-- Drop serial_number index if it exists
DROP INDEX IF EXISTS idx_devices_serial_number;

-- Drop serial_number constraint if it exists (SQLite: drop via UNIQUE constraint)
-- Note: SQLite doesn't support dropping constraints directly, so we recreate the table without it
-- This is handled by the schema.sqlite3.sql which defines the table without serial_number

-- Add unique constraint on name (globally unique)
CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_device_name ON devices(name);

COMMIT;

-- Migration: Remove company and license information from devices table
-- Purpose: Simplify device management - company info is external to device management

BEGIN TRANSACTION;

-- Drop company-related columns
ALTER TABLE devices DROP COLUMN IF EXISTS company_name;
ALTER TABLE devices DROP COLUMN IF EXISTS company_contact_email;
ALTER TABLE devices DROP COLUMN IF EXISTS company_support_contact;
ALTER TABLE devices DROP COLUMN IF EXISTS company_tags;
ALTER TABLE devices DROP COLUMN IF EXISTS user_count;

-- Drop license-related columns
ALTER TABLE devices DROP COLUMN IF EXISTS license_is_active;
ALTER TABLE devices DROP COLUMN IF EXISTS license_valid_to;
ALTER TABLE devices DROP COLUMN IF EXISTS license_description;

-- Drop company certificate column
ALTER TABLE devices DROP COLUMN IF EXISTS company_certificate;

-- Drop company-based index
DROP INDEX IF EXISTS idx_devices_company;

COMMIT;

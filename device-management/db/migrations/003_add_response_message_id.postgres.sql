-- Migration: Add response_message_id for O(1) message correlation lookup (PostgreSQL)
-- Adds direct pointer to response message in action_message_tracking table
-- Fixes GetDeviceConfig large config scaling issue

-- PostgreSQL version - supports IF NOT EXISTS for columns
ALTER TABLE action_message_tracking
ADD COLUMN IF NOT EXISTS response_message_id TEXT;

-- No index needed on response_message_id since lookups are via action_id
-- which already has an index, and we're just reading the stored ID

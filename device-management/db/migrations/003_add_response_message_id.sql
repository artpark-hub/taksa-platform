-- Migration: Add response_message_id for O(1) message correlation lookup
-- Adds direct pointer to response message in action_message_tracking table
-- Fixes GetDeviceConfig large config scaling issue (issue: message was pushed beyond 100-message search window)

-- SQLite version
-- ALTER TABLE ADD COLUMN ... IF NOT EXISTS is not supported in SQLite
-- Errors from existing columns are expected and safe to ignore

ALTER TABLE action_message_tracking ADD COLUMN response_message_id TEXT;

-- No index needed on response_message_id since lookups are via action_id
-- which already has an index, and we're just reading the stored ID

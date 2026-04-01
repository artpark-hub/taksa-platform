-- Migration: Add error_message column to actions table
-- Purpose: Store error details when actions fail

ALTER TABLE actions ADD COLUMN error_message TEXT;

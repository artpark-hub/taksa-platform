-- Migration: Add error_message column to actions table (PostgreSQL)
-- Purpose: Store error details when actions fail

ALTER TABLE actions ADD COLUMN IF NOT EXISTS error_message TEXT;

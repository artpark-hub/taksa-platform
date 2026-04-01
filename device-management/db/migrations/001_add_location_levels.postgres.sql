-- Migration: Add 7-level location support to devices table (PostgreSQL)
-- Extends location hierarchy from 4 levels (company, plant, area, zone) 
-- to 7 levels (company, plant, area, zone, line, workCell, workUnit)
-- 
-- PostgreSQL version - supports IF NOT EXISTS for columns

ALTER TABLE devices ADD COLUMN IF NOT EXISTS location_line TEXT;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS location_work_cell TEXT;
ALTER TABLE devices ADD COLUMN IF NOT EXISTS location_work_unit TEXT;

-- Migration: Add 7-level location support to devices table
-- Extends location hierarchy from 4 levels (company, plant, area, zone) 
-- to 7 levels (company, plant, area, zone, line, workCell, workUnit)
-- 
-- SQLite version - no IF NOT EXISTS support for ALTER TABLE
-- Errors from existing columns are expected and safe to ignore

ALTER TABLE devices ADD COLUMN location_line TEXT;
ALTER TABLE devices ADD COLUMN location_work_cell TEXT;
ALTER TABLE devices ADD COLUMN location_work_unit TEXT;

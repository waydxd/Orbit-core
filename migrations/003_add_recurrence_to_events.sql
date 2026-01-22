-- Migration: 003_add_recurrence_to_events.sql
-- Description: Add recurrence fields to events table

ALTER TABLE events
ADD COLUMN is_recurring BOOLEAN DEFAULT FALSE,
ADD COLUMN recurrence_rule TEXT,
ADD COLUMN recurrence_exception TEXT;


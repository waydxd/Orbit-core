-- Migration: 004_add_index_on_events_is_recurring.sql
-- Description: Add indexes to improve event listing performance

-- Index for non-recurring events, optimized for user and time window
CREATE INDEX idx_events_user_time ON events (user_id, start_time, end_time) WHERE is_recurring = false;

-- Index for recurring events, optimized for user and series start
CREATE INDEX idx_events_user_recurring ON events (user_id, start_time) WHERE is_recurring = true;


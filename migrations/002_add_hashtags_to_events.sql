-- Migration: 002_add_hashtags_to_events.sql
-- Description: Add hashtags column to events table

-- Add hashtags column to events table
ALTER TABLE events
ADD COLUMN IF NOT EXISTS hashtags TEXT[] DEFAULT '{}';

-- Create GIN index on hashtags for efficient array searching
CREATE INDEX IF NOT EXISTS idx_events_hashtags ON events USING GIN(hashtags);

-- Comment on column
COMMENT ON COLUMN events.hashtags IS 'Array of hashtag strings for categorizing and searching events';


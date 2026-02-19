-- Migration: 003_habit_tracking.sql
-- Description: Create tables for habit tracking feature
-- This feature tracks recurring events and suggests auto-scheduling when patterns are detected

-- Create event_frequency table to track recurring event patterns
CREATE TABLE IF NOT EXISTS event_frequency (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Event signature fields for matching similar events
    title VARCHAR(255) NOT NULL,
    description TEXT,
    location VARCHAR(255),
    -- Duration in minutes (calculated from end_time - start_time)
    duration_minutes INTEGER NOT NULL,
    -- Time of day when event typically occurs (stored as minutes from midnight)
    time_of_day INTEGER NOT NULL,
    -- Day of week (0-6, Sunday = 0) for weekly patterns
    day_of_week INTEGER NOT NULL CHECK (day_of_week >= 0 AND day_of_week <= 6),
    -- Count of how many times this pattern has occurred
    occurrence_count INTEGER NOT NULL DEFAULT 1,
    -- Threshold to trigger suggestion (default 3)
    suggestion_threshold INTEGER NOT NULL DEFAULT 3,
    -- Whether a suggestion has been shown for this pattern
    suggestion_shown BOOLEAN NOT NULL DEFAULT FALSE,
    -- Whether the user accepted the habit suggestion
    habit_accepted BOOLEAN NOT NULL DEFAULT FALSE,
    -- Timestamps of the last few occurrences (as JSON array)
    occurrence_timestamps JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    -- Unique constraint to ensure one record per unique event pattern per user
    UNIQUE(user_id, title, duration_minutes, time_of_day, day_of_week)
);

CREATE INDEX IF NOT EXISTS idx_event_frequency_user_id ON event_frequency(user_id);
CREATE INDEX IF NOT EXISTS idx_event_frequency_occurrence_count ON event_frequency(occurrence_count);
CREATE INDEX IF NOT EXISTS idx_event_frequency_suggestion_shown ON event_frequency(suggestion_shown);

-- Create habit_suggestions table to track pending habit suggestions
CREATE TABLE IF NOT EXISTS habit_suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_frequency_id UUID NOT NULL REFERENCES event_frequency(id) ON DELETE CASCADE,
    -- Suggested event details
    title VARCHAR(255) NOT NULL,
    description TEXT,
    location VARCHAR(255),
    duration_minutes INTEGER NOT NULL,
    time_of_day INTEGER NOT NULL,
    day_of_week INTEGER NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    -- Status: pending, accepted, rejected, expired
    status VARCHAR(20) CHECK (status IN ('pending', 'accepted', 'rejected', 'expired')) DEFAULT 'pending',
    -- When accepted, store the end date for recurring events (5 years from acceptance)
    recurrence_end_date TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE DEFAULT (CURRENT_TIMESTAMP + INTERVAL '7 days')
);

CREATE INDEX IF NOT EXISTS idx_habit_suggestions_user_id ON habit_suggestions(user_id);
CREATE INDEX IF NOT EXISTS idx_habit_suggestions_status ON habit_suggestions(status);
CREATE INDEX IF NOT EXISTS idx_habit_suggestions_event_frequency_id ON habit_suggestions(event_frequency_id);

-- Create recurring_events table to store accepted recurring events
CREATE TABLE IF NOT EXISTS recurring_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    habit_suggestion_id UUID REFERENCES habit_suggestions(id) ON DELETE SET NULL,
    -- Event template details
    title VARCHAR(255) NOT NULL,
    description TEXT,
    location VARCHAR(255),
    duration_minutes INTEGER NOT NULL,
    time_of_day INTEGER NOT NULL,
    day_of_week INTEGER NOT NULL,
    -- Recurrence settings
    start_date TIMESTAMP WITH TIME ZONE NOT NULL,
    end_date TIMESTAMP WITH TIME ZONE NOT NULL,
    -- Whether this recurring event is active
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_recurring_events_user_id ON recurring_events(user_id);
CREATE INDEX IF NOT EXISTS idx_recurring_events_is_active ON recurring_events(is_active);

-- Create trigger for updated_at columns
DROP TRIGGER IF EXISTS update_event_frequency_updated_at ON event_frequency;
CREATE TRIGGER update_event_frequency_updated_at BEFORE UPDATE ON event_frequency
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_habit_suggestions_updated_at ON habit_suggestions;
CREATE TRIGGER update_habit_suggestions_updated_at BEFORE UPDATE ON habit_suggestions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_recurring_events_updated_at ON recurring_events;
CREATE TRIGGER update_recurring_events_updated_at BEFORE UPDATE ON recurring_events
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();


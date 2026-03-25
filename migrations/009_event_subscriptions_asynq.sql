-- Migration: 009_event_subscriptions_asynq.sql
-- Description: Extend event_subscriptions to support Asynq task-queue based scheduling

-- Add job_id column to store the Asynq Task ID (used for cancellation)
ALTER TABLE event_subscriptions ADD COLUMN IF NOT EXISTS job_id VARCHAR(512);

-- Add status column to replace the boolean is_sent with a richer enum.
-- All rows (new and existing) default to 'pending'; sent rows are backfilled below.
ALTER TABLE event_subscriptions ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'pending';
ALTER TABLE event_subscriptions ADD CONSTRAINT chk_event_subscriptions_status
    CHECK (status IN ('pending', 'sent', 'cancelled', 'failed'));

-- Backfill status for existing rows that were already marked sent under the old schema.
-- At migration time all pre-existing rows will have the new column default ('pending'),
-- so this UPDATE simply promotes the ones where is_sent = true to 'sent'.
UPDATE event_subscriptions SET status = 'sent' WHERE is_sent = true;

-- Index job_id for fast lookup during cancellation
CREATE INDEX IF NOT EXISTS idx_event_subscriptions_job_id ON event_subscriptions(job_id) WHERE job_id IS NOT NULL;

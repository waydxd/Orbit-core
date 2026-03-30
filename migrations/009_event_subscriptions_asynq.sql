-- Migration: 009_event_subscriptions_asynq.sql
-- Description: Extend event_subscriptions to support Asynq task-queue based scheduling

-- Add job_id column to store the Asynq Task ID (used for cancellation)
ALTER TABLE event_subscriptions ADD COLUMN IF NOT EXISTS job_id VARCHAR(512);

-- Add status column to replace the boolean is_sent with a richer enum.
-- All rows (new and existing) default to 'pending'; sent rows are backfilled below.
ALTER TABLE event_subscriptions ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'pending';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint c
        JOIN pg_class t ON t.oid = c.conrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE c.conname = 'chk_event_subscriptions_status'
          AND t.relname = 'event_subscriptions'
    ) THEN
        ALTER TABLE event_subscriptions
            ADD CONSTRAINT chk_event_subscriptions_status
            CHECK (status IN ('pending', 'sent', 'cancelled', 'failed'));
    END IF;
END
$$;

-- Backfill status for existing rows that were already marked sent under the old schema.
-- At migration time all pre-existing rows will have the new column default ('pending'),
-- so this UPDATE simply promotes the ones where is_sent = true to 'sent'.
UPDATE event_subscriptions SET status = 'sent' WHERE is_sent = true;

-- Index job_id for fast lookup during cancellation
CREATE INDEX IF NOT EXISTS idx_event_subscriptions_job_id ON event_subscriptions(job_id) WHERE job_id IS NOT NULL;

-- Keep one active subscription per user/event pair so lookup and cancellation remain deterministic.
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_subscriptions_user_event_active
    ON event_subscriptions(user_id, event_id)
    WHERE status NOT IN ('cancelled', 'sent', 'failed');
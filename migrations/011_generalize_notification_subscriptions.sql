-- Migration: 010_generalize_notification_subscriptions.sql
-- Description: Generalize event_subscriptions to support both event and task reminders.

-- 1. Drop the FK constraint on event_id so the column can reference tasks too.
ALTER TABLE event_subscriptions DROP CONSTRAINT IF EXISTS event_subscriptions_event_id_fkey;

-- 2. Add entity_type column (backfill existing rows as 'event').
ALTER TABLE event_subscriptions ADD COLUMN IF NOT EXISTS entity_type VARCHAR(20) NOT NULL DEFAULT 'event';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint c
        JOIN pg_class t ON t.oid = c.conrelid
        JOIN pg_namespace n ON n.oid = t.relnamespace
        WHERE c.conname = 'chk_entity_type'
          AND t.relname = 'event_subscriptions'
    ) THEN
        ALTER TABLE event_subscriptions
            ADD CONSTRAINT chk_entity_type
            CHECK (entity_type IN ('event', 'task'));
    END IF;
END
$$;

-- 3. Rename event_id -> entity_id.
ALTER TABLE event_subscriptions RENAME COLUMN event_id TO entity_id;

-- 4. Rebuild the unique partial index with entity_type included.
DROP INDEX IF EXISTS idx_event_subscriptions_user_event_active;
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_subscriptions_user_entity_active
    ON event_subscriptions(user_id, entity_id, entity_type)
    WHERE status NOT IN ('cancelled', 'sent', 'failed');

-- 5. Rebuild the entity lookup index as a composite (entity_id, entity_type) partial index.
DROP INDEX IF EXISTS idx_event_subscriptions_event_id;
CREATE INDEX IF NOT EXISTS idx_event_subscriptions_entity_id
    ON event_subscriptions(entity_id, entity_type)
    WHERE status NOT IN ('cancelled', 'sent', 'failed');

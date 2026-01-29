-- Migration: 005_add_user_profile_fields.sql
-- Description: Add user profile fields for personalization

-- Add username column (will be made NOT NULL after backfilling existing users)
ALTER TABLE users ADD COLUMN IF NOT EXISTS username VARCHAR(50);

-- Add profile picture URL
ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_picture TEXT;

-- Add region
ALTER TABLE users ADD COLUMN IF NOT EXISTS region VARCHAR(100);

-- Add timezone
ALTER TABLE users ADD COLUMN IF NOT EXISTS timezone VARCHAR(100);

-- Add gender with inclusive options
ALTER TABLE users ADD COLUMN IF NOT EXISTS gender VARCHAR(50);

-- Add birth date for age-based features and birthday reminders
ALTER TABLE users ADD COLUMN IF NOT EXISTS birth_date DATE;

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_birth_date ON users(birth_date);

-- Add unique constraint on username
-- Note: For production deployments with existing users, you should:
-- 1. Run this migration without the UNIQUE constraint and NOT NULL
-- 2. Backfill usernames for existing users using the application's username generator
-- 3. Then add the constraints with a separate migration:
--    ALTER TABLE users ALTER COLUMN username SET NOT NULL;
--    ALTER TABLE users ADD CONSTRAINT unique_username UNIQUE (username);
-- For now, we add the unique constraint to enforce it for new users
CREATE UNIQUE INDEX IF NOT EXISTS unique_username ON users(username) WHERE username IS NOT NULL;

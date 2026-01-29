-- Migration: 005_add_user_profile_fields.sql
-- Description: Add user profile fields for personalization

-- Add username column (non-null with default random generation handled in application)
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

-- Add unique constraint on username (nullable unique)
ALTER TABLE users ADD CONSTRAINT unique_username UNIQUE (username);

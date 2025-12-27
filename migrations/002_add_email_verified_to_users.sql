-- Migration: 002_add_email_verified_to_users.sql
-- Description: Add email_verified column to users table

ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_users_email_verified ON users(email_verified);


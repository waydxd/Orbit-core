-- Add image_url column to events to store API paths to event images
ALTER TABLE events ADD COLUMN IF NOT EXISTS image_url TEXT[];

-- Add profile_pic_url column to users to store API path to user avatar
ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_pic_url VARCHAR(500);

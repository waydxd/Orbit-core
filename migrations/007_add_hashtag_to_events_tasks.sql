-- Migration: 007_add_hashtag_to_events_tasks.sql
-- Description: Add hashtag field (array of strings) to events and tasks tables

ALTER TABLE events
ADD COLUMN hashtags TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE tasks
ADD COLUMN hashtags TEXT[] NOT NULL DEFAULT '{}';

-- Add username column to devices table
-- This stores the system username from device auth challenges for SSH host creation

ALTER TABLE devices ADD COLUMN IF NOT EXISTS username TEXT;

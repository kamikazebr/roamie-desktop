-- Add username column to device_auth_challenges table
-- This stores the system username from the device for SSH host creation

ALTER TABLE device_auth_challenges ADD COLUMN username TEXT;

-- Add OS type and hardware ID fields to device authorization challenges
ALTER TABLE device_auth_challenges
  ADD COLUMN IF NOT EXISTS os_type TEXT,
  ADD COLUMN IF NOT EXISTS hardware_id TEXT;

-- Add comment to explain the fields
COMMENT ON COLUMN device_auth_challenges.os_type IS 'Operating system type (linux, macos, windows, etc.)';
COMMENT ON COLUMN device_auth_challenges.hardware_id IS '8-character hardware identifier derived from machine-id or MAC address';

-- Add public_key and wg_device_id columns to device_auth_challenges table
-- This enables auto-registration of WireGuard devices after QR code approval

ALTER TABLE device_auth_challenges
ADD COLUMN IF NOT EXISTS public_key TEXT,
ADD COLUMN IF NOT EXISTS wg_device_id UUID REFERENCES devices(id) ON DELETE SET NULL;

-- Add index for faster lookups
CREATE INDEX IF NOT EXISTS idx_device_auth_challenges_wg_device_id ON device_auth_challenges(wg_device_id);

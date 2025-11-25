-- Migration 010: Add heartbeat and tunnel support
-- Adds device identification fields (parsed from device_name)
-- Adds heartbeat tracking (last_seen)
-- Adds SSH tunnel support (tunnel_port)

-- Add new structured fields
ALTER TABLE devices
ADD COLUMN IF NOT EXISTS hardware_id VARCHAR(8),
ADD COLUMN IF NOT EXISTS os_type VARCHAR(20),
ADD COLUMN IF NOT EXISTS display_name VARCHAR(100),
ADD COLUMN IF NOT EXISTS last_seen TIMESTAMP DEFAULT NOW(),
ADD COLUMN IF NOT EXISTS tunnel_port INTEGER;

-- Parse existing device_name format: "android-username-a1b2c3d4"
-- Separator is "-" (hyphen)
-- Format: <os>-<username>-<deviceid>
UPDATE devices
SET
  os_type = SPLIT_PART(device_name, '-', 1),
  hardware_id = SPLIT_PART(device_name, '-', 3)
WHERE hardware_id IS NULL OR hardware_id = '';

-- Create unique constraint to prevent duplicate devices
-- Same user + same hardware = same device
CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_user_hardware
ON devices(user_id, hardware_id) WHERE active = true;

-- Add constraint for unique tunnel ports
CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_tunnel_port_unique
ON devices(tunnel_port) WHERE tunnel_port IS NOT NULL;

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_devices_hardware_id ON devices(hardware_id);
CREATE INDEX IF NOT EXISTS idx_devices_os_type ON devices(os_type);
CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices(last_seen);
CREATE INDEX IF NOT EXISTS idx_devices_tunnel_port ON devices(tunnel_port) WHERE tunnel_port IS NOT NULL;

-- Add comments for documentation
COMMENT ON COLUMN devices.device_name IS 'DEPRECATED: Original device identifier in format "os-username-hwid". Use hardware_id + os_type instead.';
COMMENT ON COLUMN devices.hardware_id IS 'Hardware identifier (8-char hex) extracted from device_name. Unique per physical device.';
COMMENT ON COLUMN devices.os_type IS 'Operating system type: android, ios, linux, macos, windows';
COMMENT ON COLUMN devices.display_name IS 'Optional user-friendly device name';
COMMENT ON COLUMN devices.last_seen IS 'Last heartbeat timestamp. Device considered online if < 60s ago.';
COMMENT ON COLUMN devices.tunnel_port IS 'Allocated SSH reverse tunnel port (10000-20000 range). NULL if tunnel not active.';

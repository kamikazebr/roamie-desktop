-- Migration 011: Add SSH tunnel key storage and tunnel_enabled flag
-- This migration adds support for SSH reverse tunnels as an alternative to WireGuard VPN

-- Add tunnel SSH public key storage (used by tunnel server for authentication)
ALTER TABLE devices ADD COLUMN IF NOT EXISTS tunnel_ssh_key TEXT;

-- Add tunnel enabled flag (per-device control via Flutter app)
ALTER TABLE devices ADD COLUMN IF NOT EXISTS tunnel_enabled BOOLEAN DEFAULT false;

-- Create index for efficient lookup by SSH key
CREATE INDEX IF NOT EXISTS idx_devices_tunnel_ssh_key ON devices(tunnel_ssh_key) WHERE tunnel_ssh_key IS NOT NULL;

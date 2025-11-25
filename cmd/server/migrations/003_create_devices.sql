-- Create devices table
CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_name VARCHAR(100) NOT NULL,
    public_key VARCHAR(44) UNIQUE NOT NULL,
    vpn_ip INET UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    last_handshake TIMESTAMP,
    active BOOLEAN DEFAULT true,
    UNIQUE(user_id, device_name)
);

CREATE INDEX idx_devices_user ON devices(user_id);
CREATE INDEX idx_devices_active ON devices(active);
CREATE INDEX idx_devices_ip ON devices(vpn_ip);
CREATE INDEX idx_devices_public_key ON devices(public_key);

-- Add comments
COMMENT ON TABLE devices IS 'User devices with WireGuard configs';
COMMENT ON COLUMN devices.vpn_ip IS 'IP address within user subnet';
COMMENT ON COLUMN devices.public_key IS 'WireGuard public key (44 base64 chars)';

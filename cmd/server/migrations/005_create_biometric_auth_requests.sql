-- Create biometric_auth_requests table
CREATE TABLE IF NOT EXISTS biometric_auth_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id UUID REFERENCES devices(id) ON DELETE SET NULL,
    username VARCHAR(100) NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    command TEXT NOT NULL,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'denied', 'expired', 'timeout')),
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    responded_at TIMESTAMP,
    response TEXT,
    ip_address INET
);

CREATE INDEX idx_biometric_auth_user ON biometric_auth_requests(user_id);
CREATE INDEX idx_biometric_auth_status ON biometric_auth_requests(status, expires_at);
CREATE INDEX idx_biometric_auth_created ON biometric_auth_requests(created_at);
CREATE INDEX idx_biometric_auth_device ON biometric_auth_requests(device_id);

-- Add comments
COMMENT ON TABLE biometric_auth_requests IS 'Biometric authentication requests from Linux systems';
COMMENT ON COLUMN biometric_auth_requests.username IS 'Linux system username requesting authentication';
COMMENT ON COLUMN biometric_auth_requests.hostname IS 'Linux system hostname';
COMMENT ON COLUMN biometric_auth_requests.command IS 'Command being authorized (e.g., sudo, ssh)';
COMMENT ON COLUMN biometric_auth_requests.status IS 'Request status: pending, approved, denied, expired, timeout';
COMMENT ON COLUMN biometric_auth_requests.expires_at IS 'Request expiration time (typically 30 seconds from creation)';
COMMENT ON COLUMN biometric_auth_requests.device_id IS 'Device making the request (optional, can be NULL for unregistered devices)';

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    subnet CIDR UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    max_devices INT DEFAULT 5,
    active BOOLEAN DEFAULT true
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_subnet ON users(subnet);
CREATE INDEX idx_users_active ON users(active);

-- Add comments
COMMENT ON TABLE users IS 'VPN users with dedicated subnets';
COMMENT ON COLUMN users.subnet IS 'Dedicated /29 subnet for user devices (e.g., 10.100.0.0/29)';

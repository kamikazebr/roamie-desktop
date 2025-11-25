-- Create auth_codes table
CREATE TABLE IF NOT EXISTS auth_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    code VARCHAR(6) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    used BOOLEAN DEFAULT false
);

CREATE INDEX idx_auth_codes_email ON auth_codes(email);
CREATE INDEX idx_auth_codes_expires ON auth_codes(expires_at);
CREATE INDEX idx_auth_codes_used ON auth_codes(used);

-- Add comments
COMMENT ON TABLE auth_codes IS 'Email authentication codes (6-digit, 5min expiration)';
COMMENT ON COLUMN auth_codes.code IS '6-digit numeric code sent via email';

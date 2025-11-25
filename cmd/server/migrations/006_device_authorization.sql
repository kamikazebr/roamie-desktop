-- Device authorization challenges (temporary, 5 min TTL)
CREATE TABLE device_auth_challenges (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  device_id UUID NOT NULL,
  hostname TEXT NOT NULL,
  ip_address INET NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending', -- pending, approved, denied, expired
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMP DEFAULT NOW(),
  expires_at TIMESTAMP NOT NULL,
  approved_at TIMESTAMP
);

CREATE INDEX idx_device_auth_status ON device_auth_challenges(status, expires_at);
CREATE INDEX idx_device_auth_user ON device_auth_challenges(user_id);

-- Refresh tokens (long-lived, 1 year)
CREATE TABLE refresh_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  device_id UUID NOT NULL,
  token TEXT UNIQUE NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  created_at TIMESTAMP DEFAULT NOW(),
  last_used_at TIMESTAMP
);

CREATE INDEX idx_refresh_tokens_token ON refresh_tokens(token);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);

-- Add firebase_uid to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS firebase_uid TEXT UNIQUE;
CREATE INDEX IF NOT EXISTS idx_users_firebase_uid ON users(firebase_uid);

-- Create network_conflicts table
CREATE TABLE IF NOT EXISTS network_conflicts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cidr CIDR NOT NULL,
    source VARCHAR(50) NOT NULL,
    description TEXT,
    detected_at TIMESTAMP DEFAULT NOW(),
    active BOOLEAN DEFAULT true
);

CREATE INDEX idx_conflicts_cidr ON network_conflicts(cidr);
CREATE INDEX idx_conflicts_active ON network_conflicts(active);
CREATE INDEX idx_conflicts_source ON network_conflicts(source);

-- Add comments
COMMENT ON TABLE network_conflicts IS 'Detected network CIDR conflicts (Docker, VPNs, etc)';
COMMENT ON COLUMN network_conflicts.source IS 'Source of conflict: docker, system, vpn, manual';

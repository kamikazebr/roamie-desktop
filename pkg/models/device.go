package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Device struct {
	ID     uuid.UUID `json:"id" db:"id"`
	UserID uuid.UUID `json:"user_id" db:"user_id"`

	// Maintained for backward compatibility with Flutter app
	// Format: "android-username-a1b2c3d4"
	DeviceName string `json:"device_name" db:"device_name"`

	// Structured fields (auto-parsed from device_name)
	HardwareID  string  `json:"hardware_id" db:"hardware_id"`             // "a1b2c3d4"
	OSType      string  `json:"os_type" db:"os_type"`                     // "android", "ios", "linux", etc
	DisplayName *string `json:"display_name,omitempty" db:"display_name"` // Optional user-friendly name

	// WireGuard fields
	PublicKey string  `json:"public_key" db:"public_key"`
	VpnIP     string  `json:"vpn_ip" db:"vpn_ip"`
	Username  *string `json:"username,omitempty" db:"username"`

	// Heartbeat tracking
	LastSeen time.Time `json:"last_seen" db:"last_seen"`
	IsOnline bool      `json:"is_online" db:"-"` // Calculated field, not stored in DB

	// SSH Tunnel fields
	TunnelPort    *int    `json:"tunnel_port,omitempty" db:"tunnel_port"`       // Allocated port 10000-20000
	TunnelSSHKey  *string `json:"tunnel_ssh_key,omitempty" db:"tunnel_ssh_key"` // SSH public key for tunnel auth
	TunnelEnabled bool    `json:"tunnel_enabled" db:"tunnel_enabled"`           // Per-device tunnel control

	// Metadata
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	LastHandshake *time.Time `json:"last_handshake,omitempty" db:"last_handshake"`
	Active        bool       `json:"active" db:"active"`
}

// ParseDeviceName extracts os_type and hardware_id from device_name
// Expected format: "android-username-a1b2c3d4"
// Separator: "-" (hyphen)
func (d *Device) ParseDeviceName() {
	parts := strings.Split(d.DeviceName, "-")
	if len(parts) >= 3 {
		d.OSType = parts[0]
		d.HardwareID = parts[2]
	}
}

type NetworkConflict struct {
	ID          uuid.UUID `json:"id" db:"id"`
	CIDR        string    `json:"cidr" db:"cidr"`
	Source      string    `json:"source" db:"source"`
	Description string    `json:"description,omitempty" db:"description"`
	DetectedAt  time.Time `json:"detected_at" db:"detected_at"`
	Active      bool      `json:"active" db:"active"`
}

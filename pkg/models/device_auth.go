package models

import (
	"time"

	"github.com/google/uuid"
)

// DeviceAuthChallenge represents a pending device authorization request
type DeviceAuthChallenge struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	DeviceID   uuid.UUID  `json:"device_id" db:"device_id"`
	Hostname   string     `json:"hostname" db:"hostname"`
	IPAddress  string     `json:"ip_address" db:"ip_address"`
	Username   *string    `json:"username,omitempty" db:"username"`     // System username for SSH host creation
	PublicKey  *string    `json:"public_key,omitempty" db:"public_key"` // WireGuard public key for auto-registration
	OSType     *string    `json:"os_type,omitempty" db:"os_type"`       // OS type (linux, macos, windows, etc.)
	HardwareID *string    `json:"hardware_id,omitempty" db:"hardware_id"` // 8-char hardware identifier
	Status     string     `json:"status" db:"status"`                   // pending, approved, denied, expired
	UserID     *uuid.UUID `json:"user_id,omitempty" db:"user_id"`
	WgDeviceID *uuid.UUID `json:"wg_device_id,omitempty" db:"wg_device_id"` // ID of registered WireGuard device after approval
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	ApprovedAt *time.Time `json:"approved_at,omitempty" db:"approved_at"`
}

// RefreshToken represents a long-lived refresh token for device authentication
type RefreshToken struct {
	ID         uuid.UUID  `json:"id" db:"id"`
	UserID     uuid.UUID  `json:"user_id" db:"user_id"`
	DeviceID   uuid.UUID  `json:"device_id" db:"device_id"`
	Token      string     `json:"token" db:"token"`
	ExpiresAt  time.Time  `json:"expires_at" db:"expires_at"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
}

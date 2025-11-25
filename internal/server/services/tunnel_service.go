package services

import (
	"context"
	"fmt"

	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/google/uuid"
)

// TunnelService handles tunnel-related business logic
type TunnelService struct {
	deviceRepo *storage.DeviceRepository
}

// NewTunnelService creates a new tunnel service
func NewTunnelService(deviceRepo *storage.DeviceRepository) *TunnelService {
	return &TunnelService{
		deviceRepo: deviceRepo,
	}
}

// AuthorizedTunnelKey represents an SSH key authorized for tunnel access
type AuthorizedTunnelKey struct {
	DeviceID   uuid.UUID `json:"device_id"`
	DeviceName string    `json:"device_name"`
	PublicKey  string    `json:"public_key"`
	Comment    string    `json:"comment"`
}

// GetAuthorizedTunnelKeys returns all tunnel SSH keys for devices in the same user account
// This is used by clients to populate their ~/.ssh/authorized_keys file
func (s *TunnelService) GetAuthorizedTunnelKeys(ctx context.Context, userID uuid.UUID) ([]AuthorizedTunnelKey, error) {
	// Get all devices for this user
	devices, err := s.deviceRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get devices: %w", err)
	}

	var keys []AuthorizedTunnelKey
	for _, device := range devices {
		// Skip devices without tunnel SSH keys
		if device.TunnelSSHKey == nil || *device.TunnelSSHKey == "" {
			continue
		}

		// Skip inactive or tunnel-disabled devices
		if !device.Active || !device.TunnelEnabled {
			continue
		}

		// Add key with metadata
		keys = append(keys, AuthorizedTunnelKey{
			DeviceID:   device.ID,
			DeviceName: device.DeviceName,
			PublicKey:  *device.TunnelSSHKey,
			Comment:    fmt.Sprintf("Roamie Tunnel Access - %s (%s)", device.DeviceName, device.ID),
		})
	}

	return keys, nil
}

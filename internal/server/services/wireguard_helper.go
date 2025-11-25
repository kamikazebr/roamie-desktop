package services

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
)

// WireGuardManager interface for WireGuard operations
type WireGuardManager interface {
	AddPeer(publicKey, vpnIP string) error
	RemovePeer(publicKey string) error
}

// DeviceRepository interface for device operations
type DeviceRepository interface {
	Delete(ctx context.Context, id uuid.UUID) error
}

// AddDeviceToWireGuard handles adding a device peer to WireGuard interface.
// It handles device replacement (removing old peer) and optionally rolls back
// the device registration if WireGuard configuration fails.
//
// Parameters:
//   - ctx: Context for database operations
//   - wgManager: WireGuard manager instance
//   - deviceRepo: Device repository for rollback (can be nil if rollbackOnError is false)
//   - result: Device registration result from DeviceService.RegisterDevice()
//   - rollbackOnError: If true, deletes device from DB when WireGuard fails
//
// Returns:
//   - error: nil on success, descriptive error on failure
func AddDeviceToWireGuard(
	ctx context.Context,
	wgManager WireGuardManager,
	deviceRepo DeviceRepository,
	result *DeviceRegistrationResult,
	rollbackOnError bool,
) error {
	// Handle device replacement - remove old peer from WireGuard
	if result.WasReplaced && result.ReplacedDevice != nil {
		log.Printf("Removing old peer from WireGuard: %s", result.ReplacedDevice.PublicKey)
		if err := wgManager.RemovePeer(result.ReplacedDevice.PublicKey); err != nil {
			// Log warning but continue - old peer might not exist in WireGuard
			log.Printf("Warning: Failed to remove old peer: %v", err)
		}
	}

	// Add new peer to WireGuard
	if err := wgManager.AddPeer(result.Device.PublicKey, result.Device.VpnIP); err != nil {
		// WireGuard configuration failed
		if rollbackOnError && deviceRepo != nil {
			// Attempt to rollback device registration
			if deleteErr := deviceRepo.Delete(ctx, result.Device.ID); deleteErr != nil {
				return fmt.Errorf("WireGuard setup failed and rollback failed: %v (original error: %v)", deleteErr, err)
			}
			return fmt.Errorf("WireGuard setup failed (device rolled back): %w", err)
		}
		return fmt.Errorf("failed to add peer to WireGuard: %w", err)
	}

	return nil
}

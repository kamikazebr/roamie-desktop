package services

import (
	"context"
	"fmt"
	"log"

	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/kamikazebr/roamie-desktop/pkg/utils"
	"github.com/google/uuid"
)

type DeviceService struct {
	deviceRepo *storage.DeviceRepository
	userRepo   *storage.UserRepository
	subnetPool *SubnetPool
	authRepo   *storage.DeviceAuthRepository
}

func NewDeviceService(
	deviceRepo *storage.DeviceRepository,
	userRepo *storage.UserRepository,
	subnetPool *SubnetPool,
	authRepo *storage.DeviceAuthRepository,
) *DeviceService {
	return &DeviceService{
		deviceRepo: deviceRepo,
		userRepo:   userRepo,
		subnetPool: subnetPool,
		authRepo:   authRepo,
	}
}

// DeviceRegistrationResult contains the result of a device registration
type DeviceRegistrationResult struct {
	Device         *models.Device // The registered device (new or existing)
	ReplacedDevice *models.Device // The old device that was replaced (nil if none)
	WasReplaced    bool           // True if a device was replaced
}

func (s *DeviceService) RegisterDevice(ctx context.Context, userID uuid.UUID, deviceName, publicKey string, username *string, osType, hardwareID, displayName *string, deviceID *uuid.UUID) (*DeviceRegistrationResult, error) {
	// Validate inputs
	if deviceName == "" {
		return nil, fmt.Errorf("device name is required")
	}

	if !utils.IsValidWireGuardKey(publicKey) {
		return nil, fmt.Errorf("invalid WireGuard public key format")
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Check if device name already exists for this user
	existing, err := s.deviceRepo.GetByUserAndName(ctx, userID, deviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing device: %w", err)
	}

	// BYPASS: If device exists with same public key, return it (idempotent)
	if existing != nil && existing.PublicKey == publicKey {
		return &DeviceRegistrationResult{
			Device:         existing,
			ReplacedDevice: nil,
			WasReplaced:    false,
		}, nil
	}

	// BYPASS: If device exists with same hardware_id for this user, return it (idempotent)
	if hardwareID != nil && *hardwareID != "" {
		devices, err := s.deviceRepo.GetByUserID(ctx, userID)
		if err == nil {
			for _, dev := range devices {
				if dev.HardwareID == *hardwareID && dev.PublicKey == publicKey {
					return &DeviceRegistrationResult{
						Device:         &dev,
						ReplacedDevice: nil,
						WasReplaced:    false,
					}, nil
				}
			}
		}
	}

	// REPLACE: If device exists with different public key, mark for replacement
	var replacingDevice *models.Device
	if existing != nil && existing.PublicKey != publicKey {
		replacingDevice = existing
	}

	// Check if public key already exists for a different user/device
	existingKey, err := s.deviceRepo.GetByPublicKey(ctx, publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing public key: %w", err)
	}
	if existingKey != nil {
		// Allow if it's the same device being replaced
		if replacingDevice == nil || existingKey.ID != replacingDevice.ID {
			return nil, fmt.Errorf("public key already registered")
		}
	}

	// Check device limit (don't count device being replaced)
	deviceCount, err := s.deviceRepo.CountActiveByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count devices: %w", err)
	}
	effectiveCount := deviceCount
	if replacingDevice != nil {
		effectiveCount-- // Don't count the device we're replacing
	}
	if effectiveCount >= user.MaxDevices {
		return nil, fmt.Errorf("maximum device limit (%d) reached", user.MaxDevices)
	}

	// Determine IP address
	var vpnIP string
	if replacingDevice != nil {
		// Reuse IP from device being replaced
		vpnIP = replacingDevice.VpnIP
	} else {
		// Allocate new IP within user's subnet
		userDevices, err := s.deviceRepo.GetByUserID(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user devices: %w", err)
		}

		existingIPs := make([]string, len(userDevices))
		for i, d := range userDevices {
			existingIPs[i] = d.VpnIP
		}

		vpnIP, err = s.subnetPool.GetNextAvailableIP(ctx, user.Subnet, existingIPs)
		if err != nil {
			return nil, fmt.Errorf("failed to allocate IP: %w", err)
		}
	}

	// If replacing, delete old device first to avoid unique constraint violations
	if replacingDevice != nil {
		if err := s.deviceRepo.Delete(ctx, replacingDevice.ID); err != nil {
			return nil, fmt.Errorf("failed to delete old device: %w", err)
		}
	}

	// Create new device
	device := &models.Device{
		UserID:      userID,
		DeviceName:  deviceName,
		PublicKey:   publicKey,
		VpnIP:       vpnIP,
		Username:    username,
		Active:      true,
		DisplayName: displayName,
	}

	// Use provided device ID if available, otherwise generate new one
	if deviceID != nil {
		device.ID = *deviceID
	} else {
		device.ID = uuid.New()
	}

	// Set structured fields if provided, otherwise parse from device_name
	if osType != nil && *osType != "" {
		device.OSType = *osType
	}
	if hardwareID != nil && *hardwareID != "" {
		device.HardwareID = *hardwareID
	}

	// If structured fields not provided, parse from device_name for backward compatibility
	if device.OSType == "" || device.HardwareID == "" {
		device.ParseDeviceName()
	}

	if err := s.deviceRepo.Create(ctx, device); err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}

	// Return result with replacement info
	// Note: Handler must remove old WireGuard peer (replacingDevice is already deleted from DB)
	return &DeviceRegistrationResult{
		Device:         device,
		ReplacedDevice: replacingDevice,
		WasReplaced:    replacingDevice != nil,
	}, nil
}

func (s *DeviceService) GetUserDevices(ctx context.Context, userID uuid.UUID) ([]models.Device, error) {
	return s.deviceRepo.GetByUserID(ctx, userID)
}

func (s *DeviceService) GetDevice(ctx context.Context, deviceID uuid.UUID, userID uuid.UUID) (*models.Device, error) {
	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, fmt.Errorf("device not found")
	}

	// Verify device belongs to user
	if device.UserID != userID {
		return nil, fmt.Errorf("device does not belong to user")
	}

	return device, nil
}

// GetDeviceByID gets a device by ID without user validation (internal use)
func (s *DeviceService) GetDeviceByID(ctx context.Context, deviceID uuid.UUID) (*models.Device, error) {
	device, err := s.deviceRepo.GetByID(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, fmt.Errorf("device not found")
	}
	return device, nil
}

func (s *DeviceService) DeleteDevice(ctx context.Context, deviceID uuid.UUID, userID uuid.UUID) error {
	device, err := s.GetDevice(ctx, deviceID, userID)
	if err != nil {
		return err
	}

	// Delete refresh tokens for this device
	if err := s.authRepo.DeleteRefreshTokensByDeviceID(ctx, device.ID); err != nil {
		log.Printf("Warning: failed to delete refresh tokens for device %s: %v", device.ID, err)
		// Continue with deletion even if token cleanup fails
	} else {
		log.Printf("Deleted refresh tokens for device %s", device.ID)
	}

	// Delete device auth challenges for this device
	if err := s.authRepo.DeleteChallengesByDeviceID(ctx, device.ID); err != nil {
		log.Printf("Warning: failed to delete auth challenges for device %s: %v", device.ID, err)
		// Continue with deletion even if challenge cleanup fails
	} else {
		log.Printf("Deleted auth challenges for device %s", device.ID)
	}

	// Delete the device itself
	if err := s.deviceRepo.Delete(ctx, device.ID); err != nil {
		return fmt.Errorf("failed to delete device: %w", err)
	}

	log.Printf("Successfully deleted device %s for user %s", device.ID, userID)
	return nil
}

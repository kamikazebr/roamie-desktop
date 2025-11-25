package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
)

type DeviceAuthService struct {
	deviceAuthRepo *storage.DeviceAuthRepository
	userRepo       *storage.UserRepository
	deviceService  *DeviceService
}

func NewDeviceAuthService(
	deviceAuthRepo *storage.DeviceAuthRepository,
	userRepo *storage.UserRepository,
) *DeviceAuthService {
	return &DeviceAuthService{
		deviceAuthRepo: deviceAuthRepo,
		userRepo:       userRepo,
		deviceService:  nil, // Will be set later to avoid circular dependency
	}
}

// SetDeviceService sets the device service (called after initialization)
func (s *DeviceAuthService) SetDeviceService(deviceService *DeviceService) {
	s.deviceService = deviceService
}

// CreateChallenge creates a new device authorization challenge
func (s *DeviceAuthService) CreateChallenge(ctx context.Context, deviceID uuid.UUID, hostname, ipAddress string, username *string, publicKey *string, osType *string, hardwareID *string) (*models.DeviceAuthChallenge, error) {
	challenge := &models.DeviceAuthChallenge{
		ID:         uuid.New(),
		DeviceID:   deviceID,
		Hostname:   hostname,
		IPAddress:  ipAddress,
		Username:   username,
		PublicKey:  publicKey,
		OSType:     osType,
		HardwareID: hardwareID,
		Status:     "pending",
		ExpiresAt:  time.Now().Add(5 * time.Minute),
	}

	if err := s.deviceAuthRepo.CreateChallenge(ctx, challenge); err != nil {
		return nil, fmt.Errorf("failed to create challenge: %w", err)
	}

	return challenge, nil
}

// GetChallenge gets a challenge by ID and updates expired status
func (s *DeviceAuthService) GetChallenge(ctx context.Context, challengeID uuid.UUID) (*models.DeviceAuthChallenge, error) {
	challenge, err := s.deviceAuthRepo.GetChallenge(ctx, challengeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get challenge: %w", err)
	}

	if challenge == nil {
		return nil, fmt.Errorf("challenge not found")
	}

	// Auto-expire if time passed
	if time.Now().After(challenge.ExpiresAt) && challenge.Status == "pending" {
		s.deviceAuthRepo.UpdateChallengeStatus(ctx, challengeID, "expired", nil)
		challenge.Status = "expired"
	}

	return challenge, nil
}

// ListPendingChallenges lists all pending challenges
func (s *DeviceAuthService) ListPendingChallenges(ctx context.Context) ([]*models.DeviceAuthChallenge, error) {
	// First expire old challenges
	s.deviceAuthRepo.ExpireOldChallenges(ctx)

	challenges, err := s.deviceAuthRepo.ListPendingChallenges(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending challenges: %w", err)
	}

	return challenges, nil
}

// ApproveChallenge approves or denies a challenge and auto-registers device if public_key present
func (s *DeviceAuthService) ApproveChallenge(
	ctx context.Context,
	challengeID, userID uuid.UUID,
	approved bool,
	wgManager WireGuardManager,
	deviceRepo DeviceRepository,
) error {
	// First, check current challenge status for idempotency
	challenge, err := s.deviceAuthRepo.GetChallenge(ctx, challengeID)
	if err != nil {
		return fmt.Errorf("failed to get challenge: %w", err)
	}
	if challenge == nil {
		return fmt.Errorf("challenge not found")
	}

	status := "denied"
	if approved {
		status = "approved"
	}

	// Idempotency: If already processed with same decision, return success
	if challenge.Status == status {
		fmt.Printf("[DeviceAuth] Challenge %s already %s, returning success (idempotent)\n", challengeID, status)
		return nil
	}

	// If expired, return specific error
	if challenge.Status == "expired" || time.Now().After(challenge.ExpiresAt) {
		return fmt.Errorf("challenge has expired")
	}

	// If already processed with different decision, return error
	if challenge.Status != "pending" {
		return fmt.Errorf("challenge already processed with different decision")
	}

	// Update challenge status
	if err := s.deviceAuthRepo.UpdateChallengeStatus(ctx, challengeID, status, &userID); err != nil {
		return fmt.Errorf("failed to update challenge status: %w", err)
	}

	// If approved and has public_key, auto-register WireGuard device
	if approved && s.deviceService != nil {
		challenge, err := s.deviceAuthRepo.GetChallenge(ctx, challengeID)
		if err == nil && challenge != nil && challenge.PublicKey != nil && *challenge.PublicKey != "" {
			// Register device with hostname as device name and fields from challenge
			// Use challenge's device_id to ensure client and server IDs match
			device, err := s.deviceService.RegisterDevice(
				ctx,
				userID,
				challenge.Hostname,
				*challenge.PublicKey,
				challenge.Username,
				challenge.OSType,
				challenge.HardwareID,
				nil,                 // displayName
				&challenge.DeviceID, // Use client-provided device ID
			)
			if err != nil {
				// Log error but don't fail the approval
				fmt.Printf("Warning: failed to auto-register device: %v\n", err)
			} else {
				// Store device ID in challenge for retrieval during polling
				s.deviceAuthRepo.UpdateChallengeDeviceID(ctx, challengeID, &device.Device.ID)

				// Add device to WireGuard interface
				if wgManager != nil {
					if err := AddDeviceToWireGuard(ctx, wgManager, deviceRepo, device, false); err != nil {
						fmt.Printf("Warning: failed to add peer to WireGuard: %v\n", err)
					}
				}
			}
		}
	}

	return nil
}

// GenerateRefreshToken generates a new secure refresh token
func (s *DeviceAuthService) GenerateRefreshToken(ctx context.Context, userID, deviceID uuid.UUID) (string, error) {
	// Generate secure random token (64 bytes = 512 bits)
	tokenBytes := make([]byte, 64)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}

	tokenString := base64.URLEncoding.EncodeToString(tokenBytes)

	token := &models.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		DeviceID:  deviceID,
		Token:     tokenString,
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour), // 1 year
	}

	if err := s.deviceAuthRepo.CreateRefreshToken(ctx, token); err != nil {
		return "", fmt.Errorf("failed to create refresh token: %w", err)
	}

	return tokenString, nil
}

// ValidateRefreshToken validates a refresh token and returns the associated data
func (s *DeviceAuthService) ValidateRefreshToken(ctx context.Context, tokenString string) (*models.RefreshToken, error) {
	token, err := s.deviceAuthRepo.GetRefreshToken(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	if token == nil {
		return nil, fmt.Errorf("invalid or expired refresh token")
	}

	// Update last used timestamp
	s.deviceAuthRepo.UpdateRefreshTokenLastUsed(ctx, token.ID)

	return token, nil
}

// RevokeRefreshToken revokes a refresh token
func (s *DeviceAuthService) RevokeRefreshToken(ctx context.Context, tokenString string) error {
	if err := s.deviceAuthRepo.DeleteRefreshToken(ctx, tokenString); err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}

	return nil
}

// ListUserDevices lists all devices (refresh tokens) for a user
func (s *DeviceAuthService) ListUserDevices(ctx context.Context, userID uuid.UUID) ([]*models.RefreshToken, error) {
	tokens, err := s.deviceAuthRepo.ListUserRefreshTokens(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user devices: %w", err)
	}

	return tokens, nil
}

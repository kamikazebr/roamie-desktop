package services

import (
	"context"
	"fmt"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
)

type BiometricAuthService struct {
	authRepo   *storage.BiometricAuthRepository
	userRepo   *storage.UserRepository
	deviceRepo *storage.DeviceRepository
}

func NewBiometricAuthService(
	authRepo *storage.BiometricAuthRepository,
	userRepo *storage.UserRepository,
	deviceRepo *storage.DeviceRepository,
) *BiometricAuthService {
	return &BiometricAuthService{
		authRepo:   authRepo,
		userRepo:   userRepo,
		deviceRepo: deviceRepo,
	}
}

// CreateRequest creates a new biometric auth request
func (s *BiometricAuthService) CreateRequest(
	ctx context.Context,
	userID uuid.UUID,
	username, hostname, command string,
	deviceIDStr, ipAddress string,
) (*models.BiometricAuthRequest, error) {
	// Validate user exists
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Parse device ID if provided
	var deviceID *uuid.UUID
	if deviceIDStr != "" {
		parsed, err := uuid.Parse(deviceIDStr)
		if err == nil {
			// Verify device belongs to user
			device, err := s.deviceRepo.GetByID(ctx, parsed)
			if err == nil && device != nil && device.UserID == userID {
				deviceID = &parsed
			}
		}
	}

	// Create request with 30-second expiration
	req := &models.BiometricAuthRequest{
		UserID:    userID,
		DeviceID:  deviceID,
		Username:  username,
		Hostname:  hostname,
		Command:   command,
		Status:    "pending",
		ExpiresAt: time.Now().UTC().Add(30 * time.Second),
	}

	if ipAddress != "" {
		req.IPAddress = &ipAddress
	}

	if err := s.authRepo.Create(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to create auth request: %w", err)
	}

	return req, nil
}

// ListPendingRequests returns all pending auth requests for a user
func (s *BiometricAuthService) ListPendingRequests(ctx context.Context, userID uuid.UUID) ([]models.BiometricAuthRequest, error) {
	// Mark expired requests first
	if _, err := s.authRepo.MarkExpired(ctx); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: failed to mark expired requests: %v\n", err)
	}

	requests, err := s.authRepo.ListPending(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending requests: %w", err)
	}

	return requests, nil
}

// RespondToRequest processes a user's response to an auth request
func (s *BiometricAuthService) RespondToRequest(
	ctx context.Context,
	userID uuid.UUID,
	requestID uuid.UUID,
	response string,
) (*models.BiometricAuthRequest, error) {
	// Validate response
	if response != "approved" && response != "denied" {
		return nil, fmt.Errorf("invalid response: must be 'approved' or 'denied'")
	}

	// Get the request
	req, err := s.authRepo.GetByID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth request: %w", err)
	}
	if req == nil {
		return nil, fmt.Errorf("auth request not found")
	}

	// Verify request belongs to user
	if req.UserID != userID {
		return nil, fmt.Errorf("unauthorized: request does not belong to user")
	}

	// Check if request is still pending
	if req.Status != "pending" {
		return nil, fmt.Errorf("request is not pending (status: %s)", req.Status)
	}

	// Check if request has expired
	if time.Now().UTC().After(req.ExpiresAt) {
		// Mark as expired
		_ = s.authRepo.UpdateStatus(ctx, requestID, "expired", nil)
		return nil, fmt.Errorf("request has expired")
	}

	// Update status
	if err := s.authRepo.UpdateStatus(ctx, requestID, response, &response); err != nil {
		return nil, fmt.Errorf("failed to update auth request: %w", err)
	}

	// Get updated request
	req, err = s.authRepo.GetByID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get updated request: %w", err)
	}

	return req, nil
}

// PollRequestStatus polls the status of an auth request (used by Linux system)
func (s *BiometricAuthService) PollRequestStatus(ctx context.Context, requestID uuid.UUID) (*models.BiometricAuthRequest, error) {
	req, err := s.authRepo.GetByID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth request: %w", err)
	}
	if req == nil {
		return nil, fmt.Errorf("auth request not found")
	}

	// Check if expired
	if req.Status == "pending" && time.Now().UTC().After(req.ExpiresAt) {
		// Mark as timeout
		_ = s.authRepo.UpdateStatus(ctx, requestID, "timeout", nil)
		req.Status = "timeout"
	}

	return req, nil
}

// GetStats returns authentication statistics for a user
func (s *BiometricAuthService) GetStats(ctx context.Context, userID uuid.UUID, days int) (*models.BiometricAuthStats, error) {
	since := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	stats, err := s.authRepo.GetStats(ctx, userID, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	return stats, nil
}

// CleanupOldRequests removes old auth requests (for maintenance)
func (s *BiometricAuthService) CleanupOldRequests(ctx context.Context, olderThanDays int) (int, error) {
	duration := time.Duration(olderThanDays) * 24 * time.Hour
	count, err := s.authRepo.DeleteOld(ctx, duration)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old requests: %w", err)
	}
	return count, nil
}

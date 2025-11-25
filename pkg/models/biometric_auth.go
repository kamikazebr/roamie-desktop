package models

import (
	"time"

	"github.com/google/uuid"
)

// BiometricAuthRequest represents a biometric authentication request in the database
type BiometricAuthRequest struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	UserID      uuid.UUID  `json:"user_id" db:"user_id"`
	DeviceID    *uuid.UUID `json:"device_id,omitempty" db:"device_id"`
	Username    string     `json:"username" db:"username"`
	Hostname    string     `json:"hostname" db:"hostname"`
	Command     string     `json:"command" db:"command"`
	Status      string     `json:"status" db:"status"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at" db:"expires_at"`
	RespondedAt *time.Time `json:"responded_at,omitempty" db:"responded_at"`
	Response    *string    `json:"response,omitempty" db:"response"`
	IPAddress   *string    `json:"ip_address,omitempty" db:"ip_address"`
}

// Biometric Auth API types

// CreateBiometricAuthRequest is sent from Linux system to create a new auth request
type CreateBiometricAuthRequest struct {
	Username  string `json:"username" validate:"required"`
	Hostname  string `json:"hostname" validate:"required"`
	Command   string `json:"command" validate:"required"`
	DeviceID  string `json:"device_id,omitempty"`
	IPAddress string `json:"ip_address,omitempty"`
}

// CreateBiometricAuthResponse is returned after creating a new auth request
type CreateBiometricAuthResponse struct {
	RequestID string `json:"request_id"`
	ExpiresAt string `json:"expires_at"`
	ExpiresIn int    `json:"expires_in"`
}

// ListPendingAuthRequestsResponse lists all pending auth requests for a user
type ListPendingAuthRequestsResponse struct {
	Requests []BiometricAuthRequest `json:"requests"`
	Count    int                    `json:"count"`
}

// RespondToAuthRequest is sent from Flutter app to approve/deny a request
type RespondToAuthRequest struct {
	RequestID string `json:"request_id" validate:"required,uuid"`
	Response  string `json:"response" validate:"required,oneof=approved denied"`
}

// RespondToAuthResponse is returned after responding to a request
type RespondToAuthResponse struct {
	Message   string `json:"message"`
	Status    string `json:"status"`
	RequestID string `json:"request_id"`
}

// PollAuthStatusResponse is returned when Linux polls for auth status
type PollAuthStatusResponse struct {
	Status      string `json:"status"`
	Response    string `json:"response,omitempty"`
	Message     string `json:"message,omitempty"`
	RespondedAt string `json:"responded_at,omitempty"`
}

// BiometricAuthStats provides statistics about auth requests
type BiometricAuthStats struct {
	TotalRequests    int `json:"total_requests"`
	ApprovedRequests int `json:"approved_requests"`
	DeniedRequests   int `json:"denied_requests"`
	ExpiredRequests  int `json:"expired_requests"`
	PendingRequests  int `json:"pending_requests"`
}

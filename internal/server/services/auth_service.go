package services

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/kamikazebr/roamie-desktop/pkg/utils"
	"github.com/google/uuid"
)

type AuthService struct {
	authRepo     *storage.AuthRepository
	userRepo     *storage.UserRepository
	emailService *EmailService
	subnetPool   *SubnetPool
}

func NewAuthService(
	authRepo *storage.AuthRepository,
	userRepo *storage.UserRepository,
	emailService *EmailService,
	subnetPool *SubnetPool,
) *AuthService {
	return &AuthService{
		authRepo:     authRepo,
		userRepo:     userRepo,
		emailService: emailService,
		subnetPool:   subnetPool,
	}
}

func (s *AuthService) RequestCode(ctx context.Context, email string) (int, error) {
	if !utils.IsValidEmail(email) {
		return 0, fmt.Errorf("invalid email format")
	}

	// Generate 6-digit code
	code, err := utils.GenerateAuthCode()
	if err != nil {
		return 0, fmt.Errorf("failed to generate code: %w", err)
	}

	// Get expiration from env or default to 5 minutes
	expirationSec := 300
	if envExp := os.Getenv("AUTH_CODE_EXPIRATION"); envExp != "" {
		if exp, err := strconv.Atoi(envExp); err == nil {
			expirationSec = exp
		}
	}

	authCode := &models.AuthCode{
		Email:     email,
		Code:      code,
		ExpiresAt: time.Now().UTC().Add(time.Duration(expirationSec) * time.Second),
	}

	// Save to database
	if err := s.authRepo.CreateCode(ctx, authCode); err != nil {
		return 0, fmt.Errorf("failed to save auth code: %w", err)
	}

	// Send email
	if err := s.emailService.SendAuthCode(email, code); err != nil {
		return 0, fmt.Errorf("failed to send email: %w", err)
	}

	return expirationSec, nil
}

func (s *AuthService) VerifyCode(ctx context.Context, email, code string) (string, time.Time, error) {
	if !utils.IsValidEmail(email) {
		return "", time.Time{}, fmt.Errorf("invalid email format")
	}

	// Get valid code
	authCode, err := s.authRepo.GetValidCode(ctx, email, code)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("database error: %w", err)
	}

	if authCode == nil {
		// Check if code exists but is invalid (expired or used)
		anyCode, err := s.authRepo.GetCode(ctx, email, code)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("database error: %w", err)
		}

		if anyCode != nil {
			if anyCode.Used {
				return "", time.Time{}, fmt.Errorf("code has already been used")
			}
			if anyCode.ExpiresAt.Before(time.Now().UTC()) {
				return "", time.Time{}, fmt.Errorf("code has expired")
			}
		}
		return "", time.Time{}, fmt.Errorf("invalid code")
	}

	// Mark code as used
	if err := s.authRepo.MarkCodeUsed(ctx, authCode.ID.String()); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to mark code as used: %w", err)
	}

	// Get or create user
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		// Create new user with allocated subnet
		subnet, err := s.subnetPool.AllocateSubnet(ctx)
		if err != nil {
			return "", time.Time{}, fmt.Errorf("failed to allocate subnet: %w", err)
		}

		maxDevices := 5
		if envMax := os.Getenv("MAX_DEVICES_PER_USER"); envMax != "" {
			if max, err := strconv.Atoi(envMax); err == nil {
				maxDevices = max
			}
		}

		user = &models.User{
			Email:      email,
			Subnet:     subnet,
			MaxDevices: maxDevices,
			Active:     true,
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			return "", time.Time{}, fmt.Errorf("failed to create user: %w", err)
		}

		// Send welcome email (non-blocking, ignore errors)
		go s.emailService.SendWelcomeEmail(email, subnet)
	}

	// Generate JWT
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", time.Time{}, fmt.Errorf("JWT_SECRET not configured")
	}

	expirationStr := os.Getenv("JWT_EXPIRATION")
	if expirationStr == "" {
		expirationStr = "168h" // 7 days default
	}

	expiration, err := time.ParseDuration(expirationStr)
	if err != nil {
		expiration = 168 * time.Hour
	}

	token, err := utils.GenerateJWT(user.ID, user.Email, jwtSecret, expiration)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate JWT: %w", err)
	}

	expiresAt := time.Now().UTC().Add(expiration)
	return token, expiresAt, nil
}

func (s *AuthService) CleanupExpiredCodes(ctx context.Context) error {
	return s.authRepo.DeleteExpiredCodes(ctx)
}

// GetUserByID retrieves a user by their ID
func (s *AuthService) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

// GenerateToken generates a JWT token for a user
func (s *AuthService) GenerateToken(userID uuid.UUID, email string) (string, time.Time, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return "", time.Time{}, fmt.Errorf("JWT_SECRET not configured")
	}

	// Use 30 days expiration for device tokens
	expiration := 30 * 24 * time.Hour

	token, err := utils.GenerateJWT(userID, email, jwtSecret, expiration)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate JWT: %w", err)
	}

	expiresAt := time.Now().UTC().Add(expiration)
	return token, expiresAt, nil
}

// GetOrCreateUserByFirebaseUID gets or creates a user by email (primary identifier)
// Firebase UID is stored but not used for lookup to support account switching on same device
func (s *AuthService) GetOrCreateUserByFirebaseUID(ctx context.Context, firebaseUID, email string) (*models.User, error) {
	// Try to get existing user by EMAIL (not Firebase UID)
	// This ensures each email gets a unique subnet, even if Firebase UID is reused
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	if user != nil {
		return user, nil
	}

	// User doesn't exist, create new user with allocated subnet
	subnet, err := s.subnetPool.AllocateSubnet(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate subnet: %w", err)
	}

	maxDevices := 5
	if envMax := os.Getenv("MAX_DEVICES_PER_USER"); envMax != "" {
		if max, err := strconv.Atoi(envMax); err == nil {
			maxDevices = max
		}
	}

	user = &models.User{
		Email:      email,
		Subnet:     subnet,
		MaxDevices: maxDevices,
		Active:     true,
	}

	if err := s.userRepo.CreateWithFirebaseUID(ctx, user, firebaseUID); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Send welcome email (non-blocking, ignore errors)
	go s.emailService.SendWelcomeEmail(email, subnet)

	return user, nil
}

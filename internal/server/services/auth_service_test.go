package services

import (
	"context"
	"testing"

	"github.com/kamikazebr/roamie-desktop/internal/testutil"
	"github.com/google/uuid"
)

func setupAuthService(t *testing.T, tdb *testutil.TestDB) *AuthService {
	t.Helper()

	t.Setenv("JWT_SECRET", "test-secret-key-for-testing")
	t.Setenv("SKIP_EMAIL_SEND", "true")
	t.Setenv("WG_BASE_NETWORK", "10.200.0.0/16")
	t.Setenv("WG_SUBNET_SIZE", "29")

	repos := tdb.Repositories()
	subnetPool, err := NewSubnetPool(repos.Users, repos.Conflicts)
	if err != nil {
		t.Fatalf("Failed to create subnet pool: %v", err)
	}

	// EmailService with nil client - SKIP_EMAIL_SEND prevents actual sending
	emailService := &EmailService{}

	return NewAuthService(repos.Auth, repos.Users, emailService, subnetPool)
}

// --- GetOrCreateUserByFirebaseUID tests ---

func TestAuthService_GetOrCreateUserByFirebaseUID_NewUser(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()
	service := setupAuthService(t, tdb)

	email := testutil.GenerateTestEmail()
	firebaseUID := "firebase-" + uuid.New().String()

	// Create new user via Firebase auth
	user, err := service.GetOrCreateUserByFirebaseUID(ctx, firebaseUID, email)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify user was created with correct data
	if user.Email != email {
		t.Errorf("Email mismatch: expected %s, got %s", email, user.Email)
	}

	if user.Subnet == "" {
		t.Error("User should have allocated subnet")
	}

	if user.MaxDevices != 5 {
		t.Errorf("MaxDevices mismatch: expected 5, got %d", user.MaxDevices)
	}

	// Cleanup
	tdb.DeleteTestUser(ctx, user.ID)
}

func TestAuthService_GetOrCreateUserByFirebaseUID_ExistingUser(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()
	service := setupAuthService(t, tdb)

	// Create existing user
	email := testutil.GenerateTestEmail()
	existingUser := tdb.CreateTestUser(ctx, email, testutil.GenerateTestSubnet(100))
	defer tdb.DeleteTestUser(ctx, existingUser.ID)

	// Try to get/create with same email but different Firebase UID
	firebaseUID := "firebase-different-" + uuid.New().String()
	user, err := service.GetOrCreateUserByFirebaseUID(ctx, firebaseUID, email)
	if err != nil {
		t.Fatalf("Failed to get existing user: %v", err)
	}

	// Should return existing user, not create new one
	if user.ID != existingUser.ID {
		t.Errorf("Should return existing user ID %s, got %s", existingUser.ID, user.ID)
	}

	if user.Subnet != existingUser.Subnet {
		t.Errorf("Subnet should match existing user: expected %s, got %s", existingUser.Subnet, user.Subnet)
	}
}

// --- GenerateToken tests ---

func TestAuthService_GenerateToken_Success(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()
	service := setupAuthService(t, tdb)

	// Create test user
	email := testutil.GenerateTestEmail()
	testUser := tdb.CreateTestUser(ctx, email, testutil.GenerateTestSubnet(101))
	defer tdb.DeleteTestUser(ctx, testUser.ID)

	// Generate token
	token, expiresAt, err := service.GenerateToken(testUser.ID, email)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if token == "" {
		t.Error("Token should not be empty")
	}

	if expiresAt.IsZero() {
		t.Error("ExpiresAt should not be zero")
	}
}

func TestAuthService_GenerateToken_MissingJWTSecret(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()

	// Setup without JWT_SECRET
	t.Setenv("JWT_SECRET", "")
	t.Setenv("SKIP_EMAIL_SEND", "true")
	t.Setenv("WG_BASE_NETWORK", "10.200.0.0/16")
	t.Setenv("WG_SUBNET_SIZE", "29")

	repos := tdb.Repositories()
	subnetPool, _ := NewSubnetPool(repos.Users, repos.Conflicts)
	service := NewAuthService(repos.Auth, repos.Users, &EmailService{}, subnetPool)

	// Create test user
	email := testutil.GenerateTestEmail()
	testUser := tdb.CreateTestUser(ctx, email, testutil.GenerateTestSubnet(102))
	defer tdb.DeleteTestUser(ctx, testUser.ID)

	// Generate token should fail
	_, _, err := service.GenerateToken(testUser.ID, email)
	if err == nil {
		t.Error("Expected error when JWT_SECRET is missing")
	}

	expectedErr := "JWT_SECRET not configured"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%v'", expectedErr, err)
	}
}

// --- GetUserByID tests ---

func TestAuthService_GetUserByID_Success(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()
	service := setupAuthService(t, tdb)

	// Create test user
	email := testutil.GenerateTestEmail()
	testUser := tdb.CreateTestUser(ctx, email, testutil.GenerateTestSubnet(103))
	defer tdb.DeleteTestUser(ctx, testUser.ID)

	// Get user by ID
	user, err := service.GetUserByID(ctx, testUser.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if user.ID != testUser.ID {
		t.Errorf("User ID mismatch: expected %s, got %s", testUser.ID, user.ID)
	}

	if user.Email != email {
		t.Errorf("Email mismatch: expected %s, got %s", email, user.Email)
	}
}

func TestAuthService_GetUserByID_NotFound(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()
	service := setupAuthService(t, tdb)

	// Try to get non-existent user
	nonExistentID := uuid.New()
	_, err := service.GetUserByID(ctx, nonExistentID)
	if err == nil {
		t.Error("Expected error for non-existent user")
	}

	expectedErr := "user not found"
	if err != nil && err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%v'", expectedErr, err)
	}
}

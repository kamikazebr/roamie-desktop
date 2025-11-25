package storage

import (
	"context"
	"testing"

	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// TestCreateDevice_UsesProvidedID verifies that Create uses the provided device.ID
// This is a critical test for the device auth flow where client provides device_id
func TestCreateDevice_UsesProvidedID(t *testing.T) {
	// Skip if no database configured (use e2e database for testing)
	dbURL := "postgres://roamie:roamie_test_password@localhost:5436/roamie_vpn?sslmode=disable"

	sqlxDB, err := sqlx.Connect("postgres", dbURL)
	if err != nil {
		t.Skip("Skipping test: database not available")
		return
	}
	db := &DB{sqlxDB}
	defer db.Close()

	repo := NewDeviceRepository(db)
	ctx := context.Background()

	// Create a test user first
	userID := uuid.New()
	_, err = db.ExecContext(ctx, `
		INSERT INTO users (id, email, subnet, max_devices, firebase_uid)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, "test@example.com", "10.100.0.0/29", 5, "test-firebase-uid-"+userID.String())
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)

	// Test: Create device with specific ID
	expectedID := uuid.New()
	device := &models.Device{
		ID:         expectedID,
		UserID:     userID,
		DeviceName: "test-device",
		PublicKey:  "dGVzdC1wdWJsaWMta2V5LWZvci10ZXN0aW5nLTEyMzQ1", // Base64 encoded, 44 chars
		VpnIP:      "10.100.0.2",
		Active:     true,
	}

	err = repo.Create(ctx, device)
	if err != nil {
		t.Fatalf("Failed to create device: %v", err)
	}
	defer db.ExecContext(ctx, "DELETE FROM devices WHERE id = $1", expectedID)

	// Verify the device ID matches what we provided
	if device.ID != expectedID {
		t.Errorf("Device ID mismatch: expected %s, got %s", expectedID, device.ID)
	}

	// Verify in database
	var dbID uuid.UUID
	err = db.QueryRowContext(ctx, "SELECT id FROM devices WHERE public_key = $1", device.PublicKey).Scan(&dbID)
	if err != nil {
		t.Fatalf("Failed to query device: %v", err)
	}

	if dbID != expectedID {
		t.Errorf("Database ID mismatch: expected %s, got %s", expectedID, dbID)
	}

	t.Logf("✓ Device created with expected ID: %s", expectedID)
}

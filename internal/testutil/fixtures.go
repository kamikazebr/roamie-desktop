package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/kamikazebr/roamie-desktop/pkg/models"
	"github.com/google/uuid"
)

// TestUser creates a user fixture for testing
type TestUser struct {
	ID          uuid.UUID
	Email       string
	Subnet      string
	FirebaseUID string
}

// CreateTestUser creates a test user in the database
func (tdb *TestDB) CreateTestUser(ctx context.Context, email, subnet string) *TestUser {
	tdb.t.Helper()

	id := uuid.New()
	firebaseUID := "firebase-" + id.String()

	_, err := tdb.DB.ExecContext(ctx, `
		INSERT INTO users (id, email, subnet, max_devices, firebase_uid)
		VALUES ($1, $2, $3, $4, $5)
	`, id, email, subnet, 5, firebaseUID)
	if err != nil {
		tdb.t.Fatalf("Failed to create test user: %v", err)
	}

	return &TestUser{
		ID:          id,
		Email:       email,
		Subnet:      subnet,
		FirebaseUID: firebaseUID,
	}
}

// DeleteTestUser removes a test user from the database
func (tdb *TestDB) DeleteTestUser(ctx context.Context, userID uuid.UUID) {
	tdb.t.Helper()
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM devices WHERE user_id = $1", userID)
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM users WHERE id = $1", userID)
}

// CreateTestDevice creates a test device in the database
func (tdb *TestDB) CreateTestDevice(ctx context.Context, userID uuid.UUID, name, vpnIP string) *models.Device {
	tdb.t.Helper()

	id := uuid.New()
	publicKey := GenerateTestWireGuardKey()
	hardwareID := id.String()[:8] // 8-char hardware ID
	osType := "linux"             // Default os type
	now := time.Now()

	device := &models.Device{
		ID:            id,
		UserID:        userID,
		DeviceName:    name,
		PublicKey:     publicKey,
		VpnIP:         vpnIP,
		HardwareID:    hardwareID,
		OSType:        osType,
		TunnelEnabled: false,
		Active:        true,
		CreatedAt:     now,
		LastSeen:      now,
	}

	_, err := tdb.DB.ExecContext(ctx, `
		INSERT INTO devices (id, user_id, device_name, public_key, vpn_ip, hardware_id, os_type, tunnel_enabled, active, created_at, last_seen)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, device.ID, device.UserID, device.DeviceName, device.PublicKey, device.VpnIP, device.HardwareID, device.OSType, device.TunnelEnabled, device.Active, device.CreatedAt, device.LastSeen)
	if err != nil {
		tdb.t.Fatalf("Failed to create test device: %v", err)
	}

	return device
}

// DeleteTestDevice removes a test device from the database
func (tdb *TestDB) DeleteTestDevice(ctx context.Context, deviceID uuid.UUID) {
	tdb.t.Helper()
	_, _ = tdb.DB.ExecContext(ctx, "DELETE FROM devices WHERE id = $1", deviceID)
}

// GenerateTestWireGuardKey generates a valid-looking WireGuard public key for testing
func GenerateTestWireGuardKey() string {
	// Generate a 44-character base64-like string (simulates WireGuard key format)
	// WireGuard keys are exactly 44 characters (32 bytes base64 encoded)
	id := uuid.New().String()
	// Remove hyphens from UUID, take 32 chars and add padding to get 44 chars
	raw := id[:8] + id[9:13] + id[14:18] + id[19:23] + id[24:36]
	return raw + "AAAAAAAAAA=="
}

// GenerateTestEmail generates a unique test email
func GenerateTestEmail() string {
	return fmt.Sprintf("test-%s@example.com", uuid.New().String()[:8])
}

// GenerateTestSubnet generates a unique test subnet
func GenerateTestSubnet(index int) string {
	// Generate subnets like 10.200.x.0/29
	octet := (index % 256)
	return fmt.Sprintf("10.200.%d.0/29", octet)
}

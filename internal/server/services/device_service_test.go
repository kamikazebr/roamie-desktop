package services

import (
	"context"
	"testing"

	"github.com/kamikazebr/roamie-desktop/internal/testutil"
	"github.com/google/uuid"
)

func TestDeviceService_RegisterDevice_RequiresDeviceName(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()
	repos := tdb.Repositories()

	// Set required env vars for SubnetPool
	t.Setenv("WG_BASE_NETWORK", "10.200.0.0/16")
	t.Setenv("WG_SUBNET_SIZE", "29")

	subnetPool, err := NewSubnetPool(repos.Users, repos.Conflicts)
	if err != nil {
		t.Fatalf("Failed to create subnet pool: %v", err)
	}

	service := NewDeviceService(repos.Devices, repos.Users, subnetPool, repos.DeviceAuth)

	// Create test user
	testUser := tdb.CreateTestUser(ctx, testutil.GenerateTestEmail(), testutil.GenerateTestSubnet(1))
	defer tdb.DeleteTestUser(ctx, testUser.ID)

	// Test: empty device name should fail
	_, err = service.RegisterDevice(ctx, testUser.ID, "", testutil.GenerateTestWireGuardKey(), nil, nil, nil, nil, nil)
	if err == nil {
		t.Error("Expected error for empty device name, got nil")
	}
	if err != nil && err.Error() != "device name is required" {
		t.Errorf("Expected 'device name is required' error, got: %v", err)
	}
}

func TestDeviceService_RegisterDevice_ValidatesWireGuardKey(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()
	repos := tdb.Repositories()

	t.Setenv("WG_BASE_NETWORK", "10.200.0.0/16")
	t.Setenv("WG_SUBNET_SIZE", "29")

	subnetPool, err := NewSubnetPool(repos.Users, repos.Conflicts)
	if err != nil {
		t.Fatalf("Failed to create subnet pool: %v", err)
	}

	service := NewDeviceService(repos.Devices, repos.Users, subnetPool, repos.DeviceAuth)

	testUser := tdb.CreateTestUser(ctx, testutil.GenerateTestEmail(), testutil.GenerateTestSubnet(2))
	defer tdb.DeleteTestUser(ctx, testUser.ID)

	// Test: invalid WireGuard key should fail
	_, err = service.RegisterDevice(ctx, testUser.ID, "test-device", "invalid-key", nil, nil, nil, nil, nil)
	if err == nil {
		t.Error("Expected error for invalid WireGuard key, got nil")
	}
	if err != nil && err.Error() != "invalid WireGuard public key format" {
		t.Errorf("Expected 'invalid WireGuard public key format' error, got: %v", err)
	}
}

func TestDeviceService_RegisterDevice_FailsForNonexistentUser(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()
	repos := tdb.Repositories()

	t.Setenv("WG_BASE_NETWORK", "10.200.0.0/16")
	t.Setenv("WG_SUBNET_SIZE", "29")

	subnetPool, err := NewSubnetPool(repos.Users, repos.Conflicts)
	if err != nil {
		t.Fatalf("Failed to create subnet pool: %v", err)
	}

	service := NewDeviceService(repos.Devices, repos.Users, subnetPool, repos.DeviceAuth)

	// Test: non-existent user should fail
	nonExistentUserID := uuid.New()
	_, err = service.RegisterDevice(ctx, nonExistentUserID, "test-device", testutil.GenerateTestWireGuardKey(), nil, nil, nil, nil, nil)
	if err == nil {
		t.Error("Expected error for non-existent user, got nil")
	}
}

func TestDeviceService_GetDevice_ReturnsDevice(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()
	repos := tdb.Repositories()

	t.Setenv("WG_BASE_NETWORK", "10.200.0.0/16")
	t.Setenv("WG_SUBNET_SIZE", "29")

	subnetPool, err := NewSubnetPool(repos.Users, repos.Conflicts)
	if err != nil {
		t.Fatalf("Failed to create subnet pool: %v", err)
	}

	service := NewDeviceService(repos.Devices, repos.Users, subnetPool, repos.DeviceAuth)

	// Create test user and device
	testUser := tdb.CreateTestUser(ctx, testutil.GenerateTestEmail(), testutil.GenerateTestSubnet(3))
	defer tdb.DeleteTestUser(ctx, testUser.ID)

	testDevice := tdb.CreateTestDevice(ctx, testUser.ID, "test-device", "10.200.0.2")
	defer tdb.DeleteTestDevice(ctx, testDevice.ID)

	// Test: get device
	device, err := service.GetDevice(ctx, testDevice.ID, testUser.ID)
	if err != nil {
		t.Fatalf("Failed to get device: %v", err)
	}

	if device.ID != testDevice.ID {
		t.Errorf("Device ID mismatch: expected %s, got %s", testDevice.ID, device.ID)
	}
	if device.DeviceName != "test-device" {
		t.Errorf("Device name mismatch: expected 'test-device', got '%s'", device.DeviceName)
	}
}

func TestDeviceService_GetDevice_FailsForWrongUser(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()
	repos := tdb.Repositories()

	t.Setenv("WG_BASE_NETWORK", "10.200.0.0/16")
	t.Setenv("WG_SUBNET_SIZE", "29")

	subnetPool, err := NewSubnetPool(repos.Users, repos.Conflicts)
	if err != nil {
		t.Fatalf("Failed to create subnet pool: %v", err)
	}

	service := NewDeviceService(repos.Devices, repos.Users, subnetPool, repos.DeviceAuth)

	// Create two users
	user1 := tdb.CreateTestUser(ctx, testutil.GenerateTestEmail(), testutil.GenerateTestSubnet(4))
	defer tdb.DeleteTestUser(ctx, user1.ID)

	user2 := tdb.CreateTestUser(ctx, testutil.GenerateTestEmail(), testutil.GenerateTestSubnet(5))
	defer tdb.DeleteTestUser(ctx, user2.ID)

	// Create device for user1
	testDevice := tdb.CreateTestDevice(ctx, user1.ID, "test-device", "10.200.0.10")
	defer tdb.DeleteTestDevice(ctx, testDevice.ID)

	// Test: user2 should not be able to access user1's device
	_, err = service.GetDevice(ctx, testDevice.ID, user2.ID)
	if err == nil {
		t.Error("Expected error when accessing another user's device, got nil")
	}
}

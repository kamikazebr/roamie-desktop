package services

import (
	"context"
	"testing"

	"github.com/kamikazebr/roamie-desktop/internal/testutil"
)

func TestSubnetPool_AllocateSubnet(t *testing.T) {
	tdb := testutil.GetTestDB(t)
	if tdb == nil {
		return
	}
	defer tdb.Close()

	ctx := context.Background()
	repos := tdb.Repositories()

	t.Setenv("WG_BASE_NETWORK", "10.250.0.0/16")
	t.Setenv("WG_SUBNET_SIZE", "29")

	pool, err := NewSubnetPool(repos.Users, repos.Conflicts)
	if err != nil {
		t.Fatalf("Failed to create subnet pool: %v", err)
	}

	// Allocate first subnet
	subnet1, err := pool.AllocateSubnet(ctx)
	if err != nil {
		t.Fatalf("Failed to allocate first subnet: %v", err)
	}

	if subnet1 == "" {
		t.Error("Expected non-empty subnet")
	}

	t.Logf("Allocated subnet: %s", subnet1)

	// The subnet should be in the correct format
	if len(subnet1) < 10 {
		t.Errorf("Subnet format seems wrong: %s", subnet1)
	}
}

func TestSubnetPool_IsIPInSubnet(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		subnet   string
		expected bool
	}{
		{
			name:     "IP in /29 subnet - first usable",
			ip:       "10.100.0.2",
			subnet:   "10.100.0.0/29",
			expected: true,
		},
		{
			name:     "IP in /29 subnet - last usable",
			ip:       "10.100.0.6",
			subnet:   "10.100.0.0/29",
			expected: true,
		},
		{
			name:     "IP outside /29 subnet",
			ip:       "10.100.0.10",
			subnet:   "10.100.0.0/29",
			expected: false,
		},
		{
			name:     "IP in different /16 network",
			ip:       "10.200.0.2",
			subnet:   "10.100.0.0/29",
			expected: false,
		},
		{
			name:     "IP in larger /16 subnet",
			ip:       "10.100.100.50",
			subnet:   "10.100.0.0/16",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIPInSubnet(tt.ip, tt.subnet)
			if result != tt.expected {
				t.Errorf("IsIPInSubnet(%s, %s) = %v, expected %v", tt.ip, tt.subnet, result, tt.expected)
			}
		})
	}
}


package wireguard

import (
	"runtime"
	"strings"
	"testing"
)

func TestCheckInstalled(t *testing.T) {
	installed := CheckInstalled()
	t.Logf("WireGuard installed: %v", installed)
	// Don't fail - just log the result
}

func TestRunPreflight(t *testing.T) {
	result := RunPreflight()
	if result == nil {
		t.Fatal("RunPreflight() returned nil")
	}

	t.Logf("Installed: %v", result.Installed)
	t.Logf("ConfigDir: %s", result.ConfigDir)
	t.Logf("ConfigDirExists: %v", result.ConfigDirExists)
	t.Logf("CanAutoInstall: %v", result.CanAutoInstall)
	t.Logf("InstallMethod: %s", result.InstallMethod)
}

func TestGetInstallInstructions(t *testing.T) {
	instructions := GetInstallInstructions()
	if instructions == "" {
		t.Error("GetInstallInstructions() returned empty string")
	}
	t.Logf("Instructions:\n%s", instructions)
}

func TestCanAutoInstall(t *testing.T) {
	result := RunPreflight()

	switch runtime.GOOS {
	case "darwin":
		// On macOS, CanAutoInstall depends on brew availability
		t.Logf("macOS: CanAutoInstall=%v (depends on brew)", result.CanAutoInstall)
		if result.CanAutoInstall {
			if result.InstallMethod != "brew" {
				t.Errorf("Expected InstallMethod=brew, got %s", result.InstallMethod)
			}
		}
	case "linux":
		// On Linux, depends on package manager (apt, dnf, pacman)
		t.Logf("Linux: CanAutoInstall=%v, InstallMethod=%s", result.CanAutoInstall, result.InstallMethod)
	case "windows":
		// On Windows, depends on winget
		t.Logf("Windows: CanAutoInstall=%v, InstallMethod=%s", result.CanAutoInstall, result.InstallMethod)
	}
}

// TestAutoInstallFlow tests the full auto-install flow
// This test should only run when WireGuard is NOT installed
// and CanAutoInstall is true. It's designed to be run in CI
// after uninstalling WireGuard.
func TestAutoInstallFlow(t *testing.T) {
	result := RunPreflight()

	// Skip if already installed
	if result.Installed {
		t.Skip("WireGuard already installed, skipping auto-install test")
	}

	// Skip if can't auto-install
	if !result.CanAutoInstall {
		t.Skip("Cannot auto-install (no package manager available)")
	}

	t.Logf("Testing auto-install with method: %s", result.InstallMethod)

	// Call PromptInstall which should auto-install
	ok, err := PromptInstall()
	if err != nil {
		t.Fatalf("PromptInstall() failed: %v", err)
	}

	if !ok {
		t.Fatal("PromptInstall() returned false (installation failed or cancelled)")
	}

	// Verify installation
	if !CheckInstalled() {
		t.Fatal("WireGuard still not installed after PromptInstall()")
	}

	t.Log("Auto-install successful!")
}

// TestGenerateConfigFile_SplitTunnel tests that DNS is NOT included for split tunnel
func TestGenerateConfigFile_SplitTunnel(t *testing.T) {
	config := WireGuardConfig{
		PrivateKey: "test-private-key",
		Address:    "10.100.0.2",
		ServerKey:  "test-server-key",
		Endpoint:   "vpn.example.com:51820",
		AllowedIPs: "10.100.0.0/29", // Split tunnel - only VPN subnet
		DNS:        "1.1.1.1",
	}

	result := GenerateConfigFile(config)

	// Split tunnel should NOT have DNS line (avoids systemd-resolved conflicts on Ubuntu)
	if strings.Contains(result, "DNS =") {
		t.Errorf("Split tunnel config should NOT contain DNS line.\nConfig:\n%s", result)
	}

	// Should still have all other required fields
	if !strings.Contains(result, "PrivateKey = test-private-key") {
		t.Error("Missing PrivateKey")
	}
	if !strings.Contains(result, "Address = 10.100.0.2/32") {
		t.Error("Missing Address")
	}
	if !strings.Contains(result, "AllowedIPs = 10.100.0.0/29") {
		t.Error("Missing AllowedIPs")
	}

	t.Logf("Split tunnel config (no DNS):\n%s", result)
}

// TestGenerateConfigFile_FullTunnel tests that DNS IS included for full tunnel
func TestGenerateConfigFile_FullTunnel(t *testing.T) {
	config := WireGuardConfig{
		PrivateKey: "test-private-key",
		Address:    "10.100.0.2",
		ServerKey:  "test-server-key",
		Endpoint:   "vpn.example.com:51820",
		AllowedIPs: "0.0.0.0/0", // Full tunnel - all traffic
		DNS:        "1.1.1.1, 8.8.8.8",
	}

	result := GenerateConfigFile(config)

	// Full tunnel SHOULD have DNS line
	if !strings.Contains(result, "DNS = 1.1.1.1, 8.8.8.8") {
		t.Errorf("Full tunnel config should contain DNS line.\nConfig:\n%s", result)
	}

	t.Logf("Full tunnel config (with DNS):\n%s", result)
}

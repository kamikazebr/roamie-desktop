package wireguard

import (
	"runtime"
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

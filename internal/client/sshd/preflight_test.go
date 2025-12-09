package sshd

import (
	"runtime"
	"testing"
)

func TestCheck(t *testing.T) {
	result, err := Check()
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Check() returned nil result")
	}

	// Log the results for CI visibility
	t.Logf("Installed: %v", result.Installed)
	t.Logf("Running: %v", result.Running)
	t.Logf("CanAutoInstall: %v", result.CanAutoInstall)
	t.Logf("Port: %d", result.Port)
}

func TestIsRunning(t *testing.T) {
	running := IsRunning()
	t.Logf("IsRunning: %v", running)

	// Cross-check with Check()
	result, err := Check()
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}

	if running != result.Running {
		t.Errorf("IsRunning() = %v, but Check().Running = %v", running, result.Running)
	}
}

func TestGetInstallInstructions(t *testing.T) {
	instructions := GetInstallInstructions()
	if instructions == "" {
		t.Error("GetInstallInstructions() returned empty string")
	}
	t.Logf("Instructions:\n%s", instructions)
}

// TestCheckPlatformSpecific tests platform-specific behavior
func TestCheckPlatformSpecific(t *testing.T) {
	result, err := Check()
	if err != nil {
		t.Fatalf("Check() returned error: %v", err)
	}

	switch runtime.GOOS {
	case "darwin":
		// macOS always has SSH installed (built-in)
		if !result.Installed {
			t.Error("On macOS, SSH should always be installed (built-in)")
		}
		// Note: CanAutoInstall is only set when SSH is NOT running
		// because Check() returns early if port 22 is listening.
		// This is expected behavior - no need to auto-install if already running.
		if result.Running {
			t.Logf("macOS: SSH is running, CanAutoInstall=%v (expected false when running)", result.CanAutoInstall)
		} else {
			// Only check CanAutoInstall when SSH is not running
			if !result.CanAutoInstall {
				t.Error("On macOS with SSH not running, CanAutoInstall should be true")
			}
		}
	case "linux":
		// Linux behavior depends on distro, just log
		t.Logf("Linux: Installed=%v, Running=%v, CanAutoInstall=%v", result.Installed, result.Running, result.CanAutoInstall)
	}
}

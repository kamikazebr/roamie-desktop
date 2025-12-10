//go:build darwin
// +build darwin

package sshd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kamikazebr/roamie-desktop/internal/client/ui"
)

// checkPlatform performs macOS-specific SSH daemon checks
func checkPlatform(result *PreflightResult) (*PreflightResult, error) {
	// macOS has SSH built-in, just needs to be enabled
	result.Installed = true

	// Check if Remote Login (SSH) is enabled
	result.Running = isSSHDServiceRunning()

	// macOS can enable SSH via systemsetup
	result.CanAutoInstall = true

	return result, nil
}

// isSSHDServiceRunning checks if SSH is enabled on macOS
func isSSHDServiceRunning() bool {
	// Check via launchctl
	cmd := exec.Command("launchctl", "list", "com.openssh.sshd")
	if err := cmd.Run(); err == nil {
		return true
	}

	// Fallback: check systemsetup (requires sudo)
	cmd = exec.Command("sudo", "systemsetup", "-getremotelogin")
	output, err := cmd.Output()
	if err == nil && strings.Contains(string(output), "On") {
		return true
	}

	return false
}

// enableSSHDService enables Remote Login (SSH) on macOS
func enableSSHDService() error {
	fmt.Println("Enabling Remote Login (SSH)...")
	fmt.Println("Running: sudo systemsetup -setremotelogin on")
	fmt.Println()

	cmd := exec.Command("sudo", "systemsetup", "-setremotelogin", "on")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable Remote Login: %w", err)
	}

	fmt.Println("✓ Remote Login (SSH) enabled!")
	return nil
}

// promptInstallPlatform handles macOS-specific installation prompts using TUI
func promptInstallPlatform(result *PreflightResult) (bool, error) {
	options := []ui.SelectOption{
		{
			Label:       "Enable SSH via command line",
			Description: "sudo systemsetup -setremotelogin on",
			Value:       "enable",
		},
		{
			Label:       "Show manual instructions",
			Description: "System Preferences > Sharing > Remote Login",
			Value:       "manual",
		},
		{Label: "Cancel", Value: "cancel"},
	}

	selected, err := ui.Select("SSH is installed but needs to be enabled", options)
	if err != nil {
		return false, err
	}

	switch selected {
	case 0: // Enable
		if err := enableSSHDService(); err != nil {
			fmt.Printf("\n   Failed to enable SSH: %v\n", err)
			fmt.Println()
			fmt.Println(getInstallInstructions())
			return false, nil
		}
		return true, nil

	case 1: // Manual instructions
		fmt.Println()
		fmt.Println(getInstallInstructions())
		fmt.Println()
		return false, nil

	default: // Cancel or abort
		return false, nil
	}
}

// getInstallInstructions returns macOS-specific installation instructions
func getInstallInstructions() string {
	return `SSH server is not enabled.

On macOS, SSH is installed but needs to be enabled:

Option 1 - Command line:
  sudo systemsetup -setremotelogin on

Option 2 - System Preferences:
  1. Open System Preferences
  2. Go to Sharing
  3. Check 'Remote Login'`
}

package wireguard

import (
	"fmt"
	"os"
	"os/exec"
)

// PreflightResult contains the result of the preflight check
type PreflightResult struct {
	Installed       bool
	ConfigDirExists bool
	ConfigDir       string
	CanAutoInstall  bool
	InstallMethod   string // "brew", "winget", "apt", "dnf", etc.
}

// CheckInstalled checks if WireGuard tools are installed on the system
func CheckInstalled() bool {
	// Check for wg command (works on all platforms)
	if _, err := exec.LookPath("wg"); err == nil {
		return true
	}
	// Check for wg-quick (Linux/macOS)
	if _, err := exec.LookPath("wg-quick"); err == nil {
		return true
	}
	return false
}

// EnsureConfigDir creates the WireGuard config directory if it doesn't exist
func EnsureConfigDir() error {
	configDir := getWireGuardConfigDir()

	// Check if directory exists
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		// Create directory with appropriate permissions
		if err := os.MkdirAll(configDir, 0700); err != nil {
			return fmt.Errorf("failed to create WireGuard config directory %s: %w\nHint: Try running with sudo", configDir, err)
		}
		fmt.Printf("✓ Created WireGuard config directory: %s\n", configDir)
	}

	return nil
}

// RunPreflight performs all pre-flight checks and returns the result
func RunPreflight() *PreflightResult {
	result := &PreflightResult{
		Installed:       CheckInstalled(),
		ConfigDir:       getWireGuardConfigDir(),
		CanAutoInstall:  canAutoInstall(),
		InstallMethod:   getInstallMethod(),
	}

	// Check if config dir exists
	if _, err := os.Stat(result.ConfigDir); err == nil {
		result.ConfigDirExists = true
	}

	return result
}

// PromptInstall prompts the user to install WireGuard and optionally installs it
// Returns true if installation was successful or user wants to proceed anyway
func PromptInstall() (bool, error) {
	result := RunPreflight()

	if result.Installed {
		return true, nil
	}

	fmt.Println("\n⚠️  WireGuard is not installed on your system.")
	fmt.Println()

	// Platform-specific installation
	return promptInstallPlatform(result)
}

// GetInstallInstructions returns platform-specific installation instructions
func GetInstallInstructions() string {
	return getInstallInstructions()
}

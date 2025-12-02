//go:build darwin
// +build darwin

package wireguard

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// canAutoInstall checks if we can auto-install WireGuard on macOS
// Only if brew is available
func canAutoInstall() bool {
	return isBrewAvailable()
}

// getInstallMethod returns the available installation method
func getInstallMethod() string {
	return "brew"
}

// isBrewAvailable checks if Homebrew is installed
func isBrewAvailable() bool {
	// Check common brew paths based on architecture
	brewPaths := []string{
		"/opt/homebrew/bin/brew", // Apple Silicon
		"/usr/local/bin/brew",    // Intel
	}

	for _, path := range brewPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	// Fallback to PATH lookup
	_, err := exec.LookPath("brew")
	return err == nil
}

// getBrewPath returns the path to brew binary
func getBrewPath() string {
	if runtime.GOARCH == "arm64" {
		return "/opt/homebrew/bin/brew"
	}
	return "/usr/local/bin/brew"
}


// installWithBrew installs WireGuard using Homebrew
func installWithBrew() error {
	brewPath := getBrewPath()

	// If brew not in expected path, try to find it
	if _, err := os.Stat(brewPath); err != nil {
		if path, err := exec.LookPath("brew"); err == nil {
			brewPath = path
		} else {
			return fmt.Errorf("brew not found after installation")
		}
	}

	fmt.Println("📦 Installing WireGuard via Homebrew...")
	fmt.Println()

	cmd := exec.Command(brewPath, "install", "wireguard-tools")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install WireGuard: %w", err)
	}

	fmt.Println("\n✓ WireGuard installed successfully!")
	return nil
}

// promptInstallPlatform handles macOS-specific installation - automatic if brew available
func promptInstallPlatform(result *PreflightResult) (bool, error) {
	// If brew is not available, show instructions and exit
	if !isBrewAvailable() {
		fmt.Println("Homebrew is required to install WireGuard.")
		fmt.Println()
		fmt.Println("Please install Homebrew and WireGuard, then run this command again:")
		fmt.Println()
		fmt.Println("  /bin/bash -c \"$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"")
		fmt.Println("  brew install wireguard-tools")
		fmt.Println()
		return false, nil
	}

	// Brew available - install WireGuard automatically
	if err := installWithBrew(); err != nil {
		return false, fmt.Errorf("failed to install WireGuard: %w\n\nTo install manually:\n  brew install wireguard-tools", err)
	}

	// Verify installation
	if !CheckInstalled() {
		return false, fmt.Errorf("WireGuard installation completed but 'wg' command not found.\nTry opening a new terminal and running the command again")
	}

	return true, nil
}

// getInstallInstructions returns macOS-specific installation instructions
func getInstallInstructions() string {
	if isBrewAvailable() {
		return `WireGuard is not installed.

Roamie will automatically install it when you run:
  sudo roamie auth login`
	}

	return `WireGuard is not installed.

Please install Homebrew and WireGuard first:
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  brew install wireguard-tools

Then run:
  sudo roamie auth login`
}

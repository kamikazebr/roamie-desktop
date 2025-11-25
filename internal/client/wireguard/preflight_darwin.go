//go:build darwin
// +build darwin

package wireguard

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// canAutoInstall checks if we can auto-install WireGuard on macOS
func canAutoInstall() bool {
	return isBrewAvailable()
}

// getInstallMethod returns the available installation method
func getInstallMethod() string {
	if isBrewAvailable() {
		return "brew"
	}
	return "manual"
}

// isBrewAvailable checks if Homebrew is installed
func isBrewAvailable() bool {
	_, err := exec.LookPath("brew")
	return err == nil
}

// installWithBrew installs WireGuard using Homebrew
func installWithBrew() error {
	fmt.Println("Installing WireGuard via Homebrew...")
	fmt.Println("Running: brew install wireguard-tools")
	fmt.Println()

	cmd := exec.Command("brew", "install", "wireguard-tools")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install WireGuard: %w", err)
	}

	fmt.Println("\n✓ WireGuard installed successfully!")
	return nil
}

// promptInstallPlatform handles macOS-specific installation prompts
func promptInstallPlatform(result *PreflightResult) (bool, error) {
	if result.CanAutoInstall {
		fmt.Println("WireGuard can be installed automatically via Homebrew.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  [1] Install via Homebrew (recommended)")
		fmt.Println("      brew install wireguard-tools")
		fmt.Println()
		fmt.Println("  [2] Download WireGuard.app (GUI)")
		fmt.Println("      https://apps.apple.com/app/wireguard/id1451685025")
		fmt.Println()
		fmt.Println("  [3] Cancel and install manually")
		fmt.Println()

		fmt.Print("Choose option [1/2/3]: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1", "":
			if err := installWithBrew(); err != nil {
				return false, err
			}
			return true, nil
		case "2":
			fmt.Println("\nPlease install WireGuard.app from the App Store:")
			fmt.Println("  https://apps.apple.com/app/wireguard/id1451685025")
			fmt.Println("\nAfter installation, run this command again.")
			return false, nil
		default:
			fmt.Println("\nInstallation cancelled.")
			fmt.Println("To install manually, run:")
			fmt.Println("  brew install wireguard-tools")
			fmt.Println("\nOr download WireGuard.app from:")
			fmt.Println("  https://apps.apple.com/app/wireguard/id1451685025")
			return false, nil
		}
	}

	// Homebrew not available
	fmt.Println("To install WireGuard on macOS:")
	fmt.Println()
	fmt.Println("Option 1: Install Homebrew first, then WireGuard:")
	fmt.Println("  /bin/bash -c \"$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\"")
	fmt.Println("  brew install wireguard-tools")
	fmt.Println()
	fmt.Println("Option 2: Download WireGuard.app from the App Store:")
	fmt.Println("  https://apps.apple.com/app/wireguard/id1451685025")
	fmt.Println()
	fmt.Println("After installation, run this command again.")
	return false, nil
}

// getInstallInstructions returns macOS-specific installation instructions
func getInstallInstructions() string {
	if isBrewAvailable() {
		return `WireGuard is not installed.

To install via Homebrew:
  brew install wireguard-tools

Or download WireGuard.app from:
  https://apps.apple.com/app/wireguard/id1451685025`
	}

	return `WireGuard is not installed.

To install on macOS:
  1. Install Homebrew: /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  2. Install WireGuard: brew install wireguard-tools

Or download WireGuard.app from:
  https://apps.apple.com/app/wireguard/id1451685025`
}

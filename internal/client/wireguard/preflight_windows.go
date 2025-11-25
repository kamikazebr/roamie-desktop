//go:build windows
// +build windows

package wireguard

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// canAutoInstall checks if we can auto-install WireGuard on Windows
func canAutoInstall() bool {
	return isWingetAvailable()
}

// getInstallMethod returns the available installation method
func getInstallMethod() string {
	if isWingetAvailable() {
		return "winget"
	}
	return "manual"
}

// isWingetAvailable checks if winget is installed
func isWingetAvailable() bool {
	_, err := exec.LookPath("winget")
	return err == nil
}

// installWithWinget installs WireGuard using winget
func installWithWinget() error {
	fmt.Println("Installing WireGuard via winget...")
	fmt.Println("Running: winget install WireGuard.WireGuard")
	fmt.Println()

	cmd := exec.Command("winget", "install", "WireGuard.WireGuard", "--accept-source-agreements", "--accept-package-agreements")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install WireGuard: %w", err)
	}

	fmt.Println("\n✓ WireGuard installed successfully!")
	fmt.Println("\nNote: You may need to restart your terminal or computer for the WireGuard CLI tools to be available in PATH.")
	return nil
}

// promptInstallPlatform handles Windows-specific installation prompts
func promptInstallPlatform(result *PreflightResult) (bool, error) {
	if result.CanAutoInstall {
		fmt.Println("WireGuard can be installed automatically via winget.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  [1] Install via winget (recommended)")
		fmt.Println("      winget install WireGuard.WireGuard")
		fmt.Println()
		fmt.Println("  [2] Download installer from website")
		fmt.Println("      https://www.wireguard.com/install/")
		fmt.Println()
		fmt.Println("  [3] Cancel and install manually")
		fmt.Println()

		fmt.Print("Choose option [1/2/3]: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1", "":
			if err := installWithWinget(); err != nil {
				return false, err
			}
			return true, nil
		case "2":
			fmt.Println("\nPlease download and install WireGuard from:")
			fmt.Println("  https://www.wireguard.com/install/")
			fmt.Println("\nAfter installation, run this command again.")
			return false, nil
		default:
			fmt.Println("\nInstallation cancelled.")
			fmt.Println("To install manually:")
			fmt.Println("  winget install WireGuard.WireGuard")
			fmt.Println("\nOr download from:")
			fmt.Println("  https://www.wireguard.com/install/")
			return false, nil
		}
	}

	// Winget not available
	fmt.Println("To install WireGuard on Windows:")
	fmt.Println()
	fmt.Println("Option 1: Using winget (Windows Package Manager):")
	fmt.Println("  winget install WireGuard.WireGuard")
	fmt.Println()
	fmt.Println("Option 2: Download the installer:")
	fmt.Println("  https://www.wireguard.com/install/")
	fmt.Println()
	fmt.Println("After installation, run this command again.")
	return false, nil
}

// getInstallInstructions returns Windows-specific installation instructions
func getInstallInstructions() string {
	if isWingetAvailable() {
		return `WireGuard is not installed.

To install via winget:
  winget install WireGuard.WireGuard

Or download from:
  https://www.wireguard.com/install/`
	}

	return `WireGuard is not installed.

To install on Windows:
  1. Using winget: winget install WireGuard.WireGuard
  2. Or download from: https://www.wireguard.com/install/`
}

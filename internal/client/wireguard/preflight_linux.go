//go:build linux
// +build linux

package wireguard

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// canAutoInstall checks if we can auto-install WireGuard on Linux
func canAutoInstall() bool {
	// Check for apt (Debian/Ubuntu)
	if _, err := exec.LookPath("apt"); err == nil {
		return true
	}
	// Check for dnf (Fedora/RHEL)
	if _, err := exec.LookPath("dnf"); err == nil {
		return true
	}
	// Check for pacman (Arch)
	if _, err := exec.LookPath("pacman"); err == nil {
		return true
	}
	return false
}

// getInstallMethod returns the available installation method
func getInstallMethod() string {
	if _, err := exec.LookPath("apt"); err == nil {
		return "apt"
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return "dnf"
	}
	if _, err := exec.LookPath("pacman"); err == nil {
		return "pacman"
	}
	return "manual"
}

// installWithPackageManager installs WireGuard using the system package manager
func installWithPackageManager(method string) error {
	var cmd *exec.Cmd

	switch method {
	case "apt":
		fmt.Println("Installing WireGuard via apt...")
		fmt.Println("Running: sudo apt update && sudo apt install -y wireguard")
		fmt.Println()
		// Run apt update first
		updateCmd := exec.Command("sudo", "apt", "update")
		updateCmd.Stdout = os.Stdout
		updateCmd.Stderr = os.Stderr
		if err := updateCmd.Run(); err != nil {
			return fmt.Errorf("failed to update apt: %w", err)
		}
		cmd = exec.Command("sudo", "apt", "install", "-y", "wireguard")

	case "dnf":
		fmt.Println("Installing WireGuard via dnf...")
		fmt.Println("Running: sudo dnf install -y wireguard-tools")
		fmt.Println()
		cmd = exec.Command("sudo", "dnf", "install", "-y", "wireguard-tools")

	case "pacman":
		fmt.Println("Installing WireGuard via pacman...")
		fmt.Println("Running: sudo pacman -S --noconfirm wireguard-tools")
		fmt.Println()
		cmd = exec.Command("sudo", "pacman", "-S", "--noconfirm", "wireguard-tools")

	default:
		return fmt.Errorf("unknown package manager: %s", method)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install WireGuard: %w", err)
	}

	fmt.Println("\n✓ WireGuard installed successfully!")
	return nil
}

// promptInstallPlatform handles Linux-specific installation prompts
func promptInstallPlatform(result *PreflightResult) (bool, error) {
	method := getInstallMethod()

	if result.CanAutoInstall {
		var installCmd string
		switch method {
		case "apt":
			installCmd = "sudo apt install wireguard"
		case "dnf":
			installCmd = "sudo dnf install wireguard-tools"
		case "pacman":
			installCmd = "sudo pacman -S wireguard-tools"
		}

		fmt.Println("WireGuard can be installed automatically.")
		fmt.Println()
		fmt.Printf("  Command: %s\n", installCmd)
		fmt.Println()
		fmt.Print("Install now? [Y/n]: ")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "" || input == "y" || input == "yes" {
			if err := installWithPackageManager(method); err != nil {
				return false, err
			}
			return true, nil
		}

		fmt.Println("\nInstallation cancelled.")
		fmt.Printf("To install manually, run: %s\n", installCmd)
		return false, nil
	}

	// No known package manager
	fmt.Println("To install WireGuard on Linux:")
	fmt.Println()
	fmt.Println("  Debian/Ubuntu:")
	fmt.Println("    sudo apt update && sudo apt install wireguard")
	fmt.Println()
	fmt.Println("  Fedora/RHEL:")
	fmt.Println("    sudo dnf install wireguard-tools")
	fmt.Println()
	fmt.Println("  Arch Linux:")
	fmt.Println("    sudo pacman -S wireguard-tools")
	fmt.Println()
	fmt.Println("After installation, run this command again.")
	return false, nil
}

// getInstallInstructions returns Linux-specific installation instructions
func getInstallInstructions() string {
	method := getInstallMethod()

	switch method {
	case "apt":
		return `WireGuard is not installed.

To install on Debian/Ubuntu:
  sudo apt update && sudo apt install wireguard`

	case "dnf":
		return `WireGuard is not installed.

To install on Fedora/RHEL:
  sudo dnf install wireguard-tools`

	case "pacman":
		return `WireGuard is not installed.

To install on Arch Linux:
  sudo pacman -S wireguard-tools`

	default:
		return `WireGuard is not installed.

To install on Linux:
  Debian/Ubuntu: sudo apt install wireguard
  Fedora/RHEL:   sudo dnf install wireguard-tools
  Arch Linux:    sudo pacman -S wireguard-tools`
	}
}

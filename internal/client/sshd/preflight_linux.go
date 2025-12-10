//go:build linux
// +build linux

package sshd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kamikazebr/roamie-desktop/internal/client/ui"
)

// checkPlatform performs Linux-specific SSH daemon checks
func checkPlatform(result *PreflightResult) (*PreflightResult, error) {
	// Check if openssh-server is installed
	result.Installed = isSSHDInstalled()

	// Check if sshd service is running via systemctl
	if result.Installed {
		result.Running = isSSHDServiceRunning()
	}

	// Check if we can auto-install
	result.CanAutoInstall = canAutoInstallSSHD()

	return result, nil
}

// isSSHDInstalled checks if openssh-server is installed
func isSSHDInstalled() bool {
	// Check for sshd binary
	if _, err := exec.LookPath("sshd"); err == nil {
		return true
	}

	// Check via dpkg-query (Debian/Ubuntu) - more reliable than dpkg -l
	if _, err := exec.LookPath("dpkg-query"); err == nil {
		cmd := exec.Command("dpkg-query", "-W", "-f=${Status}", "openssh-server")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), "install ok installed") {
			return true
		}
	}

	// Check via rpm (Fedora/RHEL)
	if _, err := exec.LookPath("rpm"); err == nil {
		cmd := exec.Command("rpm", "-q", "openssh-server")
		if err := cmd.Run(); err == nil {
			return true
		}
	}

	// Check via pacman (Arch)
	if _, err := exec.LookPath("pacman"); err == nil {
		cmd := exec.Command("pacman", "-Q", "openssh")
		if err := cmd.Run(); err == nil {
			return true
		}
	}

	return false
}

// isSSHDServiceRunning checks if sshd service is active
func isSSHDServiceRunning() bool {
	// Try systemctl first (modern Linux)
	if _, err := exec.LookPath("systemctl"); err == nil {
		// Check both 'sshd' and 'ssh' service names
		for _, service := range []string{"sshd", "ssh"} {
			cmd := exec.Command("systemctl", "is-active", "--quiet", service)
			if err := cmd.Run(); err == nil {
				return true
			}
		}
	}

	// Fallback: check if sshd process is running
	cmd := exec.Command("pgrep", "-x", "sshd")
	if err := cmd.Run(); err == nil {
		return true
	}

	return false
}

// canAutoInstallSSHD checks if we can auto-install openssh-server
func canAutoInstallSSHD() bool {
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

// getPackageManager returns the available package manager
func getPackageManager() string {
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

// isRoot checks if the current process is running as root
func isRoot() bool {
	return os.Geteuid() == 0
}

// runCommand runs a command with sudo if not root
func runCommand(args ...string) *exec.Cmd {
	if isRoot() {
		return exec.Command(args[0], args[1:]...)
	}
	return exec.Command("sudo", args...)
}

// installSSHD installs openssh-server using the system package manager
func installSSHD(method string) error {
	var cmd *exec.Cmd

	switch method {
	case "apt":
		fmt.Println("Installing OpenSSH server via apt...")
		fmt.Println()
		// Run apt update first
		updateCmd := runCommand("apt", "update")
		updateCmd.Stdout = os.Stdout
		updateCmd.Stderr = os.Stderr
		if err := updateCmd.Run(); err != nil {
			return fmt.Errorf("failed to update apt: %w", err)
		}
		cmd = runCommand("apt", "install", "-y", "openssh-server")

	case "dnf":
		fmt.Println("Installing OpenSSH server via dnf...")
		fmt.Println()
		cmd = runCommand("dnf", "install", "-y", "openssh-server")

	case "pacman":
		fmt.Println("Installing OpenSSH server via pacman...")
		fmt.Println()
		cmd = runCommand("pacman", "-S", "--noconfirm", "openssh")

	default:
		return fmt.Errorf("unknown package manager: %s", method)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install openssh-server: %w", err)
	}

	fmt.Println("\n✓ OpenSSH server installed successfully!")
	return nil
}

// startSSHDService starts and enables the sshd service
func startSSHDService() error {
	fmt.Println("Starting SSH service...")

	// Determine service name (Debian uses 'ssh', others use 'sshd')
	serviceName := "sshd"
	if _, err := exec.LookPath("apt"); err == nil {
		serviceName = "ssh"
	}

	// Enable service
	enableCmd := runCommand("systemctl", "enable", serviceName)
	enableCmd.Stdout = os.Stdout
	enableCmd.Stderr = os.Stderr
	if err := enableCmd.Run(); err != nil {
		fmt.Printf("Warning: failed to enable service: %v\n", err)
	}

	// Start service
	startCmd := runCommand("systemctl", "start", serviceName)
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	if err := startCmd.Run(); err != nil {
		return fmt.Errorf("failed to start sshd: %w", err)
	}

	fmt.Println("✓ SSH service started!")
	return nil
}

// promptInstallPlatform handles Linux-specific installation prompts using TUI
func promptInstallPlatform(result *PreflightResult) (bool, error) {
	method := getPackageManager()

	// Case 1: Installed but not running
	if result.Installed && !result.Running {
		options := []ui.SelectOption{
			{Label: "Start SSH service now", Value: "start"},
			{Label: "Cancel", Value: "cancel"},
		}

		selected, err := ui.Select("OpenSSH is installed but not running", options)
		if err != nil {
			return false, err
		}

		switch selected {
		case 0: // Start
			if err := startSSHDService(); err != nil {
				return false, err
			}
			return true, nil
		default: // Cancel or abort
			return false, nil
		}
	}

	// Case 2: Not installed, can auto-install
	if result.CanAutoInstall {
		var installCmd, startCmd string
		switch method {
		case "apt":
			installCmd = "sudo apt install openssh-server"
			startCmd = "sudo systemctl enable --now ssh"
		case "dnf":
			installCmd = "sudo dnf install openssh-server"
			startCmd = "sudo systemctl enable --now sshd"
		case "pacman":
			installCmd = "sudo pacman -S openssh"
			startCmd = "sudo systemctl enable --now sshd"
		}

		options := []ui.SelectOption{
			{
				Label:       "Install and start now",
				Description: fmt.Sprintf("%s && %s", installCmd, startCmd),
				Value:       "install",
			},
			{Label: "Show manual instructions", Value: "manual"},
			{Label: "Cancel", Value: "cancel"},
		}

		selected, err := ui.Select("OpenSSH server can be installed automatically", options)
		if err != nil {
			return false, err
		}

		switch selected {
		case 0: // Install
			if err := installSSHD(method); err != nil {
				return false, err
			}
			if err := startSSHDService(); err != nil {
				return false, err
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

	// Case 3: Not installed, cannot auto-install
	options := []ui.SelectOption{
		{Label: "Show installation instructions", Value: "show"},
		{Label: "Cancel", Value: "cancel"},
	}

	selected, err := ui.Select("OpenSSH server is not installed", options)
	if err != nil {
		return false, err
	}

	switch selected {
	case 0: // Show instructions
		fmt.Println()
		fmt.Println(getInstallInstructions())
		fmt.Println()
		return false, nil
	default: // Cancel or abort
		return false, nil
	}
}

// getInstallInstructions returns Linux-specific installation instructions
func getInstallInstructions() string {
	method := getPackageManager()

	switch method {
	case "apt":
		return `SSH server is not available.

To install on Debian/Ubuntu:
  sudo apt install openssh-server
  sudo systemctl enable --now ssh`

	case "dnf":
		return `SSH server is not available.

To install on Fedora/RHEL:
  sudo dnf install openssh-server
  sudo systemctl enable --now sshd`

	case "pacman":
		return `SSH server is not available.

To install on Arch Linux:
  sudo pacman -S openssh
  sudo systemctl enable --now sshd`

	default:
		return `SSH server is not available.

To install on Linux:
  Debian/Ubuntu: sudo apt install openssh-server
  Fedora/RHEL:   sudo dnf install openssh-server
  Arch Linux:    sudo pacman -S openssh

Then enable and start the service:
  sudo systemctl enable --now sshd`
	}
}

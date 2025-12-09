//go:build windows
// +build windows

package sshd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kamikazebr/roamie-desktop/internal/client/ui"
)

// checkPlatform performs Windows-specific SSH daemon checks
func checkPlatform(result *PreflightResult) (*PreflightResult, error) {
	// Check if OpenSSH Server is installed
	result.Installed = isSSHDInstalled()

	// Check if sshd service is running
	if result.Installed {
		result.Running = isSSHDServiceRunning()
	}

	// Windows 10 1809+ can install OpenSSH via PowerShell
	result.CanAutoInstall = canAutoInstallSSHD()

	return result, nil
}

// isSSHDInstalled checks if OpenSSH Server is installed on Windows
func isSSHDInstalled() bool {
	// Check if sshd.exe exists
	cmd := exec.Command("where", "sshd")
	if err := cmd.Run(); err == nil {
		return true
	}

	// Check via PowerShell for optional feature
	psCmd := `Get-WindowsCapability -Online | Where-Object Name -like 'OpenSSH.Server*' | Select-Object -ExpandProperty State`
	cmd = exec.Command("powershell", "-Command", psCmd)
	output, err := cmd.Output()
	if err == nil && strings.Contains(string(output), "Installed") {
		return true
	}

	return false
}

// isSSHDServiceRunning checks if sshd service is running on Windows
func isSSHDServiceRunning() bool {
	cmd := exec.Command("powershell", "-Command", "(Get-Service sshd -ErrorAction SilentlyContinue).Status")
	output, err := cmd.Output()
	if err == nil && strings.Contains(string(output), "Running") {
		return true
	}
	return false
}

// canAutoInstallSSHD checks if we can auto-install OpenSSH Server
func canAutoInstallSSHD() bool {
	// Check if running as Administrator
	cmd := exec.Command("net", "session")
	if err := cmd.Run(); err != nil {
		return false // Not running as admin
	}

	// Check Windows version (need 1809+)
	psCmd := `(Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion').ReleaseId`
	cmd = exec.Command("powershell", "-Command", psCmd)
	output, err := cmd.Output()
	if err == nil {
		version := strings.TrimSpace(string(output))
		// 1809 and later support OpenSSH as optional feature
		if version >= "1809" {
			return true
		}
	}

	return false
}

// installSSHD installs OpenSSH Server on Windows
func installSSHD() error {
	fmt.Println("Installing OpenSSH Server...")
	fmt.Println("Running: Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0")
	fmt.Println()

	psCmd := `Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0`
	cmd := exec.Command("powershell", "-Command", psCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install OpenSSH Server: %w", err)
	}

	fmt.Println("\n✓ OpenSSH Server installed!")
	return nil
}

// startSSHDService starts the sshd service on Windows
func startSSHDService() error {
	fmt.Println("Starting and enabling SSH service...")

	// Set startup type to automatic
	psCmd := `Set-Service -Name sshd -StartupType Automatic`
	cmd := exec.Command("powershell", "-Command", psCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: failed to set startup type: %v\n", err)
	}

	// Start service
	psCmd = `Start-Service sshd`
	cmd = exec.Command("powershell", "-Command", psCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start sshd: %w", err)
	}

	// Configure firewall
	psCmd = `New-NetFirewallRule -Name 'OpenSSH-Server-In-TCP' -DisplayName 'OpenSSH Server (sshd)' -Enabled True -Direction Inbound -Protocol TCP -Action Allow -LocalPort 22 -ErrorAction SilentlyContinue`
	cmd = exec.Command("powershell", "-Command", psCmd)
	cmd.Run() // Ignore errors if rule already exists

	fmt.Println("✓ SSH service started!")
	return nil
}

// promptInstallPlatform handles Windows-specific installation prompts using TUI
func promptInstallPlatform(result *PreflightResult) (bool, error) {
	// Case 1: Installed but not running
	if result.Installed && !result.Running {
		options := []ui.SelectOption{
			{Label: "Start SSH service now", Value: "start"},
			{Label: "Cancel", Value: "cancel"},
		}

		selected, err := ui.Select("OpenSSH Server is installed but not running", options)
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
		options := []ui.SelectOption{
			{
				Label:       "Install and start now",
				Description: "Add-WindowsCapability + Start-Service sshd",
				Value:       "install",
			},
			{Label: "Show manual instructions", Value: "manual"},
			{Label: "Cancel", Value: "cancel"},
		}

		selected, err := ui.Select("OpenSSH Server can be installed automatically", options)
		if err != nil {
			return false, err
		}

		switch selected {
		case 0: // Install
			if err := installSSHD(); err != nil {
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

	selected, err := ui.Select("OpenSSH Server is not installed", options)
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

// getInstallInstructions returns Windows-specific installation instructions
func getInstallInstructions() string {
	return `SSH server is not available.

To install OpenSSH Server on Windows:

Option 1 - PowerShell (as Administrator):
  Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0
  Start-Service sshd
  Set-Service -Name sshd -StartupType Automatic

Option 2 - Settings:
  Settings > Apps > Optional features > Add a feature
  Search for 'OpenSSH Server' and install`
}

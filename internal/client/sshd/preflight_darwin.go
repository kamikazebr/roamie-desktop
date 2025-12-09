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
	fmt.Println("Habilitando Remote Login (SSH)...")
	fmt.Println("Executando: sudo systemsetup -setremotelogin on")
	fmt.Println()

	cmd := exec.Command("sudo", "systemsetup", "-setremotelogin", "on")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("falha ao habilitar Remote Login: %w", err)
	}

	fmt.Println("✓ Remote Login (SSH) habilitado!")
	return nil
}

// promptInstallPlatform handles macOS-specific installation prompts using TUI
func promptInstallPlatform(result *PreflightResult) (bool, error) {
	options := []ui.SelectOption{
		{
			Label:       "Habilitar SSH via linha de comando",
			Description: "sudo systemsetup -setremotelogin on",
			Value:       "enable",
		},
		{
			Label:       "Mostrar instruções manuais",
			Description: "System Preferences > Sharing > Remote Login",
			Value:       "manual",
		},
		{Label: "Cancelar", Value: "cancel"},
	}

	selected, err := ui.Select("SSH está instalado mas precisa ser habilitado", options)
	if err != nil {
		return false, err
	}

	switch selected {
	case 0: // Enable
		if err := enableSSHDService(); err != nil {
			fmt.Printf("\n   Falha ao habilitar SSH: %v\n", err)
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
	return `SSH server não está habilitado.

No macOS, SSH está instalado mas precisa ser habilitado:

Opção 1 - Linha de comando:
  sudo systemsetup -setremotelogin on

Opção 2 - System Preferences:
  1. Abra System Preferences
  2. Vá em Sharing
  3. Marque 'Remote Login'`
}

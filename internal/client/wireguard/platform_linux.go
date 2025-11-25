//go:build linux
// +build linux

package wireguard

import (
	"fmt"
	"os/exec"
)

// connectPlatform connects to WireGuard using Linux-specific methods (wg-quick)
func connectPlatform(interfaceName, configPath string) error {
	// Check if interface already exists
	checkCmd := exec.Command("wg", "show", interfaceName)
	if checkCmd.Run() == nil {
		// Interface exists, bring it down first
		fmt.Println("Bringing down existing interface...")
		downCmd := exec.Command("wg-quick", "down", interfaceName)
		if output, err := downCmd.CombinedOutput(); err != nil {
			// Ignore error if interface is already down
			fmt.Printf("Note: %s\n", string(output))
		}
	}

	// Bring interface up using wg-quick
	cmd := exec.Command("wg-quick", "up", interfaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to connect: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// disconnectPlatform disconnects from WireGuard using Linux-specific methods
func disconnectPlatform(interfaceName string) error {
	cmd := exec.Command("wg-quick", "down", interfaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to disconnect: %w\nOutput: %s", err, string(output))
	}
	return nil
}

//go:build darwin
// +build darwin

package wireguard

import (
	"fmt"
	"os/exec"
)

// connectPlatform connects to WireGuard on macOS
func connectPlatform(interfaceName, configPath string) error {
	// Check if wg-quick is available (user may have compiled WireGuard from source)
	if _, err := exec.LookPath("wg-quick"); err == nil {
		return connectWithWgQuick(interfaceName, configPath)
	}

	// Check if wg command is available (Homebrew wireguard-tools)
	if _, err := exec.LookPath("wg"); err == nil {
		return fmt.Errorf(`WireGuard tools found but wg-quick is not available.

On macOS, you have two options:
1. Install WireGuard from Homebrew:
   brew install wireguard-tools

2. Use WireGuard.app (GUI):
   Download from: https://www.wireguard.com/install/

Note: wg-quick may not be included in Homebrew installation.
You may need to manually configure the interface or use the GUI app.`)
	}

	// No WireGuard tools found
	return fmt.Errorf(`WireGuard is not installed on this system.

To install WireGuard on macOS:
1. Using Homebrew (recommended):
   brew install wireguard-tools

2. Using the official GUI app:
   Download from: https://www.wireguard.com/install/

After installation, try running this command again.`)
}

// connectWithWgQuick attempts to connect using wg-quick (if available)
func connectWithWgQuick(interfaceName, configPath string) error {
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

// disconnectPlatform disconnects from WireGuard on macOS
func disconnectPlatform(interfaceName string) error {
	// Check if wg-quick is available
	if _, err := exec.LookPath("wg-quick"); err != nil {
		return fmt.Errorf("wg-quick not found. Please use WireGuard.app to disconnect or install wireguard-tools via Homebrew")
	}

	cmd := exec.Command("wg-quick", "down", interfaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to disconnect: %w\nOutput: %s", err, string(output))
	}
	return nil
}

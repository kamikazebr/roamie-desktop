//go:build windows
// +build windows

package wireguard

import "fmt"

// connectPlatform connects to WireGuard on Windows
// Note: Windows requires the official WireGuard app to be installed
func connectPlatform(interfaceName, configPath string) error {
	return fmt.Errorf("Windows support requires the official WireGuard app. Import the config file at: %s", configPath)
}

// disconnectPlatform disconnects from WireGuard on Windows
func disconnectPlatform(interfaceName string) error {
	return fmt.Errorf("Windows support requires the official WireGuard app. Use the WireGuard GUI to disconnect")
}

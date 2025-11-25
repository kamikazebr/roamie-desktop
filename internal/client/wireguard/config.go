package wireguard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type WireGuardConfig struct {
	PrivateKey string
	Address    string
	ServerKey  string
	Endpoint   string
	AllowedIPs string
	DNS        string
}

func GenerateConfigFile(config WireGuardConfig) string {
	dns := config.DNS
	if dns == "" {
		dns = "1.1.1.1, 8.8.8.8"
	}

	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/32
DNS = %s

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = 25
`, config.PrivateKey, config.Address, dns, config.ServerKey, config.Endpoint, config.AllowedIPs)
}

// getWireGuardConfigDir returns the WireGuard configuration directory for the current platform
func getWireGuardConfigDir() string {
	switch runtime.GOOS {
	case "darwin":
		// macOS: Try common Homebrew paths
		// Apple Silicon (M1/M2)
		if _, err := os.Stat("/opt/homebrew/etc/wireguard"); err == nil {
			return "/opt/homebrew/etc/wireguard"
		}
		// Intel Mac
		if _, err := os.Stat("/usr/local/etc/wireguard"); err == nil {
			return "/usr/local/etc/wireguard"
		}
		// Fallback to /etc/wireguard (if user compiled from source)
		return "/etc/wireguard"
	default:
		// Linux and other Unix-like systems
		return "/etc/wireguard"
	}
}

// getWireGuardConfigPath returns the full path to the config file for the interface
func getWireGuardConfigPath(interfaceName string) string {
	return filepath.Join(getWireGuardConfigDir(), interfaceName+".conf")
}

// GetWireGuardConfigPath returns the full path to the config file for the interface (public version)
func GetWireGuardConfigPath(interfaceName string) string {
	return getWireGuardConfigPath(interfaceName)
}

func SaveConfig(configContent string, interfaceName string) (string, error) {
	configPath := getWireGuardConfigPath(interfaceName)

	// Save to WireGuard config directory (requires sudo)
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return "", fmt.Errorf("failed to save config to %s: %w\nHint: Try running 'device add' with sudo", configPath, err)
	}

	return configPath, nil
}

func Connect(interfaceName string, wgConfig WireGuardConfig) error {
	// Generate config from provided device info
	config := GenerateConfigFile(wgConfig)

	// Save config to platform-specific WireGuard directory
	configPath := getWireGuardConfigPath(interfaceName)
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		return fmt.Errorf("failed to write config to %s: %w\nHint: Run with sudo", configPath, err)
	}

	// Connect using platform-specific method
	return connectPlatform(interfaceName, configPath)
}

func Disconnect(interfaceName string) error {
	return disconnectPlatform(interfaceName)
}

func GetStatus(interfaceName string) (string, error) {
	// Try without sudo first
	cmd := exec.Command("wg", "show", interfaceName)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), nil
	}

	// If failed, try with sudo
	cmd = exec.Command("sudo", "wg", "show", interfaceName)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get status (try running: sudo roamie status): %w\nOutput: %s", err, string(output))
	}
	return string(output), nil
}

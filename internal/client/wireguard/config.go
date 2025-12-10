package wireguard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	// Only set DNS for full tunnel (0.0.0.0/0)
	// Split tunnel uses system DNS which avoids systemd-resolved conflicts on Ubuntu
	dnsLine := ""
	if strings.Contains(config.AllowedIPs, "0.0.0.0/0") {
		dns := config.DNS
		if dns == "" {
			dns = "1.1.1.1, 8.8.8.8"
		}
		dnsLine = fmt.Sprintf("DNS = %s\n", dns)
	}

	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/32
%s
[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = 25
`, config.PrivateKey, config.Address, dnsLine, config.ServerKey, config.Endpoint, config.AllowedIPs)
}

// getWireGuardConfigDir returns the WireGuard configuration directory for the current platform
func getWireGuardConfigDir() string {
	switch runtime.GOOS {
	case "darwin":
		// macOS: Try common Homebrew paths
		// Apple Silicon (M1/M2) - check if directory exists first
		if _, err := os.Stat("/opt/homebrew/etc/wireguard"); err == nil {
			return "/opt/homebrew/etc/wireguard"
		}
		// Intel Mac - check if directory exists
		if _, err := os.Stat("/usr/local/etc/wireguard"); err == nil {
			return "/usr/local/etc/wireguard"
		}
		// Check if /opt/homebrew exists (Apple Silicon) - prefer this path
		if _, err := os.Stat("/opt/homebrew"); err == nil {
			return "/opt/homebrew/etc/wireguard"
		}
		// Check if /usr/local exists (Intel Mac) - prefer this path
		if _, err := os.Stat("/usr/local"); err == nil {
			return "/usr/local/etc/wireguard"
		}
		// Fallback to /etc/wireguard (if user compiled from source)
		return "/etc/wireguard"

	case "windows":
		// Windows: WireGuard stores configs in Program Files or user's AppData
		// Check for WireGuard installation directory
		programFiles := os.Getenv("ProgramFiles")
		if programFiles != "" {
			wgPath := filepath.Join(programFiles, "WireGuard", "Data", "Configurations")
			if _, err := os.Stat(wgPath); err == nil {
				return wgPath
			}
		}
		// Fallback to user's AppData
		appData := os.Getenv("LOCALAPPDATA")
		if appData != "" {
			return filepath.Join(appData, "WireGuard", "Configurations")
		}
		// Last resort fallback
		return "C:\\Program Files\\WireGuard\\Data\\Configurations"

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
	// Ensure config directory exists
	if err := EnsureConfigDir(); err != nil {
		return "", fmt.Errorf("failed to prepare WireGuard config directory: %w", err)
	}

	configPath := getWireGuardConfigPath(interfaceName)

	// Save to WireGuard config directory (requires sudo)
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return "", fmt.Errorf("failed to save config to %s: %w\nHint: Try running 'device add' with sudo", configPath, err)
	}

	return configPath, nil
}

func Connect(interfaceName string, wgConfig WireGuardConfig) error {
	// Ensure config directory exists
	if err := EnsureConfigDir(); err != nil {
		return fmt.Errorf("failed to prepare WireGuard config directory: %w", err)
	}

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

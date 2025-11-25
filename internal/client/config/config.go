package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kamikazebr/roamie-desktop/pkg/utils"
)

const (
	ConfigFile = "config.json"
)

// GetConfigDir returns the config directory path for the current user
// When running with sudo, it returns the actual user's home directory (not /root)
func GetConfigDir() (string, error) {
	_, home, err := utils.GetActualUser()
	if err != nil {
		return "", fmt.Errorf("failed to get user directory: %w", err)
	}

	return filepath.Join(home, ".roamie"), nil
}

type Config struct {
	// Authentication
	ServerURL       string        `json:"server_url"`
	DeviceID        string        `json:"device_id"`
	JWT             string        `json:"jwt"`
	RefreshToken    string        `json:"refresh_token"`
	ExpiresAt       time.Time     `json:"expires_at"`
	CreatedAt       time.Time     `json:"created_at"`
	SSHSyncEnabled  bool          `json:"ssh_sync_enabled"`
	SSHSyncInterval time.Duration `json:"ssh_sync_interval"`

	// WireGuard Device Info
	DeviceName      string `json:"device_name,omitempty"`
	PrivateKey      string `json:"private_key,omitempty"`
	PublicKey       string `json:"public_key,omitempty"`
	VpnIP           string `json:"vpn_ip,omitempty"`
	Subnet          string `json:"subnet,omitempty"`
	ServerPublicKey string `json:"server_public_key,omitempty"`
	ServerEndpoint  string `json:"server_endpoint,omitempty"`
	AllowedIPs      string `json:"allowed_ips,omitempty"`
}

// DefaultConfig returns a config with default values
func DefaultConfig() *Config {
	return &Config{
		SSHSyncEnabled:  true,          // Enable SSH sync by default
		SSHSyncInterval: 1 * time.Hour, // Default 1 hour
	}
}

// Load loads the configuration from disk
func Load() (*Config, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(configDir, ConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No config exists
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply defaults for SSH sync if not set
	if config.SSHSyncInterval == 0 {
		config.SSHSyncInterval = 1 * time.Hour
	}
	// SSHSyncEnabled defaults to false if not set (zero value for bool)
	// User must explicitly enable it

	return &config, nil
}

// Save saves the configuration to disk
func (c *Config) Save() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, ConfigFile)

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with restrictive permissions (only owner can read)
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	// Fix ownership if running under sudo
	utils.FixFileOwnership(configDir)
	utils.FixFileOwnership(configPath)

	return nil
}

// Delete deletes the configuration file
func Delete() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	configPath := filepath.Join(configDir, ConfigFile)
	return os.Remove(configPath)
}

// IsExpired returns true if the JWT has expired
func (c *Config) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}

// ExpiresIn returns the duration until the JWT expires
func (c *Config) ExpiresIn() time.Duration {
	return time.Until(c.ExpiresAt)
}

// MigrateFromLegacyStorage migrates device info from ~/.roamie-vpn/devices/ to ~/.roamie/config.json
// This is called automatically when loading config
func MigrateFromLegacyStorage() error {
	_, home, err := utils.GetActualUser()
	if err != nil {
		return nil // Skip migration if can't get home
	}

	legacyDir := filepath.Join(home, ".roamie-vpn")
	devicesDir := filepath.Join(legacyDir, "devices")

	// Check if legacy storage exists
	entries, err := os.ReadDir(devicesDir)
	if err != nil {
		return nil // No legacy storage, nothing to migrate
	}

	// Check if new config already exists
	cfg, err := Load()
	if err != nil || cfg == nil {
		return nil // No config to migrate to
	}

	// If config already has device info, skip migration
	if cfg.PrivateKey != "" {
		return nil
	}

	// Find first device file
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		// Read device file
		devicePath := filepath.Join(devicesDir, entry.Name())
		data, err := os.ReadFile(devicePath)
		if err != nil {
			continue
		}

		// Parse device info
		var deviceInfo map[string]interface{}
		if err := json.Unmarshal(data, &deviceInfo); err != nil {
			continue
		}

		// Merge into config
		if deviceName, ok := deviceInfo["device_name"].(string); ok {
			cfg.DeviceName = deviceName
		}
		if privateKey, ok := deviceInfo["private_key"].(string); ok {
			cfg.PrivateKey = privateKey
		}
		if publicKey, ok := deviceInfo["public_key"].(string); ok {
			cfg.PublicKey = publicKey
		}
		if vpnIP, ok := deviceInfo["vpn_ip"].(string); ok {
			cfg.VpnIP = vpnIP
		}
		if subnet, ok := deviceInfo["subnet"].(string); ok {
			cfg.Subnet = subnet
		}
		if serverPublicKey, ok := deviceInfo["server_public_key"].(string); ok {
			cfg.ServerPublicKey = serverPublicKey
		}
		if serverEndpoint, ok := deviceInfo["server_endpoint"].(string); ok {
			cfg.ServerEndpoint = serverEndpoint
		}
		if allowedIPs, ok := deviceInfo["allowed_ips"].(string); ok {
			cfg.AllowedIPs = allowedIPs
		}

		// Save merged config
		if err := cfg.Save(); err != nil {
			return err
		}

		// Delete legacy directory
		os.RemoveAll(legacyDir)

		return nil
	}

	return nil
}

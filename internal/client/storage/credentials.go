package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/kamikazebr/roamie-desktop/pkg/utils"
)

type Credentials struct {
	Token     string `json:"token"`
	Email     string `json:"email"`
	ExpiresAt string `json:"expires_at"`
}

type DeviceInfo struct {
	DeviceID        string `json:"device_id"`
	DeviceName      string `json:"device_name"`
	PrivateKey      string `json:"private_key"`
	PublicKey       string `json:"public_key"`
	VpnIP           string `json:"vpn_ip"`
	Subnet          string `json:"subnet"`
	ServerPublicKey string `json:"server_public_key"`
	ServerEndpoint  string `json:"server_endpoint"`
	AllowedIPs      string `json:"allowed_ips"`
}

func getConfigDir() (string, error) {
	var home string

	// Check if running under sudo
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		// Running as sudo, get the real user's home directory
		// This works cross-platform (Linux: /home/user, macOS: /Users/user)
		u, err := user.Lookup(sudoUser)
		if err == nil {
			home = u.HomeDir
		} else {
			// Fallback to UserHomeDir if lookup fails
			home, err = os.UserHomeDir()
			if err != nil {
				return "", err
			}
		}
	} else {
		// Normal operation
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}

	configDir := filepath.Join(home, ".roamie-desktop")
	if err := utils.MkdirAllWithOwnership(configDir, 0700); err != nil {
		return "", err
	}
	return configDir, nil
}

func SaveCredentials(creds *Credentials) error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	filePath := filepath.Join(configDir, "credentials.json")
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return utils.WriteFileWithOwnership(filePath, data, 0600)
}

func LoadCredentials() (*Credentials, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(configDir, "credentials.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not logged in. Run 'roamie login' first")
		}
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

func DeleteCredentials() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	filePath := filepath.Join(configDir, "credentials.json")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func SaveDeviceInfo(device *DeviceInfo) error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	devicesDir := filepath.Join(configDir, "devices")
	if err := utils.MkdirAllWithOwnership(devicesDir, 0700); err != nil {
		return err
	}

	filePath := filepath.Join(devicesDir, fmt.Sprintf("%s.json", device.DeviceID))
	data, err := json.MarshalIndent(device, "", "  ")
	if err != nil {
		return err
	}

	return utils.WriteFileWithOwnership(filePath, data, 0600)
}

func LoadDeviceInfo(deviceID string) (*DeviceInfo, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(configDir, "devices", fmt.Sprintf("%s.json", deviceID))
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var device DeviceInfo
	if err := json.Unmarshal(data, &device); err != nil {
		return nil, err
	}

	return &device, nil
}

func ListDevices() ([]DeviceInfo, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	devicesDir := filepath.Join(configDir, "devices")
	entries, err := os.ReadDir(devicesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []DeviceInfo{}, nil
		}
		return nil, err
	}

	var devices []DeviceInfo
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(devicesDir, entry.Name()))
		if err != nil {
			continue
		}

		var device DeviceInfo
		if err := json.Unmarshal(data, &device); err != nil {
			continue
		}

		devices = append(devices, device)
	}

	return devices, nil
}

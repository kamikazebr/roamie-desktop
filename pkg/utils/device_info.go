package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// DetectOS returns the operating system type in a standardized format
// Returns: "linux", "macos", "windows", "freebsd", "android", "ios"
func DetectOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macos"
	case "linux":
		// Could be Linux or Android, but we'll default to linux
		// Android detection would require additional checks
		return "linux"
	case "windows":
		return "windows"
	case "freebsd":
		return "freebsd"
	default:
		return runtime.GOOS
	}
}

// GetHardwareID generates a stable 8-character hardware identifier
// Uses machine-id on Linux, or falls back to hostname hash
// Returns "00000000" if unable to determine a hardware ID
func GetHardwareID() string {
	// Try machine-id first (Linux/systemd)
	if id, err := getMachineID(); err == nil && id != "" {
		return id[:8]
	}

	// Fallback: hash of hostname
	hostname, err := os.Hostname()
	if err != nil {
		// Last resort fallback
		return "00000000"
	}

	hash := sha256.Sum256([]byte(hostname))
	return hex.EncodeToString(hash[:4]) // 8 hex chars from 4 bytes
}

// getMachineID attempts to read the machine ID from common locations
func getMachineID() (string, error) {
	// Try systemd machine-id (most Linux distros)
	paths := []string{
		"/etc/machine-id",
		"/var/lib/dbus/machine-id",
	}

	for _, path := range paths {
		if data, err := os.ReadFile(path); err == nil {
			id := strings.TrimSpace(string(data))
			if len(id) >= 8 {
				return id, nil
			}
		}
	}

	return "", fmt.Errorf("machine-id not found")
}

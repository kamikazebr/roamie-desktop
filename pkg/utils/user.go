package utils

import (
	"os"
	"os/user"
	"strconv"
)

// GetActualUser returns the actual user info, automatically detecting SUDO_USER
// when running with sudo. This ensures operations use the real user's home directory
// and username, not root's, even when invoked via sudo.
//
// Returns:
//   - username: The actual user's username
//   - homeDir: The actual user's home directory path
//   - err: Error if user detection fails
//
// Example:
//
//	When running: sudo ./roamie login
//	Returns: ("felipenovaesrocha", "/home/felipenovaesrocha", nil)
//	Not: ("root", "/root", nil)
func GetActualUser() (username, homeDir string, err error) {
	// Check if running under sudo (SUDO_USER is automatically set by sudo)
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		u, err := user.Lookup(sudoUser)
		if err == nil {
			return u.Username, u.HomeDir, nil
		}
		// If lookup fails, fall through to standard method
	}

	// Not running under sudo, or SUDO_USER lookup failed
	// Get home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	// Get username (best effort)
	u, err := user.Current()
	if err != nil {
		// Can't get username, but we have home dir
		return "", home, nil
	}

	return u.Username, home, nil
}

// FixFileOwnership changes file/directory ownership to actual user when running under sudo.
// This ensures files created by root (when running with sudo) are owned by the actual user.
//
// Returns nil if not running under sudo or if ownership cannot be changed (silently ignores errors
// to avoid breaking functionality when ownership fixing is not critical).
func FixFileOwnership(path string) error {
	// Only needed when running under sudo
	sudoUser := os.Getenv("SUDO_USER")
	if sudoUser == "" {
		return nil // Not running under sudo, nothing to do
	}

	// Get actual user's UID/GID
	u, err := user.Lookup(sudoUser)
	if err != nil {
		return nil // Can't lookup user, skip (non-critical)
	}

	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)

	// Change ownership to actual user
	return os.Chown(path, uid, gid)
}

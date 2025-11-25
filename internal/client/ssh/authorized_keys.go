package ssh

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kamikazebr/roamie-desktop/pkg/utils"
	"golang.org/x/crypto/ssh"
)

const (
	RoamieStartMarker = "# >>> ROAMIE SSH KEYS - DO NOT EDIT THIS SECTION >>>"
	RoamieEndMarker   = "# <<< ROAMIE SSH KEYS - END <<<"
)

// AuthorizedKeysManager manages the ~/.ssh/authorized_keys file
type AuthorizedKeysManager struct {
	filePath string
}

// NewAuthorizedKeysManager creates a new authorized_keys manager
func NewAuthorizedKeysManager() (*AuthorizedKeysManager, error) {
	_, home, err := utils.GetActualUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	filePath := filepath.Join(home, ".ssh", "authorized_keys")

	return &AuthorizedKeysManager{
		filePath: filePath,
	}, nil
}

// ValidateSSHKey validates that a string is a valid SSH public key
func ValidateSSHKey(key string) error {
	_, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key))
	if err != nil {
		return fmt.Errorf("invalid SSH key format: %w", err)
	}
	return nil
}

// ReadKeys reads the authorized_keys file and returns Roamie keys and other keys separately
func (m *AuthorizedKeysManager) ReadKeys() (roamieKeys []string, otherKeys []string, err error) {
	// Check if file exists
	if _, err := os.Stat(m.filePath); os.IsNotExist(err) {
		// File doesn't exist yet, return empty slices
		return []string{}, []string{}, nil
	}

	file, err := os.Open(m.filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open authorized_keys: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inRoamieSection := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Check for section markers
		if line == RoamieStartMarker {
			inRoamieSection = true
			continue
		}
		if line == RoamieEndMarker {
			inRoamieSection = false
			continue
		}

		// Skip empty lines and comments (except inside Roamie section)
		if line == "" || (!inRoamieSection && strings.HasPrefix(line, "#")) {
			if !inRoamieSection {
				otherKeys = append(otherKeys, scanner.Text()) // Preserve original formatting
			}
			continue
		}

		// Add key to appropriate section
		if inRoamieSection {
			roamieKeys = append(roamieKeys, line)
		} else {
			otherKeys = append(otherKeys, scanner.Text()) // Preserve original formatting
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading authorized_keys: %w", err)
	}

	return roamieKeys, otherKeys, nil
}

// WriteKeys writes the authorized_keys file with Roamie section and other keys
func (m *AuthorizedKeysManager) WriteKeys(roamieKeys []string, otherKeys []string) error {
	// Validate all Roamie keys before writing
	for _, key := range roamieKeys {
		if err := ValidateSSHKey(key); err != nil {
			return fmt.Errorf("invalid key rejected: %w", err)
		}
	}

	// Create backup before modifying
	if err := m.createBackup(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Create .ssh directory if it doesn't exist
	sshDir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Build file content
	var lines []string

	// Add other keys first (preserve user's manual entries)
	lines = append(lines, otherKeys...)

	// Add Roamie section if there are keys
	if len(roamieKeys) > 0 {
		// Add blank line before Roamie section if there are other keys
		if len(otherKeys) > 0 {
			lines = append(lines, "")
		}

		lines = append(lines, RoamieStartMarker)
		lines = append(lines, roamieKeys...)
		lines = append(lines, RoamieEndMarker)
	}

	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n" // Ensure trailing newline
	}

	// Write to temporary file first (atomic write)
	tmpFile := m.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, m.filePath); err != nil {
		os.Remove(tmpFile) // Clean up temp file
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Ensure correct permissions
	if err := os.Chmod(m.filePath, 0600); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
}

// UpdateRoamieKeys updates only the Roamie section, preserving other keys
func (m *AuthorizedKeysManager) UpdateRoamieKeys(newRoamieKeys []string) (added []string, removed []string, err error) {
	// Read current keys
	currentRoamieKeys, otherKeys, err := m.ReadKeys()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read current keys: %w", err)
	}

	// Calculate diff
	added, removed = diffKeys(currentRoamieKeys, newRoamieKeys)

	// Write updated keys
	if err := m.WriteKeys(newRoamieKeys, otherKeys); err != nil {
		return nil, nil, fmt.Errorf("failed to write keys: %w", err)
	}

	return added, removed, nil
}

// createBackup creates a timestamped backup of authorized_keys
func (m *AuthorizedKeysManager) createBackup() error {
	// Check if file exists
	if _, err := os.Stat(m.filePath); os.IsNotExist(err) {
		// Nothing to backup
		return nil
	}

	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.backup.%s", m.filePath, timestamp)

	content, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	return os.WriteFile(backupPath, content, 0600)
}

// diffKeys calculates which keys were added and removed
func diffKeys(oldKeys, newKeys []string) (added []string, removed []string) {
	oldMap := make(map[string]bool)
	newMap := make(map[string]bool)

	for _, key := range oldKeys {
		oldMap[key] = true
	}
	for _, key := range newKeys {
		newMap[key] = true
	}

	// Find added keys
	for _, key := range newKeys {
		if !oldMap[key] {
			added = append(added, key)
		}
	}

	// Find removed keys
	for _, key := range oldKeys {
		if !newMap[key] {
			removed = append(removed, key)
		}
	}

	return added, removed
}

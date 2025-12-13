package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kamikazebr/roamie-desktop/pkg/utils"
)

// BackupSettings creates a backup of the settings.json file
func BackupSettings(settingsPath string) string {
	// Only backup if file exists
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		return ""
	}

	// Create backup with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupPath := fmt.Sprintf("%s.backup_%s", settingsPath, timestamp)

	// Copy original file
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return ""
	}

	if err := utils.WriteFileWithOwnership(backupPath, data, 0644); err != nil {
		return ""
	}

	return backupPath
}

// ListBackups lists all available backups
func ListBackups(settingsPath string) ([]string, error) {
	pattern := settingsPath + ".backup_*"
	backups, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	sort.Strings(backups)
	return backups, nil
}

// RestoreFromBackup restores configuration from a backup
func RestoreFromBackup(backupPath, settingsPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	return utils.WriteFileWithOwnership(settingsPath, data, 0644)
}

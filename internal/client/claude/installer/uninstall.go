package installer

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kamikazebr/roamie-desktop/pkg/utils"
)

// UninstallHooks removes Claude Code hooks from settings.json
func UninstallHooks(settingsPath string) error {
	// Read settings
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return fmt.Errorf("failed to read settings: %w", err)
	}

	settings := make(map[string]interface{})
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("invalid JSON in settings: %w", err)
	}

	// Remove ONLY Stop and Notification (preserve rest)
	if hooks, ok := settings["hooks"].(map[string]interface{}); ok {
		delete(hooks, "Stop")
		delete(hooks, "Notification")
	}

	// Write back
	data, err = json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	return utils.WriteFileWithOwnership(settingsPath, data, 0644)
}

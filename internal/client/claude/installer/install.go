package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InstallHooks installs Claude Code hooks into settings.json
func InstallHooks(settingsPath, roamiePath string) error {
	// Read existing settings or create empty
	settings := make(map[string]interface{})
	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("invalid JSON in settings: %w", err)
		}
	}

	// Prepare hooks with ABSOLUTE path to binary
	hookCmd := fmt.Sprintf("%s claude-hooks", roamiePath)
	newHooks := map[string]interface{}{
		"Stop": []map[string]interface{}{
			{"hooks": []map[string]interface{}{
				{"type": "command", "command": hookCmd, "timeout": 5},
			}},
		},
		"Notification": []map[string]interface{}{
			{"hooks": []map[string]interface{}{
				{"type": "command", "command": hookCmd, "timeout": 5},
			}},
		},
	}

	// Merge PRESERVING everything that exists
	if settings["hooks"] == nil {
		settings["hooks"] = newHooks
	} else {
		hooks, ok := settings["hooks"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid hooks format in settings")
		}
		// Update only Stop and Notification
		for k, v := range newHooks {
			hooks[k] = v
		}
	}

	// Write back
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return os.WriteFile(settingsPath, data, 0644)
}

package hooks

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kamikazebr/roamie-desktop/internal/client/claude/logger"
	"github.com/kamikazebr/roamie-desktop/internal/client/claude/notifier"
	"github.com/kamikazebr/roamie-desktop/pkg/claude"
)

// Run is the main dispatcher for Claude Code hooks
func Run() error {
	// Initialize logger
	if err := logger.Init(); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer logger.Close()

	logger.Info("Claude hooks: Processing event")

	// Read stdin
	var input map[string]interface{}
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		logger.Error("Failed to decode input: %v", err)
		return err
	}

	// Log input for debugging
	inputJSON, _ := json.Marshal(input)
	logger.Info("Input: %s", string(inputJSON))

	// Detect event (Claude Code sends hook_event_name)
	eventName, ok := input["hook_event_name"].(string)
	if !ok {
		logger.Error("Missing hook_event_name in input")
		return fmt.Errorf("missing hook_event_name")
	}

	logger.Info("Event: %s", eventName)

	// Create notifier
	n := notifier.New()

	// Dispatch to appropriate handler
	switch claude.HookEvent(eventName) {
	case claude.HookEventStop:
		return handleStop(input, n)
	case claude.HookEventNotification:
		return handleNotification(input, n)
	default:
		logger.Warning("Unknown event: %s", eventName)
		return nil
	}
}

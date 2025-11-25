package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kamikazebr/roamie-desktop/internal/client/claude/logger"
	"github.com/kamikazebr/roamie-desktop/internal/client/claude/notifier"
)

func handleNotification(input map[string]interface{}, n notifier.Notifier) error {
	message, _ := input["message"].(string)
	title, _ := input["title"].(string)
	notifType, _ := input["notification_type"].(string)
	severity, _ := input["severity"].(string)
	cwd, _ := input["cwd"].(string)

	// Extract project name from cwd
	projectName := ""
	if cwd != "" {
		projectName = filepath.Base(cwd)
	}

	logger.Info("Notification [%s]: %s - %s", notifType, title, message)

	// Define urgency and emoji based on notification type
	urgency := notifier.UrgencyNormal
	emoji := "ℹ️"

	switch notifType {
	case "permission_prompt":
		urgency = notifier.UrgencyCritical
		emoji = "🔐"
		title = "Permission Required"
		message = "Needs attention"

	case "idle_prompt":
		urgency = notifier.UrgencyCritical
		emoji = "💤"
		title = "Still waiting"
		message = "Waiting for input"

	case "auth_success":
		urgency = notifier.UrgencyNormal
		emoji = "✅"
		title = "Auth Success"

	case "elicitation_dialog":
		urgency = notifier.UrgencyNormal
		emoji = "❓"
		title = "Input Required"

	default:
		// Fallback to severity if notification_type not recognized
		if severity != "" {
			switch severity {
			case "error":
				urgency = notifier.UrgencyCritical
				emoji = "❌"
			case "warning":
				urgency = notifier.UrgencyNormal
				emoji = "⚠️"
			case "info":
				urgency = notifier.UrgencyLow
				emoji = "ℹ️"
			}
		}
		if title == "" {
			title = "Notification"
		}
	}

	// Build notification title with project name
	notifTitle := fmt.Sprintf("%s CC - %s", emoji, title)
	if projectName != "" {
		notifTitle = fmt.Sprintf("%s CC - %s [%s]", emoji, title, projectName)
	}

	// Send notification
	err := n.Send(
		notifTitle,
		message,
		urgency,
	)

	if err != nil {
		logger.Error("Failed to send notification: %v", err)
	}

	// Return empty JSON (allows continuation)
	return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{})
}

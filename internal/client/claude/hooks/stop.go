package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kamikazebr/roamie-desktop/internal/client/claude/logger"
	"github.com/kamikazebr/roamie-desktop/internal/client/claude/notifier"
)

// getTmuxSessions detects tmux sessions in the given directory
// Returns empty string if tmux not available or no sessions found
func getTmuxSessions(cwd string) string {
	// Check if tmux is available
	if _, err := exec.LookPath("tmux"); err != nil {
		return "" // tmux not installed
	}

	// Check if tmux is running
	checkCmd := exec.Command("tmux", "list-sessions")
	if err := checkCmd.Run(); err != nil {
		return "" // tmux not running
	}

	// Get sessions in this cwd
	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`tmux list-panes -a -F "#{session_name}:#{pane_current_path}" | grep ":%s$" | cut -d: -f1 | sort -u | tr '\n' ',' | sed 's/,$//'`, cwd))

	output, err := cmd.Output()
	if err != nil {
		return "" // No sessions or error
	}

	return strings.TrimSpace(string(output))
}

func handleStop(input map[string]interface{}, n notifier.Notifier) error {
	cwd, _ := input["cwd"].(string)
	projectName := filepath.Base(cwd)

	// Detect tmux sessions
	sessions := getTmuxSessions(cwd)
	sessionInfo := ""
	if sessions != "" {
		sessionInfo = fmt.Sprintf(" [tmux: %s]", sessions)
		logger.Info("Stop event for project: %s%s", projectName, sessionInfo)
	} else {
		logger.Info("Stop event for project: %s", projectName)
	}

	// Build notification message
	message := fmt.Sprintf("Waiting for response\nProject: %s", projectName)
	if sessions != "" {
		message = fmt.Sprintf("Waiting for response\nProject: %s\nSessions: %s", projectName, sessions)
	}

	// Build notification title with project name
	notifTitle := "💬 CC - Response Ready"
	if projectName != "" {
		notifTitle = fmt.Sprintf("💬 CC - Response Ready [%s]", projectName)
	}

	// Send notification
	err := n.Send(
		notifTitle,
		message,
		notifier.UrgencyNormal,
	)

	if err != nil {
		logger.Error("Failed to send notification: %v", err)
	}

	// Return correct JSON for Stop event
	response := map[string]interface{}{"continue": true}
	return json.NewEncoder(os.Stdout).Encode(response)
}

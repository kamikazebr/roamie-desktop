package notifier

import (
	"os/exec"
)

// LinuxNotifier implementa notificações para Linux usando notify-send
type LinuxNotifier struct{}

// Send envia uma notificação desktop no Linux
func (n *LinuxNotifier) Send(title, message string, urgency Urgency) error {
	urgencyStr := "normal"
	switch urgency {
	case UrgencyCritical:
		urgencyStr = "critical"
	case UrgencyLow:
		urgencyStr = "low"
	}

	cmd := exec.Command("notify-send",
		"--urgency="+urgencyStr,
		"--icon=dialog-information",
		"--app-name=Claude Code",
		title,
		message,
	)

	return cmd.Run()
}

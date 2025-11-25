package claude

// HookEvent representa os tipos de eventos do Claude Code
type HookEvent string

const (
	HookEventStop         HookEvent = "Stop"
	HookEventNotification HookEvent = "Notification"
)

// StopInput representa o input do evento Stop
type StopInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	PermissionMode string `json:"permission_mode"`
	StopHookActive bool   `json:"stop_hook_active"`
}

// NotificationInput representa o input do evento Notification
type NotificationInput struct {
	SessionID        string `json:"session_id"`
	Title            string `json:"title,omitempty"`
	Message          string `json:"message"`
	Severity         string `json:"severity,omitempty"`          // info, warning, error
	NotificationType string `json:"notification_type,omitempty"` // permission_prompt, idle_prompt, auth_success, etc
}

// StopResponse representa o output do evento Stop
type StopResponse struct {
	Continue bool `json:"continue"`
}

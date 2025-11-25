package notifier

import "runtime"

// Urgency define o nível de urgência da notificação
type Urgency int

const (
	UrgencyLow Urgency = iota
	UrgencyNormal
	UrgencyCritical
)

// Notifier é a interface para enviar notificações desktop
type Notifier interface {
	Send(title, message string, urgency Urgency) error
}

// New cria um notificador apropriado para o OS atual
func New() Notifier {
	switch runtime.GOOS {
	case "linux":
		return &LinuxNotifier{}
	// Futuro: darwin, windows
	default:
		return &LinuxNotifier{} // Fallback
	}
}

package terminal

import (
	"fmt"
	"log"
	"sync"
)

// SessionManager manages multiple terminal sessions
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// CreateSession creates a new terminal session
func (m *SessionManager) CreateSession(id string, rows, cols int, encryptionKey []byte) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if session already exists
	if _, exists := m.sessions[id]; exists {
		return nil, fmt.Errorf("session %s already exists", id)
	}

	// Create new session
	session, err := NewSession(id, rows, cols, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	m.sessions[id] = session
	log.Printf("[Manager] Created session %s (total: %d)", id, len(m.sessions))

	return session, nil
}

// GetSession retrieves an existing session
func (m *SessionManager) GetSession(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[id]
	return session, exists
}

// CloseSession closes and removes a session
func (m *SessionManager) CloseSession(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.sessions[id]
	if !exists {
		return fmt.Errorf("session %s not found", id)
	}

	if err := session.Close(); err != nil {
		return fmt.Errorf("failed to close session: %w", err)
	}

	delete(m.sessions, id)
	log.Printf("[Manager] Closed session %s (remaining: %d)", id, len(m.sessions))

	return nil
}

// ListSessions returns a list of all active session IDs
func (m *SessionManager) ListSessions() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}

	return ids
}

// CloseAll closes all sessions
func (m *SessionManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error
	for id, session := range m.sessions {
		if err := session.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close session %s: %w", id, err))
		}
	}

	m.sessions = make(map[string]*Session)
	log.Printf("[Manager] Closed all sessions")

	if len(errors) > 0 {
		return fmt.Errorf("errors closing sessions: %v", errors)
	}

	return nil
}

// Count returns the number of active sessions
func (m *SessionManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.sessions)
}

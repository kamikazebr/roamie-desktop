package terminal

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
)

// Session represents an active terminal session with PTY
type Session struct {
	id      string
	pty     *os.File
	cmd     *exec.Cmd
	state   *TerminalState
	crypto  *CryptoContext
	lastSeq uint64
	mu      sync.RWMutex
	closed  bool
}

// NewSession creates a new terminal session with PTY
func NewSession(id string, rows, cols int, encryptionKey []byte) (*Session, error) {
	// Create encryption context if key provided
	var crypto *CryptoContext
	var err error
	if encryptionKey != nil {
		crypto, err = NewCryptoContext(encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create crypto context: %w", err)
		}
	}

	// Create terminal state
	state := NewTerminalState(rows, cols)

	// Start shell
	cmd := exec.Command("/bin/bash")
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	// Start PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	// Set initial PTY size
	if err := pty.Setsize(ptmx, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}); err != nil {
		ptmx.Close()
		return nil, fmt.Errorf("failed to set pty size: %w", err)
	}

	session := &Session{
		id:     id,
		pty:    ptmx,
		cmd:    cmd,
		state:  state,
		crypto: crypto,
	}

	log.Printf("[Session %s] Created (rows=%d, cols=%d, encrypted=%v)", id, rows, cols, crypto != nil)

	return session, nil
}

// Read reads data from the PTY
func (s *Session) Read(buf []byte) (int, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return 0, io.EOF
	}
	s.mu.RUnlock()

	return s.pty.Read(buf)
}

// Write writes data to the PTY
func (s *Session) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, fmt.Errorf("session closed")
	}

	// Decrypt if encryption is enabled
	if s.crypto != nil {
		decrypted, err := s.crypto.Decrypt(data)
		if err != nil {
			return 0, fmt.Errorf("failed to decrypt: %w", err)
		}
		data = decrypted
	}

	return s.pty.Write(data)
}

// Resize resizes the PTY
func (s *Session) Resize(rows, cols int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("session closed")
	}

	if err := pty.Setsize(s.pty, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}); err != nil {
		return fmt.Errorf("failed to resize pty: %w", err)
	}

	s.state.Resize(rows, cols)
	log.Printf("[Session %s] Resized to rows=%d, cols=%d", s.id, rows, cols)

	return nil
}

// Close closes the session and cleans up resources
func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true

	// Close PTY
	if s.pty != nil {
		s.pty.Close()
	}

	// Kill process
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Signal(syscall.SIGTERM)
		s.cmd.Wait()
	}

	log.Printf("[Session %s] Closed", s.id)

	return nil
}

// ID returns the session ID
func (s *Session) ID() string {
	return s.id
}

// State returns the terminal state manager
func (s *Session) State() *TerminalState {
	return s.state
}

// IncrementSequence increments and returns the sequence number
func (s *Session) IncrementSequence() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastSeq++
	return s.lastSeq
}

// Encrypt encrypts data if encryption is enabled
func (s *Session) Encrypt(data []byte) ([]byte, error) {
	if s.crypto == nil {
		return data, nil
	}
	return s.crypto.Encrypt(data)
}

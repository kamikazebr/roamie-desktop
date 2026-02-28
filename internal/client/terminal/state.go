package terminal

import (
	"bytes"
	"sync"
)

// TerminalState manages the terminal state and tracks changes
type TerminalState struct {
	buffer bytes.Buffer
	rows   int
	cols   int
	mu     sync.RWMutex
}

// NewTerminalState creates a new terminal state manager
func NewTerminalState(rows, cols int) *TerminalState {
	return &TerminalState{
		rows: rows,
		cols: cols,
	}
}

// ProcessOutput processes terminal output data and returns changes
func (t *TerminalState) ProcessOutput(data []byte) (changed bool, diff []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Store the original buffer size
	oldSize := t.buffer.Len()

	// Append new data to buffer
	t.buffer.Write(data)

	// Check if content changed
	if t.buffer.Len() != oldSize {
		changed = true
		diff = data
	}

	return changed, diff
}

// GetState returns the current terminal state
func (t *TerminalState) GetState() []byte {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.buffer.Bytes()
}

// Resize updates the terminal dimensions
func (t *TerminalState) Resize(rows, cols int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.rows = rows
	t.cols = cols
}

// GetDimensions returns the current terminal dimensions
func (t *TerminalState) GetDimensions() (rows, cols int) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.rows, t.cols
}

// Clear clears the terminal state
func (t *TerminalState) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.buffer.Reset()
}

package terminal

import "time"

// MessageType represents the type of WebSocket message
type MessageType string

const (
	MsgTypeInit      MessageType = "init"
	MsgTypeInput     MessageType = "input"
	MsgTypeOutput    MessageType = "output"
	MsgTypeResize    MessageType = "resize"
	MsgTypeHeartbeat MessageType = "heartbeat"
	MsgTypeStateSync MessageType = "state_sync"
	MsgTypeError     MessageType = "error"
)

// Message represents a WebSocket message for terminal communication
type Message struct {
	Type      MessageType `json:"type"`
	SessionID string      `json:"session_id"`
	Sequence  uint64      `json:"sequence"`
	Data      []byte      `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// InitMessage represents a session initialization request
type InitMessage struct {
	SessionID string `json:"session_id"`
	Rows      int    `json:"rows"`
	Cols      int    `json:"cols"`
}

// ResizeMessage represents a terminal resize request
type ResizeMessage struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

// NewMessage creates a new Message with current timestamp
func NewMessage(msgType MessageType, sessionID string, sequence uint64, data []byte) *Message {
	return &Message{
		Type:      msgType,
		SessionID: sessionID,
		Sequence:  sequence,
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
}

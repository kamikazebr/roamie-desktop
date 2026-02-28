package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// Server is the WebSocket terminal server
type Server struct {
	addr     string
	manager  *SessionManager
	listener net.Listener
	wg       sync.WaitGroup
	shutdown chan struct{}
}

// NewServer creates a new WebSocket terminal server
func NewServer(addr string) *Server {
	return &Server{
		addr:     addr,
		manager:  NewSessionManager(),
		shutdown: make(chan struct{}),
	}
}

// Start starts the WebSocket server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	s.listener = listener
	log.Printf("[Server] WebSocket terminal server listening on %s", s.addr)

	mux := http.NewServeMux()
	mux.HandleFunc("/terminal", s.handleWebSocket)
	mux.HandleFunc("/sessions", s.handleListSessions)

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		<-s.shutdown
		server.Close()
	}()

	return server.Serve(listener)
}

// Stop stops the WebSocket server
func (s *Server) Stop() error {
	close(s.shutdown)
	s.manager.CloseAll()
	s.wg.Wait()
	return nil
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Server] Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("[Server] New WebSocket connection from %s", r.RemoteAddr)

	s.wg.Add(1)
	defer s.wg.Done()

	s.handleConnection(conn)
}

// handleConnection handles a single WebSocket connection
func (s *Server) handleConnection(conn *websocket.Conn) {
	var session *Session
	var sessionID string

	defer func() {
		if session != nil {
			s.manager.CloseSession(sessionID)
		}
	}()

	// Handle incoming messages
	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			if err != io.EOF && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("[Server] Error reading message: %v", err)
			}
			return
		}

		switch msg.Type {
		case MsgTypeInit:
			// Initialize new session
			var initMsg InitMessage
			if err := json.Unmarshal(msg.Data, &initMsg); err != nil {
				s.sendError(conn, "Invalid init message")
				continue
			}

			sessionID = initMsg.SessionID
			newSession, err := s.manager.CreateSession(sessionID, initMsg.Rows, initMsg.Cols, nil)
			if err != nil {
				s.sendError(conn, fmt.Sprintf("Failed to create session: %v", err))
				continue
			}

			session = newSession

			// Start PTY output reader
			go s.readPTYOutput(session, conn)

			log.Printf("[Server] Session %s initialized", sessionID)

		case MsgTypeInput:
			// Handle terminal input
			if session == nil {
				s.sendError(conn, "Session not initialized")
				continue
			}

			log.Printf("[Server] Received input for session %s (%d bytes)", sessionID, len(msg.Data))
			if _, err := session.Write(msg.Data); err != nil {
				log.Printf("[Server] Error writing to PTY: %v", err)
			}

		case MsgTypeResize:
			// Handle terminal resize
			if session == nil {
				s.sendError(conn, "Session not initialized")
				continue
			}

			var resizeMsg ResizeMessage
			if err := json.Unmarshal(msg.Data, &resizeMsg); err != nil {
				s.sendError(conn, "Invalid resize message")
				continue
			}

			if err := session.Resize(resizeMsg.Rows, resizeMsg.Cols); err != nil {
				log.Printf("[Server] Error resizing PTY: %v", err)
			}

		case MsgTypeHeartbeat:
			// Respond to heartbeat
			response := NewMessage(MsgTypeHeartbeat, sessionID, 0, nil)
			conn.WriteJSON(response)

		default:
			log.Printf("[Server] Unknown message type: %s", msg.Type)
		}
	}
}

// readPTYOutput reads output from PTY and sends to WebSocket
func (s *Server) readPTYOutput(session *Session, conn *websocket.Conn) {
	buf := make([]byte, 4096)

	for {
		n, err := session.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("[Server] Error reading from PTY: %v", err)
			}
			return
		}

		data := buf[:n]

		// Encrypt if needed
		encrypted, err := session.Encrypt(data)
		if err != nil {
			log.Printf("[Server] Error encrypting data: %v", err)
			continue
		}

		// Send to WebSocket
		msg := NewMessage(MsgTypeOutput, session.ID(), session.IncrementSequence(), encrypted)
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("[Server] Error sending output: %v", err)
			return
		}
	}
}

// sendError sends an error message to the client
func (s *Server) sendError(conn *websocket.Conn, message string) {
	msg := NewMessage(MsgTypeError, "", 0, []byte(message))
	conn.WriteJSON(msg)
}

// handleListSessions handles HTTP requests to list active sessions
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.manager.ListSessions()

	response := struct {
		Count    int      `json:"count"`
		Sessions []string `json:"sessions"`
	}{
		Count:    len(sessions),
		Sessions: sessions,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetManager returns the session manager
func (s *Server) GetManager() *SessionManager {
	return s.manager
}

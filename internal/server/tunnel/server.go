package tunnel

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/server/storage"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

var debugMode bool

func init() {
	debugMode = os.Getenv("ROAMIE_DEBUG") == "1" || os.Getenv("ROAMIE_DEBUG") == "true"
}

func debugLog(format string, v ...interface{}) {
	if debugMode {
		log.Printf("DEBUG: [SERVER] "+format, v...)
	}
}

const (
	TunnelPort       = 2222
	ServerConfigPath = "/etc/roamie-server"
	LegacyConfigPath = "/etc/roamie-desktop"
	HostKeyFile      = "ssh_host_key"
	BackupDirName    = "backups"
)

type Server struct {
	deviceRepo *storage.DeviceRepository
	authMgr    *AuthorizationManager
	listener   net.Listener
	sshConfig  *ssh.ServerConfig
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewServer creates a new SSH tunnel server
func NewServer(deviceRepo *storage.DeviceRepository) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		deviceRepo: deviceRepo,
		authMgr:    NewAuthorizationManager(deviceRepo),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Migrate from old path if needed
	if err := s.migrateConfigPath(); err != nil {
		log.Printf("Warning: failed to migrate config path: %v", err)
	}

	// Backup existing SSH keys if present
	if err := s.backupExistingKeys(); err != nil {
		log.Printf("Warning: failed to backup existing keys: %v", err)
	}

	// Load or generate host key
	hostKey, err := s.loadOrGenerateHostKey()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to load host key: %w", err)
	}

	// Configure SSH server
	s.sshConfig = &ssh.ServerConfig{
		NoClientAuth: false,
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return s.authenticateClient(conn, key)
		},
	}
	s.sshConfig.AddHostKey(hostKey)

	return s, nil
}

// migrateConfigPath migrates from /etc/roamie-desktop to /etc/roamie-server
func (s *Server) migrateConfigPath() error {
	// Check if old path exists and new path doesn't
	if _, err := os.Stat(LegacyConfigPath); os.IsNotExist(err) {
		return nil // Nothing to migrate
	}

	if _, err := os.Stat(ServerConfigPath); err == nil {
		// New path exists, check if we should still migrate
		// Only migrate if new path is empty
		entries, _ := os.ReadDir(ServerConfigPath)
		if len(entries) > 0 {
			return nil // New path has content, skip migration
		}
	}

	log.Printf("Migrating configuration from %s to %s", LegacyConfigPath, ServerConfigPath)

	// Create new directory
	if err := os.MkdirAll(ServerConfigPath, 0755); err != nil {
		return fmt.Errorf("failed to create new config directory: %w", err)
	}

	// Copy all files from old to new
	entries, err := os.ReadDir(LegacyConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read legacy directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories
		}

		oldPath := filepath.Join(LegacyConfigPath, entry.Name())
		newPath := filepath.Join(ServerConfigPath, entry.Name())

		data, err := os.ReadFile(oldPath)
		if err != nil {
			log.Printf("Warning: failed to read %s: %v", oldPath, err)
			continue
		}

		if err := os.WriteFile(newPath, data, 0600); err != nil {
			log.Printf("Warning: failed to write %s: %v", newPath, err)
			continue
		}

		log.Printf("Migrated: %s → %s", oldPath, newPath)
	}

	log.Println("✓ Configuration migration completed")
	return nil
}

// backupExistingKeys creates a backup of existing SSH keys
func (s *Server) backupExistingKeys() error {
	hostKeyPath := filepath.Join(ServerConfigPath, HostKeyFile)

	// Check if host key exists
	if _, err := os.Stat(hostKeyPath); os.IsNotExist(err) {
		return nil // No existing key to backup
	}

	// Create backup directory with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupDir := filepath.Join(ServerConfigPath, BackupDirName, fmt.Sprintf("ssh-backup-%s", timestamp))

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Copy host key
	data, err := os.ReadFile(hostKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read host key: %w", err)
	}

	backupPath := filepath.Join(backupDir, HostKeyFile)
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	// Create README
	readme := fmt.Sprintf(`SSH Host Key Backup
Created: %s

This backup contains the SSH host key for the Roamie tunnel server.
To restore this backup, copy the files back to %s/

Files backed up:
- %s

To restore:
  sudo cp %s %s/
  sudo chmod 600 %s/%s
  sudo systemctl restart roamie-server
`, time.Now().Format("2006-01-02 15:04:05"), ServerConfigPath, HostKeyFile, backupPath, ServerConfigPath, ServerConfigPath, HostKeyFile)

	readmePath := filepath.Join(backupDir, "README.txt")
	os.WriteFile(readmePath, []byte(readme), 0644)

	log.Printf("✓ SSH host key backed up to: %s", backupDir)
	return nil
}

// loadOrGenerateHostKey loads existing host key or generates a new one
func (s *Server) loadOrGenerateHostKey() (ssh.Signer, error) {
	// Ensure config directory exists
	if err := os.MkdirAll(ServerConfigPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	hostKeyPath := filepath.Join(ServerConfigPath, HostKeyFile)

	// Try to load existing key
	if data, err := os.ReadFile(hostKeyPath); err == nil {
		signer, err := ssh.ParsePrivateKey(data)
		if err == nil {
			log.Printf("✓ Loaded existing SSH host key from: %s", hostKeyPath)
			return signer, nil
		}
		log.Printf("Warning: failed to parse existing host key: %v", err)
	}

	// Generate new key
	log.Println("Generating new SSH host key...")
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	// Encode private key to PEM format
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	privateKeyBytes := pem.EncodeToMemory(privateKeyPEM)

	// Save to disk
	if err := os.WriteFile(hostKeyPath, privateKeyBytes, 0600); err != nil {
		return nil, fmt.Errorf("failed to save host key: %w", err)
	}

	// Parse to ssh.Signer
	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated key: %w", err)
	}

	log.Printf("✓ Generated and saved new SSH host key to: %s", hostKeyPath)
	return signer, nil
}

// authenticateClient validates the client's SSH public key against the database
func (s *Server) authenticateClient(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	// Get authorized key in SSH format and trim whitespace for comparison
	authorizedKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	keyFingerprint := ssh.FingerprintSHA256(key)

	debugLog("[AUTH] Authentication attempt - user=%s remote=%s fingerprint=%s", conn.User(), conn.RemoteAddr(), keyFingerprint)
	debugLog("[AUTH] Public key (first 80 chars): %.80s...", authorizedKey)

	// Look up device by SSH public key
	debugLog("[AUTH] Querying database for device with key")
	device, err := s.deviceRepo.GetByTunnelSSHKey(s.ctx, authorizedKey)
	if err != nil {
		debugLog("[AUTH] Database query failed - err=%v", err)
		log.Printf("Database error during authentication: %v", err)
		return nil, fmt.Errorf("authentication failed")
	}

	if device == nil {
		debugLog("[AUTH] Key not found in database - fingerprint=%s", keyFingerprint)
		log.Printf("Rejected connection: SSH key not found in database (key: %.100s...)", authorizedKey)
		return nil, fmt.Errorf("public key not authorized")
	}

	debugLog("[AUTH] Device found - device_id=%s active=%v tunnel_enabled=%v", device.ID, device.Active, device.TunnelEnabled)

	// Check if device is active
	if !device.Active {
		debugLog("[AUTH] Device inactive - device_id=%s", device.ID)
		log.Printf("Rejected connection: device %s is inactive", device.ID)
		return nil, fmt.Errorf("device inactive")
	}

	// Check if tunnel is enabled for this device
	if !device.TunnelEnabled {
		debugLog("[AUTH] Tunnel disabled - device_id=%s", device.ID)
		log.Printf("Rejected connection: tunnel disabled for device %s", device.ID)
		return nil, fmt.Errorf("tunnel disabled for this device")
	}

	// Check if device has allocated tunnel port
	if device.TunnelPort == nil {
		debugLog("[AUTH] No tunnel port allocated - device_id=%s", device.ID)
		log.Printf("Rejected connection: no tunnel port allocated for device %s", device.ID)
		return nil, fmt.Errorf("no tunnel port allocated")
	}

	debugLog("[AUTH] Authentication successful - device_id=%s port=%d", device.ID, *device.TunnelPort)
	log.Printf("✓ Authenticated device %s (port %d)", device.ID, *device.TunnelPort)

	// Return permissions with device info
	return &ssh.Permissions{
		Extensions: map[string]string{
			"device_id":   device.ID.String(),
			"tunnel_port": fmt.Sprintf("%d", *device.TunnelPort),
		},
	}, nil
}

// Start starts the SSH tunnel server
func (s *Server) Start() error {
	addr := fmt.Sprintf("0.0.0.0:%d", TunnelPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	log.Printf("✓ SSH tunnel server listening on %s", addr)

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

// acceptLoop accepts incoming SSH connections
func (s *Server) acceptLoop() {
	defer s.wg.Done()

	debugLog("[ACCEPT] Accept loop started")
	for {
		debugLog("[ACCEPT] Waiting for connection")
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				debugLog("[ACCEPT] Context cancelled, stopping accept loop")
				return // Server stopped
			default:
				debugLog("[ACCEPT] Accept failed - err=%v", err)
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
		}

		debugLog("[ACCEPT] Connection accepted - remote=%s", conn.RemoteAddr())
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single SSH connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	debugLog("[CONN] Handling connection - remote=%s", remoteAddr)

	// SSH handshake
	debugLog("[CONN] Starting SSH handshake - remote=%s", remoteAddr)
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.sshConfig)
	if err != nil {
		debugLog("[CONN] SSH handshake failed - remote=%s err=%v", remoteAddr, err)
		log.Printf("SSH handshake failed: %v", err)
		return
	}
	defer sshConn.Close()

	deviceID := sshConn.Permissions.Extensions["device_id"]
	tunnelPort := sshConn.Permissions.Extensions["tunnel_port"]

	debugLog("[CONN] SSH handshake complete - device_id=%s port=%s remote=%s", deviceID, tunnelPort, remoteAddr)
	log.Printf("SSH connection established for device %s (port %s)", deviceID, tunnelPort)

	// Parse device ID from permissions
	sourceDeviceID, err := uuid.Parse(deviceID)
	if err != nil {
		debugLog("[CONN] Invalid device ID - device_id=%s err=%v", deviceID, err)
		log.Printf("Invalid device ID in permissions: %v", err)
		return
	}

	// Handle global requests (port forwarding setup) and channels together
	debugLog("[CONN] Starting tunnel session handler - device_id=%s", deviceID)
	go s.handleTunnelSession(sshConn, reqs, chans, sourceDeviceID, deviceID, tunnelPort)

	// Wait for connection to close
	sshConn.Wait()
	debugLog("[CONN] SSH connection closed - device_id=%s", deviceID)
}

// handleTunnelSession manages the tunnel session including port forwarding and authorization
func (s *Server) handleTunnelSession(sshConn *ssh.ServerConn, reqs <-chan *ssh.Request, chans <-chan ssh.NewChannel, deviceID uuid.UUID, deviceIDStr, tunnelPortStr string) {
	var tunnelListener net.Listener
	defer func() {
		if tunnelListener != nil {
			tunnelListener.Close()
		}
	}()

	// Handle incoming channel requests in background (from the device)
	go func() {
		for newChannel := range chans {
			// We don't expect channels FROM the device (only TO the device)
			newChannel.Reject(ssh.UnknownChannelType, "reverse tunnel server doesn't accept channels from client")
		}
	}()

	// Handle global requests (tcpip-forward)
	debugLog("[SESSION] Waiting for global requests - device_id=%s", deviceIDStr)
	for req := range reqs {
		debugLog("[SESSION] Received request - type=%s want_reply=%v device_id=%s", req.Type, req.WantReply, deviceIDStr)
		switch req.Type {
		case "tcpip-forward":
			debugLog("[SESSION] Processing tcpip-forward request - device_id=%s", deviceIDStr)

			// Parse the forward request
			var forwardRequest struct {
				BindAddr string
				BindPort uint32
			}
			if err := ssh.Unmarshal(req.Payload, &forwardRequest); err != nil {
				debugLog("[SESSION] Failed to parse forward request - device_id=%s err=%v", deviceIDStr, err)
				log.Printf("Failed to parse tcpip-forward request: %v", err)
				if req.WantReply {
					req.Reply(false, nil)
				}
				continue
			}

			debugLog("[SESSION] Forward request parsed - device_id=%s bind_addr=%s bind_port=%d", deviceIDStr, forwardRequest.BindAddr, forwardRequest.BindPort)

			// Validate that the requested port matches the allocated port
			allocatedPort := tunnelPortStr
			requestedPort := int(forwardRequest.BindPort)
			if fmt.Sprintf("%d", requestedPort) != allocatedPort {
				debugLog("[SESSION] Port mismatch - device_id=%s requested=%d allocated=%s", deviceIDStr, requestedPort, allocatedPort)
				log.Printf("⚠️  Device %s requested port %d but allocated port is %s",
					deviceIDStr, requestedPort, allocatedPort)
				if req.WantReply {
					req.Reply(false, nil)
				}
				continue
			}

			// Start listening on the allocated port
			listenAddr := fmt.Sprintf("0.0.0.0:%d", requestedPort)
			debugLog("[SESSION] Creating listener - device_id=%s addr=%s", deviceIDStr, listenAddr)
			listener, err := net.Listen("tcp", listenAddr)
			if err != nil {
				debugLog("[SESSION] Listener creation failed - device_id=%s addr=%s err=%v", deviceIDStr, listenAddr, err)
				log.Printf("Failed to listen on %s: %v", listenAddr, err)
				if req.WantReply {
					req.Reply(false, nil)
				}
				continue
			}

			tunnelListener = listener
			debugLog("[SESSION] Listener created successfully - device_id=%s addr=%s", deviceIDStr, listenAddr)
			log.Printf("✓ Reverse tunnel established: Device %s listening on %s", deviceIDStr, listenAddr)

			// Reply success
			if req.WantReply {
				// Reply with the actual port (required by SSH protocol)
				reply := struct {
					Port uint32
				}{
					Port: uint32(requestedPort),
				}
				debugLog("[SESSION] Sending success reply - device_id=%s port=%d", deviceIDStr, requestedPort)
				req.Reply(true, ssh.Marshal(&reply))
			}

			// Handle incoming connections on this port
			debugLog("[SESSION] Starting tunnel connection handler - device_id=%s port=%d", deviceIDStr, requestedPort)
			go s.handleTunnelConnections(listener, sshConn, deviceID, requestedPort)

		case "cancel-tcpip-forward":
			log.Printf("Device %s canceling reverse tunnel", deviceIDStr)
			if tunnelListener != nil {
				tunnelListener.Close()
				tunnelListener = nil
			}
			if req.WantReply {
				req.Reply(true, nil)
			}

		case "keepalive@roamie":
			// Respond to client keepalive to maintain connection
			if req.WantReply {
				req.Reply(true, nil)
			}

		default:
			log.Printf("Rejecting unsupported request type: %s", req.Type)
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

// handleTunnelConnections handles incoming TCP connections on a tunnel port
func (s *Server) handleTunnelConnections(listener net.Listener, sshConn *ssh.ServerConn, targetDeviceID uuid.UUID, tunnelPort int) {
	debugLog("[TUNNEL] Tunnel connection handler started - device_id=%s port=%d", targetDeviceID, tunnelPort)
	for {
		// Accept incoming TCP connection
		debugLog("[TUNNEL] Waiting for tunnel connection - device_id=%s port=%d", targetDeviceID, tunnelPort)
		tcpConn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				debugLog("[TUNNEL] Context cancelled - device_id=%s port=%d", targetDeviceID, tunnelPort)
				return // Server stopping
			default:
				if !isClosedError(err) {
					debugLog("[TUNNEL] Accept failed - device_id=%s port=%d err=%v", targetDeviceID, tunnelPort, err)
					log.Printf("Failed to accept tunnel connection on port %d: %v", tunnelPort, err)
				}
				return
			}
		}

		debugLog("[TUNNEL] Tunnel connection accepted - device_id=%s port=%d remote=%s", targetDeviceID, tunnelPort, tcpConn.RemoteAddr())

		// Handle this connection in a goroutine
		go s.forwardTunnelConnection(tcpConn, sshConn, targetDeviceID, tunnelPort)
	}
}

// forwardTunnelConnection forwards a TCP connection through an SSH channel with authorization
func (s *Server) forwardTunnelConnection(tcpConn net.Conn, sshConn *ssh.ServerConn, targetDeviceID uuid.UUID, tunnelPort int) {
	defer tcpConn.Close()

	// Get remote address for logging
	remoteAddr := tcpConn.RemoteAddr().String()
	debugLog("[CHANNEL] Starting tunnel connection forward - device_id=%s port=%d remote=%s", targetDeviceID, tunnelPort, remoteAddr)
	log.Printf("Incoming connection to tunnel port %d from %s", tunnelPort, remoteAddr)

	// Parse originator address and port
	originHost, originPortStr, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		debugLog("[CHANNEL] Failed to parse remote address - remote=%s err=%v", remoteAddr, err)
		log.Printf("Failed to parse remote address %s: %v", remoteAddr, err)
		originHost = remoteAddr
		originPortStr = "0"
	}
	var originPort uint32
	if p, err := fmt.Sscanf(originPortStr, "%d", &originPort); err != nil || p != 1 {
		originPort = 0
	}
	debugLog("[CHANNEL] Origin parsed - host=%s port=%d", originHost, originPort)

	// For now, we allow all connections to the tunnel port and rely on
	// the final SSH authentication at the target device.
	// In a future enhancement, we could add IP-based or certificate-based auth here.

	// Create a forwarded-tcpip channel to the target device
	// This goes through the device's existing SSH connection
	// IMPORTANT: The Address/Port fields must match what the client requested in tcpip-forward!
	// Go's SSH library uses these fields to match the channel to the correct listener.
	// The Address should be the bind address from tcpip-forward (e.g., "0.0.0.0")
	// The Port should be the tunnel port (e.g., 10000)
	payload := struct {
		Address           string
		Port              uint32
		OriginatorAddress string
		OriginatorPort    uint32
	}{
		Address:           "0.0.0.0",
		Port:              uint32(tunnelPort), // Must match what client requested!
		OriginatorAddress: originHost,
		OriginatorPort:    originPort,
	}

	debugLog("[CHANNEL] Opening forwarded-tcpip channel - device_id=%s dest=%s:%d origin=%s:%d",
		targetDeviceID, payload.Address, payload.Port, payload.OriginatorAddress, payload.OriginatorPort)
	channel, requests, err := sshConn.OpenChannel("forwarded-tcpip", ssh.Marshal(&payload))
	if err != nil {
		debugLog("[CHANNEL] Failed to open channel - device_id=%s err=%v", targetDeviceID, err)
		log.Printf("Failed to open forwarded channel to device %s: %v", targetDeviceID, err)
		return
	}
	defer channel.Close()
	debugLog("[CHANNEL] Channel opened successfully - device_id=%s", targetDeviceID)

	// Discard channel requests
	go ssh.DiscardRequests(requests)

	log.Printf("✓ Tunnel connection established: %s → Device %s (port %d)",
		remoteAddr, targetDeviceID, tunnelPort)

	// Bidirectional forwarding between TCP connection and SSH channel
	debugLog("[CHANNEL] Starting bidirectional copy - device_id=%s", targetDeviceID)
	done := make(chan struct{}, 2)

	var toChannelBytes, toTCPBytes int64

	go func() {
		n, _ := io.Copy(channel, tcpConn)
		toChannelBytes = n
		debugLog("[CHANNEL] TCP→Channel copy complete - device_id=%s bytes=%d", targetDeviceID, n)
		done <- struct{}{}
	}()

	go func() {
		n, _ := io.Copy(tcpConn, channel)
		toTCPBytes = n
		debugLog("[CHANNEL] Channel→TCP copy complete - device_id=%s bytes=%d", targetDeviceID, n)
		done <- struct{}{}
	}()

	// Wait for either direction to complete
	<-done

	debugLog("[CHANNEL] Connection closed - device_id=%s remote=%s bytes_to_channel=%d bytes_to_tcp=%d",
		targetDeviceID, remoteAddr, toChannelBytes, toTCPBytes)
	log.Printf("Tunnel connection closed: %s → Device %s (port %d)",
		remoteAddr, targetDeviceID, tunnelPort)
}

// isClosedError checks if an error is due to a closed network connection
func isClosedError(err error) bool {
	return err != nil && (err.Error() == "use of closed network connection" ||
		err == io.EOF)
}

// Stop gracefully stops the SSH tunnel server
func (s *Server) Stop() error {
	log.Println("Stopping SSH tunnel server...")
	s.cancel()

	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for all connections to close (with timeout)
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("✓ SSH tunnel server stopped gracefully")
	case <-time.After(10 * time.Second):
		log.Println("⚠️  SSH tunnel server stop timeout")
	}

	return nil
}

// forwardConnection forwards data between two connections
func forwardConnection(dst io.Writer, src io.Reader) {
	io.Copy(dst, src)
}

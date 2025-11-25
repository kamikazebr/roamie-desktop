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

const (
	TunnelPort       = 2222
	ServerConfigPath = "/etc/roamie-server"
	LegacyConfigPath = "/etc/roamie-vpn"
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

// migrateConfigPath migrates from /etc/roamie-vpn to /etc/roamie-server
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

	// Look up device by SSH public key
	device, err := s.deviceRepo.GetByTunnelSSHKey(s.ctx, authorizedKey)
	if err != nil {
		log.Printf("Database error during authentication: %v", err)
		return nil, fmt.Errorf("authentication failed")
	}

	if device == nil {
		log.Printf("Rejected connection: SSH key not found in database")
		return nil, fmt.Errorf("public key not authorized")
	}

	// Check if device is active
	if !device.Active {
		log.Printf("Rejected connection: device %s is inactive", device.ID)
		return nil, fmt.Errorf("device inactive")
	}

	// Check if tunnel is enabled for this device
	if !device.TunnelEnabled {
		log.Printf("Rejected connection: tunnel disabled for device %s", device.ID)
		return nil, fmt.Errorf("tunnel disabled for this device")
	}

	// Check if device has allocated tunnel port
	if device.TunnelPort == nil {
		log.Printf("Rejected connection: no tunnel port allocated for device %s", device.ID)
		return nil, fmt.Errorf("no tunnel port allocated")
	}

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

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return // Server stopped
			default:
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
		}

		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

// handleConnection handles a single SSH connection
func (s *Server) handleConnection(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	// SSH handshake
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.sshConfig)
	if err != nil {
		log.Printf("SSH handshake failed: %v", err)
		return
	}
	defer sshConn.Close()

	deviceID := sshConn.Permissions.Extensions["device_id"]
	tunnelPort := sshConn.Permissions.Extensions["tunnel_port"]

	log.Printf("SSH connection established for device %s (port %s)", deviceID, tunnelPort)

	// Parse device ID from permissions
	sourceDeviceID, err := uuid.Parse(deviceID)
	if err != nil {
		log.Printf("Invalid device ID in permissions: %v", err)
		return
	}

	// Handle global requests (port forwarding setup) and channels together
	go s.handleTunnelSession(sshConn, reqs, chans, sourceDeviceID, deviceID, tunnelPort)

	// Wait for connection to close
	sshConn.Wait()
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
	for req := range reqs {
		switch req.Type {
		case "tcpip-forward":
			// Parse the forward request
			var forwardRequest struct {
				BindAddr string
				BindPort uint32
			}
			if err := ssh.Unmarshal(req.Payload, &forwardRequest); err != nil {
				log.Printf("Failed to parse tcpip-forward request: %v", err)
				if req.WantReply {
					req.Reply(false, nil)
				}
				continue
			}

			// Validate that the requested port matches the allocated port
			allocatedPort := tunnelPortStr
			requestedPort := int(forwardRequest.BindPort)
			if fmt.Sprintf("%d", requestedPort) != allocatedPort {
				log.Printf("⚠️  Device %s requested port %d but allocated port is %s",
					deviceIDStr, requestedPort, allocatedPort)
				if req.WantReply {
					req.Reply(false, nil)
				}
				continue
			}

			// Start listening on the allocated port
			listenAddr := fmt.Sprintf("0.0.0.0:%d", requestedPort)
			listener, err := net.Listen("tcp", listenAddr)
			if err != nil {
				log.Printf("Failed to listen on %s: %v", listenAddr, err)
				if req.WantReply {
					req.Reply(false, nil)
				}
				continue
			}

			tunnelListener = listener
			log.Printf("✓ Reverse tunnel established: Device %s listening on %s", deviceIDStr, listenAddr)

			// Reply success
			if req.WantReply {
				// Reply with the actual port (required by SSH protocol)
				reply := struct {
					Port uint32
				}{
					Port: uint32(requestedPort),
				}
				req.Reply(true, ssh.Marshal(&reply))
			}

			// Handle incoming connections on this port
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
	for {
		// Accept incoming TCP connection
		tcpConn, err := listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return // Server stopping
			default:
				if !isClosedError(err) {
					log.Printf("Failed to accept tunnel connection on port %d: %v", tunnelPort, err)
				}
				return
			}
		}

		// Handle this connection in a goroutine
		go s.forwardTunnelConnection(tcpConn, sshConn, targetDeviceID, tunnelPort)
	}
}

// forwardTunnelConnection forwards a TCP connection through an SSH channel with authorization
func (s *Server) forwardTunnelConnection(tcpConn net.Conn, sshConn *ssh.ServerConn, targetDeviceID uuid.UUID, tunnelPort int) {
	defer tcpConn.Close()

	// Get remote address for logging
	remoteAddr := tcpConn.RemoteAddr().String()
	log.Printf("Incoming connection to tunnel port %d from %s", tunnelPort, remoteAddr)

	// For now, we allow all connections to the tunnel port and rely on
	// the final SSH authentication at the target device.
	// In a future enhancement, we could add IP-based or certificate-based auth here.

	// Create a forwarded-tcpip channel to the target device
	// This goes through the device's existing SSH connection
	payload := struct {
		Address           string
		Port              uint32
		OriginatorAddress string
		OriginatorPort    uint32
	}{
		Address:           "127.0.0.1",
		Port:              22, // Forward to device's local SSH server
		OriginatorAddress: remoteAddr,
		OriginatorPort:    0,
	}

	channel, requests, err := sshConn.OpenChannel("forwarded-tcpip", ssh.Marshal(&payload))
	if err != nil {
		log.Printf("Failed to open forwarded channel to device %s: %v", targetDeviceID, err)
		return
	}
	defer channel.Close()

	// Discard channel requests
	go ssh.DiscardRequests(requests)

	log.Printf("✓ Tunnel connection established: %s → Device %s (port %d)",
		remoteAddr, targetDeviceID, tunnelPort)

	// Bidirectional forwarding between TCP connection and SSH channel
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(channel, tcpConn)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(tcpConn, channel)
		done <- struct{}{}
	}()

	// Wait for either direction to complete
	<-done

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

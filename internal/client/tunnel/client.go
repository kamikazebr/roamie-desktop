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

	"github.com/kamikazebr/roamie-desktop/internal/client/api"
	"github.com/kamikazebr/roamie-desktop/internal/client/config"
	sshpkg "github.com/kamikazebr/roamie-desktop/internal/client/ssh"
	"github.com/kamikazebr/roamie-desktop/pkg/utils"
	"golang.org/x/crypto/ssh"
)

const (
	TunnelKeyFile         = "tunnel_key"
	TunnelServerPort      = 2222
	LocalSSHPort          = 22
	MaxReconnectDelay     = 30 * time.Second
	InitialReconnectDelay = 1 * time.Second
	KeepaliveInterval     = 10 * time.Second
	KeepaliveTimeout      = 30 * time.Second
)

type Client struct {
	serverURL      string
	serverHost     string
	tunnelPort     int
	deviceID       string
	jwt            string
	privateKey     ssh.Signer
	sshClient      *ssh.Client
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	reconnectDelay time.Duration
	mu             sync.Mutex
	connected      bool
}

// NewClient creates a new SSH tunnel client
func NewClient(cfg *config.Config) (*Client, error) {
	return NewClientWithContext(context.Background(), cfg)
}

// NewClientWithContext creates a new SSH tunnel client with an external context
// The tunnel will stop when the context is cancelled
func NewClientWithContext(ctx context.Context, cfg *config.Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	// Extract server host from URL
	serverHost, err := extractHost(cfg.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract server host: %w", err)
	}

	// Create a cancellable context derived from the provided context
	tunnelCtx, cancel := context.WithCancel(ctx)

	c := &Client{
		serverURL:      cfg.ServerURL,
		serverHost:     serverHost,
		deviceID:       cfg.DeviceID,
		jwt:            cfg.JWT,
		ctx:            tunnelCtx,
		cancel:         cancel,
		reconnectDelay: InitialReconnectDelay,
	}

	// Load or generate SSH key
	privateKey, err := c.loadOrGenerateKey()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to load SSH key: %w", err)
	}
	c.privateKey = privateKey

	return c, nil
}

// loadOrGenerateKey loads existing SSH key or generates a new one
func (c *Client) loadOrGenerateKey() (ssh.Signer, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return nil, err
	}

	keyPath := filepath.Join(configDir, TunnelKeyFile)

	// Try to load existing key
	if data, err := os.ReadFile(keyPath); err == nil {
		signer, err := ssh.ParsePrivateKey(data)
		if err == nil {
			log.Printf("✓ Loaded existing SSH tunnel key from: %s", keyPath)
			return signer, nil
		}
		log.Printf("Warning: failed to parse existing tunnel key: %v", err)
	}

	// Generate new key
	log.Println("Generating new SSH tunnel key...")
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
	if err := os.WriteFile(keyPath, privateKeyBytes, 0600); err != nil {
		return nil, fmt.Errorf("failed to save tunnel key: %w", err)
	}

	// Fix ownership if running under sudo
	utils.FixFileOwnership(keyPath)

	// Parse to ssh.Signer
	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated key: %w", err)
	}

	log.Printf("✓ Generated and saved new SSH tunnel key to: %s", keyPath)
	return signer, nil
}

// GetPublicKey returns the SSH public key in authorized_keys format
func (c *Client) GetPublicKey() string {
	if c.privateKey == nil {
		return ""
	}
	return string(ssh.MarshalAuthorizedKey(c.privateKey.PublicKey()))
}

// RegisterKey registers the SSH public key with the server
func (c *Client) RegisterKey() error {
	publicKey := c.GetPublicKey()
	if publicKey == "" {
		return fmt.Errorf("no public key available")
	}

	client := api.NewClient(c.serverURL)
	return client.RegisterTunnelKey(c.deviceID, publicKey, c.jwt)
}

// Connect establishes the SSH tunnel with auto-reconnect
func (c *Client) Connect() error {
	// Get tunnel port from server
	client := api.NewClient(c.serverURL)
	status, err := client.GetTunnelStatus(c.jwt)
	if err != nil {
		return fmt.Errorf("failed to get tunnel status: %w", err)
	}

	// Sync tunnel authorized keys before connecting
	// This ensures other devices in the same account can access this device through the tunnel
	log.Println("→ Syncing tunnel authorized keys...")
	sshManager, err := sshpkg.NewManager(c.serverURL)
	if err != nil {
		log.Printf("Warning: failed to create SSH manager: %v", err)
	} else {
		result, err := sshManager.SyncTunnelKeys(c.jwt)
		if err != nil {
			log.Printf("Warning: failed to sync tunnel keys: %v", err)
		} else {
			log.Printf("✓ Synced %d tunnel authorized key(s)", result.Total)
		}
	}

	if len(status.Tunnels) == 0 {
		return fmt.Errorf("no tunnel port allocated, run 'roamie tunnel register' first")
	}

	// Find our device
	var tunnelPort int
	for _, t := range status.Tunnels {
		if t.DeviceID == c.deviceID {
			tunnelPort = t.Port
			break
		}
	}

	if tunnelPort == 0 {
		return fmt.Errorf("tunnel port not found for this device")
	}

	c.tunnelPort = tunnelPort
	log.Printf("Tunnel port allocated: %d", tunnelPort)

	// Start connection loop
	c.wg.Add(1)
	go c.connectionLoop()

	return nil
}

// connectionLoop maintains the SSH connection with auto-reconnect
func (c *Client) connectionLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			log.Println("Tunnel connection loop stopped")
			return
		default:
		}

		if err := c.establishConnection(); err != nil {
			log.Printf("Connection failed: %v", err)
			c.setConnected(false)

			// Exponential backoff
			log.Printf("Reconnecting in %s...", c.reconnectDelay)
			time.Sleep(c.reconnectDelay)

			// Increase delay for next attempt (max 30s)
			c.reconnectDelay *= 2
			if c.reconnectDelay > MaxReconnectDelay {
				c.reconnectDelay = MaxReconnectDelay
			}
		} else {
			// Reset delay on successful connection
			c.reconnectDelay = InitialReconnectDelay
		}
	}
}

// establishConnection creates a single SSH connection attempt
func (c *Client) establishConnection() error {
	// SSH client configuration
	sshConfig := &ssh.ClientConfig{
		User: "tunnel",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(c.privateKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Implement proper host key verification
		Timeout:         10 * time.Second,
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", c.serverHost, TunnelServerPort)
	log.Printf("Connecting to SSH tunnel server: %s", addr)

	log.Printf("DEBUG: About to dial SSH...")
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		log.Printf("DEBUG: SSH dial failed: %v", err)
		return fmt.Errorf("SSH dial failed: %w", err)
	}
	defer sshClient.Close()
	log.Printf("DEBUG: SSH dial succeeded")

	c.mu.Lock()
	c.sshClient = sshClient
	c.connected = true // Set directly to avoid deadlock (setConnected also locks)
	c.mu.Unlock()

	log.Printf("✓ SSH tunnel connected")

	// Setup reverse port forward
	log.Printf("DEBUG: About to setup reverse port forward on port %d...", c.tunnelPort)
	listener, err := sshClient.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", c.tunnelPort))
	if err != nil {
		log.Printf("DEBUG: Listen failed: %v", err)
		return fmt.Errorf("failed to setup reverse tunnel: %w", err)
	}
	defer listener.Close()
	log.Printf("DEBUG: Listener created successfully")

	log.Printf("✓ Reverse tunnel established: server port %d → localhost:%d", c.tunnelPort, LocalSSHPort)

	// Start keepalive
	c.wg.Add(1)
	go c.keepalive(sshClient)

	// Accept connections and forward to local SSH
	for {
		select {
		case <-c.ctx.Done():
			return nil
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			return fmt.Errorf("listener accept failed: %w", err)
		}

		c.wg.Add(1)
		go c.handleForward(conn)
	}
}

// keepalive sends periodic keepalive packets
func (c *Client) keepalive(client *ssh.Client) {
	defer c.wg.Done()

	ticker := time.NewTicker(KeepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			_, _, err := client.SendRequest("keepalive@roamie", true, nil)
			if err != nil {
				log.Printf("Keepalive failed: %v", err)
				client.Close()
				return
			}
		}
	}
}

// handleForward forwards a single connection to local SSH
func (c *Client) handleForward(remoteConn net.Conn) {
	defer c.wg.Done()
	defer remoteConn.Close()

	// Connect to local SSH server
	localConn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", LocalSSHPort), 10*time.Second)
	if err != nil {
		log.Printf("Failed to connect to local SSH: %v", err)
		return
	}
	defer localConn.Close()

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(localConn, remoteConn)
	}()

	go func() {
		defer wg.Done()
		io.Copy(remoteConn, localConn)
	}()

	wg.Wait()
}

// Disconnect stops the SSH tunnel
func (c *Client) Disconnect() error {
	log.Println("Disconnecting SSH tunnel...")
	c.cancel()

	c.mu.Lock()
	if c.sshClient != nil {
		c.sshClient.Close()
	}
	c.mu.Unlock()

	// Wait for all goroutines (with timeout)
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("✓ SSH tunnel disconnected gracefully")
	case <-time.After(10 * time.Second):
		log.Println("⚠️  SSH tunnel disconnect timeout")
	}

	c.setConnected(false)
	return nil
}

// IsConnected returns whether the tunnel is currently connected
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// setConnected updates the connection status
func (c *Client) setConnected(connected bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = connected
}

// GetStatus returns the current tunnel status
func (c *Client) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"connected":   c.IsConnected(),
		"server":      c.serverHost,
		"server_port": TunnelServerPort,
		"tunnel_port": c.tunnelPort,
		"local_port":  LocalSSHPort,
	}
}

// extractHost extracts the hostname from a URL (removes http://, https://, port, and path)
func extractHost(url string) (string, error) {
	// Simple extraction - remove protocol prefix
	host := url
	if len(url) > 8 && url[:8] == "https://" {
		host = url[8:]
	} else if len(url) > 7 && url[:7] == "http://" {
		host = url[7:]
	}

	// Remove path if present
	if slashIdx := strings.Index(host, "/"); slashIdx >= 0 {
		host = host[:slashIdx]
	}

	// Remove port if present (we use TunnelServerPort instead)
	if colonIdx := strings.Index(host, ":"); colonIdx >= 0 {
		host = host[:colonIdx]
	}

	if host == "" {
		return "", fmt.Errorf("invalid URL: %s", url)
	}

	return host, nil
}

package wireguard

import (
	"fmt"
	"net"
	"os"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Manager struct {
	client        *wgctrl.Client
	interfaceName string
	serverPort    int
	serverKey     wgtypes.Key
	publicKey     string
	endpoint      string
}

func NewManager() (*Manager, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}

	interfaceName := os.Getenv("WG_INTERFACE")
	if interfaceName == "" {
		interfaceName = "wg0"
	}

	// Get or generate server private key
	serverKey, err := getOrGenerateServerKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get server key: %w", err)
	}

	endpoint := os.Getenv("WG_SERVER_PUBLIC_ENDPOINT")
	if endpoint == "" {
		return nil, fmt.Errorf("WG_SERVER_PUBLIC_ENDPOINT not set")
	}

	return &Manager{
		client:        client,
		interfaceName: interfaceName,
		serverPort:    51820,
		serverKey:     serverKey,
		publicKey:     serverKey.PublicKey().String(),
		endpoint:      endpoint,
	}, nil
}

func (m *Manager) GetPublicKey() string {
	return m.publicKey
}

func (m *Manager) GetEndpoint() string {
	return m.endpoint
}

func (m *Manager) InitializeInterface() error {
	// Check if interface already exists
	_, err := m.client.Device(m.interfaceName)
	if err == nil {
		// Interface exists - backup before taking over
		backupInfo, err := BackupExistingConfig(m.interfaceName)
		if err != nil {
			return fmt.Errorf("failed to backup existing config: %w", err)
		}
		if backupInfo != nil {
			// Backup was created, log it
			fmt.Printf("✓ Existing configuration backed up to: %s\n", backupInfo.BackupPath)
		}

		// Configure firewall rules for WireGuard network
		baseNetwork := os.Getenv("WG_BASE_NETWORK")
		if baseNetwork == "" {
			baseNetwork = "10.100.0.0/16"
		}

		outInterface := GetDefaultOutInterface()
		if err := EnsureMasqueradeRule(baseNetwork, outInterface); err != nil {
			fmt.Printf("Warning: Failed to configure NAT rules: %v\n", err)
		}

		if err := EnsureForwardRule(baseNetwork); err != nil {
			fmt.Printf("Warning: Failed to configure FORWARD rules: %v\n", err)
		}

		// Now update config to take over the interface
		return m.updateInterfaceConfig()
	}

	// Interface doesn't exist, needs to be created via system commands
	// This typically requires manual setup or a setup script
	return fmt.Errorf("interface %s does not exist. Please create it first using setup script", m.interfaceName)
}

func (m *Manager) updateInterfaceConfig() error {
	config := wgtypes.Config{
		PrivateKey:   &m.serverKey,
		ListenPort:   &m.serverPort,
		ReplacePeers: false,
	}

	return m.client.ConfigureDevice(m.interfaceName, config)
}

func (m *Manager) AddPeer(publicKey, allowedIP string) error {
	// Parse public key
	key, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// Parse allowed IP
	_, ipnet, err := net.ParseCIDR(allowedIP + "/32")
	if err != nil {
		return fmt.Errorf("invalid IP address: %w", err)
	}

	// Configure peer
	peer := wgtypes.PeerConfig{
		PublicKey:         key,
		ReplaceAllowedIPs: true,
		AllowedIPs:        []net.IPNet{*ipnet},
	}

	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peer},
	}

	if err := m.client.ConfigureDevice(m.interfaceName, config); err != nil {
		return fmt.Errorf("failed to add peer: %w", err)
	}

	return nil
}

func (m *Manager) RemovePeer(publicKey string) error {
	// Parse public key
	key, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// Configure peer removal
	peer := wgtypes.PeerConfig{
		PublicKey: key,
		Remove:    true,
	}

	config := wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peer},
	}

	if err := m.client.ConfigureDevice(m.interfaceName, config); err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	return nil
}

func (m *Manager) GetPeerHandshake(publicKey string) (*time.Time, error) {
	device, err := m.client.Device(m.interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	key, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	for _, peer := range device.Peers {
		if peer.PublicKey == key {
			if !peer.LastHandshakeTime.IsZero() {
				return &peer.LastHandshakeTime, nil
			}
			return nil, nil
		}
	}

	return nil, fmt.Errorf("peer not found")
}

func (m *Manager) ListPeers() ([]wgtypes.Peer, error) {
	device, err := m.client.Device(m.interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	return device.Peers, nil
}

func (m *Manager) Close() error {
	return m.client.Close()
}

func getOrGenerateServerKey() (wgtypes.Key, error) {
	keyPath := "/etc/wireguard/server_private.key"

	// Try to read existing key
	if keyData, err := os.ReadFile(keyPath); err == nil {
		return wgtypes.ParseKey(string(keyData))
	}

	// Generate new key
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("failed to generate key: %w", err)
	}

	// Try to save (may fail if no permissions, that's ok)
	os.WriteFile(keyPath, []byte(key.String()), 0600)

	return key, nil
}

package wireguard

import (
	"fmt"
)

type PeerConfig struct {
	PublicKey  string
	AllowedIPs []string
	Endpoint   string
	LastSeen   string
}

func (m *Manager) AddPeerWithSubnet(publicKey, subnet string) error {
	// For subnet-based peers, we allow the entire subnet
	return m.AddPeer(publicKey, subnet)
}

func (m *Manager) UpdatePeerAllowedIPs(publicKey string, allowedIPs []string) error {
	// This would be used if we need to dynamically update allowed IPs
	// For now, we remove and re-add
	if err := m.RemovePeer(publicKey); err != nil {
		return fmt.Errorf("failed to remove peer: %w", err)
	}

	for _, ip := range allowedIPs {
		if err := m.AddPeer(publicKey, ip); err != nil {
			return fmt.Errorf("failed to add peer: %w", err)
		}
	}

	return nil
}

func GenerateClientConfig(privateKey, clientIP, serverPublicKey, serverEndpoint, allowedIPs string) string {
	return fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/32
DNS = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey = %s
Endpoint = %s
AllowedIPs = %s
PersistentKeepalive = 25
`, privateKey, clientIP, serverPublicKey, serverEndpoint, allowedIPs)
}

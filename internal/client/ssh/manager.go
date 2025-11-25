package ssh

import (
	"fmt"
	"log"
	"time"

	"github.com/kamikazebr/roamie-desktop/internal/client/api"
	"github.com/kamikazebr/roamie-desktop/internal/client/config"
)

// Manager handles SSH key synchronization
type Manager struct {
	apiClient   *api.Client
	keysManager *AuthorizedKeysManager
	stats       SyncStats
}

// NewManager creates a new SSH sync manager
func NewManager(serverURL string) (*Manager, error) {
	keysManager, err := NewAuthorizedKeysManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create authorized_keys manager: %w", err)
	}

	return &Manager{
		apiClient:   api.NewClient(serverURL),
		keysManager: keysManager,
		stats:       SyncStats{},
	}, nil
}

// SyncKeys synchronizes SSH keys from Firestore to authorized_keys
func (m *Manager) SyncKeys(cfg *config.Config) (*SyncResult, error) {
	if cfg == nil || cfg.JWT == "" {
		return nil, fmt.Errorf("not authenticated: JWT token required")
	}

	// Check if SSH sync is enabled
	if !cfg.SSHSyncEnabled {
		return nil, fmt.Errorf("SSH sync is disabled")
	}

	result := &SyncResult{
		SyncedAt: time.Now(),
	}

	// Fetch keys from server
	apiKeys, err := m.apiClient.GetSSHKeys(cfg.JWT)
	if err != nil {
		result.Error = fmt.Errorf("failed to fetch SSH keys: %w", err)
		m.stats.LastError = result.Error.Error()
		return result, result.Error
	}

	// Convert API keys to public key strings
	var publicKeys []string
	for _, key := range apiKeys {
		publicKeys = append(publicKeys, key.PublicKey)
	}

	// Update authorized_keys file
	added, removed, err := m.keysManager.UpdateRoamieKeys(publicKeys)
	if err != nil {
		result.Error = fmt.Errorf("failed to update authorized_keys: %w", err)
		m.stats.LastError = result.Error.Error()
		return result, result.Error
	}

	result.Added = added
	result.Removed = removed
	result.Total = len(publicKeys)

	// Update stats
	m.stats.LastSync = result.SyncedAt
	m.stats.TotalSyncs++
	m.stats.LastKeysCount = result.Total
	m.stats.LastError = ""

	// Log sync activity
	if len(added) > 0 {
		log.Printf("SSH sync: Added %d key(s)", len(added))
	}
	if len(removed) > 0 {
		log.Printf("SSH sync: Removed %d key(s)", len(removed))
	}
	if len(added) == 0 && len(removed) == 0 {
		log.Printf("SSH sync: No changes (total: %d keys)", result.Total)
	}

	return result, nil
}

// GetStats returns sync statistics
func (m *Manager) GetStats() SyncStats {
	return m.stats
}

// GetCurrentKeys returns current Roamie-managed keys from authorized_keys
func (m *Manager) GetCurrentKeys() ([]string, error) {
	roamieKeys, _, err := m.keysManager.ReadKeys()
	return roamieKeys, err
}

// SyncTunnelKeys synchronizes tunnel SSH keys from server to authorized_keys
// This allows devices in the same account to access each other through tunnels
func (m *Manager) SyncTunnelKeys(jwt string) (*SyncResult, error) {
	if jwt == "" {
		return nil, fmt.Errorf("not authenticated: JWT token required")
	}

	result := &SyncResult{
		SyncedAt: time.Now(),
	}

	// Fetch tunnel keys from server
	tunnelKeys, err := m.apiClient.GetTunnelAuthorizedKeys(jwt)
	if err != nil {
		result.Error = fmt.Errorf("failed to fetch tunnel authorized keys: %w", err)
		return result, result.Error
	}

	// Convert to public key strings with comments
	var publicKeys []string
	for _, key := range tunnelKeys {
		publicKeys = append(publicKeys, key.PublicKey)
	}

	// Update authorized_keys file
	added, removed, err := m.keysManager.UpdateRoamieKeys(publicKeys)
	if err != nil {
		result.Error = fmt.Errorf("failed to update authorized_keys: %w", err)
		return result, result.Error
	}

	result.Added = added
	result.Removed = removed
	result.Total = len(publicKeys)

	// Log sync activity
	if len(added) > 0 {
		log.Printf("Tunnel SSH sync: Added %d key(s)", len(added))
	}
	if len(removed) > 0 {
		log.Printf("Tunnel SSH sync: Removed %d key(s)", len(removed))
	}
	if len(added) == 0 && len(removed) == 0 {
		log.Printf("Tunnel SSH sync: No changes (total: %d keys)", result.Total)
	}

	return result, nil
}

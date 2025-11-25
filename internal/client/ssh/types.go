package ssh

import "time"

// SSHKey represents an SSH public key
type SSHKey struct {
	PublicKey   string `json:"publicKey"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Fingerprint string `json:"fingerprint"`
}

// SyncResult contains the result of a sync operation
type SyncResult struct {
	Added    []string  // Keys that were added
	Removed  []string  // Keys that were removed
	Total    int       // Total keys after sync
	Error    error     // Error if sync failed
	SyncedAt time.Time // Time of sync
}

// SyncStats tracks SSH key sync statistics
type SyncStats struct {
	LastSync      time.Time
	TotalSyncs    int
	LastKeysCount int
	LastError     string
}

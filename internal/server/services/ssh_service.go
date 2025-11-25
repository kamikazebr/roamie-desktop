package services

import (
	"context"
	"fmt"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"github.com/kamikazebr/roamie-desktop/pkg/crypto"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type SSHService struct {
	firestoreClient *firestore.Client
}

// SSHKey represents an SSH public key from Firestore
type SSHKey struct {
	PublicKey   string `firestore:"publicKey" json:"publicKey"`
	Name        string `firestore:"name" json:"name"`
	Type        string `firestore:"type" json:"type"`
	Fingerprint string `firestore:"fingerprint" json:"fingerprint"`
}

// EncryptedSSHKey represents an encrypted SSH key from Firestore
type EncryptedSSHKey struct {
	PublicKey           string `firestore:"publicKey" json:"publicKey"`
	EncryptedPrivateKey string `firestore:"encryptedPrivateKey" json:"encryptedPrivateKey"`
	Name                string `firestore:"name" json:"name"`
	Type                string `firestore:"type" json:"type"`
	Fingerprint         string `firestore:"fingerprint" json:"fingerprint"`
}

// EncryptionConfig represents the encryption configuration for a user
type EncryptionConfig struct {
	Salt       string `firestore:"salt" json:"salt"`
	Algorithm  string `firestore:"algorithm" json:"algorithm"`
	Iterations int    `firestore:"iterations" json:"iterations"`
}

// NewSSHService initializes SSH service with Firestore client
func NewSSHService(ctx context.Context) (*SSHService, error) {
	// Check if Firebase is configured
	credentialsPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if credentialsPath == "" {
		return nil, fmt.Errorf("FIREBASE_CREDENTIALS_PATH not set")
	}

	// Initialize Firebase app with service account credentials
	opt := option.WithCredentialsFile(credentialsPath)
	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	}

	// Get Firestore client
	firestoreClient, err := app.Firestore(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Firestore client: %w", err)
	}

	return &SSHService{
		firestoreClient: firestoreClient,
	}, nil
}

// GetUserSSHKeys fetches all SSH keys for a user from Firestore
// userEmail: email used as document ID in Firestore (users/{email}/ssh_keys)
func (s *SSHService) GetUserSSHKeys(ctx context.Context, userEmail string) ([]SSHKey, error) {
	if userEmail == "" {
		return nil, fmt.Errorf("user email is required")
	}

	// Query Firestore: users/{email}/ssh_keys
	iter := s.firestoreClient.Collection("users").
		Doc(userEmail).
		Collection("ssh_keys").
		Documents(ctx)

	var keys []SSHKey
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate SSH keys: %w", err)
		}

		var key SSHKey
		if err := doc.DataTo(&key); err != nil {
			return nil, fmt.Errorf("failed to parse SSH key document: %w", err)
		}

		keys = append(keys, key)
	}

	return keys, nil
}

// Close closes the Firestore client
func (s *SSHService) Close() error {
	if s.firestoreClient != nil {
		return s.firestoreClient.Close()
	}
	return nil
}

// GetEncryptionConfig fetches the encryption configuration (salt) for a user
// Path: users/{email}/encryption/config
func (s *SSHService) GetEncryptionConfig(ctx context.Context, userEmail, configID string) (*EncryptionConfig, error) {
	// Default config ID if not provided
	if configID == "" {
		configID = "config"
	}

	// Fetch from Firestore: users/{email}/encryption/config
	doc, err := s.firestoreClient.Collection("users").
		Doc(userEmail).
		Collection("encryption").
		Doc(configID).
		Get(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to get encryption config: %w", err)
	}

	var config EncryptionConfig
	if err := doc.DataTo(&config); err != nil {
		return nil, fmt.Errorf("failed to parse encryption config: %w", err)
	}

	return &config, nil
}

// GetEncryptedSSHKeys fetches all encrypted SSH keys for a user
func (s *SSHService) GetEncryptedSSHKeys(ctx context.Context, userEmail string) ([]EncryptedSSHKey, error) {
	if userEmail == "" {
		return nil, fmt.Errorf("user email is required")
	}

	// Query Firestore: users/{email}/ssh_keys
	iter := s.firestoreClient.Collection("users").
		Doc(userEmail).
		Collection("ssh_keys").
		Documents(ctx)

	var keys []EncryptedSSHKey
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate SSH keys: %w", err)
		}

		var key EncryptedSSHKey
		if err := doc.DataTo(&key); err != nil {
			return nil, fmt.Errorf("failed to parse SSH key document: %w", err)
		}

		keys = append(keys, key)
	}

	return keys, nil
}

// KeyValidationResult represents the result of validating a single key
type KeyValidationResult struct {
	Name        string
	Fingerprint string
	Type        string
	Valid       bool
	Error       string
}

// ValidationSummary represents the summary of key validation
type ValidationSummary struct {
	TotalKeys    int
	ValidKeys    int
	InvalidKeys  int
	Results      []KeyValidationResult
	Salt         string
	EncryptionOK bool
}

// ValidateKeyDecryption validates that encrypted SSH keys can be decrypted with the provided password
// Returns summary with per-key validation results (without exposing private keys)
func (s *SSHService) ValidateKeyDecryption(ctx context.Context, userEmail, password string) (*ValidationSummary, error) {
	if userEmail == "" {
		return nil, fmt.Errorf("user email is required")
	}
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}

	summary := &ValidationSummary{
		Results: []KeyValidationResult{},
	}

	// Get encryption config (salt)
	config, err := s.GetEncryptionConfig(ctx, userEmail, "")
	if err != nil {
		summary.EncryptionOK = false
		return summary, fmt.Errorf("failed to get encryption config: %w", err)
	}
	summary.Salt = config.Salt
	summary.EncryptionOK = true

	// Get all encrypted SSH keys
	keys, err := s.GetEncryptedSSHKeys(ctx, userEmail)
	if err != nil {
		return summary, fmt.Errorf("failed to get SSH keys: %w", err)
	}

	summary.TotalKeys = len(keys)

	// Validate each key
	for _, key := range keys {
		result := KeyValidationResult{
			Name:        key.Name,
			Fingerprint: key.Fingerprint,
			Type:        key.Type,
		}

		// Skip keys without encrypted private key
		if key.EncryptedPrivateKey == "" {
			result.Valid = false
			result.Error = "No encrypted private key found"
			summary.InvalidKeys++
			summary.Results = append(summary.Results, result)
			continue
		}

		// Attempt decryption and validation
		err := crypto.ValidateDecryption(key.EncryptedPrivateKey, password, config.Salt)
		if err != nil {
			result.Valid = false
			result.Error = err.Error()
			summary.InvalidKeys++
		} else {
			result.Valid = true
			summary.ValidKeys++
		}

		summary.Results = append(summary.Results, result)
	}

	return summary, nil
}

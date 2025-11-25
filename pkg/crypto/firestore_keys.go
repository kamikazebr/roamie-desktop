package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/ssh"
)

const (
	// PBKDF2 parameters (must match Flutter implementation)
	PBKDF2Iterations = 100000
	PBKDF2KeyLength  = 32 // 256 bits for AES-256
	AESKeySize       = 32 // 256 bits
	AESNonceSize     = 16 // 128 bits (GCM IV size)
)

// EncryptedKeyData represents the JSON structure of encrypted private keys in Firestore
type EncryptedKeyData struct {
	IV         string `json:"iv"`
	Ciphertext string `json:"ciphertext"`
}

// DeriveKey derives an AES-256 key from password and salt using PBKDF2
// Matches Flutter's implementation: PBKDF2(password, salt, 100k iterations, SHA-256)
func DeriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, PBKDF2Iterations, PBKDF2KeyLength, sha256.New)
}

// DecryptPrivateKey decrypts an SSH private key encrypted with AES-256-GCM
// Parameters:
//   - encryptedData: JSON string with format {"iv":"base64","ciphertext":"base64"}
//   - password: User's encryption password
//   - salt: PBKDF2 salt (base64-encoded)
//
// Returns:
//   - decrypted private key (PEM format)
//   - error if decryption fails
func DecryptPrivateKey(encryptedData, password, saltBase64 string) (string, error) {
	// Parse encrypted data JSON
	var encrypted EncryptedKeyData
	if err := json.Unmarshal([]byte(encryptedData), &encrypted); err != nil {
		return "", fmt.Errorf("failed to parse encrypted data JSON: %w", err)
	}

	// Decode base64 components
	iv, err := base64.StdEncoding.DecodeString(encrypted.IV)
	if err != nil {
		return "", fmt.Errorf("failed to decode IV: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	salt, err := base64.StdEncoding.DecodeString(saltBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode salt: %w", err)
	}

	// Derive decryption key from password and salt
	key := DeriveKey(password, salt)

	// Create AES-256 cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Verify IV size (accept both 12-byte standard and 16-byte alternative used by Flutter)
	if len(iv) != 12 && len(iv) != 16 {
		return "", fmt.Errorf("invalid IV size: expected 12 or 16, got %d", len(iv))
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong password or corrupted data): %w", err)
	}

	return string(plaintext), nil
}

// ValidateSSHPrivateKey checks if the decrypted data is a valid SSH private key
// Supports formats: PEM (RSA, Ed25519, ECDSA) and OpenSSH format
func ValidateSSHPrivateKey(privateKeyData string) error {
	// Try to parse as SSH private key
	_, err := ssh.ParsePrivateKey([]byte(privateKeyData))
	if err != nil {
		return fmt.Errorf("invalid SSH private key format: %w", err)
	}
	return nil
}

// ValidateDecryption attempts to decrypt an encrypted SSH key and validates the result
// Returns nil if decryption successful and key is valid, error otherwise
func ValidateDecryption(encryptedData, password, saltBase64 string) error {
	// Decrypt
	privateKey, err := DecryptPrivateKey(encryptedData, password, saltBase64)
	if err != nil {
		return err
	}

	// Validate it's a real SSH private key
	if err := ValidateSSHPrivateKey(privateKey); err != nil {
		return err
	}

	return nil
}

// GetKeyFingerprint returns the SSH fingerprint of a decrypted private key
// Useful for verification without exposing the private key
func GetKeyFingerprint(privateKeyData string) (string, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privateKeyData))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	publicKey := signer.PublicKey()
	fingerprint := ssh.FingerprintSHA256(publicKey)

	return fingerprint, nil
}

// GetKeyType returns the type of SSH key (e.g., "ssh-rsa", "ssh-ed25519")
func GetKeyType(privateKeyData string) (string, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privateKeyData))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	publicKey := signer.PublicKey()
	keyType := publicKey.Type()

	return keyType, nil
}

// SanitizePrivateKey replaces the actual key material with asterisks for logging
// Preserves PEM header/footer for format identification
func SanitizePrivateKey(privateKeyData string) string {
	lines := strings.Split(privateKeyData, "\n")
	var sanitized []string

	for _, line := range lines {
		if strings.HasPrefix(line, "-----") {
			// Keep PEM headers/footers
			sanitized = append(sanitized, line)
		} else if len(line) > 0 {
			// Replace key data with asterisks
			sanitized = append(sanitized, "****************************************************")
		} else {
			sanitized = append(sanitized, "")
		}
	}

	return strings.Join(sanitized, "\n")
}

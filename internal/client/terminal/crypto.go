package terminal

import (
	"crypto/cipher"
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/chacha20poly1305"
)

// CryptoContext handles E2E encryption for terminal sessions
type CryptoContext struct {
	cipher cipher.AEAD
	nonce  []byte
}

// NewCryptoContext creates a new encryption context with the given key
func NewCryptoContext(key []byte) (*CryptoContext, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, fmt.Errorf("invalid key size: expected %d, got %d", chacha20poly1305.KeySize, len(key))
	}

	cipher, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	return &CryptoContext{
		cipher: cipher,
		nonce:  make([]byte, cipher.NonceSize()),
	}, nil
}

// Encrypt encrypts plaintext and returns ciphertext
func (c *CryptoContext) Encrypt(plaintext []byte) ([]byte, error) {
	// Generate random nonce
	if _, err := rand.Read(c.nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and append nonce to ciphertext
	ciphertext := c.cipher.Seal(nil, c.nonce, plaintext, nil)
	return append(c.nonce, ciphertext...), nil
}

// Decrypt decrypts ciphertext and returns plaintext
func (c *CryptoContext) Decrypt(ciphertext []byte) ([]byte, error) {
	nonceSize := c.cipher.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := c.cipher.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// GenerateKey generates a random encryption key
func GenerateKey() ([]byte, error) {
	key := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

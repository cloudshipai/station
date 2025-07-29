package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"
)

// KeyVersion represents a versioned encryption key
type KeyVersion struct {
	ID        string    `json:"id"`
	Key       *Key      `json:"key"`
	CreatedAt time.Time `json:"created_at"`
	IsActive  bool      `json:"is_active"`
}

// KeyManager handles encryption key rotation and versioning
type KeyManager struct {
	keys      map[string]*KeyVersion
	activeKey *KeyVersion
}

// NewKeyManager creates a new key manager with an initial key
func NewKeyManager(initialKey *Key) *KeyManager {
	keyID := generateKeyID()
	keyVersion := &KeyVersion{
		ID:        keyID,
		Key:       initialKey,
		CreatedAt: time.Now(),
		IsActive:  true,
	}

	return &KeyManager{
		keys: map[string]*KeyVersion{
			keyID: keyVersion,
		},
		activeKey: keyVersion,
	}
}

// NewKeyManagerFromEnv creates a new key manager using the ENCRYPTION_KEY environment variable
func NewKeyManagerFromEnv() (*KeyManager, error) {
	keyHex := os.Getenv("ENCRYPTION_KEY")
	if keyHex == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY environment variable is required")
	}
	
	keyBytes, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ENCRYPTION_KEY: %w", err)
	}
	
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes (64 hex characters), got %d bytes", len(keyBytes))
	}
	
	key := &Key{}
	copy(key[:], keyBytes)
	
	return NewKeyManager(key), nil
}

// GetActiveKey returns the currently active encryption key
func (km *KeyManager) GetActiveKey() *KeyVersion {
	return km.activeKey
}

// GetKeyByID returns a specific key version by its ID
func (km *KeyManager) GetKeyByID(keyID string) (*KeyVersion, error) {
	key, exists := km.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key with ID %s not found", keyID)
	}
	return key, nil
}

// RotateKey creates a new key and sets it as active
func (km *KeyManager) RotateKey() (*KeyVersion, error) {
	newKey, err := GenerateRandomKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate new key: %w", err)
	}

	keyID := generateKeyID()
	newKeyVersion := &KeyVersion{
		ID:        keyID,
		Key:       newKey,
		CreatedAt: time.Now(),
		IsActive:  true,
	}

	// Mark old key as inactive
	if km.activeKey != nil {
		km.activeKey.IsActive = false
	}

	// Add new key and set as active
	km.keys[keyID] = newKeyVersion
	km.activeKey = newKeyVersion

	return newKeyVersion, nil
}

// ListKeys returns all key versions
func (km *KeyManager) ListKeys() []*KeyVersion {
	keys := make([]*KeyVersion, 0, len(km.keys))
	for _, key := range km.keys {
		keys = append(keys, key)
	}
	return keys
}

// AddKey adds an existing key version to the manager
func (km *KeyManager) AddKey(keyVersion *KeyVersion) {
	km.keys[keyVersion.ID] = keyVersion
	if keyVersion.IsActive {
		// Deactivate current active key
		if km.activeKey != nil {
			km.activeKey.IsActive = false
		}
		km.activeKey = keyVersion
	}
}

// EncryptWithVersion encrypts data with the active key and returns the key ID
func (km *KeyManager) EncryptWithVersion(data []byte) (ciphertext []byte, keyID string, err error) {
	if km.activeKey == nil {
		return nil, "", fmt.Errorf("no active key available")
	}

	ciphertext, err = Encrypt(data, km.activeKey.Key)
	if err != nil {
		return nil, "", err
	}

	return ciphertext, km.activeKey.ID, nil
}

// DecryptWithVersion decrypts data using the specified key version
func (km *KeyManager) DecryptWithVersion(ciphertext []byte, keyID string) ([]byte, error) {
	keyVersion, err := km.GetKeyByID(keyID)
	if err != nil {
		return nil, err
	}

	return Decrypt(ciphertext, keyVersion.Key)
}

// generateKeyID creates a unique identifier for a key version
func generateKeyID() string {
	// Generate a random 16-byte ID and encode as hex
	idBytes := make([]byte, 16)
	rand.Read(idBytes)
	return fmt.Sprintf("key_%d_%s", time.Now().Unix(), hex.EncodeToString(idBytes)[:8])
}

// MigrateData re-encrypts data from old key to new key (for key rotation)
func (km *KeyManager) MigrateData(oldCiphertext []byte, oldKeyID string) (newCiphertext []byte, newKeyID string, err error) {
	// Decrypt with old key
	plaintext, err := km.DecryptWithVersion(oldCiphertext, oldKeyID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decrypt with old key: %w", err)
	}

	// Encrypt with active key
	newCiphertext, newKeyID, err = km.EncryptWithVersion(plaintext)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt with new key: %w", err)
	}

	return newCiphertext, newKeyID, nil
}
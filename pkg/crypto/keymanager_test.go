package crypto

import (
	"bytes"
	"testing"
	"time"
)

func TestNewKeyManager(t *testing.T) {
	key, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	km := NewKeyManager(key)

	if km.activeKey == nil {
		t.Error("Expected active key to be set")
	}

	if !km.activeKey.IsActive {
		t.Error("Expected initial key to be active")
	}

	if len(km.keys) != 1 {
		t.Errorf("Expected 1 key in manager, got %d", len(km.keys))
	}
}

func TestKeyManager_GetActiveKey(t *testing.T) {
	key, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	km := NewKeyManager(key)
	activeKey := km.GetActiveKey()

	if activeKey == nil {
		t.Error("Expected active key to be returned")
	}

	if !bytes.Equal(activeKey.Key[:], key[:]) {
		t.Error("Active key does not match initial key")
	}
}

func TestKeyManager_RotateKey(t *testing.T) {
	initialKey, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate initial key: %v", err)
	}

	km := NewKeyManager(initialKey)
	oldKeyID := km.GetActiveKey().ID

	// Rotate the key
	newKeyVersion, err := km.RotateKey()
	if err != nil {
		t.Fatalf("Failed to rotate key: %v", err)
	}

	// Check new key properties
	if newKeyVersion.ID == oldKeyID {
		t.Error("New key should have different ID")
	}

	if !newKeyVersion.IsActive {
		t.Error("New key should be active")
	}

	if bytes.Equal(newKeyVersion.Key[:], initialKey[:]) {
		t.Error("New key should be different from initial key")
	}

	// Check that active key changed
	currentActive := km.GetActiveKey()
	if currentActive.ID != newKeyVersion.ID {
		t.Error("Active key should be the new key")
	}

	// Check that old key is now inactive
	oldKey, err := km.GetKeyByID(oldKeyID)
	if err != nil {
		t.Fatalf("Failed to get old key: %v", err)
	}

	if oldKey.IsActive {
		t.Error("Old key should be inactive after rotation")
	}

	// Check total number of keys
	if len(km.keys) != 2 {
		t.Errorf("Expected 2 keys after rotation, got %d", len(km.keys))
	}
}

func TestKeyManager_GetKeyByID(t *testing.T) {
	key, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	km := NewKeyManager(key)
	keyID := km.GetActiveKey().ID

	// Test successful retrieval
	retrievedKey, err := km.GetKeyByID(keyID)
	if err != nil {
		t.Fatalf("Failed to get key by ID: %v", err)
	}

	if retrievedKey.ID != keyID {
		t.Errorf("Expected key ID %s, got %s", keyID, retrievedKey.ID)
	}

	// Test non-existent key
	_, err = km.GetKeyByID("nonexistent")
	if err == nil {
		t.Error("Expected error when getting non-existent key")
	}
}

func TestKeyManager_EncryptDecryptWithVersion(t *testing.T) {
	key, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	km := NewKeyManager(key)
	plaintext := []byte("Hello, Station!")

	// Encrypt with version
	ciphertext, keyID, err := km.EncryptWithVersion(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt with version: %v", err)
	}

	if keyID == "" {
		t.Error("Expected key ID to be returned")
	}

	// Decrypt with version
	decrypted, err := km.DecryptWithVersion(ciphertext, keyID)
	if err != nil {
		t.Fatalf("Failed to decrypt with version: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted text doesn't match original. Got %s, want %s", decrypted, plaintext)
	}
}

func TestKeyManager_MigrateData(t *testing.T) {
	initialKey, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate initial key: %v", err)
	}

	km := NewKeyManager(initialKey)
	plaintext := []byte("Data to migrate")

	// Encrypt with initial key
	oldCiphertext, oldKeyID, err := km.EncryptWithVersion(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt with initial key: %v", err)
	}

	// Rotate key
	_, err = km.RotateKey()
	if err != nil {
		t.Fatalf("Failed to rotate key: %v", err)
	}

	// Migrate data
	newCiphertext, newKeyID, err := km.MigrateData(oldCiphertext, oldKeyID)
	if err != nil {
		t.Fatalf("Failed to migrate data: %v", err)
	}

	// Verify new key ID is different
	if newKeyID == oldKeyID {
		t.Error("New key ID should be different after migration")
	}

	// Verify data can be decrypted with new key
	decrypted, err := km.DecryptWithVersion(newCiphertext, newKeyID)
	if err != nil {
		t.Fatalf("Failed to decrypt migrated data: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Migrated data doesn't match original. Got %s, want %s", decrypted, plaintext)
	}

	// Verify old data can still be decrypted with old key
	oldDecrypted, err := km.DecryptWithVersion(oldCiphertext, oldKeyID)
	if err != nil {
		t.Fatalf("Failed to decrypt with old key: %v", err)
	}

	if !bytes.Equal(plaintext, oldDecrypted) {
		t.Error("Old encrypted data should still be decryptable")
	}
}

func TestKeyManager_ListKeys(t *testing.T) {
	key, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	km := NewKeyManager(key)

	// Initially should have 1 key
	keys := km.ListKeys()
	if len(keys) != 1 {
		t.Errorf("Expected 1 key initially, got %d", len(keys))
	}

	// Rotate key and check again
	_, err = km.RotateKey()
	if err != nil {
		t.Fatalf("Failed to rotate key: %v", err)
	}

	keys = km.ListKeys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys after rotation, got %d", len(keys))
	}

	// Count active keys (should be 1)
	activeCount := 0
	for _, key := range keys {
		if key.IsActive {
			activeCount++
		}
	}

	if activeCount != 1 {
		t.Errorf("Expected exactly 1 active key, got %d", activeCount)
	}
}

func TestKeyManager_AddKey(t *testing.T) {
	initialKey, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate initial key: %v", err)
	}

	km := NewKeyManager(initialKey)

	// Create a new key version
	newKey, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate new key: %v", err)
	}

	newKeyVersion := &KeyVersion{
		ID:        "custom_key_id",
		Key:       newKey,
		CreatedAt: time.Now(),
		IsActive:  true,
	}

	// Add the new key
	km.AddKey(newKeyVersion)

	// Check that it became the active key
	activeKey := km.GetActiveKey()
	if activeKey.ID != "custom_key_id" {
		t.Errorf("Expected active key ID 'custom_key_id', got '%s'", activeKey.ID)
	}

	// Check that the old key is now inactive
	keys := km.ListKeys()
	inactiveCount := 0
	for _, key := range keys {
		if !key.IsActive {
			inactiveCount++
		}
	}

	if inactiveCount != 1 {
		t.Errorf("Expected 1 inactive key, got %d", inactiveCount)
	}
}
package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key, err := GenerateRandomKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	plaintext := []byte("Hello, Station!")

	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted text doesn't match original. Got %s, want %s", decrypted, plaintext)
	}
}

func TestNewKey(t *testing.T) {
	keyBytes := make([]byte, 32)
	for i := range keyBytes {
		keyBytes[i] = byte(i)
	}

	key, err := NewKey(keyBytes)
	if err != nil {
		t.Fatalf("Failed to create key: %v", err)
	}

	if !bytes.Equal(key[:], keyBytes) {
		t.Errorf("Key bytes don't match")
	}
}

func TestNewKeyInvalidSize(t *testing.T) {
	_, err := NewKey([]byte("too short"))
	if err == nil {
		t.Error("Expected error for invalid key size")
	}
}
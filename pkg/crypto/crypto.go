package crypto

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/nacl/secretbox"
)

const (
	KeySize   = 32
	NonceSize = 24
)

type Key [KeySize]byte

func NewKey(keyBytes []byte) (*Key, error) {
	if len(keyBytes) != KeySize {
		return nil, fmt.Errorf("key must be exactly %d bytes, got %d", KeySize, len(keyBytes))
	}
	
	var key Key
	copy(key[:], keyBytes)
	return &key, nil
}

func Encrypt(data []byte, key *Key) ([]byte, error) {
	var nonce [NonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	encrypted := secretbox.Seal(nonce[:], data, &nonce, (*[KeySize]byte)(key))
	return encrypted, nil
}

func Decrypt(ciphertext []byte, key *Key) ([]byte, error) {
	if len(ciphertext) < NonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	var nonce [NonceSize]byte
	copy(nonce[:], ciphertext[:NonceSize])

	decrypted, ok := secretbox.Open(nil, ciphertext[NonceSize:], &nonce, (*[KeySize]byte)(key))
	if !ok {
		return nil, fmt.Errorf("decryption failed")
	}

	return decrypted, nil
}

func GenerateRandomKey() (*Key, error) {
	var key Key
	if _, err := rand.Read(key[:]); err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	return &key, nil
}
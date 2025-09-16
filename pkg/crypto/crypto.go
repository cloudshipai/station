package crypto

// KeyManager is a stub interface for the removed crypto functionality
type KeyManager struct{}

// NewKeyManagerFromEnv creates a stub key manager since crypto was removed
func NewKeyManagerFromEnv() (*KeyManager, error) {
	return &KeyManager{}, nil
}

// Note: This is a stub implementation since the crypto package was removed
// during cleanup. The MCP handlers that use this are deprecated and should
// be removed in favor of file-based configuration.
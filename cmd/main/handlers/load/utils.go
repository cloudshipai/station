package load

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
	"station/pkg/crypto"
)

// generateUniqueConfigName adds a timestamp suffix to prevent duplicates
func (h *LoadHandler) generateUniqueConfigName(baseName string) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s", baseName, timestamp)
}

// createKeyManagerFromConfig creates a key manager using the encryption key from config file
func createKeyManagerFromConfig() (*crypto.KeyManager, error) {
	// Get encryption key from viper (config file)
	encryptionKey := viper.GetString("encryption_key")
	return crypto.NewKeyManagerFromConfig(encryptionKey)
}

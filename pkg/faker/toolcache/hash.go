package toolcache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// GenerateConfigHash creates a deterministic hash from faker configuration
// This ensures the same configuration always generates the same cache key
func GenerateConfigHash(fakerName, aiInstruction, aiModel string) string {
	// Normalize inputs
	fakerName = strings.TrimSpace(fakerName)
	aiInstruction = strings.TrimSpace(aiInstruction)
	aiModel = strings.TrimSpace(aiModel)

	// Use default model if not specified
	if aiModel == "" {
		aiModel = "default"
	}

	// Create deterministic string from configuration
	// Format: fakerName|aiInstruction|aiModel
	configStr := fmt.Sprintf("%s|%s|%s", fakerName, aiInstruction, aiModel)

	// Generate SHA256 hash
	hash := sha256.Sum256([]byte(configStr))
	hashStr := hex.EncodeToString(hash[:])

	// Return first 16 chars for readability (still highly unique)
	// Format: fakerName-hash16
	return fmt.Sprintf("%s-%s", fakerName, hashStr[:16])
}

// GenerateConfigHashWithEnv creates a hash including environment variables
// Useful when env vars affect tool generation (advanced use case)
func GenerateConfigHashWithEnv(fakerName, aiInstruction, aiModel string, envVars map[string]string) string {
	// Start with base config hash
	baseStr := fmt.Sprintf("%s|%s|%s",
		strings.TrimSpace(fakerName),
		strings.TrimSpace(aiInstruction),
		strings.TrimSpace(aiModel))

	// Add sorted environment variables for determinism
	if len(envVars) > 0 {
		var envKeys []string
		for k := range envVars {
			envKeys = append(envKeys, k)
		}
		sort.Strings(envKeys)

		var envPairs []string
		for _, k := range envKeys {
			envPairs = append(envPairs, fmt.Sprintf("%s=%s", k, envVars[k]))
		}
		baseStr += "|" + strings.Join(envPairs, ",")
	}

	// Generate SHA256 hash
	hash := sha256.Sum256([]byte(baseStr))
	hashStr := hex.EncodeToString(hash[:])

	// Return fakerName-hash16 format
	return fmt.Sprintf("%s-%s", strings.TrimSpace(fakerName), hashStr[:16])
}

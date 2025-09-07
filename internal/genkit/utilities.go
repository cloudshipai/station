// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Utility functions for the Station GenKit generator
// Contains helper functions for ID generation, JSON conversion, and timestamp utilities
package genkit

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openai/openai-go"
)

// generateToolCallID creates a unique tool call ID for OpenAI API
// This is critical for proper tool calling functionality
func generateToolCallID() string {
	// Generate 8 random bytes and convert to hex
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return fmt.Sprintf("call_%x", bytes)
}

// jsonStringToMap converts a JSON string to a map
func jsonStringToMap(jsonString string) map[string]any {
	var result map[string]any
	err := json.Unmarshal([]byte(jsonString), &result)
	if err != nil {
		return map[string]any{"error": "Failed to parse JSON", "raw": jsonString}
	}
	return result
}

// anyToJSONString converts any data structure to a JSON string
func anyToJSONString(data any) string {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("Error marshaling to JSON: %v", err)
	}
	return string(jsonBytes)
}

// getCurrentTimestampNano returns the current timestamp in nanoseconds
func getCurrentTimestampNano() int64 {
	return time.Now().UnixNano()
}

// getMaxTokensFromRequest extracts max_tokens from the OpenAI request (unused but kept for compatibility)
func getMaxTokensFromRequest(req *openai.ChatCompletionNewParams) any {
	// This function is unused but preserved for potential future use
	return nil
}
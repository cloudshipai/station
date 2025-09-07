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

// Context management functionality for Station GenKit generator
// This file contains the surgical fix for the Node.js agent timeout issue
// Functions are called from generate.go before each API request
package genkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"station/internal/logging"

	"github.com/openai/openai-go"
)

// manageContextSize checks for large tool responses and summarizes them to prevent context overflow
// This is the surgical fix for the Node.js agent timeout issue - called before each API request
func (g *StationModelGenerator) manageContextSize(_ context.Context) error {
	const maxTokenThreshold = 1000 // Lowered for debugging - will be 10000 in production

	logging.Debug("Station GenKit: Context management called with %d messages", len(g.request.Messages))
	
	optimizedAny := false
	for i, msg := range g.request.Messages {
		logging.Debug("Station GenKit: Checking message %d, type: %T", i, msg)
		
		if msg.OfTool != nil {
			// This is a tool response message - check its size
			content, tokenCount := g.extractAndCountToolContent(msg.OfTool.Content)
			logging.Debug("Station GenKit: Tool message %d has %d tokens", i, tokenCount)
			
			if tokenCount > maxTokenThreshold {
				logging.Debug("Station GenKit: Optimizing large tool response (tokens: %d)", tokenCount)
				
				// Optimize the content
				optimizedContent := g.summarizeOrTruncateToolContent(content, maxTokenThreshold)
				
				// Update the message with optimized content
				g.request.Messages[i] = openai.ChatCompletionMessageParamUnion{
					OfTool: &openai.ChatCompletionToolMessageParam{
						ToolCallID: msg.OfTool.ToolCallID,
						Content: openai.ChatCompletionToolMessageParamContentUnion{
							OfString: openai.String(optimizedContent),
						},
					},
				}
				
				optimizedAny = true
				
				if g.logCallback != nil {
					g.logCallback(map[string]interface{}{
						"event":          "context_optimization",
						"message":        fmt.Sprintf("Large tool response optimized from %d to %d tokens", tokenCount, g.estimateTokenCount(optimizedContent)),
						"original_tokens": tokenCount,
						"optimized_tokens": g.estimateTokenCount(optimizedContent),
						"timestamp_nano": getCurrentTimestampNano(),
					})
				}
			}
		} else if msg.OfAssistant != nil {
			logging.Debug("Station GenKit: Assistant message %d", i)
		} else if msg.OfUser != nil {
			logging.Debug("Station GenKit: User message %d", i)  
		} else if msg.OfSystem != nil {
			logging.Debug("Station GenKit: System message %d", i)
		} else {
			logging.Debug("Station GenKit: Unknown message type %d", i)
		}
	}

	if optimizedAny {
		if g.logCallback != nil {
			g.logCallback(map[string]interface{}{
				"event":   "context_management_completed",
				"message": "Context management: Large tool responses optimized",
				"timestamp_nano": getCurrentTimestampNano(),
			})
		}
		logging.Debug("Station GenKit: Context optimization applied")
	} else {
		logging.Debug("Station GenKit: No context optimization needed")
	}

	return nil
}

// extractAndCountToolContent extracts string content and estimates token count from tool message content
func (g *StationModelGenerator) extractAndCountToolContent(content any) (string, int) {
	var contentStr string
	
	// Handle different content types
	switch v := content.(type) {
	case string:
		contentStr = v
	case *string:
		if v != nil {
			contentStr = *v
		}
	default:
		// For complex objects, convert to JSON
		if jsonBytes, err := json.Marshal(v); err == nil {
			contentStr = string(jsonBytes)
		} else {
			contentStr = fmt.Sprintf("%v", v)
		}
	}
	
	tokenCount := g.estimateTokenCount(contentStr)
	return contentStr, tokenCount
}

// estimateTokenCount provides a rough token count estimate (1 token ≈ 4 characters)
func (g *StationModelGenerator) estimateTokenCount(text string) int {
	return len(text) / 4
}

// summarizeOrTruncateToolContent reduces large tool content while preserving key information
func (g *StationModelGenerator) summarizeOrTruncateToolContent(content string, maxTokens int) string {
	currentTokens := g.estimateTokenCount(content)
	if currentTokens <= maxTokens {
		return content
	}

	// Calculate target length (chars = tokens * 4, with some buffer for optimization marker)
	maxChars := maxTokens * 4 - 200 // Reserve space for optimization marker
	
	if len(content) <= maxChars {
		return content
	}

	// Truncate and add optimization marker
	truncated := content[:maxChars]
	
	// Try to truncate at a reasonable boundary (newline, period, etc.)
	if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > maxChars/2 {
		truncated = truncated[:lastNewline]
	} else if lastPeriod := strings.LastIndex(truncated, "."); lastPeriod > maxChars/2 {
		truncated = truncated[:lastPeriod+1]
	}

	return truncated + "\n\n---\n⚠️ CONTEXT OPTIMIZED: Content truncated to fit context window. Original size: " + 
		fmt.Sprintf("%d tokens, optimized to ~%d tokens", currentTokens, g.estimateTokenCount(truncated)) + "\n---"
}
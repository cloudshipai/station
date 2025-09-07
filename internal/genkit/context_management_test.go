// Copyright 2025 Station
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

package genkit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/assert"
)

func TestContextManagement(t *testing.T) {
	tests := []struct {
		name          string
		toolResponses []string
		expectOptimization bool
		expectedMessages   int
	}{
		{
			name:          "Small tool responses should not be optimized",
			toolResponses: []string{"small response", "another small one"},
			expectOptimization: false,
			expectedMessages:   2,
		},
		{
			name:          "Large tool response should be optimized",
			toolResponses: []string{strings.Repeat("Large response content. ", 2000)}, // ~42k chars = ~10.5k tokens
			expectOptimization: true,
			expectedMessages:   1,
		},
		{
			name:          "Mix of small and large responses",
			toolResponses: []string{
				"small response",
				strings.Repeat("Large security scan output. ", 2000), // Large
				"another small response",
			},
			expectOptimization: true,
			expectedMessages:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator := createTestGenerator()
			optimizationApplied := false

			// Set up log callback to track optimization
			generator.logCallback = func(logEntry map[string]interface{}) {
				if message, ok := logEntry["message"].(string); ok {
					if strings.Contains(message, "Context management: Large tool responses optimized") {
						optimizationApplied = true
					}
				}
			}

			// Create test messages with tool responses
			generator.request.Messages = createTestMessagesWithToolResponses(tt.toolResponses)

			// Apply context management
			err := generator.manageContextSize(context.Background())

			// Assertions
			assert.NoError(t, err)
			assert.Equal(t, tt.expectOptimization, optimizationApplied, "Optimization expectation mismatch")
			assert.Len(t, generator.request.Messages, tt.expectedMessages, "Message count mismatch")

			// Verify large responses were actually optimized
			if tt.expectOptimization {
				for _, msg := range generator.request.Messages {
					if msg.OfTool != nil {
						content := extractContentString(msg.OfTool.Content)
						tokenCount := len(content) / 4
						assert.LessOrEqual(t, tokenCount, 12000, "Tool response should be optimized to under 12k tokens")
						
						// Check for optimization marker
						if tokenCount < 12000 && len(content) > 100 {
							assert.Contains(t, content, "CONTEXT OPTIMIZED", "Large response should contain optimization marker")
						}
					}
				}
			}
		})
	}
}

func TestExtractAndCountToolContent(t *testing.T) {
	generator := createTestGenerator()

	tests := []struct {
		name     string
		content  interface{}
		expectTokens int
		expectString string
	}{
		{
			name:     "String content",
			content:  "Hello world",
			expectTokens: 2, // 11 chars / 4 = 2.75 ≈ 2
			expectString: "Hello world",
		},
		{
			name:     "JSON object content",
			content:  map[string]interface{}{"result": "success", "data": []string{"item1", "item2"}},
			expectTokens: 15, // JSON string will be longer
			expectString: `{"data":["item1","item2"],"result":"success"}`,
		},
		{
			name:     "Large string content",
			content:  strings.Repeat("Large content ", 1000), // 14k chars = 3.5k tokens
			expectTokens: 3500,
			expectString: strings.Repeat("Large content ", 1000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contentStr, tokenCount := generator.extractAndCountToolContent(tt.content)
			
			assert.Equal(t, tt.expectString, contentStr)
			assert.InDelta(t, tt.expectTokens, tokenCount, 10, "Token count should be approximately correct")
		})
	}
}

func TestSummarizeOrTruncateToolContent(t *testing.T) {
	generator := createTestGenerator()
	
	tests := []struct {
		name       string
		content    string
		maxTokens  int
		expectTruncation bool
	}{
		{
			name:       "Small content should not be truncated",
			content:    "Small response",
			maxTokens:  1000,
			expectTruncation: false,
		},
		{
			name:       "Large content should be truncated",
			content:    strings.Repeat("Large security scan result. ", 2000), // ~56k chars
			maxTokens:  5000, // Max 20k chars
			expectTruncation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.summarizeOrTruncateToolContent(tt.content, tt.maxTokens)
			
			if tt.expectTruncation {
				assert.Less(t, len(result), len(tt.content), "Result should be shorter than original")
				assert.Contains(t, result, "CONTEXT OPTIMIZED", "Truncated content should contain marker")
				
				// Should be close to max length
				maxChars := tt.maxTokens * 4
				assert.Less(t, len(result), maxChars + 500, "Result should be around max length")
			} else {
				assert.Equal(t, tt.content, result, "Small content should not be modified")
			}
		})
	}
}

func TestEstimateTokenCount(t *testing.T) {
	generator := createTestGenerator()
	
	tests := []struct {
		text     string
		expected int
	}{
		{"", 0},
		{"Hello", 1}, // 5 chars / 4 = 1.25 ≈ 1
		{"Hello world", 2}, // 11 chars / 4 = 2.75 ≈ 2
		{strings.Repeat("x", 400), 100}, // 400 chars / 4 = 100
	}

	for _, tt := range tests {
		result := generator.estimateTokenCount(tt.text)
		assert.Equal(t, tt.expected, result)
	}
}

// Helper functions for testing

func createTestGenerator() *StationModelGenerator {
	return &StationModelGenerator{
		client:   &openai.Client{},
		modelName: "gpt-4o",
		request:  &openai.ChatCompletionNewParams{},
	}
}

func createTestMessagesWithToolResponses(responses []string) []openai.ChatCompletionMessageParamUnion {
	messages := make([]openai.ChatCompletionMessageParamUnion, len(responses))
	
	for i, response := range responses {
		messages[i] = openai.ChatCompletionMessageParamUnion{
			OfTool: &openai.ChatCompletionToolMessageParam{
				ToolCallID: fmt.Sprintf("call_%d", i),
				Content: openai.ChatCompletionToolMessageParamContentUnion{
					OfString: openai.String(response),
				},
			},
		}
	}
	
	return messages
}

func extractContentString(content openai.ChatCompletionToolMessageParamContentUnion) string {
	// Simple string extraction for test purposes
	return fmt.Sprintf("%v", content)
}
package services

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestMessageStruct tests Message structure creation and fields
func TestMessageStruct(t *testing.T) {
	tests := []struct {
		name        string
		message     Message
		description string
	}{
		{
			name: "User message",
			message: Message{
				Content: "Hello",
				Role:    RoleUser,
			},
			description: "Should create user message",
		},
		{
			name: "Assistant message",
			message: Message{
				Content: "Hi there!",
				Role:    RoleAssistant,
			},
			description: "Should create assistant message",
		},
		{
			name: "System message",
			message: Message{
				Content: "You are a helpful assistant",
				Role:    RoleSystem,
			},
			description: "Should create system message",
		},
		{
			name: "Message with extra data",
			message: Message{
				Content: "Test",
				Role:    RoleUser,
				Extra: map[string]interface{}{
					"metadata": "value",
					"priority": 1,
				},
			},
			description: "Should include extra fields",
		},
		{
			name: "Empty message",
			message: Message{
				Content: "",
				Role:    "",
			},
			description: "Should handle empty message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.message.Content != tt.message.Content {
				t.Error("Content field mismatch")
			}

			if tt.message.Role != tt.message.Role {
				t.Error("Role field mismatch")
			}

			t.Logf("Message: role=%s, content=%s, extra=%v",
				tt.message.Role, tt.message.Content, tt.message.Extra)
		})
	}
}

// TestRoleConstants tests role constant values
func TestRoleConstants(t *testing.T) {
	tests := []struct {
		name        string
		role        string
		expected    string
		description string
	}{
		{
			name:        "User role",
			role:        RoleUser,
			expected:    "user",
			description: "Should equal 'user'",
		},
		{
			name:        "Assistant role",
			role:        RoleAssistant,
			expected:    "assistant",
			description: "Should equal 'assistant'",
		},
		{
			name:        "System role",
			role:        RoleSystem,
			expected:    "system",
			description: "Should equal 'system'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.role != tt.expected {
				t.Errorf("Role constant = %s, want %s", tt.role, tt.expected)
			}
		})
	}
}

// TestBackwardCompatibilityAssistant tests backward compatibility variable
func TestBackwardCompatibilityAssistant(t *testing.T) {
	if Assistant != RoleAssistant {
		t.Errorf("Assistant backward compatibility variable = %s, want %s", Assistant, RoleAssistant)
	}

	if Assistant != "assistant" {
		t.Errorf("Assistant value = %s, want 'assistant'", Assistant)
	}
}

// TestMessageJSONSerialization tests JSON marshaling
func TestMessageJSONSerialization(t *testing.T) {
	tests := []struct {
		name        string
		message     Message
		wantContain string
		description string
	}{
		{
			name: "Basic message",
			message: Message{
				Content: "Hello",
				Role:    RoleUser,
			},
			wantContain: "\"content\":\"Hello\"",
			description: "Should serialize to JSON",
		},
		{
			name: "Message with extra",
			message: Message{
				Content: "Test",
				Role:    RoleSystem,
				Extra: map[string]interface{}{
					"key": "value",
				},
			},
			wantContain: "\"extra\"",
			description: "Should include extra field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.message)
			if err != nil {
				t.Fatalf("Failed to marshal message: %v", err)
			}

			jsonStr := string(jsonData)
			if !strings.Contains(jsonStr, tt.wantContain) {
				t.Errorf("JSON should contain %q, got: %s", tt.wantContain, jsonStr)
			}

			t.Logf("Serialized JSON: %s", jsonStr)
		})
	}
}

// TestMessageJSONDeserialization tests JSON unmarshaling
func TestMessageJSONDeserialization(t *testing.T) {
	tests := []struct {
		name        string
		jsonStr     string
		wantRole    string
		wantContent string
		wantErr     bool
		description string
	}{
		{
			name:        "Valid JSON",
			jsonStr:     `{"content":"Hello","role":"user"}`,
			wantRole:    "user",
			wantContent: "Hello",
			wantErr:     false,
			description: "Should deserialize valid JSON",
		},
		{
			name:        "JSON with extra",
			jsonStr:     `{"content":"Test","role":"system","extra":{"key":"value"}}`,
			wantRole:    "system",
			wantContent: "Test",
			wantErr:     false,
			description: "Should deserialize with extra fields",
		},
		{
			name:        "Invalid JSON",
			jsonStr:     `{invalid json}`,
			wantErr:     true,
			description: "Should fail on invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg Message
			err := json.Unmarshal([]byte(tt.jsonStr), &msg)

			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if msg.Role != tt.wantRole {
					t.Errorf("Role = %s, want %s", msg.Role, tt.wantRole)
				}

				if msg.Content != tt.wantContent {
					t.Errorf("Content = %s, want %s", msg.Content, tt.wantContent)
				}

				t.Logf("Deserialized message: role=%s, content=%s", msg.Role, msg.Content)
			}
		})
	}
}

// TestMessageOmitEmptyExtra tests that empty Extra field is omitted
func TestMessageOmitEmptyExtra(t *testing.T) {
	msg := Message{
		Content: "Test",
		Role:    RoleUser,
		Extra:   nil, // Should be omitted
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	jsonStr := string(jsonData)
	if strings.Contains(jsonStr, "extra") {
		t.Errorf("JSON should omit empty extra field, got: %s", jsonStr)
	}
}

// TestMessageExtraFieldTypes tests various types in Extra map
func TestMessageExtraFieldTypes(t *testing.T) {
	tests := []struct {
		name        string
		extra       map[string]interface{}
		description string
	}{
		{
			name: "String values",
			extra: map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
			description: "Should handle string values",
		},
		{
			name: "Numeric values",
			extra: map[string]interface{}{
				"int":   42,
				"float": 3.14,
			},
			description: "Should handle numeric values",
		},
		{
			name: "Boolean values",
			extra: map[string]interface{}{
				"flag": true,
			},
			description: "Should handle boolean values",
		},
		{
			name: "Nested maps",
			extra: map[string]interface{}{
				"nested": map[string]interface{}{
					"inner": "value",
				},
			},
			description: "Should handle nested structures",
		},
		{
			name: "Array values",
			extra: map[string]interface{}{
				"list": []string{"a", "b", "c"},
			},
			description: "Should handle array values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{
				Content: "Test",
				Role:    RoleUser,
				Extra:   tt.extra,
			}

			// Verify serialization works
			jsonData, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			// Verify deserialization works
			var decoded Message
			if err := json.Unmarshal(jsonData, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if decoded.Extra == nil {
				t.Error("Extra field should not be nil after deserialization")
			}

			t.Logf("Extra field: %v", msg.Extra)
		})
	}
}

// TestMessageRoleValidation tests role value validation
func TestMessageRoleValidation(t *testing.T) {
	validRoles := []string{RoleUser, RoleAssistant, RoleSystem}

	for _, role := range validRoles {
		msg := Message{
			Content: "Test",
			Role:    role,
		}

		// Verify role is set correctly
		if msg.Role != role {
			t.Errorf("Role = %s, want %s", msg.Role, role)
		}
	}
}

// TestMessageContentVariations tests different content types
func TestMessageContentVariations(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		description string
	}{
		{
			name:        "Simple text",
			content:     "Hello, world!",
			description: "Should handle simple text",
		},
		{
			name:        "Multiline text",
			content:     "Line 1\nLine 2\nLine 3",
			description: "Should handle multiline content",
		},
		{
			name:        "Special characters",
			content:     "Special: @#$%^&*()",
			description: "Should handle special characters",
		},
		{
			name:        "Unicode characters",
			content:     "Unicode: 你好世界 🌍",
			description: "Should handle unicode",
		},
		{
			name:        "JSON-like content",
			content:     `{"key": "value"}`,
			description: "Should handle JSON-like strings",
		},
		{
			name:        "Empty content",
			content:     "",
			description: "Should handle empty content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{
				Content: tt.content,
				Role:    RoleUser,
			}

			if msg.Content != tt.content {
				t.Errorf("Content = %q, want %q", msg.Content, tt.content)
			}

			// Verify JSON round-trip
			jsonData, _ := json.Marshal(msg)
			var decoded Message
			if err := json.Unmarshal(jsonData, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if decoded.Content != tt.content {
				t.Errorf("After round-trip: content = %q, want %q", decoded.Content, tt.content)
			}
		})
	}
}

// Benchmark tests
func BenchmarkMessageCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Message{
			Content: "Test message",
			Role:    RoleUser,
			Extra: map[string]interface{}{
				"key": "value",
			},
		}
	}
}

func BenchmarkMessageJSONMarshal(b *testing.B) {
	msg := Message{
		Content: "Test message",
		Role:    RoleUser,
		Extra: map[string]interface{}{
			"key": "value",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		json.Marshal(msg)
	}
}

func BenchmarkMessageJSONUnmarshal(b *testing.B) {
	jsonStr := `{"content":"Test","role":"user","extra":{"key":"value"}}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var msg Message
		json.Unmarshal([]byte(jsonStr), &msg)
	}
}

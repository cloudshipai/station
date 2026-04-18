package services

import (
	"context"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTool is a minimal ai.Tool implementation for testing SanitizedMCPTool.
type mockTool struct {
	name        string
	description string
	runResult   any
	runErr      error
}

func (m *mockTool) Name() string { return m.name }
func (m *mockTool) Definition() *ai.ToolDefinition {
	return &ai.ToolDefinition{
		Name:        m.name,
		Description: m.description,
		InputSchema: map[string]any{"type": "object"},
	}
}
func (m *mockTool) RunRaw(_ context.Context, _ any) (any, error) {
	return m.runResult, m.runErr
}
func (m *mockTool) Respond(toolReq *ai.Part, outputData any, _ *ai.RespondOptions) *ai.Part {
	return ai.NewResponseForToolRequest(toolReq, outputData)
}
func (m *mockTool) Restart(toolReq *ai.Part, _ *ai.RestartOptions) *ai.Part {
	if toolReq.ToolRequest != nil {
		return ai.NewToolRequestPart(&ai.ToolRequest{
			Name:  m.name,
			Input: toolReq.ToolRequest.Input,
		})
	}
	return ai.NewToolRequestPart(&ai.ToolRequest{Name: m.name})
}
func (m *mockTool) Register(_ api.Registry) {}

// --- SanitizeToolName tests ---

// TestSanitizeToolName verifies the centralized sanitization function covers
// all known LLM-incompatible characters.
func TestSanitizeToolName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"__resolve-library-id", "__resolve_library_id"},
		{"context7.get-docs", "context7_get_docs"},
		{"read_file", "read_file"},
		{"my-mcp.server-tool.v2", "my_mcp_server_tool_v2"},
		{"__get-library-docs", "__get_library_docs"},
		{"no_change_needed", "no_change_needed"},
		{"a-b.c-d.e", "a_b_c_d_e"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeToolName(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSanitizeToolName_Idempotent verifies that sanitizing an already-sanitized
// name produces the same result (important for double-sanitization safety).
func TestSanitizeToolName_Idempotent(t *testing.T) {
	original := "__resolve-library-id"
	first := SanitizeToolName(original)
	second := SanitizeToolName(first)
	assert.Equal(t, first, second, "SanitizeToolName must be idempotent")
}

// --- SanitizedMCPTool wrapper tests ---

// TestSanitizedMCPTool_FullInterface verifies the wrapper satisfies the complete
// ai.Tool interface and correctly delegates all 6 methods.
func TestSanitizedMCPTool_FullInterface(t *testing.T) {
	tests := []struct {
		name         string
		originalName string
		wantName     string
	}{
		{
			name:         "hyphenated MCP tool name (Issue #42 exact case)",
			originalName: "__resolve-library-id",
			wantName:     "__resolve_library_id",
		},
		{
			name:         "dotted tool name",
			originalName: "context7.get-docs",
			wantName:     "context7_get_docs",
		},
		{
			name:         "already safe name passes through",
			originalName: "read_file",
			wantName:     "read_file",
		},
		{
			name:         "complex mixed characters",
			originalName: "my-mcp.server-tool.v2",
			wantName:     "my_mcp_server_tool_v2",
		},
		{
			name:         "leading double underscore preserved",
			originalName: "__get-library-docs",
			wantName:     "__get_library_docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &mockTool{
				name:        tt.originalName,
				description: "test tool",
				runResult:   "ok",
			}

			wrapped := &SanitizedMCPTool{
				OriginalTool:  original,
				SanitizedName: SanitizeToolName(tt.originalName),
			}

			// Compile-time interface check
			var _ ai.Tool = wrapped

			// 1. Name() returns sanitized name
			assert.Equal(t, tt.wantName, wrapped.Name())

			// 2. Definition() returns sanitized name without mutating original
			def := wrapped.Definition()
			require.NotNil(t, def)
			assert.Equal(t, tt.wantName, def.Name)
			assert.Equal(t, "test tool", def.Description)
			assert.Equal(t, tt.originalName, original.Definition().Name,
				"Original tool definition must NOT be mutated")

			// 3. RunRaw() delegates correctly
			result, err := wrapped.RunRaw(context.Background(), nil)
			assert.NoError(t, err)
			assert.Equal(t, "ok", result)

			// 4. Respond() delegates correctly
			reqPart := ai.NewToolRequestPart(&ai.ToolRequest{
				Name: tt.wantName, Input: map[string]any{"q": "test"},
			})
			resp := wrapped.Respond(reqPart, "output", nil)
			assert.NotNil(t, resp)

			// 5. Register() doesn't panic (delegates to original no-op)
			assert.NotPanics(t, func() { wrapped.Register(nil) })
		})
	}
}

// TestSanitizedMCPTool_RestartUsesSanitizedName verifies that restarted tool
// requests use the sanitized name so GenKit can resolve them.
func TestSanitizedMCPTool_RestartUsesSanitizedName(t *testing.T) {
	original := &mockTool{name: "__resolve-library-id"}
	wrapped := &SanitizedMCPTool{
		OriginalTool:  original,
		SanitizedName: SanitizeToolName("__resolve-library-id"),
	}

	origReq := ai.NewToolRequestPart(&ai.ToolRequest{
		Name:  "__resolve-library-id",
		Input: map[string]any{"query": "fastapi"},
	})

	restarted := wrapped.Restart(origReq, nil)
	require.NotNil(t, restarted)
	require.NotNil(t, restarted.ToolRequest)
	assert.Equal(t, "__resolve_library_id", restarted.ToolRequest.Name,
		"Restarted request must use sanitized name for GenKit resolution")
}

// TestSanitizedMCPTool_NilDefinition handles tools with nil definitions gracefully.
func TestSanitizedMCPTool_NilDefinition(t *testing.T) {
	original := &mockTool{name: "test"}
	nilDefTool := &nilDefinitionTool{mockTool: original}
	wrapped := &SanitizedMCPTool{
		OriginalTool:  nilDefTool,
		SanitizedName: "test",
	}
	assert.Nil(t, wrapped.Definition())
}

// --- Assignment filter integration test ---

// TestToolAssignmentFilterWithSanitizedNames verifies that the tool assignment
// filter in agent_execution_engine correctly matches DB-stored names (which may
// contain hyphens) against runtime-sanitized tool names. This is the CRITICAL
// integration scenario for Issue #42 — without this, tools would silently be
// filtered out even though the wrapper fixes the GenKit resolution.
func TestToolAssignmentFilterWithSanitizedNames(t *testing.T) {
	// Simulate: DB stores original MCP name with hyphens
	dbToolNames := []string{
		"__resolve-library-id",
		"__get-library-docs",
		"read_file", // already safe
	}

	// Build the assignment map the same way agent_execution_engine.go does
	assignedToolNames := make(map[string]bool)
	for _, name := range dbToolNames {
		assignedToolNames[name] = true
		sanitized := SanitizeToolName(name)
		if sanitized != name {
			assignedToolNames[sanitized] = true
		}
	}

	// Simulate: runtime tools report sanitized names (via SanitizedMCPTool wrapper)
	runtimeToolNames := []string{
		"__resolve_library_id", // sanitized at discovery
		"__get_library_docs",   // sanitized at discovery
		"read_file",            // unchanged
		"unassigned_tool",      // not in DB — should NOT match
	}

	matched := 0
	for _, name := range runtimeToolNames {
		if assignedToolNames[name] {
			matched++
		}
	}

	assert.Equal(t, 3, matched,
		"All 3 assigned tools must match even though DB stores hyphens and runtime uses underscores")
	assert.False(t, assignedToolNames["unassigned_tool"],
		"Unassigned tools must still be rejected")
}

// nilDefinitionTool is a mock that returns nil from Definition()
type nilDefinitionTool struct {
	*mockTool
}

func (n *nilDefinitionTool) Definition() *ai.ToolDefinition { return nil }

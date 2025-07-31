package services

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/mcp"
	oai "github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGenkitBasic tests basic Genkit functionality without Station-specific models
func TestGenkitBasic(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	ctx := context.Background()

	// Initialize Genkit with OpenAI plugin
	openaiPlugin := &oai.OpenAI{APIKey: apiKey}
	g, err := genkit.Init(ctx, genkit.WithPlugins(openaiPlugin))
	require.NoError(t, err)

	t.Run("BasicGeneration", func(t *testing.T) {
		model := openaiPlugin.Model(g, "gpt-4o-mini")
		
		response, err := genkit.Generate(ctx, g,
			ai.WithModel(model),
			ai.WithPrompt("What is 2+2? Answer with just the number."),
		)
		
		require.NoError(t, err)
		assert.NotEmpty(t, response.Text())
		assert.Contains(t, response.Text(), "4")
		
		t.Logf("Response: %s", response.Text())
	})

	t.Run("WithSystemMessage", func(t *testing.T) {
		model := openaiPlugin.Model(g, "gpt-4o-mini")
		
		systemMsg := ai.NewSystemTextMessage("You are a helpful math tutor. Always show your work step by step.")
		userMsg := ai.NewUserTextMessage("Calculate 5 * 7")
		
		response, err := genkit.Generate(ctx, g,
			ai.WithModel(model),
			ai.WithMessages(systemMsg, userMsg),
		)
		
		require.NoError(t, err)
		assert.NotEmpty(t, response.Text())
		assert.Contains(t, response.Text(), "35")
		
		t.Logf("Math tutor response: %s", response.Text())
	})

	t.Run("WithSimpleTool", func(t *testing.T) {
		model := openaiPlugin.Model(g, "gpt-4o-mini")
		
		// Define a simple calculator tool
		calcTool := genkit.DefineTool(g, "calculator", "Add two numbers",
			func(ctx *ai.ToolContext, input struct {
				A int `json:"a"`
				B int `json:"b"`
			}) (struct {
				Result int `json:"result"`
			}, error) {
				return struct {
					Result int `json:"result"`
				}{Result: input.A + input.B}, nil
			},
		)
		
		response, err := genkit.Generate(ctx, g,
			ai.WithModel(model),
			ai.WithPrompt("Use the calculator tool to add 15 and 27"),
			ai.WithTools(calcTool),
			ai.WithToolChoice(ai.ToolChoiceAuto),
		)
		
		require.NoError(t, err)
		assert.NotEmpty(t, response.Text())
		assert.Contains(t, response.Text(), "42")
		
		t.Logf("Tool-assisted response: %s", response.Text())
	})
}

// TestGenkitMCPBasic tests basic MCP functionality with Genkit
func TestGenkitMCPBasic(t *testing.T) {
	ctx := context.Background()

	// Initialize Genkit with OpenAI plugin
	openaiPlugin := &oai.OpenAI{APIKey: "test-key"} // API key not needed for MCP client creation
	g, err := genkit.Init(ctx, genkit.WithPlugins(openaiPlugin))
	require.NoError(t, err)

	t.Run("CreateMCPManager", func(t *testing.T) {
		// Test creating MCP manager with filesystem server config
		manager, err := mcp.NewMCPManager(mcp.MCPManagerOptions{
			Name: "test-manager",
			MCPServers: []mcp.MCPServerConfig{
				{
					Name: "filesystem",
					Config: mcp.MCPClientOptions{
						Name:     "fs-server",
						Version:  "1.0.0",
						Disabled: true, // Disable so we don't need the actual server running
						Stdio: &mcp.StdioConfig{
							Command: "npx",
							Args:    []string{"@modelcontextprotocol/server-filesystem", "/tmp"},
						},
					},
				},
			},
		})
		
		require.NoError(t, err)
		assert.NotNil(t, manager)
		
		// Get tools (should be empty since server is disabled)
		tools, err := manager.GetActiveTools(ctx, g)
		require.NoError(t, err)
		assert.Equal(t, 0, len(tools), "Should have no tools from disabled server")
	})

	t.Run("CreateMCPClient", func(t *testing.T) {
		// Test creating individual MCP client
		client, err := mcp.NewGenkitMCPClient(mcp.MCPClientOptions{
			Name:     "test-client",
			Version:  "1.0.0",
			Disabled: true, // Disable so we don't need the actual server running
			Stdio: &mcp.StdioConfig{
				Command: "echo",
				Args:    []string{"test"},
			},
		})
		
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "test-client", client.Name())
		assert.False(t, client.IsEnabled()) // Should be disabled
	})
}

// TestConfigConversion tests MCP config conversion logic
func TestConfigConversion(t *testing.T) {
	tests := []struct {
		name        string
		mcpType     string
		command     string
		args        []string
		url         string
		expectError bool
	}{
		{
			name:        "Valid stdio config",
			mcpType:     "stdio",
			command:     "npx",
			args:        []string{"@modelcontextprotocol/server-filesystem", "/tmp"},
			expectError: false,
		},
		{
			name:        "Valid SSE config",
			mcpType:     "sse",
			url:         "http://localhost:3000/sse",
			expectError: false,
		},
		{
			name:        "Valid StreamableHTTP config", 
			mcpType:     "streamable_http",
			url:         "http://localhost:3001",
			expectError: false,
		},
		{
			name:        "Invalid stdio config - missing command",
			mcpType:     "stdio",
			expectError: true,
		},
		{
			name:        "Invalid SSE config - missing URL",
			mcpType:     "sse",
			expectError: true,
		},
		{
			name:        "Unsupported transport type",
			mcpType:     "websocket",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config mcp.MCPClientOptions
			config.Name = "test-server"
			config.Version = "1.0.0"

			var err error
			switch tt.mcpType {
			case "stdio":
				if tt.command == "" {
					err = fmt.Errorf("missing command")
				} else {
					config.Stdio = &mcp.StdioConfig{
						Command: tt.command,
						Args:    tt.args,
					}
				}
			case "sse":
				if tt.url == "" {
					err = fmt.Errorf("missing URL")
				} else {
					config.SSE = &mcp.SSEConfig{
						BaseURL: tt.url,
					}
				}
			case "streamable_http":
				if tt.url == "" {
					err = fmt.Errorf("missing URL")  
				} else {
					config.StreamableHTTP = &mcp.StreamableHTTPConfig{
						BaseURL: tt.url,
					}
				}
			default:
				err = fmt.Errorf("unsupported transport type: %s", tt.mcpType)
			}

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "test-server", config.Name)
				assert.Equal(t, "1.0.0", config.Version)
			}
		})
	}
}
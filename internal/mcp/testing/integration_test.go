package testing

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"station/internal/mcp/adapter"
)

func TestMCPProxyAdapter(t *testing.T) {
	ctx := context.Background()

	// Test basic proxy functionality
	t.Run("BasicProxyFunctionality", func(t *testing.T) {
		// Create mock servers
		fsServer := NewMockFileSystemServer()
		dbServer := NewMockDatabaseServer()

		// Create in-process clients for the mock servers
		fsClient, err := client.NewInProcessClient(fsServer.GetServer())
		require.NoError(t, err)
		
		dbClient, err := client.NewInProcessClient(dbServer.GetServer())
		require.NoError(t, err)

		// Start clients
		require.NoError(t, fsClient.Start(ctx))
		require.NoError(t, dbClient.Start(ctx))

		// Initialize clients
		initRequest := mcp.InitializeRequest{}
		initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
		initRequest.Params.ClientInfo = mcp.Implementation{Name: "Test Client", Version: "1.0.0"}
		initRequest.Params.Capabilities = mcp.ClientCapabilities{}

		_, err = fsClient.Initialize(ctx, initRequest)
		require.NoError(t, err)
		
		_, err = dbClient.Initialize(ctx, initRequest)
		require.NoError(t, err)

		// Test tool discovery from mock servers
		fsTools, err := fsClient.ListTools(ctx, mcp.ListToolsRequest{})
		require.NoError(t, err)
		assert.Len(t, fsTools.Tools, 3) // read_file, write_file, list_files

		dbTools, err := dbClient.ListTools(ctx, mcp.ListToolsRequest{})
		require.NoError(t, err)
		assert.Len(t, dbTools.Tools, 2) // query_db, insert_record

		// Test tool calls
		readResult, err := fsClient.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "read_file",
				Arguments: map[string]any{"path": "/test/file.txt"},
			},
		})
		require.NoError(t, err)
		assert.Contains(t, readResult.Content[0].(mcp.TextContent).Text, "Mock file content")

		queryResult, err := dbClient.CallTool(ctx, mcp.CallToolRequest{
			Params: mcp.CallToolParams{
				Name:      "query_db", 
				Arguments: map[string]any{"sql": "SELECT * FROM users"},
			},
		})
		require.NoError(t, err)
		assert.Contains(t, queryResult.Content[0].(mcp.TextContent).Text, "Mock DB Result")

		// Cleanup
		fsClient.Close()
		dbClient.Close()
	})

	// Test proxy server tool aggregation 
	t.Run("ProxyServerToolAggregation", func(t *testing.T) {
		// Create proxy server for agent with selected tools
		agentID := int64(123)
		selectedTools := []string{"read_file", "query_db"}
		config := adapter.ProxyServerConfig{
			Name:        "Test Proxy",
			Version:     "1.0.0",
			Description: "Test proxy for MCP adapter",
		}

		proxy := adapter.NewMCPProxyServer(agentID, selectedTools, "test", config)

		// Verify proxy was created
		stats := proxy.GetProxyStats()
		assert.Equal(t, agentID, stats["agent_id"])
		assert.Equal(t, 0, stats["connected_servers"]) // No servers connected yet
		assert.Equal(t, 0, stats["total_tools"])       // No tools registered yet

		// Clean up
		proxy.Close()
	})

	// Test tool registry functionality
	t.Run("ToolRegistryFunctionality", func(t *testing.T) {
		registry := adapter.NewToolRegistry()

		// Create test tools
		testTool1 := mcp.NewTool("test_tool_1", mcp.WithDescription("Test tool 1"))
		testTool2 := mcp.NewTool("test_tool_2", mcp.WithDescription("Test tool 2"))

		// Register tools
		registry.RegisterTool("server1", testTool1)
		registry.RegisterTool("server2", testTool2)

		// Verify registration
		assert.True(t, registry.HasTool("test_tool_1"))
		assert.True(t, registry.HasTool("test_tool_2"))
		assert.False(t, registry.HasTool("nonexistent_tool"))

		// Test tool mapping retrieval
		mapping1, err := registry.GetToolMapping("test_tool_1")
		require.NoError(t, err)
		assert.Equal(t, "server1", mapping1.ServerID)
		assert.Equal(t, "test_tool_1", mapping1.ToolName)

		// Test getting all tools
		allTools := registry.GetAllTools()
		assert.Len(t, allTools, 2)

		// Test removing tools from server
		registry.RemoveToolsFromServer("server1")
		assert.False(t, registry.HasTool("test_tool_1"))
		assert.True(t, registry.HasTool("test_tool_2"))
	})

	// Test session manager functionality
	t.Run("SessionManagerFunctionality", func(t *testing.T) {
		sessionMgr := adapter.NewSessionManager()

		// Create agent session
		agentID := int64(456)
		selectedTools := []string{"tool1", "tool2", "tool3"}
		session := sessionMgr.CreateAgentSession(agentID, selectedTools, "production")

		// Verify session creation
		assert.Equal(t, agentID, session.AgentID)
		assert.Equal(t, selectedTools, session.SelectedTools)
		assert.Equal(t, "production", session.Environment)

		// Test session retrieval
		retrievedSession, err := sessionMgr.GetAgentSession(agentID)
		require.NoError(t, err)
		assert.Equal(t, session.AgentID, retrievedSession.AgentID)

		// Test tool filtering
		allTools := []mcp.Tool{
			mcp.NewTool("tool1", mcp.WithDescription("Tool 1")),
			mcp.NewTool("tool2", mcp.WithDescription("Tool 2")),
			mcp.NewTool("tool3", mcp.WithDescription("Tool 3")),
			mcp.NewTool("tool4", mcp.WithDescription("Tool 4")), // This should be filtered out
		}

		filteredTools, err := sessionMgr.FilterToolsForAgent(agentID, allTools)
		require.NoError(t, err)
		assert.Len(t, filteredTools, 3) // Only tools 1, 2, 3 should be included

		// Test tool authorization
		assert.True(t, sessionMgr.IsToolAllowedForAgent(agentID, "tool1"))
		assert.True(t, sessionMgr.IsToolAllowedForAgent(agentID, "tool2"))
		assert.False(t, sessionMgr.IsToolAllowedForAgent(agentID, "tool4"))

		// Test session update
		newTools := []string{"tool1", "tool4"}
		err = sessionMgr.UpdateAgentTools(agentID, newTools)
		require.NoError(t, err)

		// Verify update
		assert.True(t, sessionMgr.IsToolAllowedForAgent(agentID, "tool1"))
		assert.False(t, sessionMgr.IsToolAllowedForAgent(agentID, "tool2"))
		assert.True(t, sessionMgr.IsToolAllowedForAgent(agentID, "tool4"))

		// Test session removal
		sessionMgr.RemoveAgentSession(agentID)
		assert.False(t, sessionMgr.HasAgentSession(agentID))
	})

	// Test client manager functionality
	t.Run("ClientManagerFunctionality", func(t *testing.T) {
		clientMgr := adapter.NewClientManager()

		// Create server config
		serverConfig := adapter.MCPServerConfig{
			ID:      "test_server",
			Name:    "Test Server",
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"hello"},
			Timeout: 30,
		}

		// Add server
		err := clientMgr.AddServer(serverConfig)
		require.NoError(t, err)

		// Verify server config
		retrievedConfig, err := clientMgr.GetServerConfig("test_server")
		require.NoError(t, err)
		assert.Equal(t, serverConfig.ID, retrievedConfig.ID)
		assert.Equal(t, serverConfig.Name, retrievedConfig.Name)

		// Test duplicate server addition
		err = clientMgr.AddServer(serverConfig)
		assert.Error(t, err) // Should fail due to duplicate ID

		// Test getting non-existent client
		_, err = clientMgr.GetClient("nonexistent")
		assert.Error(t, err)

		// Note: We don't test actual connection here as it would require
		// a real MCP server process. In a full test suite, we'd use 
		// mock stdio servers or in-process servers.
	})
}

// TestMCPProxyIntegration tests the full integration with real mock servers
func TestMCPProxyIntegration(t *testing.T) {
	t.Run("FullIntegrationTest", func(t *testing.T) {
		// This test would require implementing in-process server connections
		// in the ClientManager to work with our mock servers.
		// For now, we'll test the components in isolation.

		// Create mock servers
		fsServer := NewMockFileSystemServer()
		dbServer := NewMockDatabaseServer()

		// Verify mock servers have tools
		fsTools := fsServer.GetTools()
		dbTools := dbServer.GetTools()
		assert.Len(t, fsTools, 3)
		assert.Len(t, dbTools, 2)

		// Create proxy server
		agentID := int64(789)
		selectedTools := []string{"read_file", "query_db"}
		config := adapter.ProxyServerConfig{
			Name:        "Integration Test Proxy",
			Version:     "1.0.0",
			Description: "Full integration test proxy",
		}

		proxy := adapter.NewMCPProxyServer(agentID, selectedTools, "test", config)

		// Verify proxy statistics
		stats := proxy.GetProxyStats()
		assert.Equal(t, agentID, stats["agent_id"])

		// Clean up
		proxy.Close()
	})
}
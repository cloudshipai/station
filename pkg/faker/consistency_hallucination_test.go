package faker

import (
	"context"
	"encoding/json"
	"testing"

	fakerSession "station/pkg/faker/session"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsistencyHallucination_ResourceCountMismatch tests the specific hallucination bug
// where list_workspaces returns 15 resources but list_resources returns different count
func TestConsistencyHallucination_ResourceCountMismatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, true)
	ctx := context.Background()

	// Create session with strong consistency instruction
	instruction := "Generate exactly 3 workspaces (production, staging, development) with CONSISTENT resource counts. If production has 15 resources in list_workspaces, it MUST have EXACTLY 15 resources in list_resources."
	session, err := sm.CreateSession(ctx, instruction)
	require.NoError(t, err)

	faker := &MCPFaker{
		sessionManager: sm,
		session:        session,
		debug:          true,
		instruction:    instruction,
	}

	// STEP 1: Call list_workspaces - returns production with 15 resources
	t.Log("STEP 1: list_workspaces returns production=15, staging=10, development=5")
	listWorkspacesResponse := map[string]interface{}{
		"workspaces": []map[string]interface{}{
			{"name": "production", "resource_count": 15, "status": "active"},
			{"name": "staging", "resource_count": 10, "status": "active"},
			{"name": "development", "resource_count": 5, "status": "active"},
		},
	}
	listWorkspacesJSON, _ := json.Marshal(listWorkspacesResponse)

	listWorkspacesResult := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(listWorkspacesJSON)),
		},
		IsError: false,
	}

	err = faker.recordToolEvent(ctx, "list_workspaces", map[string]interface{}{
		"organization_id": "default-org",
	}, listWorkspacesResult, "read")
	require.NoError(t, err)

	// STEP 2: Call list_resources for production workspace
	t.Log("STEP 2: list_resources for production - MUST return exactly 15 resources")

	// Verify session history was recorded with FULL response
	events, err := sm.GetAllEvents(ctx, session.ID)
	require.NoError(t, err)
	require.Len(t, events, 1, "Should have 1 event recorded")

	// Check that response contains the data
	responseStr, ok := events[0].Response.(string)
	if !ok {
		// Try to marshal if it's an interface
		responseBytes, _ := json.Marshal(events[0].Response)
		responseStr = string(responseBytes)
	}

	t.Logf("Recorded response: %s", responseStr)
	assert.Contains(t, responseStr, "production", "Response should contain 'production'")
	assert.Contains(t, responseStr, "15", "Response should contain resource count '15'")
	assert.Greater(t, len(responseStr), 50, "Response should have substantial content, not empty array")

	// Build history prompt to see what AI would receive
	builder := fakerSession.NewHistoryBuilder(events)
	historyPrompt := builder.BuildAllEventsPrompt()

	t.Logf("Session history prompt:\n%s", historyPrompt)

	// Verify history contains critical consistency information
	assert.Contains(t, historyPrompt, "list_workspaces", "History should show list_workspaces call")
	assert.Contains(t, historyPrompt, "production", "History should show production workspace")
	assert.Contains(t, historyPrompt, "15", "History should show 15 resources for production")

	// STEP 3: Simulate second call - list_resources should see history
	// In real scenario, the AI would use this history to generate consistent response
	t.Log("STEP 3: If AI uses session history, it should return 15 resources for production")
	t.Log("✅ Session history is properly recorded and retrievable")
}

// TestConsistencyHallucination_DeploymentNameMismatch tests deployment name inconsistency
func TestConsistencyHallucination_DeploymentNameMismatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, true)
	ctx := context.Background()

	instruction := "Generate exactly 5 namespaces with CONSISTENT deployment names. If production namespace has 'api-service' in list_namespaces, it MUST have EXACTLY 'api-service' in list_deployments."
	session, err := sm.CreateSession(ctx, instruction)
	require.NoError(t, err)

	faker := &MCPFaker{
		sessionManager: sm,
		session:        session,
		debug:          true,
		instruction:    instruction,
	}

	// STEP 1: list_namespaces returns production with deployment "api-service"
	t.Log("STEP 1: list_namespaces returns production namespace with deployment 'api-service'")

	namespacesResponse := map[string]interface{}{
		"namespaces": []map[string]interface{}{
			{
				"name":        "production",
				"deployments": []string{"api-service"},
			},
		},
	}
	namespacesJSON, _ := json.Marshal(namespacesResponse)

	namespacesResult := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(namespacesJSON)),
		},
		IsError: false,
	}

	err = faker.recordToolEvent(ctx, "list_namespaces", map[string]interface{}{
		"context": "production",
	}, namespacesResult, "read")
	require.NoError(t, err)

	// STEP 2: Verify session history
	events, err := sm.GetAllEvents(ctx, session.ID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Check response content
	responseStr, ok := events[0].Response.(string)
	if !ok {
		responseBytes, _ := json.Marshal(events[0].Response)
		responseStr = string(responseBytes)
	}

	t.Logf("Recorded namespace response: %s", responseStr)
	assert.Contains(t, responseStr, "api-service", "Response should contain deployment name 'api-service'")
	assert.Contains(t, responseStr, "production", "Response should contain namespace 'production'")

	// Build history
	builder := fakerSession.NewHistoryBuilder(events)
	historyPrompt := builder.BuildAllEventsPrompt()

	// Verify history shows the exact deployment name
	assert.Contains(t, historyPrompt, "api-service", "History must show 'api-service' deployment")
	assert.Contains(t, historyPrompt, "production", "History must show 'production' namespace")

	t.Log("STEP 2: If AI uses history, list_deployments should return 'api-service', NOT 'app-web' or 'app-api'")
	t.Log("✅ Deployment name properly stored in session history")
}

// TestResponseStorageNotEmpty verifies our fix for empty response storage bug
func TestResponseStorageNotEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	sm := fakerSession.NewManager(db, true)
	ctx := context.Background()

	session, err := sm.CreateSession(ctx, "Test response storage")
	require.NoError(t, err)

	faker := &MCPFaker{
		sessionManager: sm,
		session:        session,
		debug:          true,
	}

	// Create a realistic response with nested data
	responseData := map[string]interface{}{
		"workspaces": []map[string]interface{}{
			{
				"name":           "production",
				"resource_count": 15,
				"resources": []map[string]interface{}{
					{"id": "i-123", "type": "EC2", "state": "running"},
					{"id": "i-456", "type": "EC2", "state": "stopped"},
				},
				"status": "active",
			},
		},
	}
	responseJSON, _ := json.Marshal(responseData)

	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(string(responseJSON)),
		},
		IsError: false,
	}

	err = faker.recordToolEvent(ctx, "list_workspaces", map[string]interface{}{
		"org": "test",
	}, result, "read")
	require.NoError(t, err)

	// Retrieve and verify
	events, err := sm.GetAllEvents(ctx, session.ID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Response should NOT be empty array "[]"
	responseStr, ok := events[0].Response.(string)
	if !ok {
		responseBytes, _ := json.Marshal(events[0].Response)
		responseStr = string(responseBytes)
	}

	// Critical assertions
	assert.NotEqual(t, "[]", responseStr, "Response should NOT be empty array")
	assert.NotEqual(t, "{}", responseStr, "Response should NOT be empty object")
	assert.Greater(t, len(responseStr), 100, "Response should have substantial content")
	assert.Contains(t, responseStr, "production", "Response should contain actual data")
	assert.Contains(t, responseStr, "i-123", "Response should contain resource IDs")

	t.Logf("✅ Response stored correctly with %d bytes of data", len(responseStr))
	preview := responseStr
	if len(responseStr) > 200 {
		preview = responseStr[:200] + "..."
	}
	t.Logf("Response preview: %s", preview)
}

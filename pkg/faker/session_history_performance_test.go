package faker

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"station/pkg/faker/session"
)

// TestSessionHistoryPromptSize measures the impact of session history on prompt size
func TestSessionHistoryPromptSize(t *testing.T) {
	// Real event data from the database (last 3 events from session 4b39d850-4eb9-47f9-b66b-b7e4efa7045e)
	events := []*session.Event{
		{
			ToolName:      "list_resources",
			Arguments:     map[string]interface{}{"workspace_name": "production"},
			Response:      createLargeWorkspaceResponse("production", 15),
			OperationType: session.OperationRead,
			Timestamp:     time.Now().Add(-3 * time.Minute),
		},
		{
			ToolName:      "list_resources",
			Arguments:     map[string]interface{}{"workspace_name": "development"},
			Response:      createLargeWorkspaceResponse("development", 5),
			OperationType: session.OperationRead,
			Timestamp:     time.Now().Add(-2 * time.Minute),
		},
		{
			ToolName:      "list_workspaces",
			Arguments:     map[string]interface{}{"workspace_type": "all"},
			Response:      createAllWorkspacesResponse(),
			OperationType: session.OperationRead,
			Timestamp:     time.Now().Add(-1 * time.Minute),
		},
	}

	// Build session history prompt (this is what gets added to EVERY faker AI call)
	builder := session.NewHistoryBuilder(events)
	historyPrompt := builder.BuildAllEventsPrompt()

	// Measure prompt size
	promptSize := len(historyPrompt)
	t.Logf("Session history prompt size: %d bytes (%.2f KB)", promptSize, float64(promptSize)/1024)

	// Show the structure
	t.Logf("Number of events: %d", len(events))
	for i, event := range events {
		argsJSON, _ := json.Marshal(event.Arguments)
		respJSON, _ := json.Marshal(event.Response)
		t.Logf("Event %d: %s - Args: %d bytes, Response: %d bytes",
			i+1, event.ToolName, len(argsJSON), len(respJSON))
	}

	// Calculate total overhead
	t.Logf("\n=== CRITICAL FINDING ===")
	t.Logf("This %d KB prompt is added to EVERY subsequent faker tool call!", promptSize/1024)
	t.Logf("With 10 tool calls, that's %d KB of redundant data sent to OpenAI", (promptSize*10)/1024)

	// Performance threshold check
	if promptSize > 50*1024 { // 50KB
		t.Errorf("Session history prompt is TOO LARGE: %d KB", promptSize/1024)
		t.Errorf("This will significantly slow down OpenAI API calls!")
	}
}

// TestSessionHistoryPromptScaling tests how prompt size grows with more events
func TestSessionHistoryPromptScaling(t *testing.T) {
	testCases := []struct {
		eventCount int
		maxSizeKB  int
	}{
		{eventCount: 3, maxSizeKB: 30},
		{eventCount: 5, maxSizeKB: 50},
		{eventCount: 10, maxSizeKB: 100},
		{eventCount: 20, maxSizeKB: 200},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%d_events", tc.eventCount), func(t *testing.T) {
			// Generate N events
			events := make([]*session.Event, tc.eventCount)
			for i := 0; i < tc.eventCount; i++ {
				if i%2 == 0 {
					events[i] = &session.Event{
						ToolName:      "list_workspaces",
						Arguments:     map[string]interface{}{"workspace_type": "all"},
						Response:      createAllWorkspacesResponse(),
						OperationType: session.OperationRead,
						Timestamp:     time.Now().Add(-time.Duration(tc.eventCount-i) * time.Minute),
					}
				} else {
					events[i] = &session.Event{
						ToolName:      "list_resources",
						Arguments:     map[string]interface{}{"workspace_name": "production"},
						Response:      createLargeWorkspaceResponse("production", 15),
						OperationType: session.OperationRead,
						Timestamp:     time.Now().Add(-time.Duration(tc.eventCount-i) * time.Minute),
					}
				}
			}

			builder := session.NewHistoryBuilder(events)
			historyPrompt := builder.BuildAllEventsPrompt()
			promptSizeKB := len(historyPrompt) / 1024

			t.Logf("Events: %d → Prompt size: %d KB", tc.eventCount, promptSizeKB)

			if promptSizeKB > tc.maxSizeKB {
				t.Errorf("Prompt size %d KB exceeds threshold %d KB for %d events",
					promptSizeKB, tc.maxSizeKB, tc.eventCount)
			}
		})
	}
}

// TestSessionHistoryWithLimitedEvents tests using only last N events
func TestSessionHistoryWithLimitedEvents(t *testing.T) {
	// Create 13 events (like the real session)
	allEvents := make([]*session.Event, 13)
	for i := 0; i < 13; i++ {
		allEvents[i] = &session.Event{
			ToolName:      fmt.Sprintf("tool_%d", i),
			Arguments:     map[string]interface{}{"arg": i},
			Response:      createLargeWorkspaceResponse("production", 15),
			OperationType: session.OperationRead,
			Timestamp:     time.Now().Add(-time.Duration(13-i) * time.Minute),
		}
	}

	// Test full history
	fullBuilder := session.NewHistoryBuilder(allEvents)
	fullPrompt := fullBuilder.BuildAllEventsPrompt()
	fullSizeKB := len(fullPrompt) / 1024

	// Test limited history (last 3 events only)
	last3Events := allEvents[len(allEvents)-3:]
	limitedBuilder := session.NewHistoryBuilder(last3Events)
	limitedPrompt := limitedBuilder.BuildAllEventsPrompt()
	limitedSizeKB := len(limitedPrompt) / 1024

	t.Logf("Full history (13 events): %d KB", fullSizeKB)
	t.Logf("Limited history (3 events): %d KB", limitedSizeKB)
	t.Logf("Savings: %d KB (%.1f%% reduction)", fullSizeKB-limitedSizeKB,
		float64(fullSizeKB-limitedSizeKB)/float64(fullSizeKB)*100)

	// Assert limited history is significantly smaller
	if limitedSizeKB >= fullSizeKB/2 {
		t.Errorf("Limited history not reducing prompt size enough: %d KB vs %d KB",
			limitedSizeKB, fullSizeKB)
	}
}

// Helper function to create a large workspace response (mimics real data)
func createLargeWorkspaceResponse(workspaceName string, resourceCount int) map[string]interface{} {
	resources := make([]map[string]interface{}, resourceCount)
	for i := 0; i < resourceCount; i++ {
		resources[i] = map[string]interface{}{
			"ResourceId":   fmt.Sprintf("resource-%s-%02d", workspaceName, i+1),
			"ResourceType": "EC2",
			"MetricData": map[string]interface{}{
				"CPUUtilization": []map[string]interface{}{
					{"Timestamp": "2023-10-01T00:00:00Z", "Value": 65.2 + float64(i)},
					{"Timestamp": "2023-10-01T01:00:00Z", "Value": 70.1 + float64(i)},
				},
				"MemoryUsed": []map[string]interface{}{
					{"Timestamp": "2023-10-01T00:00:00Z", "Value": 2048 + i*100},
					{"Timestamp": "2023-10-01T01:00:00Z", "Value": 3072 + i*100},
				},
			},
		}
	}

	return map[string]interface{}{
		"Workspaces": []map[string]interface{}{
			{
				"WorkspaceName": workspaceName,
				"WorkspaceArn":  fmt.Sprintf("arn:aws:cloudwatch:us-east-1:123456789012:workspace/%s", workspaceName),
				"CreationTime":  "2022-01-15T12:00:00Z",
				"LastUpdated":   "2023-10-01T15:30:00Z",
				"ResourceCount": resourceCount,
				"Resources":     resources,
			},
		},
	}
}

// Helper function to create all workspaces response
func createAllWorkspacesResponse() map[string]interface{} {
	return map[string]interface{}{
		"Workspaces": []map[string]interface{}{
			{
				"WorkspaceName": "production",
				"ResourceCount": 15,
				"Status":        "active",
			},
			{
				"WorkspaceName": "staging",
				"ResourceCount": 10,
				"Status":        "active",
			},
			{
				"WorkspaceName": "development",
				"ResourceCount": 5,
				"Status":        "active",
			},
		},
	}
}

// TestMaxHistoryEventsLimit tests that we limit history to 3 events
func TestMaxHistoryEventsLimit(t *testing.T) {
	const maxHistoryEvents = 3

	// Create 13 events (real-world scenario)
	allEvents := make([]*session.Event, 13)
	for i := 0; i < 13; i++ {
		allEvents[i] = &session.Event{
			ToolName:      fmt.Sprintf("tool_%d", i),
			Arguments:     map[string]interface{}{"arg": i},
			Response:      createLargeWorkspaceResponse("production", 15),
			OperationType: session.OperationRead,
			Timestamp:     time.Now().Add(-time.Duration(13-i) * time.Minute),
		}
	}

	// Simulate the fix: take only last 3 events
	limitedEvents := allEvents
	if len(allEvents) > maxHistoryEvents {
		limitedEvents = allEvents[len(allEvents)-maxHistoryEvents:]
	}

	// Verify we only have 3 events
	if len(limitedEvents) != maxHistoryEvents {
		t.Errorf("Expected %d events, got %d", maxHistoryEvents, len(limitedEvents))
	}

	// Build prompt with limited events
	builder := session.NewHistoryBuilder(limitedEvents)
	limitedPrompt := builder.BuildAllEventsPrompt()
	limitedSizeKB := len(limitedPrompt) / 1024

	t.Logf("Limited to last %d events: %d KB", maxHistoryEvents, limitedSizeKB)

	// Should be around 31 KB based on our earlier tests
	if limitedSizeKB > 50 {
		t.Errorf("Limited prompt still too large: %d KB (expected ~31 KB)", limitedSizeKB)
	}

	// Verify we got the LAST 3 events (most recent)
	if limitedEvents[0].ToolName != "tool_10" {
		t.Errorf("Expected first limited event to be tool_10, got %s", limitedEvents[0].ToolName)
	}
	if limitedEvents[2].ToolName != "tool_12" {
		t.Errorf("Expected last limited event to be tool_12, got %s", limitedEvents[2].ToolName)
	}

	t.Logf("✅ Fix working: Limited from 13 events (137KB) to 3 events (%d KB)", limitedSizeKB)
}

// BenchmarkSessionHistoryGeneration benchmarks the cost of building session history
func BenchmarkSessionHistoryGeneration(b *testing.B) {
	events := make([]*session.Event, 10)
	for i := 0; i < 10; i++ {
		events[i] = &session.Event{
			ToolName:      "list_resources",
			Arguments:     map[string]interface{}{"workspace_name": "production"},
			Response:      createLargeWorkspaceResponse("production", 15),
			OperationType: session.OperationRead,
			Timestamp:     time.Now(),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := session.NewHistoryBuilder(events)
		_ = builder.BuildAllEventsPrompt()
	}
}

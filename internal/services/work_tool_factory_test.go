package services

import (
	"context"
	"testing"
	"time"

	"station/internal/lattice/work"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLatticeRegistry struct {
	stations []LatticeStationInfo
}

func (m *mockLatticeRegistry) ListStationsInfo(ctx context.Context) ([]LatticeStationInfo, error) {
	return m.stations, nil
}

type mockWorkDispatcher struct {
	assignedWork  []*work.WorkAssignment
	workResponses map[string]*work.WorkResponse
	workStatuses  map[string]*work.WorkStatus
}

func newMockWorkDispatcher() *mockWorkDispatcher {
	return &mockWorkDispatcher{
		workResponses: make(map[string]*work.WorkResponse),
		workStatuses:  make(map[string]*work.WorkStatus),
	}
}

func TestWorkToolFactory_NewWorkToolFactory(t *testing.T) {
	t.Run("ReturnsNilWhenDispatcherNil", func(t *testing.T) {
		factory := NewWorkToolFactory(nil, nil, "station-1")
		assert.Nil(t, factory)
	})

	t.Run("ReturnsFactoryWhenDispatcherProvided", func(t *testing.T) {
		dispatcher := &work.Dispatcher{}
		factory := NewWorkToolFactory(dispatcher, nil, "station-1")
		assert.NotNil(t, factory)
	})

	t.Run("AcceptsOptionalRegistry", func(t *testing.T) {
		dispatcher := &work.Dispatcher{}
		registry := &mockLatticeRegistry{}
		factory := NewWorkToolFactory(dispatcher, registry, "station-1")
		assert.NotNil(t, factory)
	})
}

func TestWorkToolFactory_IsEnabled(t *testing.T) {
	t.Run("FalseWhenNil", func(t *testing.T) {
		var factory *WorkToolFactory
		assert.False(t, factory.IsEnabled())
	})

	t.Run("FalseWhenDispatcherNil", func(t *testing.T) {
		factory := &WorkToolFactory{dispatcher: nil}
		assert.False(t, factory.IsEnabled())
	})

	t.Run("TrueWhenDispatcherSet", func(t *testing.T) {
		dispatcher := &work.Dispatcher{}
		factory := NewWorkToolFactory(dispatcher, nil, "station-1")
		assert.True(t, factory.IsEnabled())
	})
}

func TestWorkToolFactory_ShouldAddTools(t *testing.T) {
	dispatcher := &work.Dispatcher{}
	factory := NewWorkToolFactory(dispatcher, nil, "station-1")

	tests := []struct {
		name           string
		latticeEnabled bool
		expectAddTools bool
	}{
		{
			name:           "AddToolsWhenLatticeEnabled",
			latticeEnabled: true,
			expectAddTools: true,
		},
		{
			name:           "NoToolsWhenLatticeDisabled",
			latticeEnabled: false,
			expectAddTools: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldAdd := factory.ShouldAddTools(tt.latticeEnabled)
			assert.Equal(t, tt.expectAddTools, shouldAdd)
		})
	}
}

func TestWorkToolFactory_GetWorkTools(t *testing.T) {
	dispatcher := &work.Dispatcher{}
	factory := NewWorkToolFactory(dispatcher, nil, "station-1")

	t.Run("ReturnsNilWhenNotEnabled", func(t *testing.T) {
		var nilFactory *WorkToolFactory
		tools := nilFactory.GetWorkTools()
		assert.Nil(t, tools)
	})

	t.Run("ReturnsFourTools", func(t *testing.T) {
		tools := factory.GetWorkTools()
		require.Len(t, tools, 4)

		toolNames := make(map[string]bool)
		for _, tool := range tools {
			toolNames[tool.Name()] = true
		}

		assert.True(t, toolNames["assign_work"], "Should have assign_work tool")
		assert.True(t, toolNames["await_work"], "Should have await_work tool")
		assert.True(t, toolNames["check_work"], "Should have check_work tool")
		assert.True(t, toolNames["list_agents"], "Should have list_agents tool")
	})
}

func TestWorkToolFactory_ToolNames(t *testing.T) {
	dispatcher := &work.Dispatcher{}
	factory := NewWorkToolFactory(dispatcher, nil, "station-1")

	tools := factory.GetWorkTools()
	require.Len(t, tools, 4)

	expectedNames := []string{"assign_work", "await_work", "check_work", "list_agents"}
	for i, tool := range tools {
		assert.Equal(t, expectedNames[i], tool.Name())
	}
}

func TestWorkToolFactory_ParseAssignWorkRequest(t *testing.T) {
	dispatcher := &work.Dispatcher{}
	factory := NewWorkToolFactory(dispatcher, nil, "station-1")

	t.Run("ParsesBasicFields", func(t *testing.T) {
		input := map[string]any{
			"agent_name":     "test-agent",
			"task":           "do something",
			"target_station": "station-2",
		}

		req := factory.parseAssignWorkRequest(input)

		assert.Equal(t, "test-agent", req.AgentName)
		assert.Equal(t, "do something", req.Task)
		assert.Equal(t, "station-2", req.TargetStation)
		assert.Equal(t, 300, req.TimeoutSecs)
	})

	t.Run("ParsesAgentID", func(t *testing.T) {
		input := map[string]any{
			"agent_id": "agent-123",
			"task":     "do something",
		}

		req := factory.parseAssignWorkRequest(input)

		assert.Equal(t, "agent-123", req.AgentID)
	})

	t.Run("ParsesTimeout", func(t *testing.T) {
		input := map[string]any{
			"agent_name":   "test-agent",
			"task":         "do something",
			"timeout_secs": float64(600),
		}

		req := factory.parseAssignWorkRequest(input)

		assert.Equal(t, 600, req.TimeoutSecs)
	})

	t.Run("ParsesContext", func(t *testing.T) {
		input := map[string]any{
			"agent_name": "test-agent",
			"task":       "do something",
			"context": map[string]any{
				"key1": "value1",
				"key2": "value2",
			},
		}

		req := factory.parseAssignWorkRequest(input)

		require.NotNil(t, req.Context)
		assert.Equal(t, "value1", req.Context["key1"])
		assert.Equal(t, "value2", req.Context["key2"])
	})
}

func TestWorkToolFactory_DiscoverStationForAgent(t *testing.T) {
	dispatcher := &work.Dispatcher{}

	t.Run("ReturnsEmptyWhenNoRegistry", func(t *testing.T) {
		factory := NewWorkToolFactory(dispatcher, nil, "station-1")
		stationID := factory.discoverStationForAgent(context.Background(), "test-agent")
		assert.Empty(t, stationID)
	})

	t.Run("FindsStationWithAgent", func(t *testing.T) {
		registry := &mockLatticeRegistry{
			stations: []LatticeStationInfo{
				{
					StationID:   "station-1",
					StationName: "Station One",
					Agents: []LatticeAgentInfo{
						{Name: "agent-a"},
					},
				},
				{
					StationID:   "station-2",
					StationName: "Station Two",
					Agents: []LatticeAgentInfo{
						{Name: "agent-b"},
						{Name: "test-agent"},
					},
				},
			},
		}

		factory := NewWorkToolFactory(dispatcher, registry, "station-1")
		stationID := factory.discoverStationForAgent(context.Background(), "test-agent")
		assert.Equal(t, "station-2", stationID)
	})

	t.Run("ReturnsEmptyWhenAgentNotFound", func(t *testing.T) {
		registry := &mockLatticeRegistry{
			stations: []LatticeStationInfo{
				{
					StationID: "station-1",
					Agents: []LatticeAgentInfo{
						{Name: "agent-a"},
					},
				},
			},
		}

		factory := NewWorkToolFactory(dispatcher, registry, "station-1")
		stationID := factory.discoverStationForAgent(context.Background(), "non-existent")
		assert.Empty(t, stationID)
	})
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		expect bool
	}{
		{"HelloWorld", "world", true},
		{"HelloWorld", "WORLD", true},
		{"HelloWorld", "hello", true},
		{"HelloWorld", "lowor", true},
		{"HelloWorld", "xyz", false},
		{"", "", true},
		{"Hello", "", true},
		{"", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := containsIgnoreCase(tt.s, tt.substr)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestToLower(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"HELLO", "hello"},
		{"Hello", "hello"},
		{"hello", "hello"},
		{"HeLLo WoRLD", "hello world"},
		{"123ABC", "123abc"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toLower(tt.input)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestAssignWorkRequest_Defaults(t *testing.T) {
	dispatcher := &work.Dispatcher{}
	factory := NewWorkToolFactory(dispatcher, nil, "station-1")

	input := map[string]any{
		"agent_name": "test-agent",
		"task":       "do something",
	}

	req := factory.parseAssignWorkRequest(input)

	assert.Equal(t, 300, req.TimeoutSecs, "Default timeout should be 300 seconds")
	assert.Empty(t, req.TargetStation, "Default target station should be empty")
	assert.Nil(t, req.Context, "Default context should be nil")
}

func TestWorkToolFactory_ResponseTypes(t *testing.T) {
	t.Run("AssignWorkResponse", func(t *testing.T) {
		resp := AssignWorkResponse{
			Success: true,
			WorkID:  "work-123",
		}
		assert.True(t, resp.Success)
		assert.Equal(t, "work-123", resp.WorkID)
	})

	t.Run("AwaitWorkResponse", func(t *testing.T) {
		resp := AwaitWorkResponse{
			Success:    true,
			Status:     work.MsgWorkComplete,
			Result:     "task completed",
			DurationMs: 1500.0,
			StationID:  "station-1",
		}
		assert.True(t, resp.Success)
		assert.Equal(t, work.MsgWorkComplete, resp.Status)
	})

	t.Run("CheckWorkResponse", func(t *testing.T) {
		resp := CheckWorkResponse{
			Success:    true,
			Status:     "PENDING",
			IsComplete: false,
		}
		assert.True(t, resp.Success)
		assert.False(t, resp.IsComplete)
	})

	t.Run("ListAgentsResponse", func(t *testing.T) {
		resp := ListAgentsResponse{
			Success: true,
			Agents: []WorkToolAgentInfo{
				{Name: "agent-1", StationID: "station-1"},
				{Name: "agent-2", StationID: "station-2"},
			},
		}
		assert.True(t, resp.Success)
		assert.Len(t, resp.Agents, 2)
	})
}

func TestWorkToolAgentInfo(t *testing.T) {
	info := WorkToolAgentInfo{
		Name:        "test-agent",
		Description: "A test agent",
		StationID:   "station-1",
		StationName: "Station One",
		Tags:        []string{"production", "sre"},
	}

	assert.Equal(t, "test-agent", info.Name)
	assert.Equal(t, "A test agent", info.Description)
	assert.Equal(t, "station-1", info.StationID)
	assert.Equal(t, "Station One", info.StationName)
	assert.Contains(t, info.Tags, "production")
}

func TestLatticeAgentInfo(t *testing.T) {
	info := LatticeAgentInfo{
		ID:           "agent-123",
		Name:         "test-agent",
		Description:  "A test agent",
		Capabilities: []string{"kubernetes", "aws"},
	}

	assert.Equal(t, "agent-123", info.ID)
	assert.Equal(t, "test-agent", info.Name)
	assert.Contains(t, info.Capabilities, "kubernetes")
}

func TestLatticeStationInfo(t *testing.T) {
	info := LatticeStationInfo{
		StationID:   "station-1",
		StationName: "Station One",
		Agents: []LatticeAgentInfo{
			{Name: "agent-1"},
			{Name: "agent-2"},
		},
		Tags: []string{"production"},
	}

	assert.Equal(t, "station-1", info.StationID)
	assert.Len(t, info.Agents, 2)
	assert.Contains(t, info.Tags, "production")
}

func TestWorkToolFactory_TimeoutConversion(t *testing.T) {
	dispatcher := &work.Dispatcher{}
	factory := NewWorkToolFactory(dispatcher, nil, "station-1")

	input := map[string]any{
		"agent_name":   "test-agent",
		"task":         "long task",
		"timeout_secs": float64(3600),
	}

	req := factory.parseAssignWorkRequest(input)

	timeout := time.Duration(req.TimeoutSecs) * time.Second
	assert.Equal(t, time.Hour, timeout)
}

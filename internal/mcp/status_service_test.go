package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusService_GetEnvironmentStatuses(t *testing.T) {
	testDB, repos := setupTestDB(t)
	defer testDB.Close()

	statusService := NewStatusService(repos)

	// Create test environment
	env, err := repos.Environments.Create("test", nil, 1)
	require.NoError(t, err)

	// Create test agent
	_, err = repos.Agents.Create("Test Agent", "Test Description", "Test prompt", 5, env.ID, 1, nil, nil, false)
	require.NoError(t, err)

	// Get environment statuses
	statuses, err := statusService.GetEnvironmentStatuses("test")
	require.NoError(t, err)

	assert.Len(t, statuses, 1)
	assert.Equal(t, "test", statuses[0].Environment.Name)
	assert.Len(t, statuses[0].Agents, 1)
	assert.Equal(t, "Test Agent", statuses[0].Agents[0].Agent.Name)
	assert.Equal(t, "no tools", statuses[0].Agents[0].Status) // No tools assigned
}

func TestStatusService_GetEnvironmentStatuses_AllEnvironments(t *testing.T) {
	testDB, repos := setupTestDB(t)
	defer testDB.Close()

	statusService := NewStatusService(repos)

	// Create multiple test environments
	env1, err := repos.Environments.Create("dev", nil, 1)
	require.NoError(t, err)

	env2, err := repos.Environments.Create("staging", nil, 1)
	require.NoError(t, err)

	// Create agents in different environments
	_, err = repos.Agents.Create("Dev Agent", "Dev Agent", "Dev prompt", 3, env1.ID, 1, nil, nil, false)
	require.NoError(t, err)

	_, err = repos.Agents.Create("Staging Agent", "Staging Agent", "Staging prompt", 5, env2.ID, 1, nil, nil, false)
	require.NoError(t, err)

	// Get all environment statuses
	statuses, err := statusService.GetEnvironmentStatuses("default")
	require.NoError(t, err)

	// Should return at least 2 environments (dev, staging, plus potentially default)
	assert.GreaterOrEqual(t, len(statuses), 2)
	
	// Find our test environments
	var devStatus, stagingStatus *EnvironmentStatus
	for _, status := range statuses {
		if status.Environment.Name == "dev" {
			devStatus = status
		} else if status.Environment.Name == "staging" {
			stagingStatus = status
		}
	}

	assert.NotNil(t, devStatus)
	assert.NotNil(t, stagingStatus)
	assert.Len(t, devStatus.Agents, 1)
	assert.Len(t, stagingStatus.Agents, 1)
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short_string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact_length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long_string",
			input:    "this is a very long string",
			maxLen:   10,
			expected: "this is...",
		},
		{
			name:     "minimum_truncation",
			input:    "abcd",
			maxLen:   3,
			expected: "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateString(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}
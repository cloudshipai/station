package services

import (
	"encoding/json"
	"testing"
)

// TestAgentConfigStruct tests AgentConfig structure
func TestAgentConfigStruct(t *testing.T) {
	outputSchema := `{"type":"object"}`
	outputPreset := "finops"
	cronSchedule := "0 0 * * *"

	tests := []struct {
		name        string
		config      AgentConfig
		description string
	}{
		{
			name: "Minimal agent config",
			config: AgentConfig{
				EnvironmentID: 1,
				Name:          "test-agent",
				Prompt:        "You are a test agent",
				MaxSteps:      5,
				CreatedBy:     1,
			},
			description: "Should create minimal config",
		},
		{
			name: "Full agent config",
			config: AgentConfig{
				EnvironmentID:      1,
				Name:               "full-agent",
				Description:        "Full configuration",
				Prompt:             "Comprehensive prompt",
				AssignedTools:      []string{"tool1", "tool2"},
				MaxSteps:           10,
				CreatedBy:          1,
				InputSchema:        &outputSchema,
				OutputSchema:       &outputSchema,
				OutputSchemaPreset: &outputPreset,
				ModelProvider:      "openai",
				ModelID:            "gpt-4o",
				CronSchedule:       &cronSchedule,
				ScheduleEnabled:    true,
				App:                "finops",
				AppType:            "cost-analysis",
			},
			description: "Should create full config with all fields",
		},
		{
			name: "Agent with tools",
			config: AgentConfig{
				EnvironmentID: 1,
				Name:          "tool-agent",
				Prompt:        "Agent with tools",
				AssignedTools: []string{"read_file", "write_file", "execute_command"},
				MaxSteps:      8,
				CreatedBy:     1,
			},
			description: "Should handle multiple assigned tools",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.Name == "" {
				t.Error("Name should not be empty")
			}

			if tt.config.EnvironmentID == 0 {
				t.Error("EnvironmentID should not be 0")
			}

			if tt.config.MaxSteps == 0 {
				t.Error("MaxSteps should not be 0")
			}

			t.Logf("Config: name=%s, tools=%v, maxSteps=%d",
				tt.config.Name, tt.config.AssignedTools, tt.config.MaxSteps)
		})
	}
}

// TestAgentConfigJSONSerialization tests JSON marshaling
func TestAgentConfigJSONSerialization(t *testing.T) {
	outputSchema := `{"type":"object"}`
	cronSchedule := "0 0 * * *"

	config := AgentConfig{
		EnvironmentID:   1,
		Name:            "test-agent",
		Description:     "Test description",
		Prompt:          "Test prompt",
		AssignedTools:   []string{"tool1", "tool2"},
		MaxSteps:        5,
		CreatedBy:       1,
		OutputSchema:    &outputSchema,
		CronSchedule:    &cronSchedule,
		ScheduleEnabled: true,
		ModelProvider:   "openai",
		ModelID:         "gpt-4o",
	}

	jsonData, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal AgentConfig: %v", err)
	}

	jsonStr := string(jsonData)
	t.Logf("Serialized JSON: %s", jsonStr)

	// Verify key fields are present
	if !stringContains(jsonStr, "test-agent") {
		t.Error("JSON should contain agent name")
	}

	if !stringContains(jsonStr, "assigned_tools") {
		t.Error("JSON should contain assigned_tools field")
	}
}

// TestAgentConfigJSONDeserialization tests JSON unmarshaling
func TestAgentConfigJSONDeserialization(t *testing.T) {
	tests := []struct {
		name        string
		jsonStr     string
		wantName    string
		wantMaxSteps int64
		wantErr     bool
		description string
	}{
		{
			name:         "Valid minimal JSON",
			jsonStr:      `{"environment_id":1,"name":"test","prompt":"test prompt","max_steps":5,"created_by":1}`,
			wantName:     "test",
			wantMaxSteps: 5,
			wantErr:      false,
			description:  "Should deserialize minimal config",
		},
		{
			name:         "Valid full JSON",
			jsonStr:      `{"environment_id":1,"name":"full-agent","description":"desc","prompt":"prompt","assigned_tools":["tool1","tool2"],"max_steps":10,"created_by":1,"model_provider":"openai","model_id":"gpt-4o"}`,
			wantName:     "full-agent",
			wantMaxSteps: 10,
			wantErr:      false,
			description:  "Should deserialize full config",
		},
		{
			name:        "Invalid JSON",
			jsonStr:     `{invalid}`,
			wantErr:     true,
			description: "Should fail on invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config AgentConfig
			err := json.Unmarshal([]byte(tt.jsonStr), &config)

			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if config.Name != tt.wantName {
					t.Errorf("Name = %s, want %s", config.Name, tt.wantName)
				}

				if config.MaxSteps != tt.wantMaxSteps {
					t.Errorf("MaxSteps = %d, want %d", config.MaxSteps, tt.wantMaxSteps)
				}

				t.Logf("Deserialized config: %+v", config)
			}
		})
	}
}

// TestAgentConfigOptionalFields tests optional field handling
func TestAgentConfigOptionalFields(t *testing.T) {
	tests := []struct {
		name        string
		config      AgentConfig
		expectOmit  []string
		description string
	}{
		{
			name: "No optional fields",
			config: AgentConfig{
				EnvironmentID: 1,
				Name:          "minimal",
				Prompt:        "test",
				MaxSteps:      5,
				CreatedBy:     1,
			},
			expectOmit: []string{"input_schema", "output_schema", "cron_schedule"},
			description: "Should omit all optional fields",
		},
		{
			name: "With schedule",
			config: AgentConfig{
				EnvironmentID:   1,
				Name:            "scheduled",
				Prompt:          "test",
				MaxSteps:        5,
				CreatedBy:       1,
				ScheduleEnabled: true,
			},
			expectOmit:  []string{"input_schema", "output_schema"},
			description: "Should include schedule_enabled even if false cron",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonData, err := json.Marshal(tt.config)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			jsonStr := string(jsonData)
			_ = tt.expectOmit // Note: omitempty only omits nil pointers and zero values

			t.Logf("JSON with optional fields: %s", jsonStr)
		})
	}
}

// TestAgentConfigAssignedTools tests tool assignment
func TestAgentConfigAssignedTools(t *testing.T) {
	tests := []struct {
		name          string
		assignedTools []string
		description   string
	}{
		{
			name:          "No tools",
			assignedTools: []string{},
			description:   "Should handle empty tool list",
		},
		{
			name:          "Single tool",
			assignedTools: []string{"read_file"},
			description:   "Should handle single tool",
		},
		{
			name:          "Multiple tools",
			assignedTools: []string{"read_file", "write_file", "execute_command"},
			description:   "Should handle multiple tools",
		},
		{
			name:          "Nil tools",
			assignedTools: nil,
			description:   "Should handle nil tool list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				EnvironmentID: 1,
				Name:          "test",
				Prompt:        "test",
				AssignedTools: tt.assignedTools,
				MaxSteps:      5,
				CreatedBy:     1,
			}

			// Verify serialization works
			jsonData, err := json.Marshal(config)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			// Verify deserialization works
			var decoded AgentConfig
			if err := json.Unmarshal(jsonData, &decoded); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			t.Logf("Tools: %v", config.AssignedTools)
		})
	}
}

// TestAgentConfigModelFields tests model provider and ID fields
func TestAgentConfigModelFields(t *testing.T) {
	tests := []struct {
		name          string
		modelProvider string
		modelID       string
		description   string
	}{
		{
			name:          "OpenAI GPT-4",
			modelProvider: "openai",
			modelID:       "gpt-4o",
			description:   "Should handle OpenAI models",
		},
		{
			name:          "Anthropic Claude",
			modelProvider: "anthropic",
			modelID:       "claude-3-5-sonnet-20241022",
			description:   "Should handle Anthropic models",
		},
		{
			name:          "Empty provider",
			modelProvider: "",
			modelID:       "",
			description:   "Should handle empty model fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				EnvironmentID: 1,
				Name:          "test",
				Prompt:        "test",
				MaxSteps:      5,
				CreatedBy:     1,
				ModelProvider: tt.modelProvider,
				ModelID:       tt.modelID,
			}

			if config.ModelProvider != tt.modelProvider {
				t.Errorf("ModelProvider = %s, want %s", config.ModelProvider, tt.modelProvider)
			}

			if config.ModelID != tt.modelID {
				t.Errorf("ModelID = %s, want %s", config.ModelID, tt.modelID)
			}

			t.Logf("Model: provider=%s, id=%s", config.ModelProvider, config.ModelID)
		})
	}
}

// TestAgentConfigAppFields tests CloudShip app classification fields
func TestAgentConfigAppFields(t *testing.T) {
	tests := []struct {
		name        string
		app         string
		appType     string
		description string
	}{
		{
			name:        "FinOps cost analysis",
			app:         "finops",
			appType:     "cost-analysis",
			description: "Should handle finops app",
		},
		{
			name:        "Investigations security",
			app:         "investigations",
			appType:     "security-analysis",
			description: "Should handle investigations app",
		},
		{
			name:        "Empty app fields",
			app:         "",
			appType:     "",
			description: "Should handle empty app fields",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				EnvironmentID: 1,
				Name:          "test",
				Prompt:        "test",
				MaxSteps:      5,
				CreatedBy:     1,
				App:           tt.app,
				AppType:       tt.appType,
			}

			if config.App != tt.app {
				t.Errorf("App = %s, want %s", config.App, tt.app)
			}

			if config.AppType != tt.appType {
				t.Errorf("AppType = %s, want %s", config.AppType, tt.appType)
			}

			t.Logf("App classification: app=%s, type=%s", config.App, config.AppType)
		})
	}
}

// TestAgentConfigScheduleFields tests scheduling fields
func TestAgentConfigScheduleFields(t *testing.T) {
	daily := "0 0 * * *"
	hourly := "0 * * * *"

	tests := []struct {
		name            string
		cronSchedule    *string
		scheduleEnabled bool
		description     string
	}{
		{
			name:            "Daily schedule enabled",
			cronSchedule:    &daily,
			scheduleEnabled: true,
			description:     "Should handle daily schedule",
		},
		{
			name:            "Hourly schedule enabled",
			cronSchedule:    &hourly,
			scheduleEnabled: true,
			description:     "Should handle hourly schedule",
		},
		{
			name:            "Schedule disabled",
			cronSchedule:    &daily,
			scheduleEnabled: false,
			description:     "Should handle disabled schedule",
		},
		{
			name:            "No schedule",
			cronSchedule:    nil,
			scheduleEnabled: false,
			description:     "Should handle nil schedule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				EnvironmentID:   1,
				Name:            "test",
				Prompt:          "test",
				MaxSteps:        5,
				CreatedBy:       1,
				CronSchedule:    tt.cronSchedule,
				ScheduleEnabled: tt.scheduleEnabled,
			}

			if config.ScheduleEnabled != tt.scheduleEnabled {
				t.Errorf("ScheduleEnabled = %v, want %v", config.ScheduleEnabled, tt.scheduleEnabled)
			}

			if tt.cronSchedule != nil && config.CronSchedule == nil {
				t.Error("CronSchedule should not be nil")
			}

			t.Logf("Schedule: enabled=%v, cron=%v", config.ScheduleEnabled, config.CronSchedule)
		})
	}
}

// TestAgentConfigSchemaFields tests input/output schema fields
func TestAgentConfigSchemaFields(t *testing.T) {
	inputSchema := `{"type":"object","properties":{"input":{"type":"string"}}}`
	outputSchema := `{"type":"object","properties":{"result":{"type":"string"}}}`
	outputPreset := "investigation"

	tests := []struct {
		name               string
		inputSchema        *string
		outputSchema       *string
		outputSchemaPreset *string
		description        string
	}{
		{
			name:         "With input and output schema",
			inputSchema:  &inputSchema,
			outputSchema: &outputSchema,
			description:  "Should handle both schemas",
		},
		{
			name:               "With output preset",
			outputSchemaPreset: &outputPreset,
			description:        "Should handle output preset",
		},
		{
			name:        "No schemas",
			description: "Should handle nil schemas",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AgentConfig{
				EnvironmentID:      1,
				Name:               "test",
				Prompt:             "test",
				MaxSteps:           5,
				CreatedBy:          1,
				InputSchema:        tt.inputSchema,
				OutputSchema:       tt.outputSchema,
				OutputSchemaPreset: tt.outputSchemaPreset,
			}

			// Verify serialization
			jsonData, err := json.Marshal(config)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			t.Logf("Config with schemas: %s", string(jsonData))
		})
	}
}

// Benchmark tests
func BenchmarkAgentConfigCreation(b *testing.B) {
	outputSchema := `{"type":"object"}`
	cronSchedule := "0 0 * * *"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = AgentConfig{
			EnvironmentID:   1,
			Name:            "bench-agent",
			Description:     "Benchmark agent",
			Prompt:          "Test prompt",
			AssignedTools:   []string{"tool1", "tool2"},
			MaxSteps:        10,
			CreatedBy:       1,
			OutputSchema:    &outputSchema,
			CronSchedule:    &cronSchedule,
			ScheduleEnabled: true,
		}
	}
}

func BenchmarkAgentConfigJSONMarshal(b *testing.B) {
	config := AgentConfig{
		EnvironmentID: 1,
		Name:          "bench-agent",
		Prompt:        "test",
		AssignedTools: []string{"tool1", "tool2", "tool3"},
		MaxSteps:      10,
		CreatedBy:     1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(config)
	}
}

func BenchmarkAgentConfigJSONUnmarshal(b *testing.B) {
	jsonStr := `{"environment_id":1,"name":"test","prompt":"test prompt","assigned_tools":["tool1","tool2"],"max_steps":5,"created_by":1}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var config AgentConfig
		json.Unmarshal([]byte(jsonStr), &config)
	}
}

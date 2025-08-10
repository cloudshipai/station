package agent_bundle

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentBundleManifest_Serialization(t *testing.T) {
	now := time.Now()
	
	manifest := &AgentBundleManifest{
		Name:        "test-agent",
		Version:     "1.0.0",
		Description: "A test agent for development",
		Author:      "Test Author",
		AgentType:   "task",
		SupportedSchedules: []string{"0 */4 * * *", "@daily"},
		RequiredModel: &ModelRequirement{
			Provider:       "anthropic",
			MinContextSize: 100000,
			RequiresTools:  true,
			Models:         []string{"claude-3-5-sonnet-20241022"},
		},
		MCPBundles: []MCPBundleDependency{
			{
				Name:        "aws-tools",
				Version:     ">=1.0.0",
				Source:      "registry",
				Required:    true,
				Description: "AWS management tools",
			},
		},
		RequiredTools: []ToolRequirement{
			{
				Name:         "s3-list-buckets",
				ServerName:   "aws-s3",
				MCPBundle:    "aws-tools",
				Required:     true,
				Alternatives: []string{"aws-s3-list"},
			},
		},
		RequiredVariables: map[string]VariableSpec{
			"CLIENT_NAME": {
				Type:        "string",
				Description: "Name of the client",
				Required:    true,
			},
			"AWS_REGION": {
				Type:        "string",
				Description: "AWS region for operations",
				Required:    false,
				Default:     "us-east-1",
				Enum:        []string{"us-east-1", "us-west-2", "eu-west-1"},
			},
			"AWS_ACCESS_KEY": {
				Type:        "string",
				Description: "AWS Access Key ID",
				Required:    true,
				Pattern:     "^AKIA[0-9A-Z]{16}$",
				Sensitive:   true,
			},
		},
		Tags:           []string{"aws", "management", "client-service"},
		License:        "MIT",
		Homepage:       "https://github.com/company/aws-agent",
		StationVersion: ">=0.1.0",
		CreatedAt:      now,
	}

	// Test JSON serialization
	jsonData, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON deserialization
	var deserializedManifest AgentBundleManifest
	err = json.Unmarshal(jsonData, &deserializedManifest)
	require.NoError(t, err)

	// Verify all fields are correctly serialized/deserialized
	assert.Equal(t, manifest.Name, deserializedManifest.Name)
	assert.Equal(t, manifest.Version, deserializedManifest.Version)
	assert.Equal(t, manifest.Description, deserializedManifest.Description)
	assert.Equal(t, manifest.Author, deserializedManifest.Author)
	assert.Equal(t, manifest.AgentType, deserializedManifest.AgentType)
	assert.Equal(t, manifest.SupportedSchedules, deserializedManifest.SupportedSchedules)
	
	// Test nested structures
	require.NotNil(t, deserializedManifest.RequiredModel)
	assert.Equal(t, manifest.RequiredModel.Provider, deserializedManifest.RequiredModel.Provider)
	assert.Equal(t, manifest.RequiredModel.MinContextSize, deserializedManifest.RequiredModel.MinContextSize)
	
	assert.Len(t, deserializedManifest.MCPBundles, 1)
	assert.Equal(t, manifest.MCPBundles[0].Name, deserializedManifest.MCPBundles[0].Name)
	
	assert.Len(t, deserializedManifest.RequiredTools, 1)
	assert.Equal(t, manifest.RequiredTools[0].Name, deserializedManifest.RequiredTools[0].Name)
	
	assert.Len(t, deserializedManifest.RequiredVariables, 3)
	clientVar, exists := deserializedManifest.RequiredVariables["CLIENT_NAME"]
	assert.True(t, exists)
	assert.Equal(t, "string", clientVar.Type)
	assert.True(t, clientVar.Required)
	
	awsVar, exists := deserializedManifest.RequiredVariables["AWS_REGION"]
	assert.True(t, exists)
	assert.Equal(t, "us-east-1", awsVar.Default)
	assert.Len(t, awsVar.Enum, 3)
	
	keyVar, exists := deserializedManifest.RequiredVariables["AWS_ACCESS_KEY"]
	assert.True(t, exists)
	assert.True(t, keyVar.Sensitive)
	assert.Equal(t, "^AKIA[0-9A-Z]{16}$", keyVar.Pattern)
}

func TestAgentTemplateConfig_Serialization(t *testing.T) {
	now := time.Now()
	scheduleTemplate := "0 {{ .HOUR }} * * *"
	
	config := &AgentTemplateConfig{
		Name:              "AWS Client Manager",
		Description:       "Manages AWS resources for {{ .CLIENT_NAME }}",
		Prompt:            "You are an AWS expert managing resources for {{ .CLIENT_NAME }}. Be careful and always confirm destructive actions.",
		MaxSteps:          10,
		ScheduleTemplate:  &scheduleTemplate,
		NameTemplate:      "{{ .CLIENT_NAME }} AWS Manager",
		PromptTemplate:    "You are managing AWS for {{ .CLIENT_NAME }} in {{ .AWS_REGION }}",
		SupportedEnvs:     []string{"development", "staging", "production"},
		DefaultVars: map[string]interface{}{
			"MAX_RETRIES": 3,
			"TIMEOUT":     30,
			"LOG_LEVEL":   "info",
		},
		Version:   "1.0.0",
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Test JSON serialization
	jsonData, err := json.MarshalIndent(config, "", "  ")
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON deserialization
	var deserializedConfig AgentTemplateConfig
	err = json.Unmarshal(jsonData, &deserializedConfig)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, config.Name, deserializedConfig.Name)
	assert.Equal(t, config.Description, deserializedConfig.Description)
	assert.Equal(t, config.Prompt, deserializedConfig.Prompt)
	assert.Equal(t, config.MaxSteps, deserializedConfig.MaxSteps)
	assert.Equal(t, *config.ScheduleTemplate, *deserializedConfig.ScheduleTemplate)
	assert.Equal(t, config.NameTemplate, deserializedConfig.NameTemplate)
	assert.Equal(t, config.PromptTemplate, deserializedConfig.PromptTemplate)
	assert.Equal(t, config.SupportedEnvs, deserializedConfig.SupportedEnvs)
	assert.Equal(t, config.Version, deserializedConfig.Version)
	
	// Verify default vars (JSON converts numbers to float64)
	assert.Equal(t, 3, len(deserializedConfig.DefaultVars))
	assert.Equal(t, "info", deserializedConfig.DefaultVars["LOG_LEVEL"])
	assert.Equal(t, float64(3), deserializedConfig.DefaultVars["MAX_RETRIES"])
	assert.Equal(t, float64(30), deserializedConfig.DefaultVars["TIMEOUT"])

	// Check that template variables are present in strings
	assert.Contains(t, config.Description, "{{ .CLIENT_NAME }}")
	assert.Contains(t, config.Prompt, "{{ .CLIENT_NAME }}")
	assert.Contains(t, config.NameTemplate, "{{ .CLIENT_NAME }}")
	assert.Contains(t, config.PromptTemplate, "{{ .CLIENT_NAME }}")
	assert.Contains(t, config.PromptTemplate, "{{ .AWS_REGION }}")
}

func TestVariableSpec_Validation(t *testing.T) {
	tests := []struct {
		name     string
		spec     VariableSpec
		valid    bool
	}{
		{
			name: "valid string variable",
			spec: VariableSpec{
				Type:        "string",
				Description: "A test string variable",
				Required:    true,
				Default:     "default-value",
			},
			valid: true,
		},
		{
			name: "valid secret variable",
			spec: VariableSpec{
				Type:        "string",
				Description: "API Key",
				Required:    true,
				Pattern:     "^[A-Za-z0-9]{32}$",
				Sensitive:   true,
			},
			valid: true,
		},
		{
			name: "valid enum variable",
			spec: VariableSpec{
				Type:        "string",
				Description: "Environment selection",
				Required:    true,
				Enum:        []string{"development", "staging", "production"},
				Default:     "development",
			},
			valid: true,
		},
		{
			name: "valid number variable",
			spec: VariableSpec{
				Type:        "number",
				Description: "Timeout in seconds",
				Required:    false,
				Default:     30,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test serialization
			jsonData, err := json.Marshal(tt.spec)
			require.NoError(t, err)
			assert.NotEmpty(t, jsonData)

			// Test deserialization
			var deserializedSpec VariableSpec
			err = json.Unmarshal(jsonData, &deserializedSpec)
			require.NoError(t, err)

			// Verify fields
			assert.Equal(t, tt.spec.Type, deserializedSpec.Type)
			assert.Equal(t, tt.spec.Description, deserializedSpec.Description)
			assert.Equal(t, tt.spec.Required, deserializedSpec.Required)
			assert.Equal(t, tt.spec.Pattern, deserializedSpec.Pattern)
			assert.Equal(t, tt.spec.Enum, deserializedSpec.Enum)
			assert.Equal(t, tt.spec.Sensitive, deserializedSpec.Sensitive)
			
			// Handle JSON number conversion for default values
			if tt.spec.Default != nil {
				switch expectedDefault := tt.spec.Default.(type) {
				case int:
					assert.Equal(t, float64(expectedDefault), deserializedSpec.Default)
				default:
					assert.Equal(t, tt.spec.Default, deserializedSpec.Default)
				}
			}
		})
	}
}

func TestCreateOptions_Defaults(t *testing.T) {
	opts := CreateOptions{
		Name:        "test-agent",
		Author:      "Test Author",
		Description: "A test agent",
	}

	// Test that basic options are set correctly
	assert.Equal(t, "test-agent", opts.Name)
	assert.Equal(t, "Test Author", opts.Author)
	assert.Equal(t, "A test agent", opts.Description)

	// Test default values
	assert.Nil(t, opts.FromAgent)
	assert.Empty(t, opts.Environment)
	assert.False(t, opts.IncludeMCP)
	assert.Empty(t, opts.Variables)
	assert.Empty(t, opts.AgentType)
	assert.Empty(t, opts.Tags)
}

func TestValidationResult_EmptyState(t *testing.T) {
	result := &ValidationResult{
		Valid:   false,
		Errors:  []ValidationError{},
		Warnings: []ValidationWarning{},
	}

	// Test JSON serialization of empty validation result
	jsonData, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var deserializedResult ValidationResult
	err = json.Unmarshal(jsonData, &deserializedResult)
	require.NoError(t, err)

	assert.False(t, deserializedResult.Valid)
	assert.Empty(t, deserializedResult.Errors)
	assert.Empty(t, deserializedResult.Warnings)
}

func TestValidationResult_WithErrorsAndWarnings(t *testing.T) {
	result := &ValidationResult{
		Valid: false,
		Errors: []ValidationError{
			{
				Type:       "missing_field",
				Message:    "Required field 'name' is missing",
				Field:      "name",
				Suggestion: "Add a name field to the manifest",
			},
		},
		Warnings: []ValidationWarning{
			{
				Type:       "deprecated_field",
				Message:    "Field 'old_field' is deprecated",
				Field:      "old_field",
				Suggestion: "Use 'new_field' instead",
			},
		},
		ManifestValid:     false,
		AgentConfigValid:  true,
		ToolsValid:        true,
		DependenciesValid: false,
		VariablesValid:    true,
		Statistics: ValidationStatistics{
			TotalVariables:    5,
			RequiredVariables: 3,
			OptionalVariables: 2,
			MCPDependencies:   2,
			RequiredTools:     4,
			OptionalTools:     1,
		},
	}

	// Test JSON serialization
	jsonData, err := json.Marshal(result)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var deserializedResult ValidationResult
	err = json.Unmarshal(jsonData, &deserializedResult)
	require.NoError(t, err)

	// Verify all fields
	assert.False(t, deserializedResult.Valid)
	assert.Len(t, deserializedResult.Errors, 1)
	assert.Len(t, deserializedResult.Warnings, 1)
	
	// Check error details
	error := deserializedResult.Errors[0]
	assert.Equal(t, "missing_field", error.Type)
	assert.Equal(t, "Required field 'name' is missing", error.Message)
	assert.Equal(t, "name", error.Field)
	assert.Equal(t, "Add a name field to the manifest", error.Suggestion)

	// Check warning details
	warning := deserializedResult.Warnings[0]
	assert.Equal(t, "deprecated_field", warning.Type)
	assert.Equal(t, "Field 'old_field' is deprecated", warning.Message)
	assert.Equal(t, "old_field", warning.Field)
	assert.Equal(t, "Use 'new_field' instead", warning.Suggestion)

	// Check validation status fields
	assert.False(t, deserializedResult.ManifestValid)
	assert.True(t, deserializedResult.AgentConfigValid)
	assert.True(t, deserializedResult.ToolsValid)
	assert.False(t, deserializedResult.DependenciesValid)
	assert.True(t, deserializedResult.VariablesValid)

	// Check statistics
	stats := deserializedResult.Statistics
	assert.Equal(t, 5, stats.TotalVariables)
	assert.Equal(t, 3, stats.RequiredVariables)
	assert.Equal(t, 2, stats.OptionalVariables)
	assert.Equal(t, 2, stats.MCPDependencies)
	assert.Equal(t, 4, stats.RequiredTools)
	assert.Equal(t, 1, stats.OptionalTools)
}

func TestAgentClientVariable_Encryption(t *testing.T) {
	// Note: This test doesn't actually test encryption, just data structure
	variable := &AgentClientVariable{
		ID:               1,
		TemplateAgentID:  100,
		ClientID:         "acme-corp",
		VariableName:     "CLIENT_AWS_ACCESS_KEY",
		EncryptedValue:   "encrypted_value_placeholder", // In reality, this would be AES encrypted
		VariableType:     string(VariableTypeAgent),
		Description:      "AWS Access Key for Acme Corp",
		IsRequired:       true,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Test JSON serialization (encrypted value should be preserved)
	jsonData, err := json.Marshal(variable)
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	var deserializedVariable AgentClientVariable
	err = json.Unmarshal(jsonData, &deserializedVariable)
	require.NoError(t, err)

	// Verify all fields are preserved
	assert.Equal(t, variable.ID, deserializedVariable.ID)
	assert.Equal(t, variable.TemplateAgentID, deserializedVariable.TemplateAgentID)
	assert.Equal(t, variable.ClientID, deserializedVariable.ClientID)
	assert.Equal(t, variable.VariableName, deserializedVariable.VariableName)
	assert.Equal(t, variable.EncryptedValue, deserializedVariable.EncryptedValue)
	assert.Equal(t, variable.VariableType, deserializedVariable.VariableType)
	assert.Equal(t, variable.Description, deserializedVariable.Description)
	assert.Equal(t, variable.IsRequired, deserializedVariable.IsRequired)
}

func TestVariableTypes(t *testing.T) {
	// Test variable type constants
	assert.Equal(t, "agent", string(VariableTypeAgent))
	assert.Equal(t, "mcp", string(VariableTypeMCP))
	assert.Equal(t, "system", string(VariableTypeSystem))

	// Test that they can be used in structures
	variable := AgentClientVariable{
		VariableType: string(VariableTypeAgent),
	}
	assert.Equal(t, "agent", variable.VariableType)
}
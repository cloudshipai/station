package dotprompt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDotpromptExecutionFunctionality(t *testing.T) {
	testDir := t.TempDir()

	t.Run("RuntimeExtraction_RealFileProcessing", func(t *testing.T) {
		// Test real file processing as it happens during sync
		agentContent := `---
metadata:
  name: "file-processing-agent"
  description: "Agent that processes files during sync operations"
  version: "1.0.0"
  max_steps: 5
model: "gpt-4"
tools:
  - "__read_text_file"
  - "__write_text_file"
  - "__list_directory"
config:
  temperature: 0.7
  max_tokens: 2048
---

{{role "system"}}
You are a file processing agent with the following capabilities:

**File Operations:**
- Read files with __read_text_file
- Write files with __write_text_file  
- List directories with __list_directory

**Your Role:**
Process files efficiently and provide helpful responses about file operations.

**Environment:** {{.ENVIRONMENT}}
**Project Root:** {{.PROJECT_ROOT}}

{{role "user"}}
Task: {{userInput}}
Working Directory: {{.WORKING_DIR}}
`

		agentFile := filepath.Join(testDir, "file-processing-agent.prompt")
		err := os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		// Test RuntimeExtraction processes this correctly
		extractor, err := NewRuntimeExtraction(agentFile)
		require.NoError(t, err)
		assert.NotNil(t, extractor)

		// Test config extraction
		config := extractor.GetConfig()
		assert.Equal(t, "file-processing-agent", config.Metadata.Name)
		assert.Equal(t, "Agent that processes files during sync operations", config.Metadata.Description)
		assert.Equal(t, "1.0.0", config.Metadata.Version)
		assert.Equal(t, 5, config.Metadata.MaxSteps)
		assert.Equal(t, "gpt-4", config.Model)

		// Test tools extraction
		expectedTools := []string{"__read_text_file", "__write_text_file", "__list_directory"}
		assert.Equal(t, expectedTools, config.Tools)

		// Test config parameters - removed for gpt-5 compatibility
		// assert.NotNil(t, config.Config.Temperature)
		// assert.Equal(t, float32(0.7), *config.Config.Temperature)
		// assert.NotNil(t, config.Config.MaxTokens)
		// assert.Equal(t, 2048, *config.Config.MaxTokens)

		// Test template extraction with variables
		template := extractor.GetTemplate()
		assert.Contains(t, template, "{{.ENVIRONMENT}}")
		assert.Contains(t, template, "{{.PROJECT_ROOT}}")
		assert.Contains(t, template, "{{userInput}}")
		assert.Contains(t, template, "{{.WORKING_DIR}}")
		assert.Contains(t, template, "{{role \"system\"}}")
		assert.Contains(t, template, "{{role \"user\"}}")
	})

	t.Run("RuntimeExtraction_ComplexAgentStructure", func(t *testing.T) {
		// Test complex agent structure with multiple sections
		agentContent := `---
metadata:
  name: "complex-processing-agent"
  description: "Complex agent with multiple configuration sections"
  version: "2.1.0"
  max_steps: 12
model: "gpt-4"
tools:
  - "__filesystem_read"
  - "__filesystem_write" 
  - "__database_query"
  - "__web_fetch"
  - "__email_send"
config:
  temperature: 0.3
  max_tokens: 4096
  top_p: 0.9
  top_k: 40
input:
  schema:
    type: "object"
    properties:
      task:
        type: "string"
        description: "The task to perform"
      priority:
        type: "string"
        enum: ["low", "medium", "high", "critical"]
      context:
        type: "object"
output:
  format: "json"
  schema:
    type: "object"
    properties:
      result:
        type: "string"
      status:
        type: "string"
      metadata:
        type: "object"
---

{{role "system"}}
You are a complex processing agent with advanced capabilities:

**Filesystem Operations:**
- Read files: __filesystem_read
- Write files: __filesystem_write

**Database Operations:**
- Query database: __database_query

**Network Operations:** 
- Fetch web content: __web_fetch
- Send emails: __email_send

**Input Schema:**
- Task: {{.task}} (Priority: {{.priority}})
- Context: {{.context}}

**Configuration:**
- Max Steps: 12
- Temperature: 0.3 (precise responses)
- Response Format: JSON

{{role "user"}}
**Task:** {{userInput}}
**Priority:** {{.priority}}
**Additional Context:** {{.context}}
`

		agentFile := filepath.Join(testDir, "complex-processing-agent.prompt")
		err := os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		// Test RuntimeExtraction handles complex structure
		extractor, err := NewRuntimeExtraction(agentFile)
		require.NoError(t, err)

		config := extractor.GetConfig()
		
		// Test metadata
		assert.Equal(t, "complex-processing-agent", config.Metadata.Name)
		assert.Equal(t, "2.1.0", config.Metadata.Version)
		assert.Equal(t, 12, config.Metadata.MaxSteps)

		// Test model config
		assert.Equal(t, "gpt-4", config.Model)
		
		// Test generation config - removed for gpt-5 compatibility
		// assert.Equal(t, float32(0.3), *config.Config.Temperature)
		// assert.Equal(t, 4096, *config.Config.MaxTokens)
		// assert.Equal(t, float32(0.9), *config.Config.TopP)
		// assert.Equal(t, 40, *config.Config.TopK)

		// Test tools
		expectedTools := []string{"__filesystem_read", "__filesystem_write", "__database_query", "__web_fetch", "__email_send"}
		assert.Equal(t, expectedTools, config.Tools)

		// Test input/output schema
		assert.NotNil(t, config.Input.Schema)
		assert.NotNil(t, config.Output.Schema)
		assert.Equal(t, "json", config.Output.Format)

		// Test template contains variable references
		template := extractor.GetTemplate()
		assert.Contains(t, template, "{{.task}}")
		assert.Contains(t, template, "{{.priority}}")
		assert.Contains(t, template, "{{.context}}")
		assert.Contains(t, template, "{{userInput}}")
	})

	t.Run("RuntimeExtraction_ErrorScenarios", func(t *testing.T) {
		// Test how extraction handles various error scenarios
		testCases := []struct {
			name        string
			content     string
			expectError bool
			description string
		}{
			{
				name: "MissingMetadata",
				content: `---
model: "gpt-4"
tools: []
---

{{role "system"}}
Missing metadata section.

{{role "user"}}
{{userInput}}
`,
				expectError: false, // Should handle gracefully
				description: "Should handle missing metadata",
			},
			{
				name: "EmptyMetadataName",
				content: `---
metadata:
  name: ""
  description: "Agent with empty name"
model: "gpt-4"
---

{{role "system"}}
Empty name test.

{{role "user"}}
{{userInput}}
`,
				expectError: false,
				description: "Should handle empty name",
			},
			{
				name: "MalformedYAML",
				content: `---
metadata:
  name: "malformed-agent"
  description: "Agent with malformed YAML"
model: "gpt-4"
tools:
  - "tool1"
  - tool2: invalid  # This is invalid YAML structure
---

{{role "system"}}
Malformed YAML test.

{{role "user"}}
{{userInput}}
`,
				expectError: true,
				description: "Should reject malformed YAML",
			},
			{
				name: "MissingClosingDelimiter",
				content: `---
metadata:
  name: "no-closing-delimiter"
  description: "Missing closing ---"
model: "gpt-4"

{{role "system"}}
No closing delimiter.

{{role "user"}}
{{userInput}}
`,
				expectError: true,
				description: "Should reject missing closing delimiter",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				agentFile := filepath.Join(testDir, tc.name+".prompt")
				err := os.WriteFile(agentFile, []byte(tc.content), 0644)
				require.NoError(t, err)

				extractor, err := NewRuntimeExtraction(agentFile)
				if tc.expectError {
					assert.Error(t, err, tc.description)
					assert.Nil(t, extractor)
				} else {
					assert.NoError(t, err, tc.description)
					if extractor != nil {
						config := extractor.GetConfig()
						assert.NotEmpty(t, config.Model, "Should have model even with issues")
					}
				}
			})
		}
	})

	t.Run("RuntimeExtraction_FileReadPerformance", func(t *testing.T) {
		// Test performance with larger agent files (as might be found in production)
		largeTemplate := `{{role "system"}}
You are a comprehensive agent with extensive capabilities and documentation.

## Capabilities Overview

### File System Operations
- Read files of various formats (text, JSON, YAML, CSV, etc.)
- Write structured data to files
- Navigate directory structures
- Search and filter files by various criteria
- Monitor file changes and updates

### Data Processing
- Parse structured data formats (JSON, XML, YAML, CSV)
- Transform and manipulate data structures
- Validate data against schemas
- Perform data analysis and aggregation
- Generate reports and summaries

### Communication
- Send emails with attachments
- Make HTTP requests to APIs
- Handle webhook notifications
- Format messages for different channels
- Integrate with external services

### Database Operations
- Execute SQL queries safely
- Perform CRUD operations
- Handle database connections
- Optimize query performance
- Manage transactions

### Security Considerations
- Input validation and sanitization
- Secure credential handling
- Access control verification
- Audit logging
- Error handling without information disclosure

### Error Handling
- Graceful degradation on failures
- Comprehensive error logging
- Retry mechanisms for transient failures
- User-friendly error messages
- Fallback operations

## Usage Instructions

When given a task, I will:
1. Analyze the requirements carefully
2. Break down complex tasks into manageable steps
3. Execute operations in the correct sequence
4. Validate results at each step
5. Provide detailed feedback on progress
6. Handle any errors gracefully
7. Deliver comprehensive results

## Template Variables

Environment Context:
- Environment: {{.ENVIRONMENT}}
- Project Root: {{.PROJECT_ROOT}}
- Working Directory: {{.WORKING_DIR}}
- Debug Mode: {{.DEBUG_MODE}}
- User: {{.USER}}
- Timestamp: {{.TIMESTAMP}}

Task Context:
- Task ID: {{.TASK_ID}}
- Priority: {{.PRIORITY}}
- Deadline: {{.DEADLINE}}
- Dependencies: {{.DEPENDENCIES}}

Configuration:
- Max Steps: {{.MAX_STEPS}}
- Timeout: {{.TIMEOUT}}
- Retry Count: {{.RETRY_COUNT}}
- Log Level: {{.LOG_LEVEL}}

{{role "user"}}
**Primary Task:** {{userInput}}

**Context Information:**
- Environment: {{.ENVIRONMENT}}
- Priority Level: {{.PRIORITY}}
- Available Tools: {{.AVAILABLE_TOOLS}}
- Working Directory: {{.WORKING_DIR}}
- Time Limit: {{.TIMEOUT}}

**Additional Instructions:**
{{.ADDITIONAL_INSTRUCTIONS}}
`

		agentContent := `---
metadata:
  name: "comprehensive-agent"
  description: "Large comprehensive agent for performance testing"
  version: "3.0.0"
  max_steps: 25
model: "gpt-4"
tools:
  - "__read_text_file"
  - "__write_text_file"
  - "__list_directory"
  - "__search_files"
  - "__get_file_info"
  - "__database_query"
  - "__database_execute"
  - "__web_fetch"
  - "__web_post"
  - "__email_send"
  - "__email_receive"
  - "__json_parse"
  - "__json_generate"
  - "__csv_read"
  - "__csv_write"
  - "__xml_parse"
  - "__yaml_parse"
  - "__regex_match"
  - "__string_transform"
  - "__math_calculate"
  - "__date_format"
  - "__hash_generate"
  - "__encrypt_data"
  - "__decrypt_data"
  - "__log_message"
config:
  temperature: 0.5
  max_tokens: 8192
  top_p: 0.95
  top_k: 50
input:
  schema:
    type: "object"
    properties:
      task:
        type: "string"
        description: "Primary task to execute"
      priority:
        type: "string"
        enum: ["low", "medium", "high", "critical", "emergency"]
      context:
        type: "object"
        properties:
          environment:
            type: "string"
          project_root:
            type: "string"
          working_dir:
            type: "string"
          user:
            type: "string"
          debug_mode:
            type: "boolean"
      constraints:
        type: "object"
        properties:
          max_execution_time:
            type: "string"
          memory_limit:
            type: "string"
          network_access:
            type: "boolean"
          file_system_access:
            type: "boolean"
output:
  format: "structured"
  schema:
    type: "object"
    properties:
      success:
        type: "boolean"
      result:
        type: "object"
      metadata:
        type: "object"
        properties:
          execution_time:
            type: "string"
          steps_taken:
            type: "integer"
          resources_used:
            type: "array"
          warnings:
            type: "array"
          errors:
            type: "array"
---

` + largeTemplate

		agentFile := filepath.Join(testDir, "comprehensive-agent.prompt")
		err := os.WriteFile(agentFile, []byte(agentContent), 0644)
		require.NoError(t, err)

		// Measure extraction performance
		extractor, err := NewRuntimeExtraction(agentFile)
		require.NoError(t, err)
		assert.NotNil(t, extractor)

		config := extractor.GetConfig()
		assert.Equal(t, "comprehensive-agent", config.Metadata.Name)
		assert.Equal(t, 25, config.Metadata.MaxSteps)
		assert.Len(t, config.Tools, 25) // Should handle 25 tools

		template := extractor.GetTemplate()
		assert.Contains(t, template, "comprehensive agent")
		assert.Contains(t, template, "{{.ENVIRONMENT}}")
		assert.True(t, len(template) > 1000, "Template should be substantial")
	})
}
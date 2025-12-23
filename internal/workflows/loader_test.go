package workflows

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoader_LoadAll_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")

	loader := NewLoader(workflowsDir)
	result, err := loader.LoadAll()

	require.NoError(t, err)
	assert.Empty(t, result.Workflows)
	assert.Empty(t, result.Errors)
	assert.Equal(t, 0, result.TotalFiles)
}

func TestLoader_LoadAll_WithYAMLWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `
id: incident-triage
name: Incident Triage Workflow
version: "1.0"
description: Automated incident triage process
start: classify
states:
  - name: classify
    type: agent
    input:
      agent: incident-classifier
      task: "Classify the incident"
    transition: notify
  - name: notify
    type: agent
    input:
      agent: slack-notifier
      task: "Send notification"
    end: true
`
	workflowPath := filepath.Join(workflowsDir, "incident-triage.workflow.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(workflowContent), 0644))

	loader := NewLoader(workflowsDir)
	result, err := loader.LoadAll()

	require.NoError(t, err)
	assert.Len(t, result.Workflows, 1)
	assert.Empty(t, result.Errors)
	assert.Equal(t, 1, result.TotalFiles)

	wf := result.Workflows[0]
	assert.Equal(t, "incident-triage", wf.WorkflowID)
	assert.Equal(t, "Incident Triage Workflow", wf.Definition.Name)
	assert.Equal(t, "classify", wf.Definition.Start)
	assert.Len(t, wf.Definition.States, 2)
	assert.NotEmpty(t, wf.Checksum)
}

func TestLoader_LoadAll_WithJSONWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `{
  "id": "deploy-pipeline",
  "name": "Deployment Pipeline",
  "version": "2.0",
  "start": "build",
  "states": [
    {
      "name": "build",
      "type": "agent",
      "input": {"agent": "builder", "task": "Build the app"},
      "transition": "deploy"
    },
    {
      "name": "deploy",
      "type": "agent",
      "input": {"agent": "deployer", "task": "Deploy to prod"},
      "end": true
    }
  ]
}`
	workflowPath := filepath.Join(workflowsDir, "deploy-pipeline.workflow.json")
	require.NoError(t, os.WriteFile(workflowPath, []byte(workflowContent), 0644))

	loader := NewLoader(workflowsDir)
	result, err := loader.LoadAll()

	require.NoError(t, err)
	assert.Len(t, result.Workflows, 1)
	assert.Empty(t, result.Errors)

	wf := result.Workflows[0]
	assert.Equal(t, "deploy-pipeline", wf.WorkflowID)
	assert.Equal(t, "Deployment Pipeline", wf.Definition.Name)
}

func TestLoader_LoadAll_MultipleWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflow1 := `
id: workflow-one
name: Workflow One
start: step1
states:
  - name: step1
    type: agent
    end: true
`
	workflow2 := `
id: workflow-two
name: Workflow Two  
start: step1
states:
  - name: step1
    type: agent
    end: true
`
	workflow3 := `{
  "id": "workflow-three",
  "name": "Workflow Three",
  "start": "step1",
  "states": [{"name": "step1", "type": "agent", "end": true}]
}`

	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "workflow-one.workflow.yaml"), []byte(workflow1), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "workflow-two.workflow.yml"), []byte(workflow2), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "workflow-three.workflow.json"), []byte(workflow3), 0644))

	loader := NewLoader(workflowsDir)
	result, err := loader.LoadAll()

	require.NoError(t, err)
	assert.Len(t, result.Workflows, 3)
	assert.Empty(t, result.Errors)
	assert.Equal(t, 3, result.TotalFiles)

	ids := make(map[string]bool)
	for _, wf := range result.Workflows {
		ids[wf.WorkflowID] = true
	}
	assert.True(t, ids["workflow-one"])
	assert.True(t, ids["workflow-two"])
	assert.True(t, ids["workflow-three"])
}

func TestLoader_LoadAll_InvalidWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	invalidContent := `
id: invalid-workflow
name: Invalid Workflow
states: []
`
	workflowPath := filepath.Join(workflowsDir, "invalid.workflow.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(invalidContent), 0644))

	loader := NewLoader(workflowsDir)
	result, err := loader.LoadAll()

	require.NoError(t, err)
	assert.Empty(t, result.Workflows)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, 1, result.TotalFiles)
	assert.Contains(t, result.Errors[0].Error.Error(), "validation failed")
}

func TestLoader_LoadAll_MixedValidAndInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	validContent := `
id: valid-workflow
name: Valid Workflow
start: step1
states:
  - name: step1
    type: agent
    end: true
`
	invalidContent := `invalid yaml: [`

	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "valid.workflow.yaml"), []byte(validContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "invalid.workflow.yaml"), []byte(invalidContent), 0644))

	loader := NewLoader(workflowsDir)
	result, err := loader.LoadAll()

	require.NoError(t, err)
	assert.Len(t, result.Workflows, 1)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, 2, result.TotalFiles)
	assert.Equal(t, "valid-workflow", result.Workflows[0].WorkflowID)
}

func TestLoader_LoadFile_InfersIDFromFilename(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `
name: No ID Workflow
start: step1
states:
  - name: step1
    type: agent
    end: true
`
	workflowPath := filepath.Join(workflowsDir, "inferred-id.workflow.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(workflowContent), 0644))

	loader := NewLoader(workflowsDir)
	wf, err := loader.LoadFile(workflowPath)

	require.NoError(t, err)
	assert.Equal(t, "inferred-id", wf.WorkflowID)
	assert.Equal(t, "inferred-id", wf.Definition.ID)
}

func TestLoader_LoadFile_PreservesExplicitID(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `
id: explicit-workflow-id
name: Explicit ID Workflow
start: step1
states:
  - name: step1
    type: agent
    end: true
`
	workflowPath := filepath.Join(workflowsDir, "different-filename.workflow.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(workflowContent), 0644))

	loader := NewLoader(workflowsDir)
	wf, err := loader.LoadFile(workflowPath)

	require.NoError(t, err)
	assert.Equal(t, "explicit-workflow-id", wf.WorkflowID)
}

func TestLoader_LoadFile_ChecksumIsDeterministic(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	workflowContent := `
id: checksum-test
name: Checksum Test
start: step1
states:
  - name: step1
    type: agent
    end: true
`
	workflowPath := filepath.Join(workflowsDir, "checksum-test.workflow.yaml")
	require.NoError(t, os.WriteFile(workflowPath, []byte(workflowContent), 0644))

	loader := NewLoader(workflowsDir)

	wf1, err := loader.LoadFile(workflowPath)
	require.NoError(t, err)

	wf2, err := loader.LoadFile(workflowPath)
	require.NoError(t, err)

	assert.Equal(t, wf1.Checksum, wf2.Checksum)
	assert.Len(t, wf1.Checksum, 32)
}

func TestLoader_IgnoresNonWorkflowFiles(t *testing.T) {
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, "workflows")
	require.NoError(t, os.MkdirAll(workflowsDir, 0755))

	validWorkflow := `
id: valid
name: Valid
start: step1
states:
  - name: step1
    type: agent
    end: true
`
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "valid.workflow.yaml"), []byte(validWorkflow), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "readme.md"), []byte("# README"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "config.yaml"), []byte("key: value"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(workflowsDir, "data.json"), []byte(`{"foo":"bar"}`), 0644))

	loader := NewLoader(workflowsDir)
	result, err := loader.LoadAll()

	require.NoError(t, err)
	assert.Len(t, result.Workflows, 1)
	assert.Equal(t, 1, result.TotalFiles)
	assert.Equal(t, "valid", result.Workflows[0].WorkflowID)
}

func TestExtractWorkflowID(t *testing.T) {
	tests := []struct {
		filePath string
		expected string
	}{
		{"/path/to/incident-triage.workflow.yaml", "incident-triage"},
		{"/path/to/deploy-pipeline.workflow.yml", "deploy-pipeline"},
		{"/path/to/security-scan.workflow.json", "security-scan"},
		{"/path/to/simple.yaml", "simple"},
		{"my-workflow.workflow.yaml", "my-workflow"},
	}

	for _, tt := range tests {
		t.Run(tt.filePath, func(t *testing.T) {
			result := extractWorkflowID(tt.filePath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeChecksum(t *testing.T) {
	content1 := []byte("hello world")
	content2 := []byte("hello world")
	content3 := []byte("different content")

	checksum1 := computeChecksum(content1)
	checksum2 := computeChecksum(content2)
	checksum3 := computeChecksum(content3)

	assert.Equal(t, checksum1, checksum2)
	assert.NotEqual(t, checksum1, checksum3)
	assert.Len(t, checksum1, 32)
}

func TestConvertYAMLToJSON(t *testing.T) {
	t.Run("converts map[interface{}]interface{} to map[string]interface{}", func(t *testing.T) {
		input := map[interface{}]interface{}{
			"name": "test",
			"nested": map[interface{}]interface{}{
				"key": "value",
			},
		}

		result := convertYAMLToJSON(input)
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "test", resultMap["name"])

		nestedMap, ok := resultMap["nested"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "value", nestedMap["key"])
	})

	t.Run("handles arrays", func(t *testing.T) {
		input := []interface{}{
			map[interface{}]interface{}{"id": 1},
			map[interface{}]interface{}{"id": 2},
		}

		result := convertYAMLToJSON(input)
		resultArray, ok := result.([]interface{})
		require.True(t, ok)
		assert.Len(t, resultArray, 2)
	})

	t.Run("passes through primitives", func(t *testing.T) {
		assert.Equal(t, "test", convertYAMLToJSON("test"))
		assert.Equal(t, 42, convertYAMLToJSON(42))
		assert.Equal(t, true, convertYAMLToJSON(true))
	})
}

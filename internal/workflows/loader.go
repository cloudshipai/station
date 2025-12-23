package workflows

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type WorkflowFile struct {
	FilePath   string
	WorkflowID string
	Definition *Definition
	RawContent json.RawMessage
	Checksum   string
}

type LoadResult struct {
	Workflows  []*WorkflowFile
	Errors     []LoadError
	TotalFiles int
}

type LoadError struct {
	FilePath string
	Error    error
}

type Loader struct {
	workflowsDir string
}

func NewLoader(workflowsDir string) *Loader {
	return &Loader{workflowsDir: workflowsDir}
}

func (l *Loader) LoadAll() (*LoadResult, error) {
	result := &LoadResult{
		Workflows: []*WorkflowFile{},
		Errors:    []LoadError{},
	}

	if _, err := os.Stat(l.workflowsDir); os.IsNotExist(err) {
		return result, nil
	}

	yamlFiles, err := filepath.Glob(filepath.Join(l.workflowsDir, "*.workflow.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan workflow yaml files: %w", err)
	}

	ymlFiles, err := filepath.Glob(filepath.Join(l.workflowsDir, "*.workflow.yml"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan workflow yml files: %w", err)
	}

	jsonFiles, err := filepath.Glob(filepath.Join(l.workflowsDir, "*.workflow.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan workflow json files: %w", err)
	}

	allFiles := append(yamlFiles, ymlFiles...)
	allFiles = append(allFiles, jsonFiles...)
	result.TotalFiles = len(allFiles)

	for _, filePath := range allFiles {
		wf, err := l.LoadFile(filePath)
		if err != nil {
			result.Errors = append(result.Errors, LoadError{
				FilePath: filePath,
				Error:    err,
			})
			continue
		}
		result.Workflows = append(result.Workflows, wf)
	}

	return result, nil
}

func (l *Loader) LoadFile(filePath string) (*WorkflowFile, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	checksum := computeChecksum(content)
	workflowID := extractWorkflowID(filePath)

	var dataMap map[string]interface{}
	if strings.HasSuffix(filePath, ".json") {
		if err := json.Unmarshal(content, &dataMap); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
	} else {
		var yamlData interface{}
		if err := yaml.Unmarshal(content, &yamlData); err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}
		converted := convertYAMLToJSON(yamlData)
		var ok bool
		dataMap, ok = converted.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("workflow definition must be an object")
		}
	}

	if _, hasID := dataMap["id"]; !hasID {
		dataMap["id"] = workflowID
	}

	rawJSON, err := json.Marshal(dataMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to JSON: %w", err)
	}

	def, validationResult, err := ValidateDefinition(rawJSON)
	if err != nil && len(validationResult.Errors) > 0 {
		var errMsgs []string
		for _, ve := range validationResult.Errors {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", ve.Path, ve.Message))
		}
		return nil, fmt.Errorf("validation failed: %s", strings.Join(errMsgs, "; "))
	}

	return &WorkflowFile{
		FilePath:   filePath,
		WorkflowID: def.ID,
		Definition: def,
		RawContent: rawJSON,
		Checksum:   checksum,
	}, nil
}

func extractWorkflowID(filePath string) string {
	base := filepath.Base(filePath)
	for _, suffix := range []string{".workflow.yaml", ".workflow.yml", ".workflow.json"} {
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix)
		}
	}
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func computeChecksum(content []byte) string {
	hash := md5.Sum(content)
	return hex.EncodeToString(hash[:])
}

func convertYAMLToJSON(input interface{}) interface{} {
	switch v := input.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = convertYAMLToJSON(val)
		}
		return result
	case map[interface{}]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			strKey := fmt.Sprintf("%v", key)
			result[strKey] = convertYAMLToJSON(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = convertYAMLToJSON(val)
		}
		return result
	default:
		return v
	}
}

package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"station/pkg/models"
)

// ValidateDefinition parses and validates a workflow definition, returning both errors and warnings.
func ValidateDefinition(raw json.RawMessage) (*Definition, ValidationResult, error) {
	var result ValidationResult
	if len(raw) == 0 {
		result.Errors = append(result.Errors, ValidationIssue{
			Code:    "EMPTY_DEFINITION",
			Path:    "/",
			Message: "Workflow definition is required",
			Hint:    "Pass a Serverless Workflow-compatible object under the 'definition' field.",
		})
		return nil, result, ErrValidation
	}

	var def Definition
	if err := yaml.Unmarshal(raw, &def); err != nil {
		result.Errors = append(result.Errors, ValidationIssue{
			Code:    "INVALID_DEFINITION",
			Path:    "/",
			Message: fmt.Sprintf("Failed to parse workflow definition: %v", err),
			Hint:    "Ensure the definition is valid JSON or YAML and follows the Station workflow profile.",
		})
		return nil, result, ErrValidation
	}

	// Normalize state IDs using either id or name
	for i := range def.States {
		if def.States[i].ID == "" {
			def.States[i].ID = def.States[i].Name
		}
	}

	// Validate top-level attributes
	if def.ID == "" {
		result.Errors = append(result.Errors, ValidationIssue{
			Code:    "MISSING_WORKFLOW_ID",
			Path:    "/id",
			Message: "Workflows must declare a stable id",
			Hint:    "Add an 'id' field to the workflow definition. Example: id: incident-runbook",
		})
	}

	if len(def.States) == 0 {
		result.Errors = append(result.Errors, ValidationIssue{
			Code:    "MISSING_STATES",
			Path:    "/states",
			Message: "At least one state/step is required",
			Hint:    "Add a 'states' array with operation, switch, foreach, await, or parallel steps.",
		})
	}

	// First pass: capture IDs and detect duplicates
	stateIDs := make(map[string]int)
	for i, state := range def.States {
		pathPrefix := fmt.Sprintf("/states/%d", i)
		stableID := state.StableID()
		if stableID == "" {
			result.Errors = append(result.Errors, ValidationIssue{
				Code:    "MISSING_STEP_ID",
				Path:    pathPrefix,
				Message: "Each state/step must have an id",
				Hint:    "Set 'id' (preferred) or 'name' on every step to make it replayable.",
			})
			continue
		}

		if prev, exists := stateIDs[stableID]; exists {
			result.Errors = append(result.Errors, ValidationIssue{
				Code:    "DUPLICATE_STEP_ID",
				Path:    pathPrefix,
				Message: fmt.Sprintf("Step id '%s' is already used at states/%d", stableID, prev),
				Actual:  stableID,
				Hint:    "Ensure every step id is unique and stable across versions.",
			})
			continue
		}
		stateIDs[stableID] = i
	}

	// Second pass: validate state semantics using the full state ID map
	for i, state := range def.States {
		pathPrefix := fmt.Sprintf("/states/%d", i)
		stableID := state.StableID()
		if stableID == "" {
			continue
		}

		if state.Type == "" {
			result.Errors = append(result.Errors, ValidationIssue{
				Code:    "MISSING_TYPE",
				Path:    pathPrefix + "/type",
				Message: "Step type is required (e.g., operation, switch, foreach, await, parallel)",
				Hint:    "Set 'type' for each state. Example: type: operation",
			})
		}

		// Station profile requires explicit IO mapping, timeout, and retry.
		if state.Input == nil {
			result.Warnings = append(result.Warnings, ValidationIssue{
				Code:    "MISSING_INPUT_MAPPING",
				Path:    pathPrefix + "/input",
				Message: "Input mapping is recommended for every step",
				Hint:    "Provide 'input' mapping to pull values from workflow context.",
			})
		}

		if state.Output == nil {
			result.Warnings = append(result.Warnings, ValidationIssue{
				Code:    "MISSING_EXPORT_MAPPING",
				Path:    pathPrefix + "/output",
				Message: "Export/output mapping is recommended to persist step results",
				Hint:    "Use 'output' to map step outputs into workflow context.",
			})
		}

		if state.Retry == nil {
			result.Warnings = append(result.Warnings, ValidationIssue{
				Code:    "MISSING_RETRY_POLICY",
				Path:    pathPrefix + "/retry",
				Message: "Retry policy is recommended for durable execution",
				Hint:    "Add retry.max_attempts / retry.backoff to control step retries.",
			})
		}

		if state.Timeout == "" {
			result.Warnings = append(result.Warnings, ValidationIssue{
				Code:    "MISSING_TIMEOUT",
				Path:    pathPrefix + "/timeout",
				Message: "Timeout is recommended to prevent hung steps",
				Hint:    "Set timeout per step. Example: timeout: 5m",
			})
		}

		// Validate transitions when provided
		target := state.Transition
		if target == "" {
			target = state.Next
		}
		if target != "" {
			if _, ok := stateIDs[target]; !ok {
				result.Errors = append(result.Errors, ValidationIssue{
					Code:    "UNKNOWN_TRANSITION_TARGET",
					Path:    pathPrefix + "/transition",
					Message: fmt.Sprintf("Step transitions to unknown target '%s'", target),
					Actual:  target,
					Hint:    "Ensure the transition/next references an existing step id.",
				})
			}
		}
	}

	// Validate start state
	startID := def.Start
	if startID == "" && len(def.States) > 0 {
		startID = def.States[0].StableID()
		result.Warnings = append(result.Warnings, ValidationIssue{
			Code:    "DEFAULT_START",
			Path:    "/start",
			Message: fmt.Sprintf("No start state provided; defaulting to '%s'", startID),
			Hint:    "Set 'start' to the first step id to remove this warning.",
		})
	}

	if startID != "" {
		if _, ok := stateIDs[startID]; !ok {
			result.Errors = append(result.Errors, ValidationIssue{
				Code:    "INVALID_START",
				Path:    "/start",
				Message: fmt.Sprintf("Start state '%s' does not exist", startID),
				Actual:  startID,
				Hint:    "Update 'start' to reference a valid step id.",
			})
		}
	}
	def.Start = startID

	starlarkValidator := NewStarlarkValidator()
	starlarkIssues := starlarkValidator.ValidateWorkflowExpressions(&def)
	result.Errors = append(result.Errors, starlarkIssues...)

	if len(result.Errors) > 0 {
		return &def, result, ErrValidation
	}

	return &def, result, nil
}

type AgentLookup interface {
	GetAgentByNameAndEnvironment(ctx context.Context, name string, environmentID int64) (*models.Agent, error)
	GetAgentByNameGlobal(ctx context.Context, name string) (*models.Agent, error)
	GetEnvironmentIDByName(ctx context.Context, name string) (int64, error)
}

type AgentValidator struct {
	agentLookup AgentLookup
}

func NewAgentValidator(agentLookup AgentLookup) *AgentValidator {
	return &AgentValidator{agentLookup: agentLookup}
}

func (v *AgentValidator) ValidateAgents(ctx context.Context, def *Definition, environmentID int64) *ValidationResult {
	result := &ValidationResult{
		Errors:   []ValidationIssue{},
		Warnings: []ValidationIssue{},
	}

	if def == nil || len(def.States) == 0 {
		return result
	}

	agentCache := make(map[string]*models.Agent)
	agentSteps := []agentStepInfo{}

	for i, state := range def.States {
		path := fmt.Sprintf("/states/%d", i)
		v.collectAgentSteps(ctx, &state, path, environmentID, agentCache, &agentSteps, result)
	}

	v.validateSchemaCompatibility(agentSteps, def, result)

	return result
}

type agentStepInfo struct {
	stateID    string
	statePath  string
	agent      *models.Agent
	transition string
}

func (v *AgentValidator) collectAgentSteps(ctx context.Context, state *StateSpec, path string, envID int64, cache map[string]*models.Agent, steps *[]agentStepInfo, result *ValidationResult) {
	if isAgentRunStep(state) {
		agentName := extractAgentName(state)
		if agentName == "" {
			result.Errors = append(result.Errors, ValidationIssue{
				Code:    "AGENT_NAME_REQUIRED",
				Path:    path + "/input/agent",
				Message: "agent name is required for agent.run steps",
				Hint:    "Add 'agent: \"your-agent-name\"' to the step input",
			})
			return
		}

		agent, err := v.lookupAgent(ctx, agentName, envID, cache)
		if err != nil {
			// If not found, check if it's an explicit environment override which we might not fully validate yet
			// or simply report not found.
			// Ideally we should parse "agent@env" here too if we supported it fully in lookup.
			result.Errors = append(result.Errors, ValidationIssue{
				Code:    "AGENT_NOT_FOUND",
				Path:    path + "/input/agent",
				Message: fmt.Sprintf("agent '%s' not found (checked environment %d and global)", agentName, envID),
				Actual:  agentName,
				Hint:    "Ensure the agent exists in the environment or is globally accessible",
			})
			return
		}

		*steps = append(*steps, agentStepInfo{
			stateID:    state.StableID(),
			statePath:  path,
			agent:      agent,
			transition: getStateTransition(state),
		})
	}

	for i, branch := range state.Branches {
		branchPath := fmt.Sprintf("%s/branches/%d", path, i)
		for j := range branch.States {
			v.collectAgentSteps(ctx, &branch.States[j], fmt.Sprintf("%s/states/%d", branchPath, j), envID, cache, steps, result)
		}
	}

	if state.Iterator != nil {
		for j := range state.Iterator.States {
			v.collectAgentSteps(ctx, &state.Iterator.States[j], fmt.Sprintf("%s/iterator/states/%d", path, j), envID, cache, steps, result)
		}
	}
}

func (v *AgentValidator) lookupAgent(ctx context.Context, name string, envID int64, cache map[string]*models.Agent) (*models.Agent, error) {
	cacheKey := fmt.Sprintf("%s:%d", name, envID)
	if agent, exists := cache[cacheKey]; exists {
		return agent, nil
	}

	// Handle explicit environment override (e.g., "agent@env")
	if envOverride, agentName, hasOverride := parseAgentName(name); hasOverride {
		resolvedEnvID, err := v.agentLookup.GetEnvironmentIDByName(ctx, envOverride)
		if err != nil {
			return nil, fmt.Errorf("environment '%s' not found", envOverride)
		}

		agent, err := v.agentLookup.GetAgentByNameAndEnvironment(ctx, agentName, resolvedEnvID)
		if err == nil {
			cache[cacheKey] = agent
			return agent, nil
		}
		return nil, err
	}

	// 1. Try resolving by name in the current environment
	agent, err := v.agentLookup.GetAgentByNameAndEnvironment(ctx, name, envID)
	if err == nil {
		cache[cacheKey] = agent
		return agent, nil
	}

	// 2. Try global resolution (this matches runtime behavior)
	agent, err = v.agentLookup.GetAgentByNameGlobal(ctx, name)
	if err == nil {
		cache[cacheKey] = agent
		return agent, nil
	}

	return nil, err
}

func parseAgentName(input string) (string, string, bool) {
	if strings.Contains(input, "@") {
		parts := strings.SplitN(input, "@", 2)
		return parts[1], parts[0], true
	}
	return "", "", false
}

func (v *AgentValidator) validateSchemaCompatibility(steps []agentStepInfo, def *Definition, result *ValidationResult) {
	stepByID := make(map[string]*agentStepInfo)
	for i := range steps {
		stepByID[steps[i].stateID] = &steps[i]
	}

	checker := NewSchemaChecker()

	for _, step := range steps {
		if step.transition == "" {
			continue
		}

		nextStep, exists := stepByID[step.transition]
		if !exists {
			continue
		}

		if step.agent.OutputSchema == nil || *step.agent.OutputSchema == "" {
			continue
		}

		if nextStep.agent.InputSchema == nil || *nextStep.agent.InputSchema == "" {
			result.Warnings = append(result.Warnings, ValidationIssue{
				Code:    "SCHEMA_NO_INPUT",
				Path:    fmt.Sprintf("%s -> %s", step.statePath, nextStep.statePath),
				Message: fmt.Sprintf("agent '%s' has output_schema but next agent '%s' has no input_schema - cannot validate compatibility", step.agent.Name, nextStep.agent.Name),
			})
			continue
		}

		compat := checker.CheckCompatibility(*step.agent.OutputSchema, *nextStep.agent.InputSchema)

		if !compat.Compatible {
			for _, issue := range compat.Issues {
				result.Errors = append(result.Errors, ValidationIssue{
					Code:    "SCHEMA_INCOMPATIBLE",
					Path:    fmt.Sprintf("%s -> %s", step.statePath, nextStep.statePath),
					Message: issue,
					Hint:    fmt.Sprintf("Agent '%s' output schema is incompatible with agent '%s' input schema", step.agent.Name, nextStep.agent.Name),
				})
			}
		}

		for _, warning := range compat.Warnings {
			result.Warnings = append(result.Warnings, ValidationIssue{
				Code:    "SCHEMA_WARNING",
				Path:    fmt.Sprintf("%s -> %s", step.statePath, nextStep.statePath),
				Message: warning,
			})
		}
	}
}

func isAgentRunStep(state *StateSpec) bool {
	if state.Type == "agent" {
		return true
	}

	if state.Agent != "" {
		return true
	}

	if state.Type != "operation" && state.Type != "action" && state.Type != "function" {
		return false
	}

	if state.Input == nil {
		return false
	}

	task, ok := state.Input["task"].(string)
	if !ok {
		return false
	}

	return task == "agent.run" || task == "agent.hierarchy.run"
}

func extractAgentName(state *StateSpec) string {
	if state.Agent != "" {
		return state.Agent
	}

	if state.Input == nil {
		return ""
	}

	if name, ok := state.Input["agent"].(string); ok {
		return name
	}

	return ""
}

func getStateTransition(state *StateSpec) string {
	if state.Transition != "" {
		return state.Transition
	}
	return state.Next
}

func ValidateInputAgainstSchema(input map[string]interface{}, schemaJSON string) error {
	if schemaJSON == "" {
		return nil
	}

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		return fmt.Errorf("invalid schema JSON: %w", err)
	}

	required, _ := schema["required"].([]interface{})
	properties, _ := schema["properties"].(map[string]interface{})

	for _, req := range required {
		reqStr, ok := req.(string)
		if !ok {
			continue
		}

		if _, exists := input[reqStr]; !exists {
			return fmt.Errorf("missing required field: %s", reqStr)
		}
	}

	for key, value := range input {
		propDef, exists := properties[key]
		if !exists {
			continue
		}

		propMap, ok := propDef.(map[string]interface{})
		if !ok {
			continue
		}

		expectedType, _ := propMap["type"].(string)
		if expectedType != "" {
			if err := validateType(value, expectedType); err != nil {
				return fmt.Errorf("field '%s': %w", key, err)
			}
		}
	}

	return nil
}

func validateType(value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
	case "number", "integer":
		switch value.(type) {
		case float64, int, int64, float32:
		default:
			return fmt.Errorf("expected %s, got %T", expectedType, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("expected array, got %T", value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("expected object, got %T", value)
		}
	}
	return nil
}

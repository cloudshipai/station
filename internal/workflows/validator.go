package workflows

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
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

	if len(result.Errors) > 0 {
		return &def, result, ErrValidation
	}

	return &def, result, nil
}

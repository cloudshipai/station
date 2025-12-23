package workflows

import (
	"encoding/json"
	"testing"
)

func TestValidateDefinitionRequiresID(t *testing.T) {
	raw := json.RawMessage(`{"states":[{"id":"s1","type":"operation"}]}`)
	_, result, err := ValidateDefinition(raw)
	if err == nil {
		t.Fatalf("expected validation error for missing id")
	}
	if len(result.Errors) == 0 {
		t.Fatalf("expected at least one validation error for missing id")
	}
}

func TestValidateDefinitionValidatesTransitions(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "demo",
		"states": [
			{"id":"start","type":"operation","transition":"missing-target"}
		]
	}`)

	_, result, err := ValidateDefinition(raw)
	if err == nil {
		t.Fatalf("expected validation error for unknown transition target")
	}
	found := false
	for _, issue := range result.Errors {
		if issue.Code == "UNKNOWN_TRANSITION_TARGET" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected UNKNOWN_TRANSITION_TARGET error, got %+v", result.Errors)
	}
}

func TestValidateDefinitionAllowsWarnings(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "demo",
		"start": "start",
		"states": [
			{"id":"start","type":"operation","input":{},"output":{},"retry":{"max_attempts":3},"timeout":"5m","transition":"finish"},
			{"id":"finish","type":"operation","input":{},"output":{}}
		]
	}`)

	_, result, err := ValidateDefinition(raw)
	if err != nil {
		t.Fatalf("expected no validation errors, got %v (%+v)", err, result.Errors)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected zero errors, got %d", len(result.Errors))
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected warnings for optional fields")
	}
}

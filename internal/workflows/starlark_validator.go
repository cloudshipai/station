package workflows

import (
	"fmt"
	"strings"

	"go.starlark.net/syntax"
)

// StarlarkValidator validates Starlark expressions in workflow definitions
// at creation time using the official go.starlark.net/syntax.Parse() function.
// This catches syntax errors before runtime execution.
type StarlarkValidator struct{}

// NewStarlarkValidator creates a new StarlarkValidator instance.
func NewStarlarkValidator() *StarlarkValidator {
	return &StarlarkValidator{}
}

// ValidateWorkflowExpressions validates all Starlark expressions in a workflow definition.
// Returns validation issues for any expressions that fail to parse.
func (v *StarlarkValidator) ValidateWorkflowExpressions(def *Definition) []ValidationIssue {
	var issues []ValidationIssue

	if def == nil {
		return issues
	}

	for i, state := range def.States {
		path := fmt.Sprintf("/states/%d", i)
		stateIssues := v.validateStateExpressions(&state, path)
		issues = append(issues, stateIssues...)
	}

	return issues
}

// validateStateExpressions validates Starlark expressions within a single state.
func (v *StarlarkValidator) validateStateExpressions(state *StateSpec, path string) []ValidationIssue {
	var issues []ValidationIssue

	if state.Type == "transform" && state.Expression != "" {
		if err := v.validateTransformExpression(state.Expression); err != nil {
			issues = append(issues, ValidationIssue{
				Code:    "STARLARK_SYNTAX_ERROR",
				Path:    path + "/expression",
				Message: fmt.Sprintf("Invalid Starlark syntax in transform expression: %v", err),
				Actual:  truncateExpression(state.Expression, 100),
				Hint:    "Check Starlark syntax. Common issues: bare 'for' loops (use list comprehensions), nested .get() in dict literals (assign to variables first).",
			})
		}
	}

	for j, cond := range state.Conditions {
		if cond.If != "" {
			if err := v.validateConditionExpression(cond.If); err != nil {
				issues = append(issues, ValidationIssue{
					Code:    "STARLARK_SYNTAX_ERROR",
					Path:    fmt.Sprintf("%s/conditions/%d/if", path, j),
					Message: fmt.Sprintf("Invalid Starlark syntax in switch condition: %v", err),
					Actual:  truncateExpression(cond.If, 100),
					Hint:    "Ensure condition is a valid Starlark boolean expression. Use hasattr(obj, 'field') for safe field access.",
				})
			}
		}
	}

	for i, branch := range state.Branches {
		branchPath := fmt.Sprintf("%s/branches/%d", path, i)
		for j := range branch.States {
			branchIssues := v.validateStateExpressions(&branch.States[j], fmt.Sprintf("%s/states/%d", branchPath, j))
			issues = append(issues, branchIssues...)
		}
	}

	if state.Iterator != nil {
		for j := range state.Iterator.States {
			iterIssues := v.validateStateExpressions(&state.Iterator.States[j], fmt.Sprintf("%s/iterator/states/%d", path, j))
			issues = append(issues, iterIssues...)
		}
	}

	if state.Try != nil {
		for j := range state.Try.States {
			tryIssues := v.validateStateExpressions(&state.Try.States[j], fmt.Sprintf("%s/try/states/%d", path, j))
			issues = append(issues, tryIssues...)
		}
	}
	if state.Catch != nil {
		for j := range state.Catch.States {
			catchIssues := v.validateStateExpressions(&state.Catch.States[j], fmt.Sprintf("%s/catch/states/%d", path, j))
			issues = append(issues, catchIssues...)
		}
	}
	if state.Finally != nil {
		for j := range state.Finally.States {
			finallyIssues := v.validateStateExpressions(&state.Finally.States[j], fmt.Sprintf("%s/finally/states/%d", path, j))
			issues = append(issues, finallyIssues...)
		}
	}

	return issues
}

// validateTransformExpression validates a transform expression by wrapping it
// the same way the runtime does and parsing it with syntax.Parse().
func (v *StarlarkValidator) validateTransformExpression(expression string) error {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil
	}

	wrapped := wrapForValidation(expression)
	_, err := syntax.Parse("transform.star", wrapped, 0)
	if err != nil {
		return simplifyStarlarkError(err)
	}

	return nil
}

// validateConditionExpression validates a switch condition expression.
// Conditions are boolean expressions, wrapped as: __result__ = (expression)
func (v *StarlarkValidator) validateConditionExpression(expression string) error {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil
	}

	wrapped := fmt.Sprintf("__result__ = (%s)", expression)
	_, err := syntax.Parse("condition.star", wrapped, 0)
	if err != nil {
		return simplifyStarlarkError(err)
	}

	return nil
}

// wrapForValidation wraps an expression the same way the runtime transform executor does.
// This ensures validation matches runtime behavior.
func wrapForValidation(expression string) string {
	expression = strings.TrimSpace(expression)

	if strings.Contains(expression, "__result__") {
		return expression
	}

	lines := strings.Split(expression, "\n")

	if len(lines) == 1 && !hasControlFlow(expression) {
		return fmt.Sprintf("__result__ = %s", expression)
	}

	if hasControlFlow(expression) {
		return wrapInFunctionForValidation(lines)
	}

	return wrapMultilineForValidation(lines)
}

// hasControlFlow checks if expression has top-level control flow statements.
func hasControlFlow(expression string) bool {
	for _, line := range strings.Split(expression, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			if strings.HasPrefix(trimmed, "for ") ||
				strings.HasPrefix(trimmed, "while ") ||
				strings.HasPrefix(trimmed, "if ") {
				return true
			}
		}
	}
	return false
}

// wrapInFunctionForValidation wraps expression in a function for validation.
func wrapInFunctionForValidation(lines []string) string {
	var result strings.Builder
	result.WriteString("def __transform__():\n")

	lastMeaningfulIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lastMeaningfulIdx = i
			break
		}
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			result.WriteString("\n")
			continue
		}

		result.WriteString("    ")

		if i == lastMeaningfulIdx {
			if isReturnableExpr(trimmed) {
				result.WriteString("return ")
				result.WriteString(trimmed)
				result.WriteString("\n")
				continue
			}
		}

		result.WriteString(line)
		result.WriteString("\n")
	}

	result.WriteString("__result__ = __transform__()\n")

	return result.String()
}

// wrapMultilineForValidation wraps a simple multi-line expression for validation.
func wrapMultilineForValidation(lines []string) string {
	lastMeaningfulIdx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			lastMeaningfulIdx = i
			break
		}
	}

	if lastMeaningfulIdx == -1 {
		return strings.Join(lines, "\n")
	}

	lastLine := strings.TrimSpace(lines[lastMeaningfulIdx])

	if isAssignmentExpr(lastLine) {
		varName := extractAssignedVarName(lastLine)
		lines = append(lines, fmt.Sprintf("__result__ = %s", varName))
		return strings.Join(lines, "\n")
	}

	lines[lastMeaningfulIdx] = fmt.Sprintf("__result__ = %s", lastLine)
	return strings.Join(lines, "\n")
}

// isReturnableExpr checks if line is a returnable expression (dict, list, or pure expression).
func isReturnableExpr(line string) bool {
	if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
		return true
	}
	if !strings.Contains(line, "=") || strings.Contains(line, "==") ||
		strings.Contains(line, "!=") || strings.Contains(line, "<=") ||
		strings.Contains(line, ">=") {
		if !strings.HasPrefix(line, "def ") &&
			!strings.HasPrefix(line, "if ") &&
			!strings.HasPrefix(line, "for ") &&
			!strings.HasPrefix(line, "while ") &&
			!strings.HasPrefix(line, "return ") &&
			!strings.HasPrefix(line, "pass") &&
			!strings.HasPrefix(line, "break") &&
			!strings.HasPrefix(line, "continue") {
			return true
		}
	}
	return false
}

// isAssignmentExpr checks if line is an assignment (not comparison).
func isAssignmentExpr(line string) bool {
	return strings.Contains(line, "=") &&
		!strings.Contains(line, "==") &&
		!strings.Contains(line, "!=") &&
		!strings.Contains(line, "<=") &&
		!strings.Contains(line, ">=")
}

// extractAssignedVarName extracts the variable name from an assignment.
func extractAssignedVarName(line string) string {
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return ""
	}
	varName := strings.TrimSpace(parts[0])
	if idx := strings.Index(varName, "["); idx != -1 {
		varName = strings.TrimSpace(varName[:idx])
	}
	return varName
}

// simplifyStarlarkError extracts the core error message from syntax.Parse errors.
func simplifyStarlarkError(err error) error {
	if err == nil {
		return nil
	}
	errStr := err.Error()
	if idx := strings.Index(errStr, ": "); idx != -1 {
		rest := errStr[idx+2:]
		if idx2 := strings.Index(rest, ": "); idx2 != -1 {
			return fmt.Errorf("%s", rest[idx2+2:])
		}
	}
	return err
}

func truncateExpression(expr string, maxLen int) string {
	expr = strings.ReplaceAll(expr, "\n", " ")
	expr = strings.Join(strings.Fields(expr), " ")
	if len(expr) > maxLen {
		return expr[:maxLen-3] + "..."
	}
	return expr
}

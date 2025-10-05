package validation

import "fmt"

// Standard CloudShip app_subtype values per AGENT_DATA_STANDARDIZATION.md
// These 5 subtypes work across ALL domains (finops, security, deployments, custom)
var ValidAppSubtypes = []string{
	"investigations", // Root cause analysis, incident investigation, anomaly detection
	"opportunities",  // Cost savings, optimization suggestions, improvement recommendations
	"projections",    // Forecasting, trend predictions, future state modeling
	"inventory",      // Resource cataloging, asset tracking, configuration snapshots
	"events",         // Activity logs, deployment history, change tracking
}

// ValidateAppSubtype checks if the app_subtype is one of the 5 standard CloudShip types
func ValidateAppSubtype(appSubtype string) error {
	if appSubtype == "" {
		return nil // Empty is allowed (optional field)
	}

	for _, valid := range ValidAppSubtypes {
		if appSubtype == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid app_subtype '%s': must be one of: %v", appSubtype, ValidAppSubtypes)
}

// ValidateAppAndSubtype validates the combination of app and app_subtype per CloudShip rules:
// 1. app_subtype can only be set if app is also set
// 2. If app is set, app_subtype should be one of the 5 standard types (recommended but not required)
// 3. app can be anything (flexible, user-defined domain like finops, security, deployments, custom)
func ValidateAppAndSubtype(app, appSubtype string) error {
	// If neither is set, that's valid (optional fields)
	if app == "" && appSubtype == "" {
		return nil
	}

	// If app_subtype is set without app, that's invalid
	if app == "" && appSubtype != "" {
		return fmt.Errorf("app_subtype requires app to be set")
	}

	// If both are set, validate app_subtype
	if app != "" && appSubtype != "" {
		return ValidateAppSubtype(appSubtype)
	}

	// app is set but app_subtype is not - this is valid but suboptimal for CloudShip routing
	// We allow it but could log a warning
	return nil
}

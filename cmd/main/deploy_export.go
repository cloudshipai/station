package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"station/cmd/main/handlers"
)

var deployExportVarsCmd = &cobra.Command{
	Use:   "export-vars <environment>",
	Short: "Export deployment variables as template",
	Long: `Export environment deployment variables in YAML or env format for CI/CD secret stores.

This command generates a template of all required variables for deploying a Station environment.
Use this to set up secrets in your CI/CD pipeline (GitHub Actions, GitLab CI, etc.).

The output includes:
  - AI provider credentials (STN_AI_API_KEY, STN_AI_PROVIDER, STN_AI_MODEL)
  - CloudShip configuration (if enabled)
  - Environment-specific variables from variables.yml
  - Telemetry configuration

Values are masked for security. Replace masked values with actual secrets in your CI/CD system.`,
	Example: `  # Export as YAML for GitHub Actions secrets
  stn deploy export-vars default --format yaml > deploy-vars.yml

  # Export as .env file
  stn deploy export-vars default --format env > .env.deploy

  # Export to stdout for review
  stn deploy export-vars production`,
	Args: cobra.ExactArgs(1),
	RunE: runDeployExportVars,
}

func init() {
	deployExportVarsCmd.Flags().String("format", "yaml", "Output format (yaml, env)")
}

func runDeployExportVars(cmd *cobra.Command, args []string) error {
	envName := args[0]
	format, _ := cmd.Flags().GetString("format")

	aiConfig, err := handlers.DetectAIConfigForExport()
	if err != nil {
		return fmt.Errorf("failed to detect AI config: %w", err)
	}

	cloudShipConfig := handlers.DetectCloudShipConfigForExport()
	telemetryConfig := handlers.DetectTelemetryConfigForExport()

	envConfig, err := handlers.LoadEnvironmentConfigForExport(envName)
	if err != nil {
		return fmt.Errorf("failed to load environment config: %w", err)
	}

	vars := buildDeployVars(envName, aiConfig, cloudShipConfig, telemetryConfig, envConfig)

	switch strings.ToLower(format) {
	case "yaml":
		return outputYAML(vars)
	case "env":
		return outputEnv(vars)
	default:
		return fmt.Errorf("unsupported format: %s (use yaml or env)", format)
	}
}

func buildDeployVars(envName string, aiConfig *handlers.DeploymentAIConfig, cloudShipConfig *handlers.DeploymentCloudShipConfig, telemetryConfig *handlers.DeploymentTelemetryConfig, envConfig *handlers.EnvironmentConfig) map[string]string {
	vars := make(map[string]string)

	vars["STN_AI_PROVIDER"] = aiConfig.Provider
	vars["STN_AI_MODEL"] = aiConfig.Model

	if aiConfig.AuthType == "oauth" {
		vars["STN_AI_AUTH_TYPE"] = "oauth"
		vars["STN_AI_OAUTH_TOKEN"] = "***MASKED***"
		vars["STN_AI_OAUTH_REFRESH_TOKEN"] = "***MASKED***"
	} else {
		vars["STN_AI_API_KEY"] = "***MASKED***"
		switch strings.ToLower(aiConfig.Provider) {
		case "openai":
			vars["OPENAI_API_KEY"] = "***MASKED***"
		case "anthropic":
			vars["ANTHROPIC_API_KEY"] = "***MASKED***"
		case "google", "gemini":
			vars["GOOGLE_API_KEY"] = "***MASKED***"
		}
	}

	if cloudShipConfig != nil && cloudShipConfig.Enabled {
		vars["STN_CLOUDSHIP_ENABLED"] = "true"
		vars["STN_CLOUDSHIP_KEY"] = "***MASKED***"
		if cloudShipConfig.Name != "" {
			vars["STN_CLOUDSHIP_NAME"] = cloudShipConfig.Name
		}
		if cloudShipConfig.Endpoint != "" {
			vars["STN_CLOUDSHIP_ENDPOINT"] = cloudShipConfig.Endpoint
		}
	}

	if telemetryConfig != nil && telemetryConfig.Enabled {
		vars["STN_TELEMETRY_ENABLED"] = "true"
		vars["STN_TELEMETRY_PROVIDER"] = telemetryConfig.Provider
		if telemetryConfig.Endpoint != "" {
			vars["STN_TELEMETRY_ENDPOINT"] = telemetryConfig.Endpoint
		}
	}

	vars["STATION_ENCRYPTION_KEY"] = "***GENERATE_NEW***"

	for k, v := range envConfig.Variables {
		if strings.Contains(strings.ToLower(k), "key") ||
			strings.Contains(strings.ToLower(k), "token") ||
			strings.Contains(strings.ToLower(k), "secret") ||
			strings.Contains(strings.ToLower(k), "password") {
			vars[k] = "***MASKED***"
		} else {
			vars[k] = v
		}
	}

	return vars
}

func outputYAML(vars map[string]string) error {
	output := make(map[string]interface{})
	output["deploy_variables"] = vars
	output["_comment"] = "Replace ***MASKED*** values with actual secrets. Generate new STATION_ENCRYPTION_KEY for each deployment."

	data, err := yaml.Marshal(output)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

func outputEnv(vars map[string]string) error {
	fmt.Println("# Station Deployment Variables")
	fmt.Println("# Replace ***MASKED*** values with actual secrets")
	fmt.Println("# Generate new STATION_ENCRYPTION_KEY for each deployment")
	fmt.Println()

	for k, v := range vars {
		fmt.Printf("%s=%s\n", k, v)
	}

	return nil
}

func initDeploySubcommands() {
	deployCmd.AddCommand(deployExportVarsCmd)
}

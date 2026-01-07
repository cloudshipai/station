package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"station/cmd/main/handlers"
	"station/internal/config"
)

var deployExportVarsCmd = &cobra.Command{
	Use:   "export-vars [environment]",
	Short: "Export deployment variables as template",
	Long: `Export deployment variables in YAML or env format for CI/CD secret stores.

This command generates a template of all required variables for deploying a Station environment
or bundle. Use this to set up secrets in your CI/CD pipeline or secrets backend.

The output includes:
  - AI provider credentials (STN_AI_API_KEY, STN_AI_PROVIDER, STN_AI_MODEL)
  - CloudShip configuration (if enabled)
  - Environment-specific variables from variables.yml
  - MCP server required variables (introspected from configs)
  - Target-specific instructions (Fly.io, K8s, Ansible)

Values are masked for security. Replace masked values with actual secrets.`,
	Example: `  # Export vars for local environment
  stn deploy export-vars default --format yaml
  stn deploy export-vars default --format env > secrets.env

  # Export vars for bundle deployment
  stn deploy export-vars --bundle-id e26b414a-f076-4135-927f-810bc1dc892a

  # Export with target-specific instructions
  stn deploy export-vars default --target fly
  stn deploy export-vars default --target k8s
  stn deploy export-vars default --target ansible`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDeployExportVars,
}

func init() {
	deployExportVarsCmd.Flags().String("format", "yaml", "Output format (yaml, env, fly, k8s, ansible)")
	deployExportVarsCmd.Flags().String("bundle-id", "", "CloudShip bundle ID (alternative to environment name)")
	deployExportVarsCmd.Flags().String("target", "", "Show target-specific instructions (fly, k8s, ansible)")
}

func runDeployExportVars(cmd *cobra.Command, args []string) error {
	envName := ""
	if len(args) > 0 {
		envName = args[0]
	}
	bundleID, _ := cmd.Flags().GetString("bundle-id")
	format, _ := cmd.Flags().GetString("format")
	target, _ := cmd.Flags().GetString("target")

	if envName == "" && bundleID == "" {
		return fmt.Errorf("either environment name or --bundle-id is required")
	}
	if envName != "" && bundleID != "" {
		return fmt.Errorf("cannot specify both environment name and --bundle-id")
	}

	aiConfig, err := handlers.DetectAIConfigForExport()
	if err != nil {
		return fmt.Errorf("failed to detect AI config: %w", err)
	}

	cloudShipConfig := handlers.DetectCloudShipConfigForExport()
	telemetryConfig := handlers.DetectTelemetryConfigForExport()

	var envConfig *handlers.EnvironmentConfig
	var mcpVars map[string]string

	if bundleID != "" {
		envConfig = &handlers.EnvironmentConfig{
			Name:      bundleID,
			Variables: map[string]string{"STN_BUNDLE_ID": bundleID},
		}
		mcpVars = map[string]string{}
		fmt.Fprintf(os.Stderr, "📦 Bundle: %s\n", bundleID)
		fmt.Fprintf(os.Stderr, "   Note: Bundle variables will be loaded at runtime\n\n")
	} else {
		envConfig, err = handlers.LoadEnvironmentConfigForExport(envName)
		if err != nil {
			return fmt.Errorf("failed to load environment config: %w", err)
		}
		mcpVars = introspectMCPVariables(envConfig.Path)
		fmt.Fprintf(os.Stderr, "📦 Environment: %s\n", envName)
		if len(mcpVars) > 0 {
			fmt.Fprintf(os.Stderr, "   Found %d MCP server variables\n", len(mcpVars))
		}
		fmt.Fprintln(os.Stderr)
	}

	vars := buildDeployVars(aiConfig, cloudShipConfig, telemetryConfig, envConfig, mcpVars)

	if target != "" {
		return outputWithTargetInstructions(vars, target, bundleID)
	}

	switch strings.ToLower(format) {
	case "yaml":
		return outputYAML(vars)
	case "env":
		return outputEnv(vars)
	case "fly":
		return outputWithTargetInstructions(vars, "fly", bundleID)
	case "k8s", "kubernetes":
		return outputWithTargetInstructions(vars, "k8s", bundleID)
	case "ansible":
		return outputWithTargetInstructions(vars, "ansible", bundleID)
	default:
		return fmt.Errorf("unsupported format: %s (use yaml, env, fly, k8s, ansible)", format)
	}
}

func introspectMCPVariables(envPath string) map[string]string {
	vars := make(map[string]string)
	if envPath == "" {
		return vars
	}

	mcpDir := filepath.Join(envPath, "mcp")
	entries, err := os.ReadDir(mcpDir)
	if err != nil {
		return vars
	}

	envVarPattern := regexp.MustCompile(`\$\{?([A-Z][A-Z0-9_]*)\}?`)

	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yml") && !strings.HasSuffix(entry.Name(), ".yaml")) {
			continue
		}

		data, err := os.ReadFile(filepath.Join(mcpDir, entry.Name()))
		if err != nil {
			continue
		}

		matches := envVarPattern.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			if len(match) > 1 {
				varName := match[1]
				if _, exists := vars[varName]; !exists {
					serverName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
					vars[varName] = fmt.Sprintf("Required by: %s", serverName)
				}
			}
		}
	}

	return vars
}

func buildDeployVars(aiConfig *handlers.DeploymentAIConfig, cloudShipConfig *handlers.DeploymentCloudShipConfig, telemetryConfig *handlers.DeploymentTelemetryConfig, envConfig *handlers.EnvironmentConfig, mcpVars map[string]string) map[string]string {
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
		if isSensitiveKey(k) {
			vars[k] = "***MASKED***"
		} else {
			vars[k] = v
		}
	}

	for k, description := range mcpVars {
		if _, exists := vars[k]; !exists {
			vars[k] = fmt.Sprintf("***MASKED*** (%s)", description)
		}
	}

	return vars
}

func isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	sensitivePatterns := []string{"key", "token", "secret", "password", "credential", "auth"}
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerKey, pattern) {
			return true
		}
	}
	return false
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

	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Printf("%s=%s\n", k, vars[k])
	}

	return nil
}

func outputWithTargetInstructions(vars map[string]string, target, bundleID string) error {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	switch strings.ToLower(target) {
	case "fly", "flyio":
		fmt.Println("# Fly.io Secrets")
		fmt.Println("# Run these commands to set secrets (Fly manages them securely)")
		fmt.Println()
		if bundleID != "" {
			fmt.Printf("fly secrets set STN_BUNDLE_ID=%s \\\n", bundleID)
		}
		for i, k := range keys {
			if k == "STN_BUNDLE_ID" {
				continue
			}
			v := vars[k]
			if strings.Contains(v, "***") {
				v = "<your-value>"
			}
			if i < len(keys)-1 {
				fmt.Printf("  %s=%s \\\n", k, v)
			} else {
				fmt.Printf("  %s=%s\n", k, v)
			}
		}
		fmt.Println()
		fmt.Println("# Or set interactively:")
		fmt.Println("# fly secrets set STN_AI_API_KEY")

	case "k8s", "kubernetes":
		fmt.Println("# Kubernetes Secrets")
		fmt.Println("# Option 1: Runtime secrets backend (recommended)")
		fmt.Println("#   Container fetches secrets at startup from Vault/AWS/GCP")
		fmt.Println()
		fmt.Println("# Minimal manifest with runtime secrets:")
		fmt.Println("# env:")
		if bundleID != "" {
			fmt.Printf("#   - name: STN_BUNDLE_ID\n#     value: \"%s\"\n", bundleID)
		}
		fmt.Println("#   - name: STN_SECRETS_BACKEND")
		fmt.Println("#     value: \"vault\"  # or aws-secretsmanager, aws-ssm, gcp-secretmanager")
		fmt.Println("#   - name: STN_SECRETS_PATH")
		fmt.Println("#     value: \"secret/data/station/prod\"")
		fmt.Println()
		fmt.Println("# Option 2: External Secrets Operator")
		fmt.Println("#   Use ESO to sync secrets to K8s Secret objects")
		fmt.Println()
		fmt.Println("# Required secrets in your backend:")
		for _, k := range keys {
			if k == "STN_BUNDLE_ID" {
				continue
			}
			v := vars[k]
			if strings.Contains(v, "***") {
				fmt.Printf("#   %s: <required>\n", k)
			} else {
				fmt.Printf("#   %s: %s\n", k, v)
			}
		}

	case "ansible":
		fmt.Println("# Ansible Variables")
		fmt.Println("# Add these to your vars file or use ansible-vault for secrets")
		fmt.Println()
		fmt.Println("# vars/station.yml (encrypt with ansible-vault)")
		fmt.Println("---")
		if bundleID != "" {
			fmt.Printf("stn_bundle_id: \"%s\"\n", bundleID)
		}
		for _, k := range keys {
			if k == "STN_BUNDLE_ID" {
				continue
			}
			v := vars[k]
			yamlKey := strings.ToLower(strings.ReplaceAll(k, "_", "_"))
			if strings.Contains(v, "***") {
				fmt.Printf("%s: \"<your-value>\"  # %s\n", yamlKey, k)
			} else {
				fmt.Printf("%s: \"%s\"  # %s\n", yamlKey, v, k)
			}
		}
		fmt.Println()
		fmt.Println("# Encrypt with: ansible-vault encrypt vars/station.yml")
		fmt.Println("# Run with: ansible-playbook -i hosts playbook.yml --ask-vault-pass")

	default:
		return fmt.Errorf("unsupported target: %s (use fly, k8s, ansible)", target)
	}

	return nil
}

func initDeploySubcommands() {
	deployCmd.AddCommand(deployExportVarsCmd)
}

func getStationConfigDir() string {
	return config.GetStationConfigDir()
}

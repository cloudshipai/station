package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var bundleExportVarsCmd = &cobra.Command{
	Use:   "export-vars <bundle-source>",
	Short: "Export required variables from a bundle",
	Long: `Export deployment variables required by a bundle in YAML or env format.

This command analyzes a bundle and extracts all template variables ({{.VAR_NAME}})
used in MCP configurations, plus standard deployment variables.

The bundle source can be:
  - CloudShip Bundle ID (UUID format): Downloads and analyzes from CloudShip
  - File path: Analyzes a local bundle file (.tar.gz)

Use this to set up secrets in your CI/CD pipeline (GitHub Actions, GitLab CI, etc.)
before deploying a Station with this bundle.

Examples:
  # Export vars from CloudShip bundle as YAML
  stn bundle export-vars e26b414a-f076-4135-927f-810bc1dc892a --format yaml

  # Export vars from local bundle as .env
  stn bundle export-vars ./my-bundle.tar.gz --format env

  # Include standard deployment vars (AI config, CloudShip, telemetry)
  stn bundle export-vars ./my-bundle.tar.gz --include-deploy-vars`,
	Args: cobra.ExactArgs(1),
	RunE: runBundleExportVars,
}

func init() {
	bundleExportVarsCmd.Flags().String("format", "yaml", "Output format (yaml, env)")
	bundleExportVarsCmd.Flags().Bool("include-deploy-vars", true, "Include standard deployment variables (AI, CloudShip, telemetry)")

	bundleCmd.AddCommand(bundleExportVarsCmd)
}

func runBundleExportVars(cmd *cobra.Command, args []string) error {
	bundleSource := args[0]
	format, _ := cmd.Flags().GetString("format")
	includeDeployVars, _ := cmd.Flags().GetBool("include-deploy-vars")

	var bundlePath string
	var cleanup func()

	if isUUID(bundleSource) {
		fmt.Fprintf(os.Stderr, "Downloading bundle from CloudShip: %s\n", bundleSource)

		downloadedPath, err := downloadBundleFromCloudShip(bundleSource)
		if err != nil {
			return fmt.Errorf("failed to download bundle from CloudShip: %w", err)
		}
		bundlePath = downloadedPath
		cleanup = func() { os.Remove(downloadedPath) }
		fmt.Fprintf(os.Stderr, "Bundle downloaded successfully\n")
	} else {
		if _, err := os.Stat(bundleSource); os.IsNotExist(err) {
			return fmt.Errorf("bundle file not found: %s", bundleSource)
		}
		bundlePath = bundleSource
		cleanup = func() {}
	}
	defer cleanup()

	vars, err := extractBundleVariables(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to analyze bundle: %w", err)
	}

	output := make(map[string]string)

	for _, v := range vars {
		output[v] = "***REQUIRED***"
	}

	if includeDeployVars {
		deployVars := getStandardDeployVars()
		for k, v := range deployVars {
			output[k] = v
		}
	}

	switch format {
	case "yaml":
		return outputBundleVarsYAML(output, vars)
	case "env":
		return outputBundleVarsEnv(output, vars)
	default:
		return fmt.Errorf("unknown format: %s (use 'yaml' or 'env')", format)
	}
}

func extractBundleVariables(bundlePath string) ([]string, error) {
	file, err := os.Open(bundlePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to open gzip: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	// Regex pattern for Go template variables: {{.VAR_NAME}}
	varPattern := regexp.MustCompile(`\{\{\s*\.([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

	varsMap := make(map[string]bool)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar: %w", err)
		}

		if header.Typeflag == tar.TypeReg && strings.HasSuffix(header.Name, ".json") {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				continue
			}

			matches := varPattern.FindAllStringSubmatch(string(content), -1)
			for _, match := range matches {
				if len(match) > 1 {
					varsMap[match[1]] = true
				}
			}
		}
	}

	vars := make([]string, 0, len(varsMap))
	for v := range varsMap {
		vars = append(vars, v)
	}
	sort.Strings(vars)

	return vars, nil
}

func getStandardDeployVars() map[string]string {
	return map[string]string{
		"STN_AI_PROVIDER":        "anthropic",
		"STN_AI_MODEL":           "claude-sonnet-4-20250514",
		"STN_AI_API_KEY":         "***MASKED***",
		"STN_AI_AUTH_TYPE":       "api_key",
		"STN_CLOUDSHIP_ENABLED":  "true",
		"STN_CLOUDSHIP_KEY":      "***MASKED***",
		"STN_CLOUDSHIP_ENDPOINT": "lighthouse.cloudshipai.com:443",
		"STN_CLOUDSHIP_NAME":     "my-station",
		"WORKSPACE_PATH":         "/app/workspace",
		"STATION_ENCRYPTION_KEY": "***GENERATE_NEW***",
	}
}

func outputBundleVarsYAML(output map[string]string, bundleVars []string) error {
	type YAMLOutput struct {
		Comment        string            `yaml:"_comment"`
		BundleVars     map[string]string `yaml:"bundle_variables,omitempty"`
		DeploymentVars map[string]string `yaml:"deployment_variables"`
	}

	bundleVarsMap := make(map[string]string)
	deployVarsMap := make(map[string]string)

	bundleVarsSet := make(map[string]bool)
	for _, v := range bundleVars {
		bundleVarsSet[v] = true
	}

	keys := make([]string, 0, len(output))
	for k := range output {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := output[k]
		if bundleVarsSet[k] {
			bundleVarsMap[k] = v
		} else {
			deployVarsMap[k] = v
		}
	}

	yamlOutput := YAMLOutput{
		Comment:        "Replace ***MASKED*** and ***REQUIRED*** values with actual secrets. Generate new STATION_ENCRYPTION_KEY for each deployment.",
		BundleVars:     bundleVarsMap,
		DeploymentVars: deployVarsMap,
	}

	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	return encoder.Encode(yamlOutput)
}

func outputBundleVarsEnv(output map[string]string, bundleVars []string) error {
	fmt.Println("# Station Bundle Variables")
	fmt.Println("# Replace ***MASKED*** and ***REQUIRED*** values with actual secrets")
	fmt.Println("# Generate new STATION_ENCRYPTION_KEY for each deployment")
	fmt.Println()

	bundleVarsSet := make(map[string]bool)
	for _, v := range bundleVars {
		bundleVarsSet[v] = true
	}

	keys := make([]string, 0, len(output))
	for k := range output {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(bundleVars) > 0 {
		fmt.Println("# Bundle-specific variables")
		for _, k := range keys {
			if bundleVarsSet[k] {
				fmt.Printf("%s=%s\n", k, output[k])
			}
		}
		fmt.Println()
	}

	fmt.Println("# Deployment variables")
	for _, k := range keys {
		if !bundleVarsSet[k] {
			fmt.Printf("%s=%s\n", k, output[k])
		}
	}

	return nil
}

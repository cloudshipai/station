package deployment

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"gopkg.in/yaml.v2"
)

//go:embed templates/cli.sh
var cliTemplate string

//go:embed templates/github-actions.yml
var githubActionsTemplate string

//go:embed templates/aws-ecs.yml
var awsECSTemplate string

//go:embed templates/gcp-cloudrun.yml
var gcpCloudRunTemplate string

//go:embed templates/fly.toml
var flyTemplate string

//go:embed templates/docker-compose.yml
var dockerComposeTemplate string

//go:embed templates/wrangler.toml
var wranglerTemplate string

//go:embed templates/cloudflare-worker.js
var cloudflareWorkerTemplate string

type DeploymentConfig struct {
	EnvironmentName      string
	DockerImage          string
	APIPort              string
	MCPPort              string
	SSHPort              string
	AIProvider           string
	AIModel              string
	Debug                string
	TelemetryEnabled     string
	OpenAIAPIKey         string
	FlyRegion            string
	FlyAlwaysOn          bool
	EnvironmentVariables map[string]string

	CloudflareInstanceType string
	CloudflareSleepAfter   string
	CloudflareMaxInstances int
}

// GenerateDeploymentTemplate generates a deployment template for the specified provider
func GenerateDeploymentTemplate(provider string, config DeploymentConfig) (string, error) {
	var tmplStr string
	switch provider {
	case "cli":
		tmplStr = cliTemplate
	case "github-actions":
		tmplStr = githubActionsTemplate
	case "aws-ecs":
		tmplStr = awsECSTemplate
	case "gcp-cloudrun":
		tmplStr = gcpCloudRunTemplate
	case "fly":
		tmplStr = flyTemplate
	case "docker-compose":
		tmplStr = dockerComposeTemplate
	case "cloudflare", "wrangler":
		tmplStr = wranglerTemplate
	case "cloudflare-worker":
		tmplStr = cloudflareWorkerTemplate
	default:
		return "", fmt.Errorf("unsupported deployment provider: %s", provider)
	}

	tmpl, err := template.New("deployment").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// LoadConfigFromYAML loads Station config.yaml and converts it to DeploymentConfig
func LoadConfigFromYAML(configContent string, envVarsContent string, environmentName string, dockerImage string) (*DeploymentConfig, error) {
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(configContent), &config); err != nil {
		return nil, fmt.Errorf("failed to parse config.yaml: %w", err)
	}

	var envVars map[string]string
	if envVarsContent != "" {
		var rawVars map[string]interface{}
		if err := yaml.Unmarshal([]byte(envVarsContent), &rawVars); err != nil {
			return nil, fmt.Errorf("failed to parse variables.yml: %w", err)
		}
		envVars = make(map[string]string)
		for k, v := range rawVars {
			envVars[k] = fmt.Sprintf("%v", v)
		}
	} else {
		envVars = make(map[string]string)
	}

	deployConfig := &DeploymentConfig{
		EnvironmentName:      environmentName,
		DockerImage:          dockerImage,
		APIPort:              fmt.Sprintf("%v", getConfigValue(config, "api_port", 8585)),
		MCPPort:              fmt.Sprintf("%v", getConfigValue(config, "mcp_port", 3000)),
		SSHPort:              fmt.Sprintf("%v", getConfigValue(config, "ssh_port", 2222)),
		AIProvider:           fmt.Sprintf("%v", getConfigValue(config, "ai_provider", "openai")),
		AIModel:              fmt.Sprintf("%v", getConfigValue(config, "ai_model", "gpt-5-mini")),
		Debug:                fmt.Sprintf("%v", getConfigValue(config, "debug", false)),
		TelemetryEnabled:     fmt.Sprintf("%v", getConfigValue(config, "telemetry_enabled", true)),
		OpenAIAPIKey:         "<your-openai-api-key>",
		FlyRegion:            "ord",
		EnvironmentVariables: envVars,

		CloudflareInstanceType: "basic",
		CloudflareSleepAfter:   "10m",
		CloudflareMaxInstances: 1,
	}

	return deployConfig, nil
}

func getConfigValue(config map[string]interface{}, key string, defaultValue interface{}) interface{} {
	if val, ok := config[key]; ok {
		return val
	}
	return defaultValue
}

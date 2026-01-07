package config

type FieldType string

const (
	FieldTypeString      FieldType = "string"
	FieldTypeInt         FieldType = "int"
	FieldTypeBool        FieldType = "bool"
	FieldTypeStringSlice FieldType = "[]string"
)

// ShowWhenCondition defines when a field should be visible
type ShowWhenCondition struct {
	Field  string   `json:"field"`            // The field to check
	Values []string `json:"values,omitempty"` // Show when field equals any of these values
}

type ConfigField struct {
	Key         string             `json:"key"`
	Type        FieldType          `json:"type"`
	Description string             `json:"description"`
	Default     interface{}        `json:"default,omitempty"`
	Required    bool               `json:"required,omitempty"`
	Secret      bool               `json:"secret,omitempty"`
	Section     string             `json:"section"`
	Options     []string           `json:"options,omitempty"`
	ShowWhen    *ShowWhenCondition `json:"showWhen,omitempty"` // Conditional visibility
}

type ConfigSection struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Order       int    `json:"order"`
}

var ConfigSections = []ConfigSection{
	{Name: "ai", Description: "AI Provider Settings", Order: 1},
	{Name: "coding", Description: "Coding Backend Settings", Order: 2},
	{Name: "cloudship", Description: "CloudShip Integration", Order: 3},
	{Name: "lattice", Description: "Lattice Mesh Network", Order: 4},
	{Name: "telemetry", Description: "Telemetry & Observability", Order: 5},
	{Name: "sandbox", Description: "Sandbox Execution", Order: 6},
	{Name: "webhook", Description: "Webhook Settings", Order: 7},
	{Name: "notifications", Description: "Notification Settings", Order: 8},
	{Name: "server", Description: "Server & Port Settings", Order: 9},
}

var ConfigSchema = []ConfigField{
	// AI Provider Settings
	{Key: "ai_provider", Type: FieldTypeString, Description: "AI provider (openai, anthropic, ollama, gemini)", Default: "openai", Section: "ai", Options: []string{"openai", "anthropic", "ollama", "gemini"}},
	{Key: "ai_model", Type: FieldTypeString, Description: "AI model name (e.g., gpt-4o, claude-sonnet-4-20250514)", Default: "gpt-4o", Section: "ai"},
	{Key: "ai_api_key", Type: FieldTypeString, Description: "API key for the AI provider", Secret: true, Section: "ai"},
	{Key: "ai_base_url", Type: FieldTypeString, Description: "Base URL for OpenAI-compatible endpoints", Section: "ai"},

	// Coding Backend Settings - Common
	{Key: "coding.backend", Type: FieldTypeString, Description: "Coding backend (opencode, opencode-nats, opencode-cli, claudecode)", Default: "opencode-cli", Section: "coding", Options: []string{"opencode", "opencode-nats", "opencode-cli", "claudecode"}},
	{Key: "coding.workspace_base_path", Type: FieldTypeString, Description: "Base path for coding workspaces (defaults to /workspaces/station-coding on Fly.io, /tmp/station-coding otherwise)", Default: "", Section: "coding"},
	{Key: "coding.max_attempts", Type: FieldTypeInt, Description: "Maximum retry attempts for coding tasks", Default: 3, Section: "coding"},
	{Key: "coding.task_timeout_min", Type: FieldTypeInt, Description: "Task timeout in minutes", Default: 30, Section: "coding"},

	// OpenCode HTTP Settings (backend: opencode)
	{Key: "coding.opencode.url", Type: FieldTypeString, Description: "OpenCode HTTP server URL", Default: "http://localhost:4096", Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"opencode"}}},

	// OpenCode NATS Settings (backend: opencode-nats)
	{Key: "coding.nats.url", Type: FieldTypeString, Description: "NATS server URL", Default: "nats://127.0.0.1:4222", Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"opencode-nats"}}},
	{Key: "coding.nats.subjects.task", Type: FieldTypeString, Description: "NATS subject for task requests", Default: "station.coding.task", Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"opencode-nats"}}},
	{Key: "coding.nats.subjects.result", Type: FieldTypeString, Description: "NATS subject for task results", Default: "station.coding.result", Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"opencode-nats"}}},
	{Key: "coding.nats.subjects.stream", Type: FieldTypeString, Description: "NATS subject for streaming output", Default: "station.coding.stream", Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"opencode-nats"}}},

	// OpenCode CLI Settings (backend: opencode-cli)
	{Key: "coding.cli.binary_path", Type: FieldTypeString, Description: "Path to opencode binary", Default: "opencode", Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"opencode-cli"}}},
	{Key: "coding.cli.timeout_sec", Type: FieldTypeInt, Description: "CLI command timeout in seconds", Default: 300, Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"opencode-cli"}}},

	// Claude Code Settings (backend: claudecode)
	{Key: "coding.claudecode.binary_path", Type: FieldTypeString, Description: "Path to claude binary", Default: "claude", Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"claudecode"}}},
	{Key: "coding.claudecode.timeout_sec", Type: FieldTypeInt, Description: "Claude Code timeout in seconds", Default: 300, Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"claudecode"}}},
	{Key: "coding.claudecode.model", Type: FieldTypeString, Description: "Claude model (sonnet, opus, haiku)", Section: "coding", Options: []string{"sonnet", "opus", "haiku"}, ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"claudecode"}}},
	{Key: "coding.claudecode.max_turns", Type: FieldTypeInt, Description: "Maximum conversation turns", Default: 10, Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"claudecode"}}},
	{Key: "coding.claudecode.allowed_tools", Type: FieldTypeStringSlice, Description: "Allowed tools whitelist", Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"claudecode"}}},
	{Key: "coding.claudecode.disallowed_tools", Type: FieldTypeStringSlice, Description: "Disallowed tools blacklist", Section: "coding", ShowWhen: &ShowWhenCondition{Field: "coding.backend", Values: []string{"claudecode"}}},

	// Git Settings (all coding backends)
	{Key: "coding.git.token_env", Type: FieldTypeString, Description: "Environment variable for GitHub token", Default: "GITHUB_TOKEN", Section: "coding"},
	{Key: "coding.git.user_name", Type: FieldTypeString, Description: "Git commit author name", Default: "Station Bot", Section: "coding"},
	{Key: "coding.git.user_email", Type: FieldTypeString, Description: "Git commit author email", Default: "station@cloudship.ai", Section: "coding"},

	// CloudShip Integration
	{Key: "cloudship.enabled", Type: FieldTypeBool, Description: "Enable CloudShip integration", Default: false, Section: "cloudship"},
	{Key: "cloudship.api_key", Type: FieldTypeString, Description: "CloudShip personal API key (cst_...)", Secret: true, Section: "cloudship"},
	{Key: "cloudship.registration_key", Type: FieldTypeString, Description: "CloudShip registration key for Station management", Secret: true, Section: "cloudship"},
	{Key: "cloudship.endpoint", Type: FieldTypeString, Description: "Lighthouse gRPC endpoint", Default: "lighthouse.cloudshipai.com:443", Section: "cloudship"},
	{Key: "cloudship.use_tls", Type: FieldTypeBool, Description: "Use TLS for gRPC connection", Default: true, Section: "cloudship"},
	{Key: "cloudship.name", Type: FieldTypeString, Description: "Station name (unique across org)", Section: "cloudship"},
	{Key: "cloudship.tags", Type: FieldTypeStringSlice, Description: "Station tags for filtering", Section: "cloudship"},

	// Lattice Mesh Settings
	{Key: "lattice.station_name", Type: FieldTypeString, Description: "Station name in the lattice mesh", Section: "lattice"},
	{Key: "lattice.nats.url", Type: FieldTypeString, Description: "NATS URL to join lattice (e.g., nats://orchestrator:4222)", Section: "lattice"},
	{Key: "lattice.nats.auth.token", Type: FieldTypeString, Description: "NATS authentication token", Secret: true, Section: "lattice"},
	{Key: "lattice.nats.auth.user", Type: FieldTypeString, Description: "NATS username", Section: "lattice"},
	{Key: "lattice.nats.auth.password", Type: FieldTypeString, Description: "NATS password", Secret: true, Section: "lattice"},
	{Key: "lattice_orchestration", Type: FieldTypeBool, Description: "Run as lattice orchestrator with embedded NATS", Default: false, Section: "lattice"},
	{Key: "lattice.orchestrator.embedded_nats.port", Type: FieldTypeInt, Description: "Embedded NATS port", Default: 4222, Section: "lattice"},
	{Key: "lattice.orchestrator.embedded_nats.http_port", Type: FieldTypeInt, Description: "Embedded NATS monitoring port", Default: 8222, Section: "lattice"},
	{Key: "lattice.orchestrator.embedded_nats.auth.enabled", Type: FieldTypeBool, Description: "Enable auth for embedded NATS", Default: false, Section: "lattice"},
	{Key: "lattice.orchestrator.embedded_nats.auth.token", Type: FieldTypeString, Description: "Auth token for embedded NATS", Secret: true, Section: "lattice"},

	// Telemetry Settings
	{Key: "telemetry.enabled", Type: FieldTypeBool, Description: "Enable telemetry/tracing", Default: true, Section: "telemetry"},
	{Key: "telemetry.provider", Type: FieldTypeString, Description: "Telemetry provider (none, jaeger, otlp, cloudship)", Default: "jaeger", Section: "telemetry", Options: []string{"none", "jaeger", "otlp", "cloudship"}},
	{Key: "telemetry.endpoint", Type: FieldTypeString, Description: "OTLP endpoint URL", Default: "http://localhost:4318", Section: "telemetry"},

	// Sandbox Settings
	{Key: "sandbox.enabled", Type: FieldTypeBool, Description: "Enable sandbox code execution", Default: false, Section: "sandbox"},
	{Key: "sandbox.code_mode_enabled", Type: FieldTypeBool, Description: "Enable code mode in sandbox", Default: false, Section: "sandbox"},
	{Key: "sandbox.idle_timeout_minutes", Type: FieldTypeInt, Description: "Sandbox idle timeout in minutes", Default: 30, Section: "sandbox"},
	{Key: "sandbox.docker_image", Type: FieldTypeString, Description: "Custom Docker image for sandbox containers", Default: "ubuntu:22.04", Section: "sandbox"},
	{Key: "sandbox.registry_auth.username", Type: FieldTypeString, Description: "Registry username for private images", Section: "sandbox"},
	{Key: "sandbox.registry_auth.password", Type: FieldTypeString, Description: "Registry password or access token", Secret: true, Section: "sandbox"},
	{Key: "sandbox.registry_auth.identity_token", Type: FieldTypeString, Description: "OAuth bearer token (ECR, GCR, ACR)", Secret: true, Section: "sandbox"},
	{Key: "sandbox.registry_auth.server_address", Type: FieldTypeString, Description: "Registry server URL (e.g., ghcr.io)", Section: "sandbox"},
	{Key: "sandbox.registry_auth.docker_config_path", Type: FieldTypeString, Description: "Path to Docker config.json", Section: "sandbox"},

	// Webhook Settings
	{Key: "webhook.enabled", Type: FieldTypeBool, Description: "Enable webhook execute endpoint", Default: true, Section: "webhook"},
	{Key: "webhook.api_key", Type: FieldTypeString, Description: "Static API key for webhook auth", Secret: true, Section: "webhook"},

	// Notification Settings
	{Key: "notifications.approval_webhook_url", Type: FieldTypeString, Description: "URL to POST when approval is needed", Section: "notifications"},
	{Key: "notifications.approval_webhook_timeout", Type: FieldTypeInt, Description: "Webhook timeout in seconds", Default: 10, Section: "notifications"},
	{Key: "notify.webhook_url", Type: FieldTypeString, Description: "URL for agent notifications (e.g., ntfy.sh)", Section: "notifications"},
	{Key: "notify.format", Type: FieldTypeString, Description: "Notification format (ntfy, json, auto)", Default: "ntfy", Section: "notifications", Options: []string{"ntfy", "json", "auto"}},

	// Server Settings
	{Key: "api_port", Type: FieldTypeInt, Description: "API server port", Default: 8585, Section: "server"},
	{Key: "mcp_port", Type: FieldTypeInt, Description: "MCP server port", Default: 8586, Section: "server"},
	{Key: "debug", Type: FieldTypeBool, Description: "Enable debug mode", Default: false, Section: "server"},
	{Key: "workspace", Type: FieldTypeString, Description: "Custom workspace path", Section: "server"},
}

func GetConfigSchema() []ConfigField {
	return ConfigSchema
}

func GetConfigSections() []ConfigSection {
	return ConfigSections
}

func GetFieldByKey(key string) *ConfigField {
	for _, field := range ConfigSchema {
		if field.Key == key {
			return &field
		}
	}
	return nil
}

func GetFieldsBySection(section string) []ConfigField {
	var fields []ConfigField
	for _, field := range ConfigSchema {
		if field.Section == section {
			fields = append(fields, field)
		}
	}
	return fields
}

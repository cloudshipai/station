package deploy

// DeploymentAIConfig holds AI configuration for deployment
type DeploymentAIConfig struct {
	Provider          string
	Model             string
	APIKey            string
	AuthType          string
	OAuthToken        string
	OAuthRefreshToken string
	OAuthExpiresAt    int64
}

// DeploymentCloudShipConfig holds CloudShip configuration for deployment
type DeploymentCloudShipConfig struct {
	Enabled         bool
	RegistrationKey string
	Name            string
	Endpoint        string
	UseTLS          bool
}

// DeploymentTelemetryConfig holds telemetry configuration for deployment
type DeploymentTelemetryConfig struct {
	Enabled  bool
	Provider string
	Endpoint string
}

// EnvironmentConfig holds the loaded environment configuration
type EnvironmentConfig struct {
	Name      string
	Path      string
	Variables map[string]string
	Template  map[string]interface{}
	Agents    []string
}

// RuntimeSecretsConfig holds configuration for runtime secrets backends
type RuntimeSecretsConfig struct {
	Backend string // aws-secretsmanager, aws-ssm, vault, gcp-secretmanager, sops
	Path    string // Path in the secrets backend
}

// BaseStationImage is the default Station Docker image
const BaseStationImage = "ghcr.io/cloudshipai/station:latest"

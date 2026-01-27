package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"station/internal/config"
	"station/internal/deployment"
	_ "station/internal/deployment/secrets"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage secrets backends for runtime secret injection",
	Long: `Configure and test secrets backends that Station uses to fetch secrets at runtime.

Supported backends:
  - aws-secretsmanager: AWS Secrets Manager
  - aws-ssm: AWS Systems Manager Parameter Store  
  - vault: HashiCorp Vault
  - gcp-secretmanager: Google Cloud Secret Manager
  - sops: SOPS encrypted files

When configured, Station fetches secrets at startup and injects them into
the config and environment. This is useful for:
  - Kubernetes deployments (no secrets in manifests)
  - GitHub Actions (use OIDC instead of storing secrets)
  - Any deployment where you want centralized secret management`,
}

var secretsSetCmd = &cobra.Command{
	Use:   "set <backend> <path>",
	Short: "Configure secrets backend",
	Long: `Configure a secrets backend for runtime secret injection.

Examples:
  stn secrets set aws-secretsmanager station/prod
  stn secrets set aws-secretsmanager station/prod --region us-east-1
  stn secrets set aws-ssm /station/prod/
  stn secrets set vault secret/data/station/prod --vault-addr https://vault.example.com
  stn secrets set gcp-secretmanager projects/my-project/secrets/station-prod`,
	Args: cobra.ExactArgs(2),
	RunE: runSecretsSet,
}

var secretsTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test connection to configured secrets backend",
	Long: `Validate that Station can connect to and fetch secrets from the configured backend.

This command:
  1. Checks if a secrets backend is configured
  2. Validates credentials/connectivity
  3. Attempts to fetch secrets
  4. Shows the number of secrets found (not values)`,
	RunE: runSecretsTest,
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available secret keys (not values)",
	Long:  `List the keys available in the configured secrets backend without showing values.`,
	RunE:  runSecretsList,
}

var secretsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current secrets backend configuration",
	RunE:  runSecretsShow,
}

var secretsExportGitHubCmd = &cobra.Command{
	Use:   "export-github",
	Short: "Export secrets configuration for GitHub Actions",
	Long: `Generate GitHub Actions workflow snippet for using secrets backend.

This outputs the workflow YAML you need to add to use runtime secrets
instead of GitHub Secrets. Includes OIDC setup for AWS.`,
	RunE: runSecretsExportGitHub,
}

var secretsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear secrets backend configuration",
	RunE:  runSecretsClear,
}

func init() {
	secretsSetCmd.Flags().String("region", "", "AWS region (for aws-secretsmanager, aws-ssm)")
	secretsSetCmd.Flags().String("vault-addr", "", "Vault server address (for vault)")
	secretsSetCmd.Flags().String("vault-token", "", "Vault token (for vault, optional - uses VAULT_TOKEN if not set)")

	secretsCmd.AddCommand(secretsSetCmd)
	secretsCmd.AddCommand(secretsTestCmd)
	secretsCmd.AddCommand(secretsListCmd)
	secretsCmd.AddCommand(secretsShowCmd)
	secretsCmd.AddCommand(secretsExportGitHubCmd)
	secretsCmd.AddCommand(secretsClearCmd)
}

func runSecretsSet(cmd *cobra.Command, args []string) error {
	backend := args[0]
	path := args[1]

	validBackends := []string{"aws-secretsmanager", "aws-ssm", "vault", "gcp-secretmanager", "sops"}
	valid := false
	for _, b := range validBackends {
		if backend == b {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid backend: %s\nSupported: %s", backend, strings.Join(validBackends, ", "))
	}

	region, _ := cmd.Flags().GetString("region")
	vaultAddr, _ := cmd.Flags().GetString("vault-addr")
	vaultToken, _ := cmd.Flags().GetString("vault-token")

	viper.Set("secrets.backend", backend)
	viper.Set("secrets.path", path)

	if region != "" {
		viper.Set("secrets.region", region)
	}
	if vaultAddr != "" {
		viper.Set("secrets.vault_addr", vaultAddr)
	}
	if vaultToken != "" {
		viper.Set("secrets.vault_token", vaultToken)
	}

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✅ Secrets backend configured:\n")
	fmt.Printf("   Backend: %s\n", backend)
	fmt.Printf("   Path: %s\n", path)
	if region != "" {
		fmt.Printf("   Region: %s\n", region)
	}
	if vaultAddr != "" {
		fmt.Printf("   Vault Addr: %s\n", vaultAddr)
	}
	fmt.Println()
	fmt.Println("Run 'stn secrets test' to verify the configuration.")

	return nil
}

func runSecretsTest(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Secrets.Backend == "" {
		return fmt.Errorf("no secrets backend configured\nRun 'stn secrets set <backend> <path>' first")
	}

	fmt.Printf("🔐 Testing secrets backend: %s\n", cfg.Secrets.Backend)
	fmt.Printf("   Path: %s\n", cfg.Secrets.Path)
	if cfg.Secrets.Region != "" {
		fmt.Printf("   Region: %s\n", cfg.Secrets.Region)
	}
	fmt.Println()

	provider, ok := deployment.GetSecretProvider(cfg.Secrets.Backend)
	if !ok {
		return fmt.Errorf("unknown secrets backend: %s", cfg.Secrets.Backend)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Print("   Validating credentials... ")
	if err := provider.Validate(ctx); err != nil {
		fmt.Println("❌")
		return fmt.Errorf("validation failed: %w", err)
	}
	fmt.Println("✅")

	fmt.Print("   Fetching secrets... ")
	secrets, err := provider.GetSecrets(ctx, cfg.Secrets.Path)
	if err != nil {
		fmt.Println("❌")
		return fmt.Errorf("failed to fetch secrets: %w", err)
	}
	fmt.Println("✅")

	fmt.Printf("\n✅ Successfully connected! Found %d secrets.\n", len(secrets))

	if len(secrets) > 0 {
		fmt.Println("\nAvailable keys:")
		keys := make([]string, 0, len(secrets))
		for k := range secrets {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("   • %s\n", k)
		}
	}

	return nil
}

func runSecretsList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Secrets.Backend == "" {
		return fmt.Errorf("no secrets backend configured")
	}

	provider, ok := deployment.GetSecretProvider(cfg.Secrets.Backend)
	if !ok {
		return fmt.Errorf("unknown secrets backend: %s", cfg.Secrets.Backend)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	secrets, err := provider.GetSecrets(ctx, cfg.Secrets.Path)
	if err != nil {
		return fmt.Errorf("failed to fetch secrets: %w", err)
	}

	if len(secrets) == 0 {
		fmt.Println("No secrets found at path:", cfg.Secrets.Path)
		return nil
	}

	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		fmt.Println(k)
	}

	return nil
}

func runSecretsShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Secrets.Backend == "" {
		fmt.Println("No secrets backend configured.")
		fmt.Println()
		fmt.Println("Configure one with:")
		fmt.Println("  stn secrets set aws-secretsmanager station/prod")
		fmt.Println("  stn secrets set vault secret/data/station/prod")
		return nil
	}

	fmt.Println("Secrets Backend Configuration:")
	fmt.Printf("  Backend: %s\n", cfg.Secrets.Backend)
	fmt.Printf("  Path: %s\n", cfg.Secrets.Path)
	if cfg.Secrets.Region != "" {
		fmt.Printf("  Region: %s\n", cfg.Secrets.Region)
	}
	if cfg.Secrets.VaultAddr != "" {
		fmt.Printf("  Vault Addr: %s\n", cfg.Secrets.VaultAddr)
	}

	return nil
}

func runSecretsExportGitHub(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Secrets.Backend == "" {
		return fmt.Errorf("no secrets backend configured\nRun 'stn secrets set <backend> <path>' first")
	}

	fmt.Println("# GitHub Actions workflow snippet for secrets backend")
	fmt.Println("# Add this to your workflow file")
	fmt.Println()

	switch cfg.Secrets.Backend {
	case "aws-secretsmanager", "aws-ssm":
		region := cfg.Secrets.Region
		if region == "" {
			region = os.Getenv("AWS_REGION")
		}
		if region == "" {
			region = "us-east-1"
		}

		fmt.Println("# 1. First, configure AWS OIDC (one-time setup in AWS):")
		fmt.Println("#    - Create an IAM OIDC provider for token.actions.githubusercontent.com")
		fmt.Println("#    - Create an IAM role with trust policy for your repo")
		fmt.Println("#    - Grant the role access to your secrets")
		fmt.Println()
		fmt.Println("# 2. Add these permissions to your job:")
		fmt.Println("permissions:")
		fmt.Println("  id-token: write")
		fmt.Println("  contents: read")
		fmt.Println()
		fmt.Println("# 3. Add these steps before station-action:")
		fmt.Println("steps:")
		fmt.Println("  - uses: aws-actions/configure-aws-credentials@v4")
		fmt.Println("    with:")
		fmt.Println("      role-to-assume: arn:aws:iam::YOUR_ACCOUNT_ID:role/YOUR_ROLE_NAME")
		fmt.Printf("      aws-region: %s\n", region)
		fmt.Println()
		fmt.Println("  - uses: cloudshipai/station-action@main")
		fmt.Println("    with:")
		fmt.Println("      agent: 'Your Agent'")
		fmt.Println("      task: 'Your task'")
		fmt.Printf("      secrets-backend: %s\n", cfg.Secrets.Backend)
		fmt.Printf("      secrets-path: %s\n", cfg.Secrets.Path)
		if region != "" {
			fmt.Printf("      secrets-region: %s\n", region)
		}
		fmt.Println("    # No API keys needed! Secrets fetched from AWS at runtime")

	case "vault":
		fmt.Println("# Add these steps to your workflow:")
		fmt.Println("steps:")
		fmt.Println("  - uses: hashicorp/vault-action@v2")
		fmt.Println("    with:")
		fmt.Println("      url: https://vault.example.com")
		fmt.Println("      method: jwt")
		fmt.Println("      role: your-vault-role")
		fmt.Println("      exportEnv: false")
		fmt.Println()
		fmt.Println("  - uses: cloudshipai/station-action@main")
		fmt.Println("    with:")
		fmt.Println("      agent: 'Your Agent'")
		fmt.Println("      task: 'Your task'")
		fmt.Printf("      secrets-backend: %s\n", cfg.Secrets.Backend)
		fmt.Printf("      secrets-path: %s\n", cfg.Secrets.Path)
		fmt.Println("    env:")
		fmt.Println("      VAULT_ADDR: ${{ env.VAULT_ADDR }}")
		fmt.Println("      VAULT_TOKEN: ${{ env.VAULT_TOKEN }}")

	case "gcp-secretmanager":
		fmt.Println("# 1. Configure Workload Identity Federation in GCP")
		fmt.Println()
		fmt.Println("# 2. Add these permissions to your job:")
		fmt.Println("permissions:")
		fmt.Println("  id-token: write")
		fmt.Println("  contents: read")
		fmt.Println()
		fmt.Println("# 3. Add these steps:")
		fmt.Println("steps:")
		fmt.Println("  - uses: google-github-actions/auth@v2")
		fmt.Println("    with:")
		fmt.Println("      workload_identity_provider: projects/YOUR_PROJECT/locations/global/workloadIdentityPools/YOUR_POOL/providers/YOUR_PROVIDER")
		fmt.Println("      service_account: YOUR_SA@YOUR_PROJECT.iam.gserviceaccount.com")
		fmt.Println()
		fmt.Println("  - uses: cloudshipai/station-action@main")
		fmt.Println("    with:")
		fmt.Println("      agent: 'Your Agent'")
		fmt.Println("      task: 'Your task'")
		fmt.Printf("      secrets-backend: %s\n", cfg.Secrets.Backend)
		fmt.Printf("      secrets-path: %s\n", cfg.Secrets.Path)

	default:
		fmt.Println("  - uses: cloudshipai/station-action@main")
		fmt.Println("    with:")
		fmt.Println("      agent: 'Your Agent'")
		fmt.Println("      task: 'Your task'")
		fmt.Printf("      secrets-backend: %s\n", cfg.Secrets.Backend)
		fmt.Printf("      secrets-path: %s\n", cfg.Secrets.Path)
	}

	return nil
}

func runSecretsClear(cmd *cobra.Command, args []string) error {
	viper.Set("secrets.backend", "")
	viper.Set("secrets.path", "")
	viper.Set("secrets.region", "")
	viper.Set("secrets.vault_addr", "")
	viper.Set("secrets.vault_token", "")

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("✅ Secrets backend configuration cleared.")
	return nil
}

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"station/internal/provider"
)

var providerCmd = &cobra.Command{
	Use:   "provider",
	Short: "Manage AI provider authentication",
	Long:  `Manage authentication for AI providers like Anthropic (Claude), OpenAI, etc.`,
}

var providerLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with an AI provider",
	Long: `Authenticate with an AI provider using OAuth or API key.

For Anthropic (Claude Max/Pro subscription):
  stn provider login --anthropic
  
This will open your browser to authenticate with your Claude subscription
and create an API key that uses your subscription quota (not pay-per-token).`,
	RunE: runProviderLogin,
}

var providerStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status for AI providers",
	RunE:  runProviderStatus,
}

var providerLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored credentials for an AI provider",
	RunE:  runProviderLogout,
}

func init() {
	providerLoginCmd.Flags().Bool("anthropic", false, "Login with Anthropic (Claude Max/Pro subscription)")
	providerLoginCmd.Flags().Bool("openai", false, "Login with OpenAI (coming soon)")

	providerLogoutCmd.Flags().Bool("anthropic", false, "Logout from Anthropic")
	providerLogoutCmd.Flags().Bool("all", false, "Logout from all providers")

	providerCmd.AddCommand(providerLoginCmd)
	providerCmd.AddCommand(providerStatusCmd)
	providerCmd.AddCommand(providerLogoutCmd)
}

func runProviderLogin(cmd *cobra.Command, args []string) error {
	anthropic, _ := cmd.Flags().GetBool("anthropic")
	openai, _ := cmd.Flags().GetBool("openai")

	if !anthropic && !openai {
		return fmt.Errorf("please specify a provider: --anthropic or --openai")
	}

	if openai {
		return fmt.Errorf("OpenAI OAuth login not yet implemented. Use OPENAI_API_KEY environment variable")
	}

	if anthropic {
		return loginAnthropic()
	}

	return nil
}

func loginAnthropic() error {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          Anthropic Claude Max/Pro OAuth Login                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("This will authenticate with your Claude subscription and create")
	fmt.Println("an API key that uses your subscription quota (not pay-per-token).")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	oauth := provider.NewAnthropicOAuth()
	creds, err := oauth.Login(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	store, err := provider.LoadProviderAuth()
	if err != nil {
		return fmt.Errorf("failed to load auth store: %w", err)
	}

	if err := store.SetCredentials(creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    Authentication Successful!                 ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	if creds.Email != "" {
		fmt.Printf("  Account: %s\n", creds.Email)
	}
	fmt.Printf("  API Key: %s...%s\n", creds.APIKey[:12], creds.APIKey[len(creds.APIKey)-4:])
	fmt.Println()
	fmt.Println("Your Claude subscription is now configured for Station.")
	fmt.Println("Run 'stn agent run' to test it!")
	fmt.Println()

	return nil
}

func runProviderStatus(cmd *cobra.Command, args []string) error {
	store, err := provider.LoadProviderAuth()
	if err != nil {
		return fmt.Errorf("failed to load auth store: %w", err)
	}

	fmt.Println()
	fmt.Println("AI Provider Authentication Status")
	fmt.Println("══════════════════════════════════")
	fmt.Println()

	providers := []provider.ProviderType{
		provider.ProviderAnthropic,
		provider.ProviderOpenAI,
		provider.ProviderGoogle,
	}

	hasAny := false
	for _, p := range providers {
		creds := store.GetCredentials(p)
		if creds != nil && creds.APIKey != "" {
			hasAny = true
			status := "✓ Authenticated"
			if creds.IsExpired() {
				status = "⚠ Token Expired (refresh needed)"
			}

			fmt.Printf("  %s: %s\n", p, status)
			if creds.Email != "" {
				fmt.Printf("    Account: %s\n", creds.Email)
			}
			fmt.Printf("    API Key: %s...%s\n", creds.APIKey[:min(12, len(creds.APIKey))], creds.APIKey[max(0, len(creds.APIKey)-4):])
			fmt.Printf("    Auth Type: %s\n", creds.AuthType)
			fmt.Printf("    Updated: %s\n", creds.UpdatedAt.Format("2006-01-02 15:04:05"))
			fmt.Println()
		}
	}

	if !hasAny {
		fmt.Println("  No providers authenticated via OAuth.")
		fmt.Println()
		fmt.Println("  You can still use environment variables:")
		fmt.Println("    - ANTHROPIC_API_KEY for Claude")
		fmt.Println("    - OPENAI_API_KEY for OpenAI")
		fmt.Println("    - GOOGLE_API_KEY for Gemini")
		fmt.Println()
		fmt.Println("  Or login with your Claude subscription:")
		fmt.Println("    stn provider login --anthropic")
	}
	fmt.Println()

	return nil
}

func runProviderLogout(cmd *cobra.Command, args []string) error {
	anthropic, _ := cmd.Flags().GetBool("anthropic")
	all, _ := cmd.Flags().GetBool("all")

	if !anthropic && !all {
		return fmt.Errorf("please specify a provider: --anthropic or --all")
	}

	store, err := provider.LoadProviderAuth()
	if err != nil {
		return fmt.Errorf("failed to load auth store: %w", err)
	}

	if all {
		store.Providers = make(map[provider.ProviderType]*provider.ProviderCredentials)
		if err := store.Save(); err != nil {
			return fmt.Errorf("failed to save: %w", err)
		}
		fmt.Println("Logged out from all providers.")
		return nil
	}

	if anthropic {
		if err := store.RemoveCredentials(provider.ProviderAnthropic); err != nil {
			return fmt.Errorf("failed to remove Anthropic credentials: %w", err)
		}
		fmt.Println("Logged out from Anthropic.")
	}

	return nil
}

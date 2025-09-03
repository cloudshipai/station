package handlers

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"station/internal/db"
	"station/internal/db/repositories"
	"station/internal/theme"
	"station/pkg/models"
)

// EnvironmentHandler handles environment-related CLI commands
type EnvironmentHandler struct {
	themeManager *theme.ThemeManager
}

func NewEnvironmentHandler(themeManager *theme.ThemeManager) *EnvironmentHandler {
	return &EnvironmentHandler{themeManager: themeManager}
}


// RunEnvList lists all environments
func (h *EnvironmentHandler) RunEnvList(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🌍 Environments")
	fmt.Println(banner)

	fmt.Println(styles.Info.Render("🏠 Listing local environments"))
	return h.listEnvironmentsLocal()
}

// RunEnvCreate creates a new environment
func (h *EnvironmentHandler) RunEnvCreate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment name is required")
	}

	name := args[0]
	description, _ := cmd.Flags().GetString("description")

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🌍 Create Environment")
	fmt.Println(banner)

	fmt.Println(styles.Info.Render("🏠 Creating local environment"))
	return h.createEnvironmentLocal(name, description)
}

// RunEnvGet gets environment details
func (h *EnvironmentHandler) RunEnvGet(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment name or ID is required")
	}

	identifier := args[0]

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🌍 Environment Details")
	fmt.Println(banner)

	fmt.Println(styles.Info.Render("🏠 Getting local environment"))
	return h.getEnvironmentLocal(identifier)
}

// RunEnvUpdate updates an environment
func (h *EnvironmentHandler) RunEnvUpdate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment name or ID is required")
	}

	identifier := args[0]
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")

	if name == "" && description == "" {
		return fmt.Errorf("at least one of --name or --description must be provided")
	}

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🌍 Update Environment")
	fmt.Println(banner)

	fmt.Println(styles.Info.Render("🏠 Updating local environment"))
	return h.updateEnvironmentLocal(identifier, name, description)
}

// RunEnvDelete deletes an environment
func (h *EnvironmentHandler) RunEnvDelete(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment name or ID is required")
	}

	identifier := args[0]
	confirm, _ := cmd.Flags().GetBool("confirm")

	if !confirm {
		fmt.Printf("⚠️  This will permanently delete the environment '%s' and all associated data.\n", identifier)
		fmt.Printf("Use --confirm flag to proceed.\n")
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🌍 Delete Environment")
	fmt.Println(banner)

	fmt.Println(styles.Info.Render("🏠 Deleting local environment"))
	return h.deleteEnvironmentLocal(identifier)
}

// Local operations

func (h *EnvironmentHandler) listEnvironmentsLocal() error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	environments, err := repos.Environments.List()
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	if len(environments) == 0 {
		fmt.Println("• No environments found")
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Found %d environment(s):\n", len(environments))
	for _, env := range environments {
		fmt.Printf("• %s (ID: %d)", styles.Success.Render(env.Name), env.ID)
		if env.Description != nil {
			fmt.Printf(" - %s", *env.Description)
		}
		fmt.Printf(" [Created: %s]\n", env.CreatedAt.Format("Jan 2, 2006 15:04"))
	}

	return nil
}

func (h *EnvironmentHandler) createEnvironmentLocal(name, description string) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	
	// Get console user for created_by field
	consoleUser, err := repos.Users.GetByUsername("console")
	if err != nil {
		return fmt.Errorf("failed to get console user: %w", err)
	}
	
	var desc *string
	if description != "" {
		desc = &description
	}

	env, err := repos.Environments.Create(name, desc, consoleUser.ID)
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("✅ Environment created: %s (ID: %d)\n", styles.Success.Render(env.Name), env.ID)
	return nil
}

func (h *EnvironmentHandler) getEnvironmentLocal(identifier string) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	
	var env *models.Environment
	
	// Try as ID first, then as name
	if id, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		env, err = repos.Environments.GetByID(id)
		if err != nil {
			return fmt.Errorf("environment with ID %d not found", id)
		}
	} else {
		env, err = repos.Environments.GetByName(identifier)
		if err != nil {
			return fmt.Errorf("environment '%s' not found", identifier)
		}
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Environment: %s\n", styles.Success.Render(env.Name))
	fmt.Printf("ID: %d\n", env.ID)
	if env.Description != nil {
		fmt.Printf("Description: %s\n", *env.Description)
	}
	fmt.Printf("Created: %s\n", env.CreatedAt.Format("Jan 2, 2006 15:04"))
	fmt.Printf("Updated: %s\n", env.UpdatedAt.Format("Jan 2, 2006 15:04"))

	return nil
}

func (h *EnvironmentHandler) updateEnvironmentLocal(identifier, name, description string) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)
	
	// Find environment ID
	var envID int64
	if id, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		envID = id
	} else {
		env, err := repos.Environments.GetByName(identifier)
		if err != nil {
			return fmt.Errorf("environment '%s' not found", identifier)
		}
		envID = env.ID
	}

	// Use existing values if not provided
	if name == "" {
		env, err := repos.Environments.GetByID(envID)
		if err != nil {
			return fmt.Errorf("failed to get environment: %w", err)
		}
		name = env.Name
	}

	var desc *string
	if description != "" {
		desc = &description
	}

	err = repos.Environments.Update(envID, name, desc)
	if err != nil {
		return fmt.Errorf("failed to update environment: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("✅ Environment updated: %s\n", styles.Success.Render(name))
	return nil
}

func (h *EnvironmentHandler) deleteEnvironmentLocal(identifier string) error {
	cfg, err := loadStationConfig()
	if err != nil {
		return fmt.Errorf("failed to load Station config: %w", err)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.Close()

	repos := repositories.New(database)

	// Find environment ID
	var envID int64
	var envName string
	if id, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		env, err := repos.Environments.GetByID(id)
		if err != nil {
			return fmt.Errorf("environment with ID %d not found", id)
		}
		envID = id
		envName = env.Name
	} else {
		env, err := repos.Environments.GetByName(identifier)
		if err != nil {
			return fmt.Errorf("environment '%s' not found", identifier)
		}
		envID = env.ID
		envName = env.Name
	}

	err = repos.Environments.Delete(envID)
	if err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("✅ Environment deleted: %s\n", styles.Success.Render(envName))
	return nil
}


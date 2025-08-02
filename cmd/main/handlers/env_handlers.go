package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

// getAPIKeyFromEnv gets the API key from environment variable
func getAPIKeyFromEnv() string {
	return os.Getenv("STATION_API_KEY")
}

// makeAuthenticatedRequest creates an HTTP request with authentication header if available
func makeAuthenticatedRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	
	// Add authentication header if available
	if apiKey := getAPIKeyFromEnv(); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	return req, nil
}

// RunEnvList lists all environments
func (h *EnvironmentHandler) RunEnvList(cmd *cobra.Command, args []string) error {
	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🌍 Environments")
	fmt.Println(banner)

	endpoint, _ := cmd.Flags().GetString("endpoint")

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Listing environments from: " + endpoint))
		return h.listEnvironmentsRemote(endpoint)
	} else {
		fmt.Println(styles.Info.Render("🏠 Listing local environments"))
		return h.listEnvironmentsLocal()
	}
}

// RunEnvCreate creates a new environment
func (h *EnvironmentHandler) RunEnvCreate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment name is required")
	}

	name := args[0]
	description, _ := cmd.Flags().GetString("description")
	endpoint, _ := cmd.Flags().GetString("endpoint")

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🌍 Create Environment")
	fmt.Println(banner)

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Creating environment on: " + endpoint))
		return h.createEnvironmentRemote(name, description, endpoint)
	} else {
		fmt.Println(styles.Info.Render("🏠 Creating local environment"))
		return h.createEnvironmentLocal(name, description)
	}
}

// RunEnvGet gets environment details
func (h *EnvironmentHandler) RunEnvGet(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment name or ID is required")
	}

	identifier := args[0]
	endpoint, _ := cmd.Flags().GetString("endpoint")

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🌍 Environment Details")
	fmt.Println(banner)

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Getting environment from: " + endpoint))
		return h.getEnvironmentRemote(identifier, endpoint)
	} else {
		fmt.Println(styles.Info.Render("🏠 Getting local environment"))
		return h.getEnvironmentLocal(identifier)
	}
}

// RunEnvUpdate updates an environment
func (h *EnvironmentHandler) RunEnvUpdate(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment name or ID is required")
	}

	identifier := args[0]
	name, _ := cmd.Flags().GetString("name")
	description, _ := cmd.Flags().GetString("description")
	endpoint, _ := cmd.Flags().GetString("endpoint")

	if name == "" && description == "" {
		return fmt.Errorf("at least one of --name or --description must be provided")
	}

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🌍 Update Environment")
	fmt.Println(banner)

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Updating environment on: " + endpoint))
		return h.updateEnvironmentRemote(identifier, name, description, endpoint)
	} else {
		fmt.Println(styles.Info.Render("🏠 Updating local environment"))
		return h.updateEnvironmentLocal(identifier, name, description)
	}
}

// RunEnvDelete deletes an environment
func (h *EnvironmentHandler) RunEnvDelete(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("environment name or ID is required")
	}

	identifier := args[0]
	endpoint, _ := cmd.Flags().GetString("endpoint")
	confirm, _ := cmd.Flags().GetBool("confirm")

	if !confirm {
		fmt.Printf("⚠️  This will permanently delete the environment '%s' and all associated data.\n", identifier)
		fmt.Printf("Use --confirm flag to proceed.\n")
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	banner := styles.Banner.Render("🌍 Delete Environment")
	fmt.Println(banner)

	if endpoint != "" {
		fmt.Println(styles.Info.Render("🌐 Deleting environment from: " + endpoint))
		return h.deleteEnvironmentRemote(identifier, endpoint)
	} else {
		fmt.Println(styles.Info.Render("🏠 Deleting local environment"))
		return h.deleteEnvironmentLocal(identifier)
	}
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

// Remote operations

func (h *EnvironmentHandler) listEnvironmentsRemote(endpoint string) error {
	url := fmt.Sprintf("%s/api/v1/environments", endpoint)
	
	req, err := makeAuthenticatedRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Environments []*models.Environment `json:"environments"`
		Count        int                   `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Count == 0 {
		fmt.Println("• No environments found")
		return nil
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("Found %d environment(s):\n", result.Count)
	for _, env := range result.Environments {
		fmt.Printf("• %s (ID: %d)", styles.Success.Render(env.Name), env.ID)
		if env.Description != nil {
			fmt.Printf(" - %s", *env.Description)
		}
		fmt.Printf(" [Created: %s]\n", env.CreatedAt.Format("Jan 2, 2006 15:04"))
	}

	return nil
}

func (h *EnvironmentHandler) createEnvironmentRemote(name, description, endpoint string) error {
	createRequest := struct {
		Name        string  `json:"name"`
		Description *string `json:"description,omitempty"`
	}{
		Name: name,
	}
	
	if description != "" {
		createRequest.Description = &description
	}

	jsonData, err := json.Marshal(createRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/environments", endpoint)
	req, err := makeAuthenticatedRequest(http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Environment *models.Environment `json:"environment"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("✅ Environment created: %s (ID: %d)\n", styles.Success.Render(result.Environment.Name), result.Environment.ID)
	return nil
}

func (h *EnvironmentHandler) getEnvironmentRemote(identifier, endpoint string) error {
	// Try as ID first, then as name by listing and filtering
	var env *models.Environment
	
	if id, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		// Get by ID
		url := fmt.Sprintf("%s/api/v1/environments/%d", endpoint, id)
		req, err := makeAuthenticatedRequest(http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var result struct {
				Environment *models.Environment `json:"environment"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
				env = result.Environment
			}
		}
	}

	if env == nil {
		// Try to find by name by listing all environments
		url := fmt.Sprintf("%s/api/v1/environments", endpoint)
		req, err := makeAuthenticatedRequest(http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
		}

		var result struct {
			Environments []*models.Environment `json:"environments"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		// Find by name
		for _, e := range result.Environments {
			if e.Name == identifier {
				env = e
				break
			}
		}
	}

	if env == nil {
		return fmt.Errorf("environment '%s' not found", identifier)
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

func (h *EnvironmentHandler) updateEnvironmentRemote(identifier, name, description, endpoint string) error {
	// First find the environment ID
	var envID int64
	if id, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		envID = id
	} else {
		// Find by name
		url := fmt.Sprintf("%s/api/v1/environments", endpoint)
		req, err := makeAuthenticatedRequest(http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		var result struct {
			Environments []*models.Environment `json:"environments"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		found := false
		for _, env := range result.Environments {
			if env.Name == identifier {
				envID = env.ID
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("environment '%s' not found", identifier)
		}
	}

	updateRequest := struct {
		Name        string  `json:"name,omitempty"`
		Description *string `json:"description,omitempty"`
	}{}

	if name != "" {
		updateRequest.Name = name
	}
	if description != "" {
		updateRequest.Description = &description
	}

	jsonData, err := json.Marshal(updateRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/environments/%d", endpoint, envID)
	req, err := makeAuthenticatedRequest(http.MethodPut, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	styles := getCLIStyles(h.themeManager)
	displayName := name
	if displayName == "" {
		displayName = identifier
	}
	fmt.Printf("✅ Environment updated: %s\n", styles.Success.Render(displayName))
	return nil
}

func (h *EnvironmentHandler) deleteEnvironmentRemote(identifier, endpoint string) error {
	// First find the environment ID
	var envID int64
	var envName string
	
	if id, err := strconv.ParseInt(identifier, 10, 64); err == nil {
		envID = id
		envName = identifier
	} else {
		// Find by name
		url := fmt.Sprintf("%s/api/v1/environments", endpoint)
		req, err := makeAuthenticatedRequest(http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to connect to server: %w", err)
		}
		defer resp.Body.Close()

		var result struct {
			Environments []*models.Environment `json:"environments"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		found := false
		for _, env := range result.Environments {
			if env.Name == identifier {
				envID = env.ID
				envName = env.Name
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("environment '%s' not found", identifier)
		}
	}

	url := fmt.Sprintf("%s/api/v1/environments/%d", endpoint, envID)
	req, err := makeAuthenticatedRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: status %d: %s", resp.StatusCode, string(body))
	}

	styles := getCLIStyles(h.themeManager)
	fmt.Printf("✅ Environment deleted: %s\n", styles.Success.Render(envName))
	return nil
}
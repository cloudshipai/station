package build

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"dagger.io/dagger"
	"station/internal/db"
)

type EnvironmentBuilder struct {
	environmentName string
	environmentPath string
	buildOptions    *BuildOptions
}

type BuildOptions struct {
	Provider            string
	Model              string
	CloudShipAIKey     string
	CloudShipAIEndpoint string
	InstallShip        bool
}

type EnvironmentConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	MCPConfigs  []string               `json:"mcp_configs"`
	Agents      []string               `json:"agents"`
	Variables   map[string]interface{} `json:"variables"`
}

type BaseBuilder struct{}

func NewBaseBuilder() *BaseBuilder {
	return &BaseBuilder{}
}

func NewEnvironmentBuilder(environmentName, environmentPath string) *EnvironmentBuilder {
	return &EnvironmentBuilder{
		environmentName: environmentName,
		environmentPath: environmentPath,
		buildOptions: &BuildOptions{
			Provider: "openai",
			Model:    "gpt-4o-mini",
		},
	}
}

func NewEnvironmentBuilderWithOptions(environmentName, environmentPath string, options *BuildOptions) *EnvironmentBuilder {
	return &EnvironmentBuilder{
		environmentName: environmentName,
		environmentPath: environmentPath,
		buildOptions:    options,
	}
}

func (b *EnvironmentBuilder) Build(ctx context.Context) (string, error) {
	log.Printf("Starting build for environment: %s", b.environmentName)

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return "", fmt.Errorf("failed to connect to Dagger: %w", err)
	}
	defer client.Close()

	container, tempDBPath, err := b.buildContainer(ctx, client)
	if err != nil {
		return "", fmt.Errorf("failed to build container: %w", err)
	}

	// Clean up temp files after building container
	if tempDBPath != "" {
		defer os.Remove(tempDBPath)
	}
	defer os.Remove("stn") // Clean up temp binary

	// Try to load image directly into local Docker daemon
	imageName := fmt.Sprintf("station-%s:latest", b.environmentName)
	
	// First export to tar
	tarPath := fmt.Sprintf("station-%s.tar", b.environmentName)
	_, err = container.Export(ctx, tarPath)
	if err != nil {
		return "", fmt.Errorf("failed to export container: %w", err)
	}
	
	// Load into Docker daemon
	imageID, err := b.loadImageToDocker(tarPath, imageName)
	if err != nil {
		log.Printf("Failed to load into Docker daemon, keeping tar file: %v", err)
		log.Printf("Successfully exported container to: %s", tarPath)
		return tarPath, nil
	}
	
	// Clean up tar file since we have it in Docker now
	os.Remove(tarPath)
	
	log.Printf("Successfully loaded Docker image: %s", imageName)
	log.Printf("Image ID: %s", imageID)
	log.Printf("Run with: docker run -it %s", imageName)
	return imageName, nil
}

func (b *EnvironmentBuilder) buildContainer(ctx context.Context, client *dagger.Client) (*dagger.Container, string, error) {
	base := client.Container().From("ubuntu:22.04")

	base = base.WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "ca-certificates", "curl", "sqlite3", "git", "python3", "python3-pip", "python3-venv"}).
		WithExec([]string{"bash", "-c", "curl -fsSL https://deb.nodesource.com/setup_20.x | bash -"}).
		WithExec([]string{"apt-get", "install", "-y", "nodejs"}).
		WithExec([]string{"bash", "-c", "curl -LsSf https://astral.sh/uv/install.sh | sh"}).
		WithExec([]string{"bash", "-c", "ln -sf /root/.cargo/bin/uv /usr/local/bin/uv && ln -sf /root/.cargo/bin/uvx /usr/local/bin/uvx"})

	// Build the station binary first
	if err := b.buildStationBinary(); err != nil {
		return nil, "", fmt.Errorf("failed to build station binary: %w", err)
	}
	
	stationBinary := client.Host().Directory(".").File("stn")
	base = base.WithFile("/usr/local/bin/stn", stationBinary).
		WithExec([]string{"chmod", "+x", "/usr/local/bin/stn"})

	// Copy the source environment to the container's default environment
	// This makes the container's "default" environment contain all the source environment's content
	envDir := client.Host().Directory(b.environmentPath)
	base = base.WithExec([]string{"mkdir", "-p", "/root/.config/station/environments/default"})
	base = base.WithDirectory("/root/.config/station/environments/default", envDir)

	log.Printf("🔧 Copying environment '%s' content to container's default environment", b.environmentName)
	
	// Create workspace directory for CI/CD filesystem operations
	base = base.WithExec([]string{"mkdir", "-p", "/workspace"})

	dbPath := "/root/.config/station/station.db"

	// For now, create a minimal database setup in project directory
	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get project root: %w", err)
	}
	
	tempDBPath := filepath.Join(projectRoot, "station_build.db")
	if err := b.createMinimalDatabase(tempDBPath); err != nil {
		return nil, "", fmt.Errorf("failed to create database: %w", err)
	}
	
	// Get relative path for Dagger
	relDBPath := "station_build.db"
	dbFile := client.Host().File(relDBPath)
	base = base.WithFile(dbPath, dbFile)

	base = base.WithWorkdir("/root").
		WithEnvVariable("PATH", "/root/.local/bin:/root/.cargo/bin:/usr/local/bin:/usr/bin:/bin")

	// Build stn init command with proper flags
	initArgs := []string{"stn", "init", 
		"--provider", b.buildOptions.Provider,
		"--model", b.buildOptions.Model,
		"--yes", // Skip interactive prompts
	}
	
	// Add CloudShip AI configuration if provided
	if b.buildOptions.CloudShipAIKey != "" {
		initArgs = append(initArgs, 
			"--cloudshipai", b.buildOptions.CloudShipAIKey,
			"--cloudshipai_endpoint", b.buildOptions.CloudShipAIEndpoint)
	}
	
	// Ship CLI is installed directly in container build, no need for --ship flag
	
	base = base.WithExec(initArgs)
	
	// Pass through API keys for tool discovery during sync
	if key := os.Getenv("STN_AI_API_KEY"); key != "" {
		base = base.WithEnvVariable("STN_AI_API_KEY", key)
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		base = base.WithEnvVariable("OPENAI_API_KEY", key)
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		base = base.WithEnvVariable("ANTHROPIC_API_KEY", key)
	}
	if key := os.Getenv("GOOGLE_API_KEY"); key != "" {
		base = base.WithEnvVariable("GOOGLE_API_KEY", key)
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		base = base.WithEnvVariable("GEMINI_API_KEY", key)
	}
	if key := os.Getenv("AI_API_KEY"); key != "" {
		base = base.WithEnvVariable("AI_API_KEY", key)
	}
	
	// Station's serve command will handle template variable processing with runtime environment variables

	// Install Ship CLI before running sync to prevent MCP failures
	log.Printf("🚢 Installing Ship CLI in container for sync process...")
	base = base.WithExec([]string{"bash", "-c", "timeout 300 bash -c 'curl -fsSL --max-time 60 https://raw.githubusercontent.com/cloudshipai/ship/main/install.sh | bash' || echo 'Ship CLI installation failed or timed out'"})
	base = base.WithExec([]string{"bash", "-c", "if [ -f /root/.local/bin/ship ]; then cp /root/.local/bin/ship /usr/local/bin/ship && chmod +x /usr/local/bin/ship; fi"})

	// Clean up problematic MCP templates that require missing variables
	base = base.WithExec([]string{"bash", "-c", `
		echo "🧹 Cleaning up MCP templates with missing variables..."
		# Remove slack.json if SLACK_BOT_TOKEN is not provided
		if [ -z "$SLACK_BOT_TOKEN" ]; then
			echo "  Removing slack.json (SLACK_BOT_TOKEN not provided)"
			rm -f /root/.config/station/environments/default/slack.json
		fi
		# Add more cleanup for other templates that commonly have missing variables
		echo "  Cleanup completed"
	`})

	// Run sync with timeout and error handling to import agents into database
	// Add WAL mode to prevent database locking issues and retry logic
	base = base.WithExec([]string{"bash", "-c", `
		echo "Setting SQLite to WAL mode to prevent locking..."
		sqlite3 /root/.config/station/station.db "PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA cache_size=1000; PRAGMA temp_store=memory;"
		echo "Running sync with retries and graceful error handling..."
		for i in {1..3}; do
			echo "Sync attempt $i/3..."
			# Use --validate flag first to check for issues, then sync
			if timeout 90 stn sync default -i=false --verbose 2>&1; then
				echo "Sync successful on attempt $i"
				break
			else
				echo "Sync attempt $i failed, retrying in 5 seconds..."
				sleep 5
			fi
		done
		echo "Sync process completed"
		echo "🔍 Checking final agent count..."
		stn agent list --environment default | head -10
	`})
	
	// Install Ship CLI and Docker CLI inside the container
	if b.buildOptions.InstallShip {
		log.Printf("🚢 Downloading Ship CLI inside container...")
		
		// Download and install Ship CLI inside the container
		base = base.WithExec([]string{"bash", "-c", "curl -fsSL https://raw.githubusercontent.com/cloudshipai/ship/main/install.sh | bash"})
		
		log.Printf("🐳 Downloading Docker CLI inside container...")
		
		// Download and install Docker CLI static binary inside the container
		base = base.WithExec([]string{"bash", "-c", `
			curl -fsSL https://download.docker.com/linux/static/stable/x86_64/docker-27.1.1.tgz | tar -xz && 
			mv docker/docker /usr/local/bin/docker && 
			rm -rf docker && 
			chmod +x /usr/local/bin/docker
		`})
		
		// The Ship CLI installs to ~/.local/bin by default, so ensure it's executable and in the right location
		base = base.WithExec([]string{"bash", "-c", `
			if [ -f /root/.local/bin/ship ]; then 
				cp /root/.local/bin/ship /usr/local/bin/ship
				chmod +x /usr/local/bin/ship
			fi
		`})
		
		log.Printf("✅ Ship CLI and Docker CLI installed inside container")
	}
	
	// Set environment variables that persist at runtime
	base = base.WithEnvVariable("PATH", "/root/.local/bin:/root/.cargo/bin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")
	
	// Set working directory to /workspace for CI/CD operations
	base = base.WithWorkdir("/workspace")

	return base, tempDBPath, nil
}

func (b *EnvironmentBuilder) downloadShipCLI(ctx context.Context) (string, error) {
	// Create temporary directory for Ship CLI installation  
	tempInstallDir, err := os.MkdirTemp("", "ship-install-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp install dir: %w", err)
	}
	defer os.RemoveAll(tempInstallDir)
	
	// Create bin subdirectory for installation
	installBinDir := filepath.Join(tempInstallDir, "bin")
	if err := os.MkdirAll(installBinDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create install bin dir: %w", err)
	}
	
	// Download Ship CLI installation script
	log.Printf("Downloading Ship CLI installation script...")
	scriptURL := "https://raw.githubusercontent.com/cloudshipai/ship/main/install.sh"
	
	// Create a HOME environment that points to our temp directory
	// This will make the installation script install to $HOME/.local/bin
	tempHome := tempInstallDir
	tempLocalBin := filepath.Join(tempHome, ".local", "bin")
	if err := os.MkdirAll(tempLocalBin, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp local bin: %w", err)
	}
	
	// Set environment variables for the installation
	installEnv := []string{
		fmt.Sprintf("HOME=%s", tempHome),
		fmt.Sprintf("PATH=%s:%s", installBinDir, os.Getenv("PATH")),
		"SHIP_NO_PATH_UPDATE=1", // Don't modify PATH during installation
	}
	
	// Download and execute installation script
	cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("curl -fsSL %s | bash", scriptURL))
	cmd.Env = installEnv
	cmd.Dir = tempInstallDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to install Ship CLI: %w\nOutput: %s", err, string(output))
	}
	
	// Ship CLI should now be installed in tempHome/.local/bin/ship
	shipBinaryPath := filepath.Join(tempLocalBin, "ship")
	if _, err := os.Stat(shipBinaryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("Ship CLI binary not found at %s after installation\nOutput: %s", shipBinaryPath, string(output))
	}
	
	// Create a temp file in the current working directory (project root)  
	// This ensures Dagger can access it via client.Host().File()
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	
	finalTempFile := filepath.Join(workingDir, "ship-cli-temp")
	
	// Copy the binary to our final temp file location
	sourceFile, err := os.Open(shipBinaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source binary: %w", err)
	}
	defer sourceFile.Close()
	
	destFile, err := os.Create(finalTempFile)
	if err != nil {
		return "", fmt.Errorf("failed to create dest binary: %w", err)
	}
	defer destFile.Close()
	
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		os.Remove(finalTempFile)
		return "", fmt.Errorf("failed to copy binary: %w", err)
	}
	
	// Make executable
	if err := os.Chmod(finalTempFile, 0755); err != nil {
		os.Remove(finalTempFile)
		return "", fmt.Errorf("failed to make binary executable: %w", err)
	}
	
	log.Printf("Successfully downloaded Ship CLI to: %s", finalTempFile)
	return finalTempFile, nil
}

func (b *EnvironmentBuilder) downloadDockerCLI(ctx context.Context) (string, error) {
	// Create a temp file in the current working directory (project root)
	// This ensures Dagger can access it via client.Host().File()
	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}
	
	finalTempFile := filepath.Join(workingDir, "docker-cli-temp")
	
	// Download Docker CLI static binary directly
	log.Printf("Downloading Docker CLI static binary...")
	dockerURL := "https://download.docker.com/linux/static/stable/x86_64/docker-27.1.1.tgz"
	
	// Create temporary directory for Docker CLI download
	tempDir, err := os.MkdirTemp("", "docker-download-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Download and extract the Docker CLI binary in temp directory
	cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("curl -fsSL %s | tar -xz", dockerURL))
	cmd.Dir = tempDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to download Docker CLI: %w\nOutput: %s", err, string(output))
	}
	
	// The extracted binary should be at docker/docker in the temp directory
	dockerBinaryPath := filepath.Join(tempDir, "docker", "docker")
	if _, err := os.Stat(dockerBinaryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("Docker CLI binary not found after download: %s", dockerBinaryPath)
	}
	
	// Copy to our final temp file location
	sourceFile, err := os.Open(dockerBinaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to open source binary: %w", err)
	}
	defer sourceFile.Close()
	
	destFile, err := os.Create(finalTempFile)
	if err != nil {
		return "", fmt.Errorf("failed to create dest binary: %w", err)
	}
	defer destFile.Close()
	
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		os.Remove(finalTempFile)
		return "", fmt.Errorf("failed to copy binary: %w", err)
	}
	
	// Make executable
	if err := os.Chmod(finalTempFile, 0755); err != nil {
		os.Remove(finalTempFile)
		return "", fmt.Errorf("failed to make Docker binary executable: %w", err)
	}
	
	log.Printf("Successfully downloaded Docker CLI to: %s", finalTempFile)
	return finalTempFile, nil
}

func (b *EnvironmentBuilder) setupEnvironmentInDB(ctx context.Context, database *db.DB) error {
	configPath := filepath.Join(b.environmentPath, "config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read environment config: %w", err)
	}

	var config EnvironmentConfig
	if err == nil {
		if err := json.Unmarshal(configData, &config); err != nil {
			return fmt.Errorf("failed to unmarshal config: %w", err)
		}
	} else {
		config = EnvironmentConfig{
			Name:        b.environmentName,
			Description: fmt.Sprintf("Containerized environment: %s", b.environmentName),
		}
	}

	// TODO: Implement database setup for environment and agents
	log.Printf("Would setup environment '%s' with config: %+v", config.Name, config)
	
	agentsPath := filepath.Join(b.environmentPath, "agents")
	if _, err := os.Stat(agentsPath); err == nil {
		err := filepath.WalkDir(agentsPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".prompt") {
				log.Printf("Would import agent from: %s", path)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to scan agents: %w", err)
		}
	}

	return nil
}

func (b *EnvironmentBuilder) createMinimalDatabase(dbPath string) error {
	// Create a minimal database with proper schema
	database, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer database.Close()

	// Run migrations to set up schema
	if err := database.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Printf("Created minimal database at: %s", dbPath)
	return nil
}

func (b *EnvironmentBuilder) loadImageToDocker(tarPath, imageName string) (string, error) {
	log.Printf("Loading %s into Docker daemon as %s", tarPath, imageName)
	
	// Use docker load < tarfile to preserve full container structure
	cmd := exec.Command("docker", "load")
	
	// Open tar file
	tarFile, err := os.Open(tarPath)
	if err != nil {
		return "", fmt.Errorf("failed to open tar file: %w", err)
	}
	defer tarFile.Close()
	
	// Pipe tar content to docker load
	cmd.Stdin = tarFile
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker load failed: %w, output: %s", err, output)
	}
	
	log.Printf("Docker load output: %s", string(output))
	
	// Tag the loaded image with our desired name
	loadedImageID := b.extractImageIDFromLoadOutput(string(output))
	if loadedImageID != "" {
		tagCmd := exec.Command("docker", "tag", loadedImageID, imageName)
		if err := tagCmd.Run(); err != nil {
			log.Printf("Warning: failed to tag image: %v", err)
		}
	}
	
	// Get the final image ID
	inspectCmd := exec.Command("docker", "inspect", "--format={{.Id}}", imageName)
	imageIDBytes, err := inspectCmd.Output()
	if err != nil {
		return loadedImageID, nil // Return what we have
	}
	
	return strings.TrimSpace(string(imageIDBytes)), nil
}

func (b *EnvironmentBuilder) extractImageIDFromLoadOutput(output string) string {
	// Parse docker load output like "Loaded image: sha256:abc123..." or "Loaded image ID: sha256:abc123..."
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Loaded image ID:") {
			// Handle "Loaded image ID: sha256:abc123..."
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				return strings.TrimSpace(strings.Join(parts[2:], ":"))
			}
		} else if strings.Contains(line, "Loaded image:") {
			// Handle "Loaded image: sha256:abc123..."
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				return strings.TrimSpace(strings.Join(parts[2:], ":"))
			}
		}
	}
	return ""
}

func (b *EnvironmentBuilder) buildStationBinary() error {
	log.Printf("Building station binary...")
	
	// First, try to use local stn binary from PATH (Linux systems only)
	if runtime.GOOS == "linux" {
		if stnPath, err := exec.LookPath("stn"); err == nil {
			log.Printf("Using local Station binary from PATH: %s", stnPath)
			if err := b.copyFile(stnPath, "stn"); err != nil {
				log.Printf("Failed to copy local binary, trying download: %v", err)
			} else {
				log.Printf("Successfully copied local Station binary")
				return nil
			}
		}
	}
	
	// Second, download latest binary from install script
	log.Printf("Downloading latest Station binary...")
	if err := b.downloadStationBinary(); err != nil {
		log.Printf("Failed to download binary, trying source build: %v", err)
	} else {
		return nil
	}
	
	// Last resort: try to build from source if available
	if _, err := os.Stat("go.mod"); err == nil {
		log.Printf("Building from source...")
		cmd := exec.Command("go", "build", "-o", "stn", "./cmd/main")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err == nil {
			log.Printf("Successfully built station binary from source")
			return nil
		}
		return fmt.Errorf("source build failed: %w", err)
	}
	
	return fmt.Errorf("failed to obtain Station binary: no local binary, download failed, and no source available")
}

// copyFile copies a file from src to dst
func (b *EnvironmentBuilder) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Copy file permissions
	if info, err := sourceFile.Stat(); err == nil {
		if err := destFile.Chmod(info.Mode()); err != nil {
			return fmt.Errorf("failed to set file permissions: %w", err)
		}
	}

	return nil
}

// downloadStationBinary downloads the latest Station binary from the install script
func (b *EnvironmentBuilder) downloadStationBinary() error {
	log.Printf("Downloading latest Station binary from install script...")
	
	// Create temporary directory for installation
	tempDir, err := os.MkdirTemp("", "station-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Download and execute install script
	installURL := "https://raw.githubusercontent.com/cloudshipai/station/main/install.sh"
	cmd := exec.Command("bash", "-c", fmt.Sprintf("curl -fsSL %s | bash", installURL))
	
	// Set environment to install to temp directory
	cmd.Env = append(os.Environ(), 
		fmt.Sprintf("HOME=%s", tempDir),
		"STATION_NO_PATH_UPDATE=1", // Don't modify PATH during installation
	)
	cmd.Dir = tempDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to download Station binary: %w\nOutput: %s", err, string(output))
	}
	
	// Station binary should be installed in temp directory
	possiblePaths := []string{
		filepath.Join(tempDir, ".local", "bin", "stn"),
		filepath.Join(tempDir, "stn"),
		filepath.Join(tempDir, "bin", "stn"),
	}
	
	var stationPath string
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			stationPath = path
			break
		}
	}
	
	if stationPath == "" {
		return fmt.Errorf("Station binary not found after installation\nOutput: %s", string(output))
	}
	
	// Copy the downloaded binary to current directory
	if err := b.copyFile(stationPath, "stn"); err != nil {
		return fmt.Errorf("failed to copy downloaded binary: %w", err)
	}
	
	log.Printf("Successfully downloaded and copied Station binary")
	return nil
}

func (b *EnvironmentBuilder) exportDatabase(database *db.DB, outputPath string) error {
	sourceDB := database.Conn()
	destDB, err := db.New(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create destination database: %w", err)
	}
	defer destDB.Close()

	return b.copyDatabase(sourceDB, destDB.Conn())
}

func (b *EnvironmentBuilder) copyDatabase(source, dest *sql.DB) error {
	// Get all table names from source database
	rows, err := source.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return fmt.Errorf("failed to get table names: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, tableName)
	}

	// Copy schema and data for each table
	for _, table := range tables {
		// Get table schema
		schemaRows, err := source.Query(fmt.Sprintf("SELECT sql FROM sqlite_master WHERE name='%s'", table))
		if err != nil {
			return fmt.Errorf("failed to get schema for table %s: %w", table, err)
		}
		
		var createSQL string
		if schemaRows.Next() {
			if err := schemaRows.Scan(&createSQL); err != nil {
				schemaRows.Close()
				return fmt.Errorf("failed to scan schema for table %s: %w", table, err)
			}
		}
		schemaRows.Close()

		// Create table in destination
		if createSQL != "" {
			if _, err := dest.Exec(createSQL); err != nil {
				log.Printf("Warning: failed to create table %s: %v", table, err)
				continue
			}
		}

		// Copy data
		dataRows, err := source.Query(fmt.Sprintf("SELECT * FROM %s", table))
		if err != nil {
			log.Printf("Warning: failed to query data from table %s: %v", table, err)
			continue
		}

		columns, err := dataRows.Columns()
		if err != nil {
			dataRows.Close()
			log.Printf("Warning: failed to get columns for table %s: %v", table, err)
			continue
		}

		if len(columns) == 0 {
			dataRows.Close()
			continue
		}

		placeholders := make([]string, len(columns))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", 
			table, 
			strings.Join(columns, ","), 
			strings.Join(placeholders, ","))

		for dataRows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := dataRows.Scan(valuePtrs...); err != nil {
				log.Printf("Warning: failed to scan row from table %s: %v", table, err)
				continue
			}

			if _, err := dest.Exec(insertSQL, values...); err != nil {
				log.Printf("Warning: failed to insert row into table %s: %v", table, err)
			}
		}
		dataRows.Close()
		
		log.Printf("Copied table: %s", table)
	}

	return nil
}

func (b *BaseBuilder) Build(ctx context.Context) (string, error) {
	log.Printf("Starting build for base Station container")

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return "", fmt.Errorf("failed to connect to Dagger: %w", err)
	}
	defer client.Close()

	container, tempDBPath, err := b.buildBaseContainer(ctx, client)
	if err != nil {
		return "", fmt.Errorf("failed to build base container: %w", err)
	}

	// Clean up temp files after building container
	if tempDBPath != "" {
		defer os.Remove(tempDBPath)
	}
	defer os.Remove("stn") // Clean up temp binary

	// Try to load image directly into local Docker daemon
	imageName := "station-base:latest"
	
	// First export to tar
	tarPath := "station-base.tar"
	_, err = container.Export(ctx, tarPath)
	if err != nil {
		return "", fmt.Errorf("failed to export base container: %w", err)
	}
	
	// Load into Docker daemon
	imageID, err := b.loadImageToDocker(tarPath, imageName)
	if err != nil {
		log.Printf("Failed to load into Docker daemon, keeping tar file: %v", err)
		log.Printf("Successfully exported base container to: %s", tarPath)
		return tarPath, nil
	}
	
	// Clean up tar file since we have it in Docker now
	os.Remove(tarPath)
	
	log.Printf("Successfully loaded Docker image: %s", imageName)
	log.Printf("Image ID: %s", imageID)
	log.Printf("Run with: docker run -it %s", imageName)
	return imageName, nil
}

func (b *BaseBuilder) buildBaseContainer(ctx context.Context, client *dagger.Client) (*dagger.Container, string, error) {
	base := client.Container().From("ubuntu:22.04")

	base = base.WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y", "ca-certificates", "curl", "sqlite3", "git"})

	// Build the station binary first
	if err := b.buildStationBinary(); err != nil {
		return nil, "", fmt.Errorf("failed to build station binary: %w", err)
	}
	
	stationBinary := client.Host().Directory(".").File("stn")
	base = base.WithFile("/usr/local/bin/stn", stationBinary).
		WithExec([]string{"chmod", "+x", "/usr/local/bin/stn"})

	// Create app and config directories
	dbPath := "/app/data/station.db"
	base = base.WithExec([]string{"mkdir", "-p", "/app/data", "/app/environment", "/root/.config/station"})

	// Create minimal database in project directory
	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get project root: %w", err)
	}
	
	tempDBPath := filepath.Join(projectRoot, "station_build.db")
	if err := b.createMinimalDatabase(tempDBPath); err != nil {
		return nil, "", fmt.Errorf("failed to create database: %w", err)
	}
	
	// Get relative path for Dagger
	relDBPath := "station_build.db"
	dbFile := client.Host().File(relDBPath)
	base = base.WithFile(dbPath, dbFile)

	base = base.WithEnvVariable("STATION_CONFIG_ROOT", "/app/environment").
		WithEnvVariable("STATION_DB_PATH", dbPath).
		WithWorkdir("/app")

	// Add entrypoint script for config injection
	entryScript := `#!/bin/bash
# Station Base Container Entrypoint
# Handles environment variable injection and config mounting

echo "Station Base Container Starting..."
echo "Config Root: $STATION_CONFIG_ROOT"
echo "Database: $STATION_DB_PATH"

# Check if configs are mounted
if [ -f /app/environment/variables.yml ]; then
    echo "✓ Variables config found"
else
    echo "⚠ No variables.yml mounted. Use -v /path/to/variables.yml:/app/environment/variables.yml"
fi

if [ -f /app/environment/config.yml ]; then
    echo "✓ Environment config found"
else
    echo "⚠ No config.yml mounted. Use -v /path/to/config.yml:/app/environment/config.yml"
fi

# Execute command or start shell
if [ $# -eq 0 ]; then
    echo "Starting interactive shell..."
    exec /bin/bash
else
    exec "$@"
fi
`
	base = base.WithNewFile("/app/entrypoint.sh", entryScript).
		WithExec([]string{"chmod", "+x", "/app/entrypoint.sh"})

	return base, tempDBPath, nil
}

func (b *BaseBuilder) buildStationBinary() error {
	log.Printf("Building station binary...")
	
	cmd := exec.Command("go", "build", "-o", "stn", "./cmd/main")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build binary: %w", err)
	}
	
	log.Printf("Successfully built station binary")
	return nil
}

func (b *BaseBuilder) createMinimalDatabase(dbPath string) error {
	// Create a minimal database with proper schema
	database, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer database.Close()

	// Run migrations to set up schema
	if err := database.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Printf("Created minimal database at: %s", dbPath)
	return nil
}

func (b *BaseBuilder) loadImageToDocker(tarPath, imageName string) (string, error) {
	log.Printf("Loading %s into Docker daemon as %s", tarPath, imageName)
	
	// Use docker load < tarfile to preserve full container structure
	cmd := exec.Command("docker", "load")
	
	// Open tar file
	tarFile, err := os.Open(tarPath)
	if err != nil {
		return "", fmt.Errorf("failed to open tar file: %w", err)
	}
	defer tarFile.Close()
	
	// Pipe tar content to docker load
	cmd.Stdin = tarFile
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker load failed: %w, output: %s", err, output)
	}
	
	log.Printf("Docker load output: %s", string(output))
	
	// Tag the loaded image with our desired name
	loadedImageID := b.extractImageIDFromLoadOutput(string(output))
	if loadedImageID != "" {
		tagCmd := exec.Command("docker", "tag", loadedImageID, imageName)
		if err := tagCmd.Run(); err != nil {
			log.Printf("Warning: failed to tag image: %v", err)
		}
	}
	
	// Get the final image ID
	inspectCmd := exec.Command("docker", "inspect", "--format={{.Id}}", imageName)
	imageIDBytes, err := inspectCmd.Output()
	if err != nil {
		return loadedImageID, nil // Return what we have
	}
	
	return strings.TrimSpace(string(imageIDBytes)), nil
}

func (b *BaseBuilder) extractImageIDFromLoadOutput(output string) string {
	// Parse docker load output like "Loaded image: sha256:abc123..." or "Loaded image ID: sha256:abc123..."
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Loaded image ID:") {
			// Handle "Loaded image ID: sha256:abc123..."
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				return strings.TrimSpace(strings.Join(parts[2:], ":"))
			}
		} else if strings.Contains(line, "Loaded image:") {
			// Handle "Loaded image: sha256:abc123..."
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				return strings.TrimSpace(strings.Join(parts[2:], ":"))
			}
		}
	}
	return ""
}
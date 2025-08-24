package build

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"station/internal/db"
)

type EnvironmentBuilder struct {
	environmentName string
	environmentPath string
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
		WithExec([]string{"apt-get", "install", "-y", "ca-certificates", "curl", "sqlite3", "git"})

	// Build the station binary first
	if err := b.buildStationBinary(); err != nil {
		return nil, "", fmt.Errorf("failed to build station binary: %w", err)
	}
	
	stationBinary := client.Host().Directory(".").File("stn")
	base = base.WithFile("/usr/local/bin/stn", stationBinary).
		WithExec([]string{"chmod", "+x", "/usr/local/bin/stn"})

	envDir := client.Host().Directory(b.environmentPath)
	base = base.WithDirectory("/app/environment", envDir)

	dbPath := "/app/data/station.db"
	base = base.WithExec([]string{"mkdir", "-p", "/app/data"})

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

	base = base.WithEnvVariable("STATION_CONFIG_ROOT", "/app/environment").
		WithEnvVariable("STATION_DB_PATH", dbPath).
		WithWorkdir("/app")

	base = base.WithExec([]string{"stn", "init"})

	return base, tempDBPath, nil
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
	// Parse docker load output like "Loaded image: sha256:abc123..."
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Loaded image:") {
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
	
	cmd := exec.Command("go", "build", "-o", "stn", "./cmd/main")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build binary: %w", err)
	}
	
	log.Printf("Successfully built station binary")
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

	// Create app directories
	dbPath := "/app/data/station.db"
	base = base.WithExec([]string{"mkdir", "-p", "/app/data", "/app/environment"})

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
	// Parse docker load output like "Loaded image: sha256:abc123..."
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Loaded image:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				return strings.TrimSpace(strings.Join(parts[2:], ":"))
			}
		}
	}
	return ""
}
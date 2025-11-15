package builder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// BuildOptions configures the Docker image build
type BuildOptions struct {
	EnvironmentName string
	EnvironmentPath string
	BaseImage       string // Default: ghcr.io/cloudshipai/station:latest
	ImageName       string // Output image name
	ImageTag        string // Default: latest
	SkipSync        bool   // If true, skip sync during build (sync at runtime)
}

// DockerBuilder builds Station environment images
type DockerBuilder struct {
	opts BuildOptions
}

// NewDockerBuilder creates a new Docker builder
func NewDockerBuilder(opts BuildOptions) *DockerBuilder {
	// Set defaults
	if opts.BaseImage == "" {
		// Use local base image by default (built with Dockerfile.local)
		// Contains all latest code including faker command
		opts.BaseImage = "station:local"
	}
	if opts.ImageTag == "" {
		opts.ImageTag = "latest"
	}
	if opts.ImageName == "" {
		opts.ImageName = fmt.Sprintf("station-%s", opts.EnvironmentName)
	}

	return &DockerBuilder{opts: opts}
}

// Build creates a Docker image from the environment
func (b *DockerBuilder) Build(ctx context.Context) (string, error) {
	// Create temporary directory for build context
	tmpDir, err := os.MkdirTemp("", fmt.Sprintf("station-build-%s-*", b.opts.EnvironmentName))
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Dockerfile
	dockerfile := b.generateDockerfile()
	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return "", fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	// Copy environment files (excluding variables.yml which contains secrets)
	envTempDir := filepath.Join(tmpDir, "environment")
	if err := os.MkdirAll(envTempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp env dir: %w", err)
	}

	if err := b.copyEnvironmentFiles(envTempDir); err != nil {
		return "", fmt.Errorf("failed to copy environment files: %w", err)
	}

	// Note: No need to copy binary - base image (station:local) already has latest stn binary

	// Build Docker image
	fullImageName := fmt.Sprintf("%s:%s", b.opts.ImageName, b.opts.ImageTag)
	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", fullImageName, tmpDir)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return "", fmt.Errorf("docker build failed: %w", err)
	}

	return fullImageName, nil
}

// generateDockerfile creates the Dockerfile content
func (b *DockerBuilder) generateDockerfile() string {
	dockerfile := fmt.Sprintf(`FROM %s

# Copy environment configuration (excluding variables.yml which has secrets)
COPY --chown=station:station environment/ /home/station/.config/station/environments/default/

# Create minimal config.yaml for production deployment
# Note: AI provider/model/keys come from STATION_AI_* environment variables at runtime
RUN echo "admin_username: admin" > /home/station/.config/station/config.yaml && \
    echo "database_url: /data/station.db" >> /home/station/.config/station/config.yaml && \
    echo "local_mode: true" >> /home/station/.config/station/config.yaml && \
    echo "api_port: 8585" >> /home/station/.config/station/config.yaml && \
    echo "mcp_port: 8586" >> /home/station/.config/station/config.yaml && \
    echo "ssh_port: 2222" >> /home/station/.config/station/config.yaml && \
    echo "telemetry_enabled: true" >> /home/station/.config/station/config.yaml && \
    chown station:station /home/station/.config/station/config.yaml

# Override default ports to match deployment configuration
ENV STATION_MCP_PORT=8586
ENV STATION_API_PORT=8585
ENV STATION_SSH_PORT=2222

# Database is on persistent volume at /data/station.db
ENV DATABASE_URL=/data/station.db

WORKDIR /workspace
`, b.opts.BaseImage)

	return dockerfile
}

// copyEnvironmentFiles copies all environment files except variables.yml
func (b *DockerBuilder) copyEnvironmentFiles(destDir string) error {
	entries, err := os.ReadDir(b.opts.EnvironmentPath)
	if err != nil {
		return fmt.Errorf("failed to read environment directory: %w", err)
	}

	for _, entry := range entries {
		// Skip variables.yml - secrets should come from environment variables
		if entry.Name() == "variables.yml" {
			continue
		}

		srcPath := filepath.Join(b.opts.EnvironmentPath, entry.Name())
		dstPath := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to copy directory %s: %w", entry.Name(), err)
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", entry.Name(), err)
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// copyFile copies a single file
func (b *DockerBuilder) copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755) // Executable permissions for binary
}

// copyDirRecursive recursively copies a directory
func copyDirRecursive(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}

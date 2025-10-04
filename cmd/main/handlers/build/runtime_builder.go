package build

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"dagger.io/dagger"
)

// RuntimeBuilder creates a minimal Station runtime container for stn up/down
// Unlike EnvironmentBuilder, this doesn't run init or sync - it expects
// the user to mount their existing config and database
type RuntimeBuilder struct {
	options *RuntimeBuildOptions
}

type RuntimeBuildOptions struct {
	ImageName   string // Name for the built image
	InstallShip bool   // Whether to install Ship CLI
}

func NewRuntimeBuilder(options *RuntimeBuildOptions) *RuntimeBuilder {
	if options == nil {
		options = &RuntimeBuildOptions{
			ImageName:   "station-runtime:latest",
			InstallShip: true,
		}
	}
	return &RuntimeBuilder{
		options: options,
	}
}

// Build creates a runtime container for Station server
func (b *RuntimeBuilder) Build(ctx context.Context) error {
	log.Printf("🔨 Building Station runtime container...")

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return fmt.Errorf("failed to connect to Dagger: %w", err)
	}
	defer client.Close()

	container, err := b.buildRuntimeContainer(ctx, client)
	if err != nil {
		return fmt.Errorf("failed to build runtime container: %w", err)
	}

	// Clean up temp binary after building
	defer os.Remove("stn")

	// Export to tar file
	tarPath := "station-runtime.tar"
	_, err = container.Export(ctx, tarPath)
	if err != nil {
		return fmt.Errorf("failed to export container: %w", err)
	}

	// Load into Docker daemon
	if err := b.loadImageToDocker(tarPath); err != nil {
		log.Printf("Warning: Failed to load into Docker daemon: %v", err)
		log.Printf("Container exported to: %s", tarPath)
		log.Printf("Load manually with: docker load < %s", tarPath)
		return nil
	}

	// Clean up tar file since we loaded it
	os.Remove(tarPath)

	log.Printf("✅ Successfully built Station runtime container: %s", b.options.ImageName)
	return nil
}

func (b *RuntimeBuilder) buildRuntimeContainer(ctx context.Context, client *dagger.Client) (*dagger.Container, error) {
	// Start with Ubuntu base
	base := client.Container().From("ubuntu:22.04")

	// Install essential packages
	base = base.WithExec([]string{"apt-get", "update"}).
		WithExec([]string{"apt-get", "install", "-y",
			"ca-certificates",
			"curl",
			"git",
			"sqlite3",           // For database operations
			"python3",           // For Python-based MCP servers
			"python3-pip",
			"python3-venv",
			"build-essential",   // For compiling native modules
			"openssh-client",    // For git operations
		})

	// Install Node.js for JavaScript MCP servers
	base = base.WithExec([]string{"bash", "-c",
		"curl -fsSL https://deb.nodesource.com/setup_20.x | bash -"}).
		WithExec([]string{"apt-get", "install", "-y", "nodejs"})

	// Install uv for Python package management
	base = base.WithExec([]string{"bash", "-c",
		"curl -LsSf https://astral.sh/uv/install.sh | sh"}).
		WithExec([]string{"bash", "-c",
			"ln -sf /root/.cargo/bin/uv /usr/local/bin/uv && " +
			"ln -sf /root/.cargo/bin/uvx /usr/local/bin/uvx"})

	// Build or copy Station binary
	if err := b.getStationBinary(); err != nil {
		return nil, fmt.Errorf("failed to get Station binary: %w", err)
	}

	stationBinary := client.Host().Directory(".").File("stn")
	base = base.WithFile("/usr/local/bin/stn", stationBinary).
		WithExec([]string{"chmod", "+x", "/usr/local/bin/stn"})

	// Install Ship CLI if requested
	if b.options.InstallShip {
		log.Printf("🚢 Installing Ship CLI in container...")
		base = base.WithExec([]string{"bash", "-c",
			"timeout 300 bash -c 'curl -fsSL --max-time 60 https://raw.githubusercontent.com/cloudshipai/ship/main/install.sh | bash' || " +
			"echo 'Ship CLI installation failed or timed out'"})
		base = base.WithExec([]string{"bash", "-c",
			"if [ -f /root/.local/bin/ship ]; then " +
			"cp /root/.local/bin/ship /usr/local/bin/ship && " +
			"chmod +x /usr/local/bin/ship; fi"})
	}

	// Install Docker CLI for Docker-in-Docker support
	base = base.WithExec([]string{"bash", "-c", `
		curl -fsSL https://download.docker.com/linux/static/stable/x86_64/docker-27.1.1.tgz | tar -xz &&
		mv docker/docker /usr/local/bin/docker &&
		rm -rf docker &&
		chmod +x /usr/local/bin/docker
	`})

	// Create necessary directories
	base = base.WithExec([]string{"mkdir", "-p",
		"/workspace",                    // User workspace mount point
		"/root/.config/station",         // Config mount point
		"/root/.local/bin",              // Local binaries
		"/var/log/station",              // Logs
	})

	// Set environment variables
	base = base.
		WithEnvVariable("PATH", "/root/.local/bin:/root/.cargo/bin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin").
		WithEnvVariable("HOME", "/root").
		WithEnvVariable("STATION_RUNTIME", "docker").
		WithWorkdir("/workspace")

	// Add health check
	base = base.WithExec([]string{"bash", "-c",
		`echo '#!/bin/bash
curl -f http://localhost:3000/health || exit 1' > /usr/local/bin/health-check &&
chmod +x /usr/local/bin/health-check`})

	// Set default command to run Station server
	base = base.WithDefaultArgs([]string{"stn", "serve"})

	return base, nil
}

func (b *RuntimeBuilder) getStationBinary() error {
	// Try to use existing binary from PATH (Linux only)
	if runtime.GOOS == "linux" {
		if stnPath, err := exec.LookPath("stn"); err == nil {
			log.Printf("Using local Station binary from: %s", stnPath)
			return b.copyFile(stnPath, "stn")
		}
	}

	// Try to build from source if available
	if _, err := os.Stat("go.mod"); err == nil {
		log.Printf("Building Station binary from source...")
		cmd := exec.Command("go", "build", "-o", "stn", "./cmd/main")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err == nil {
			log.Printf("Successfully built Station binary")
			return nil
		}
		log.Printf("Failed to build from source: %v", err)
	}

	// Download latest binary as last resort
	return b.downloadStationBinary()
}

func (b *RuntimeBuilder) downloadStationBinary() error {
	log.Printf("Downloading latest Station binary...")

	tempDir, err := os.MkdirTemp("", "station-download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Download using install script
	installURL := "https://raw.githubusercontent.com/cloudshipai/station/main/install.sh"
	cmd := exec.Command("bash", "-c", fmt.Sprintf("curl -fsSL %s | bash", installURL))
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HOME=%s", tempDir),
		"STATION_NO_PATH_UPDATE=1",
	)
	cmd.Dir = tempDir

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to download Station: %w\nOutput: %s", err, string(output))
	}

	// Find the downloaded binary
	possiblePaths := []string{
		filepath.Join(tempDir, ".local", "bin", "stn"),
		filepath.Join(tempDir, "stn"),
		filepath.Join(tempDir, "bin", "stn"),
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return b.copyFile(path, "stn")
		}
	}

	return fmt.Errorf("Station binary not found after download")
}

func (b *RuntimeBuilder) copyFile(src, dst string) error {
	input, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(dst, input, 0755); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}

func (b *RuntimeBuilder) loadImageToDocker(tarPath string) error {
	log.Printf("Loading image into Docker daemon...")

	// Load the tar file
	cmd := exec.Command("docker", "load", "-i", tarPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker load failed: %w, output: %s", err, output)
	}

	log.Printf("Docker load output: %s", string(output))

	// Extract image ID from output and tag it
	imageID := b.extractImageID(string(output))
	if imageID != "" && b.options.ImageName != "" {
		tagCmd := exec.Command("docker", "tag", imageID, b.options.ImageName)
		if err := tagCmd.Run(); err != nil {
			log.Printf("Warning: Failed to tag image: %v", err)
		}
	}

	return nil
}

func (b *RuntimeBuilder) extractImageID(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Loaded image ID:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				return strings.TrimSpace(strings.Join(parts[2:], ":"))
			}
		} else if strings.Contains(line, "Loaded image:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				return strings.TrimSpace(strings.Join(parts[2:], ":"))
			}
		}
	}
	return ""
}
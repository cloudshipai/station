package build

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
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
	BuildLocal  bool   // Build locally instead of pulling from GHCR
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

// Build creates or pulls the Station runtime container
func (b *RuntimeBuilder) Build(ctx context.Context) error {
	if b.options.BuildLocal {
		return b.buildLocalImage(ctx)
	}
	return b.pullFromRegistry(ctx)
}

// pullFromRegistry pulls the Station runtime container from GitHub Container Registry
func (b *RuntimeBuilder) pullFromRegistry(ctx context.Context) error {
	log.Printf("🔨 Pulling Station runtime container from GitHub Container Registry...")

	// Determine which image to pull
	imageRef := "ghcr.io/cloudshipai/station:latest"
	if b.options.ImageName != "" && b.options.ImageName != "station-runtime:latest" {
		// If user specified a custom image name, use it as-is
		imageRef = b.options.ImageName
	}

	log.Printf("📦 Pulling image: %s", imageRef)

	// Pull the image using Docker CLI
	cmd := exec.Command("docker", "pull", imageRef)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}

	// Tag as local runtime image if needed
	if imageRef != "station-runtime:latest" {
		tagCmd := exec.Command("docker", "tag", imageRef, "station-runtime:latest")
		if err := tagCmd.Run(); err != nil {
			log.Printf("Warning: Failed to tag image: %v", err)
		}
	}

	log.Printf("✅ Successfully pulled Station runtime container: %s", imageRef)
	return nil
}

// buildLocalImage builds the container locally using Dockerfile
func (b *RuntimeBuilder) buildLocalImage(ctx context.Context) error {
	log.Printf("🔨 Building Station runtime container locally...")

	// Build Station binary for Linux
	if err := b.buildStationBinary(); err != nil {
		return fmt.Errorf("failed to build Station binary: %w", err)
	}
	defer os.Remove("stn")

	// Build Docker image
	imageName := b.options.ImageName
	if imageName == "" {
		imageName = "station-runtime:latest"
	}

	buildArgs := []string{
		"build",
		"-t", imageName,
		"-t", "station-runtime:latest",
		"-f", "Dockerfile",
	}

	if b.options.InstallShip {
		buildArgs = append(buildArgs, "--build-arg", "INSTALL_SHIP=true")
	} else {
		buildArgs = append(buildArgs, "--build-arg", "INSTALL_SHIP=false")
	}

	buildArgs = append(buildArgs, ".")

	cmd := exec.Command("docker", buildArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build Docker image: %w", err)
	}

	log.Printf("✅ Successfully built Station runtime container: %s", imageName)
	return nil
}

// buildStationBinary builds the Station binary for Linux with UI embedded
func (b *RuntimeBuilder) buildStationBinary() error {
	log.Printf("Building Station binary for Linux container...")

	cmd := exec.Command("go", "build", "-tags", "ui", "-o", "stn", "./cmd/main")
	cmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH=amd64",
		"CGO_ENABLED=0",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	log.Printf("✅ Successfully built Linux binary for container")
	return nil
}


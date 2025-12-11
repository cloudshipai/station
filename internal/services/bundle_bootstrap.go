// Package services contains the bundle bootstrap service for automatic bundle installation on startup.
package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"station/internal/cloudship"
	"station/internal/config"
	"station/internal/db/repositories"
)

// BundleBootstrapService handles automatic bundle installation from CloudShip on server startup.
type BundleBootstrapService struct {
	cfg             *config.Config
	repos           *repositories.Repositories
	cloudshipClient *cloudship.Client
	bundleService   *BundleService
}

// NewBundleBootstrapService creates a new BundleBootstrapService.
func NewBundleBootstrapService(cfg *config.Config, repos *repositories.Repositories) *BundleBootstrapService {
	var bundleService *BundleService
	if repos != nil {
		bundleService = NewBundleServiceWithRepos(repos)
	} else {
		bundleService = NewBundleService()
	}

	return &BundleBootstrapService{
		cfg:             cfg,
		repos:           repos,
		cloudshipClient: cloudship.NewClient(cfg),
		bundleService:   bundleService,
	}
}

// markerFilePath returns the path to the marker file that indicates a bundle has been installed.
func (s *BundleBootstrapService) markerFilePath(bundleID string) string {
	configDir := os.Getenv("STATION_CONFIG_DIR")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config", "station")
	}
	return filepath.Join(configDir, fmt.Sprintf(".bundle-%s-installed", bundleID))
}

// isBundleInstalled checks if a bundle has already been installed by looking for a marker file.
func (s *BundleBootstrapService) isBundleInstalled(bundleID string) bool {
	markerPath := s.markerFilePath(bundleID)
	_, err := os.Stat(markerPath)
	return err == nil
}

// markBundleInstalled creates a marker file to indicate a bundle has been installed.
func (s *BundleBootstrapService) markBundleInstalled(bundleID string) error {
	markerPath := s.markerFilePath(bundleID)
	// Create parent directory if needed
	if err := os.MkdirAll(filepath.Dir(markerPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(markerPath, []byte(bundleID), 0644)
}

// BootstrapFromCloudShip downloads and installs a bundle from CloudShip if not already installed.
// This is the main entry point for bundle bootstrapping on server startup.
func (s *BundleBootstrapService) BootstrapFromCloudShip(ctx context.Context, bundleID string) error {
	// Validate bundle ID format
	if !cloudship.IsUUID(bundleID) {
		return fmt.Errorf("invalid bundle ID format: %s (expected UUID)", bundleID)
	}

	// Check if already installed
	if s.isBundleInstalled(bundleID) {
		log.Printf("📦 Bundle %s already installed, skipping download", bundleID)
		return nil
	}

	log.Printf("📦 Bootstrapping bundle from CloudShip: %s", bundleID)

	// Download bundle from CloudShip
	log.Printf("   Downloading bundle from CloudShip...")
	bundlePath, err := s.cloudshipClient.DownloadBundle(bundleID)
	if err != nil {
		return fmt.Errorf("failed to download bundle: %w", err)
	}
	defer os.Remove(bundlePath) // Clean up temp file

	// Install bundle to the "default" environment
	log.Printf("   Installing bundle to 'default' environment...")
	result, err := s.bundleService.InstallBundleWithOptions(bundlePath, "default", true) // force=true
	if err != nil {
		return fmt.Errorf("failed to install bundle: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("bundle installation failed: %s", result.Error)
	}

	// Mark as installed
	if err := s.markBundleInstalled(bundleID); err != nil {
		log.Printf("⚠️  Warning: failed to create installation marker: %v", err)
		// Don't fail the bootstrap for this
	}

	log.Printf("✅ Bundle installed successfully: %d agents, %d MCP configs", result.InstalledAgents, result.InstalledMCPs)
	return nil
}

// CheckAndBootstrap checks for STN_BUNDLE_ID environment variable and bootstraps if set.
// Returns true if a bundle was bootstrapped, false otherwise.
func CheckAndBootstrap(ctx context.Context, cfg *config.Config, repos *repositories.Repositories) (bool, error) {
	bundleID := os.Getenv("STN_BUNDLE_ID")
	if bundleID == "" {
		return false, nil
	}

	log.Printf("🔍 Found STN_BUNDLE_ID environment variable: %s", bundleID)

	bootstrapSvc := NewBundleBootstrapService(cfg, repos)
	if err := bootstrapSvc.BootstrapFromCloudShip(ctx, bundleID); err != nil {
		return false, err
	}

	return true, nil
}

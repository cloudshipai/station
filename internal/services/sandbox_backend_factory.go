package services

import (
	"fmt"
	"os"

	"station/internal/config"
)

type SandboxBackendType string

const (
	SandboxBackendDocker      SandboxBackendType = "docker"
	SandboxBackendFlyMachines SandboxBackendType = "fly_machines"
	SandboxBackendOpenCode    SandboxBackendType = "opencode"
	SandboxBackendHost        SandboxBackendType = "host"
)

func NewSandboxBackendFromConfig(cfg *config.Config, codeModeConfig CodeModeConfig) (SandboxBackend, error) {
	backendType := SandboxBackendType(cfg.Sandbox.Backend)
	if backendType == "" {
		backendType = SandboxBackendDocker
	}

	switch backendType {
	case SandboxBackendDocker:
		return NewDockerBackend(codeModeConfig)

	case SandboxBackendFlyMachines:
		image := cfg.Sandbox.FlyMachines.Image
		if image == "" {
			image = cfg.Sandbox.DockerImage
		}
		if image == "" {
			image = codeModeConfig.DefaultImage
		}
		if image == "" {
			image = "python:3.11-slim"
		}

		flyConfig := FlyMachinesConfig{
			Enabled:        true,
			APIToken:       os.Getenv("FLY_API_TOKEN"),
			OrgSlug:        cfg.Sandbox.FlyMachines.OrgSlug,
			AppPrefix:      cfg.Sandbox.FlyMachines.AppPrefix,
			Region:         cfg.Sandbox.FlyMachines.Region,
			DefaultImage:   image,
			DefaultTimeout: codeModeConfig.DefaultTimeout,
			MaxStdoutBytes: codeModeConfig.MaxStdoutBytes,
			MemoryMB:       cfg.Sandbox.FlyMachines.MemoryMB,
			CPUKind:        cfg.Sandbox.FlyMachines.CPUKind,
			CPUs:           cfg.Sandbox.FlyMachines.CPUs,
			RegistryAuth: FlyRegistryAuth{
				Username:      cfg.Sandbox.FlyMachines.RegistryAuth.Username,
				Password:      cfg.Sandbox.FlyMachines.RegistryAuth.Password,
				ServerAddress: cfg.Sandbox.FlyMachines.RegistryAuth.ServerAddress,
			},
		}

		if flyConfig.OrgSlug == "" {
			flyConfig.OrgSlug = os.Getenv("FLY_ORG")
		}
		if flyConfig.AppPrefix == "" {
			flyConfig.AppPrefix = "stn-sandbox"
		}
		if flyConfig.Region == "" {
			flyConfig.Region = "ord"
		}
		if flyConfig.MemoryMB == 0 {
			flyConfig.MemoryMB = 256
		}
		if flyConfig.CPUKind == "" {
			flyConfig.CPUKind = "shared"
		}
		if flyConfig.CPUs == 0 {
			flyConfig.CPUs = 1
		}

		return NewFlyMachinesBackend(flyConfig)

	case SandboxBackendOpenCode:
		openCodeConfig := OpenCodeConfig{
			Enabled:                cfg.Sandbox.OpenCodeEnabled,
			ServerURL:              cfg.Sandbox.OpenCodeServerURL,
			DefaultTimeout:         codeModeConfig.DefaultTimeout,
			MaxStdoutBytes:         codeModeConfig.MaxStdoutBytes,
			WorkspaceHostPath:      "/tmp/station-opencode-workspaces",
			WorkspaceContainerPath: "/workspaces",
			Model:                  cfg.Sandbox.OpenCodeModel,
		}
		return NewOpenCodeBackend(openCodeConfig)

	case SandboxBackendHost:
		return nil, fmt.Errorf("host backend not yet implemented")

	default:
		return nil, fmt.Errorf("unknown sandbox backend: %s", backendType)
	}
}

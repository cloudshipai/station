// Station CI/CD Dagger Module
package main

import (
	"context"
	"fmt"
	"os"
)

type StationCI struct{}

// BuildStationImage creates a Station container with CI agents pre-configured
func (m *StationCI) BuildStationImage(
	ctx context.Context,
	// +optional
	baseImage string,
) *Container {
	if baseImage == "" {
		baseImage = "station-base:latest"
	}

	return dag.Container().
		From(baseImage).
		WithDirectory("/app/environment/agents", 
			dag.Host().Directory("dagger/agents")).
		WithFile("/root/.config/station/config.yaml", 
			dag.Host().File("dagger/config.yml")).
		WithFile("/app/environment/variables.yml", 
			dag.Host().File("dagger/variables.yml"))
}

// SecurityScan performs comprehensive security analysis using Station agents
func (m *StationCI) SecurityScan(
	ctx context.Context,
	source *Directory,
	// +optional
	openaiKey *Secret,
	// +optional
	encryptionKey *Secret,
) *Container {
	stationImage := m.BuildStationImage(ctx, "")
	
	container := stationImage.
		WithDirectory("/workspace", source).
		WithDirectory("/workspace/reports", dag.Directory())

	if openaiKey != nil {
		container = container.WithSecretVariable("OPENAI_API_KEY", openaiKey)
	}
	if encryptionKey != nil {
		container = container.WithSecretVariable("ENCRYPTION_KEY", encryptionKey)
	}

	// Run security analysis agent
	return container.
		WithExec([]string{
			"stn", "agent", "run", "security-scanner",
			"--input", "Perform comprehensive security analysis of /workspace and generate reports",
		})
}

// TerraformAnalysis performs Terraform-specific security and cost analysis
func (m *StationCI) TerraformAnalysis(
	ctx context.Context,
	source *Directory,
	// +optional
	openaiKey *Secret,
	// +optional
	encryptionKey *Secret,
) *Container {
	stationImage := m.BuildStationImage(ctx, "")
	
	container := stationImage.
		WithDirectory("/workspace", source).
		WithDirectory("/workspace/reports", dag.Directory())

	if openaiKey != nil {
		container = container.WithSecretVariable("OPENAI_API_KEY", openaiKey)
	}
	if encryptionKey != nil {
		container = container.WithSecretVariable("ENCRYPTION_KEY", encryptionKey)
	}

	return container.
		WithExec([]string{
			"stn", "agent", "run", "terraform-analyzer",
			"--input", "Analyze Terraform files for security, cost, and compliance issues",
		})
}

// SBOMGeneration creates software bill of materials and vulnerability reports
func (m *StationCI) SBOMGeneration(
	ctx context.Context,
	source *Directory,
	// +optional
	openaiKey *Secret,
	// +optional
	encryptionKey *Secret,
) *Container {
	stationImage := m.BuildStationImage(ctx, "")
	
	container := stationImage.
		WithDirectory("/workspace", source).
		WithDirectory("/workspace/reports", dag.Directory())

	if openaiKey != nil {
		container = container.WithSecretVariable("OPENAI_API_KEY", openaiKey)
	}
	if encryptionKey != nil {
		container = container.WithSecretVariable("ENCRYPTION_KEY", encryptionKey)
	}

	return container.
		WithExec([]string{
			"stn", "agent", "run", "sbom-generator",
			"--input", "Generate SBOM and perform vulnerability analysis for /workspace",
		})
}

// ComplianceCheck runs compliance and policy validation
func (m *StationCI) ComplianceCheck(
	ctx context.Context,
	source *Directory,
	// +optional
	framework string,
	// +optional
	openaiKey *Secret,
	// +optional
	encryptionKey *Secret,
) *Container {
	if framework == "" {
		framework = "CIS"
	}

	stationImage := m.BuildStationImage(ctx, "")
	
	container := stationImage.
		WithDirectory("/workspace", source).
		WithDirectory("/workspace/reports", dag.Directory()).
		WithEnvVariable("COMPLIANCE_FRAMEWORK", framework)

	if openaiKey != nil {
		container = container.WithSecretVariable("OPENAI_API_KEY", openaiKey)
	}
	if encryptionKey != nil {
		container = container.WithSecretVariable("ENCRYPTION_KEY", encryptionKey)
	}

	return container.
		WithExec([]string{
			"stn", "agent", "run", "compliance-checker",
			"--input", fmt.Sprintf("Run %s compliance checks on /workspace infrastructure", framework),
		})
}

// FullPipeline runs all analysis steps and consolidates reports
func (m *StationCI) FullPipeline(
	ctx context.Context,
	source *Directory,
	openaiKey *Secret,
	encryptionKey *Secret,
) *Container {
	// Run all analysis steps in parallel
	securityResults := m.SecurityScan(ctx, source, openaiKey, encryptionKey)
	terraformResults := m.TerraformAnalysis(ctx, source, openaiKey, encryptionKey)
	sbomResults := m.SBOMGeneration(ctx, source, openaiKey, encryptionKey)
	complianceResults := m.ComplianceCheck(ctx, source, "", openaiKey, encryptionKey)

	// Consolidate all reports
	return dag.Container().
		From("station-base:latest").
		WithDirectory("/reports/security", securityResults.Directory("/workspace/reports")).
		WithDirectory("/reports/terraform", terraformResults.Directory("/workspace/reports")).
		WithDirectory("/reports/sbom", sbomResults.Directory("/workspace/reports")).
		WithDirectory("/reports/compliance", complianceResults.Directory("/workspace/reports")).
		WithExec([]string{
			"stn", "agent", "run", "report-consolidator",
			"--input", "Consolidate all analysis reports in /reports into a master security report",
		})
}

// GetReports extracts all generated reports from the pipeline
func (m *StationCI) GetReports(
	ctx context.Context,
	pipeline *Container,
) *Directory {
	return pipeline.Directory("/reports")
}
package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"

	"station/pkg/bundle"
	"station/pkg/bundle/creator"
	"station/pkg/bundle/packager"
	"station/pkg/bundle/validator"
)

// BundleCLI provides CLI operations for bundle development
type BundleCLI struct {
	fs        afero.Fs
	creator   bundle.BundleCreator
	validator bundle.BundleValidator
	packager  bundle.BundlePackager
}

// NewBundleCLI creates a new bundle CLI
func NewBundleCLI(fs afero.Fs) *BundleCLI {
	if fs == nil {
		fs = afero.NewOsFs()
	}

	return &BundleCLI{
		fs:        fs,
		creator:   creator.NewCreator(),
		validator: validator.NewValidator(),
		packager:  packager.NewPackager(validator.NewValidator()),
	}
}

// CreateBundle creates a new bundle template (stn template create)
func (c *BundleCLI) CreateBundle(bundlePath string, opts bundle.CreateOptions) error {
	// Validate bundle path
	if bundlePath == "" {
		return fmt.Errorf("bundle path is required")
	}

	// Check if directory already exists
	exists, err := afero.DirExists(c.fs, bundlePath)
	if err != nil {
		return fmt.Errorf("failed to check bundle directory: %w", err)
	}
	if exists {
		return fmt.Errorf("bundle directory already exists: %s", bundlePath)
	}

	// Create the bundle
	if err := c.creator.Create(c.fs, bundlePath, opts); err != nil {
		return fmt.Errorf("failed to create bundle: %w", err)
	}

	fmt.Printf("✅ Bundle created successfully at: %s\n", bundlePath)
	fmt.Printf("📝 Next steps:\n")
	fmt.Printf("   1. Edit template.json with your MCP server configuration\n")
	fmt.Printf("   2. Update variables.schema.json if you use template variables\n")
	fmt.Printf("   3. Run 'stn template validate %s' to test your bundle\n", bundlePath)
	fmt.Printf("   4. Run 'stn template bundle %s' to package for distribution\n", bundlePath)

	return nil
}

// ValidateBundle validates a bundle and checks variable consistency (stn template validate)
func (c *BundleCLI) ValidateBundle(bundlePath string) (*ValidationSummary, error) {
	// Validate bundle path
	exists, err := afero.DirExists(c.fs, bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to check bundle directory: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("bundle directory does not exist: %s", bundlePath)
	}

	// Run validation
	result, err := c.validator.Validate(c.fs, bundlePath)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create summary
	summary := &ValidationSummary{
		BundlePath:       bundlePath,
		Valid:            result.Valid,
		Issues:           result.Issues,
		Warnings:         result.Warnings,
		VariableAnalysis: c.analyzeVariableConsistency(result),
	}

	return summary, nil
}

// PackageBundle creates a distributable package from a bundle (stn template bundle)
func (c *BundleCLI) PackageBundle(bundlePath, outputPath string, validateFirst bool) (*PackageSummary, error) {
	// Validate first if requested
	if validateFirst {
		validationSummary, err := c.ValidateBundle(bundlePath)
		if err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
		if !validationSummary.Valid {
			return &PackageSummary{
				Success:           false,
				ValidationSummary: validationSummary,
				Error:             fmt.Sprintf("Bundle has %d validation issues", len(validationSummary.Issues)),
			}, nil
		}
	}

	// Set default output path if not provided
	if outputPath == "" {
		bundleName := filepath.Base(bundlePath)
		outputPath = bundleName + ".tar.gz"
	}

	// Package the bundle
	result, err := c.packager.Package(c.fs, bundlePath, outputPath)
	if err != nil {
		return nil, fmt.Errorf("packaging failed: %w", err)
	}

	summary := &PackageSummary{
		Success:    result.Success,
		OutputPath: result.OutputPath,
		Size:       result.Size,
	}

	if validateFirst {
		validationSummary, _ := c.ValidateBundle(bundlePath)
		summary.ValidationSummary = validationSummary
	}

	return summary, nil
}

// PrintValidationSummary prints a formatted validation summary
func (c *BundleCLI) PrintValidationSummary(summary *ValidationSummary) {
	if summary.Valid {
		fmt.Printf("✅ Bundle validation: %s\n", colorGreen("PASSED"))
	} else {
		fmt.Printf("❌ Bundle validation: %s\n", colorRed("FAILED"))
	}

	fmt.Printf("📁 Bundle path: %s\n", summary.BundlePath)

	// Print issues
	if len(summary.Issues) > 0 {
		fmt.Printf("\n🚨 Issues (%d):\n", len(summary.Issues))
		for _, issue := range summary.Issues {
			fmt.Printf("   • %s: %s\n", colorRed(issue.Type), issue.Message)
			if issue.File != "" {
				fmt.Printf("     File: %s\n", issue.File)
			}
			if issue.Suggestion != "" {
				fmt.Printf("     💡 %s\n", issue.Suggestion)
			}
		}
	}

	// Print warnings
	if len(summary.Warnings) > 0 {
		fmt.Printf("\n⚠️  Warnings (%d):\n", len(summary.Warnings))
		for _, warning := range summary.Warnings {
			fmt.Printf("   • %s: %s\n", colorYellow(warning.Type), warning.Message)
			if warning.File != "" {
				fmt.Printf("     File: %s\n", warning.File)
			}
			if warning.Suggestion != "" {
				fmt.Printf("     💡 %s\n", warning.Suggestion)
			}
		}
	}

	// Print variable analysis
	if summary.VariableAnalysis != nil {
		c.printVariableAnalysis(summary.VariableAnalysis)
	}

	// Print next steps
	if summary.Valid {
		fmt.Printf("\n🎉 Bundle is ready for distribution!\n")
		fmt.Printf("📦 Run 'stn template bundle %s' to create a package\n", summary.BundlePath)
	} else {
		fmt.Printf("\n🔧 Fix the issues above and run validation again\n")
	}
}

// PrintPackageSummary prints a formatted packaging summary
func (c *BundleCLI) PrintPackageSummary(summary *PackageSummary) {
	if summary.Success {
		fmt.Printf("✅ Bundle packaging: %s\n", colorGreen("SUCCESS"))
		fmt.Printf("📦 Package created: %s\n", summary.OutputPath)
		fmt.Printf("📊 Package size: %s\n", formatBytes(summary.Size))
		fmt.Printf("📤 You can now upload it to a registry or share directly\n")
	} else {
		fmt.Printf("❌ Bundle packaging: %s\n", colorRed("FAILED"))
		if summary.Error != "" {
			fmt.Printf("💥 Error: %s\n", summary.Error)
		}

		if summary.ValidationSummary != nil {
			c.PrintValidationSummary(summary.ValidationSummary)
		}
	}
}

// Helper methods

func (c *BundleCLI) analyzeVariableConsistency(result *bundle.ValidationResult) *VariableAnalysis {
	analysis := &VariableAnalysis{
		TemplateVariables: []string{},
		SchemaVariables:   []string{},
		MissingInSchema:   []string{},
		UnusedInSchema:    []string{},
	}

	// Extract variables mentioned in warnings/issues
	for _, warning := range result.Warnings {
		if warning.Type == "undefined_variable" && strings.Contains(warning.Message, "variable") {
			// Parse variable name from message
			if parts := strings.Split(warning.Message, "'"); len(parts) >= 2 {
				varName := parts[1]
				analysis.TemplateVariables = append(analysis.TemplateVariables, varName)
				analysis.MissingInSchema = append(analysis.MissingInSchema, varName)
			}
		}
	}

	return analysis
}

func (c *BundleCLI) printVariableAnalysis(analysis *VariableAnalysis) {
	if len(analysis.TemplateVariables) == 0 && len(analysis.SchemaVariables) == 0 {
		fmt.Printf("\n📋 Variables: %s\n", colorGreen("No template variables detected"))
		return
	}

	fmt.Printf("\n📋 Variable Analysis:\n")

	if len(analysis.TemplateVariables) > 0 {
	}

	if len(analysis.MissingInSchema) > 0 {
		fmt.Printf("   ❌ Missing from schema: %s\n", colorRed(strings.Join(analysis.MissingInSchema, ", ")))
	}

	if len(analysis.UnusedInSchema) > 0 {
		fmt.Printf("   ⚠️  Unused in template: %s\n", colorYellow(strings.Join(analysis.UnusedInSchema, ", ")))
	}

	if len(analysis.MissingInSchema) == 0 && len(analysis.UnusedInSchema) == 0 {
		fmt.Printf("   ✅ Variable consistency: %s\n", colorGreen("All variables properly defined"))
	}
}

// Types for CLI responses

type ValidationSummary struct {
	BundlePath       string                   `json:"bundle_path"`
	Valid            bool                     `json:"valid"`
	Issues           []bundle.ValidationIssue `json:"issues"`
	Warnings         []bundle.ValidationIssue `json:"warnings"`
	VariableAnalysis *VariableAnalysis        `json:"variable_analysis,omitempty"`
}

type PackageSummary struct {
	Success           bool               `json:"success"`
	OutputPath        string             `json:"output_path,omitempty"`
	Size              int64              `json:"size,omitempty"`
	Error             string             `json:"error,omitempty"`
	ValidationSummary *ValidationSummary `json:"validation_summary,omitempty"`
}

type VariableAnalysis struct {
	TemplateVariables []string `json:"template_variables"`
	SchemaVariables   []string `json:"schema_variables"`
	MissingInSchema   []string `json:"missing_in_schema"`
	UnusedInSchema    []string `json:"unused_in_schema"`
}

// Utility functions for colored output

func colorGreen(text string) string {
	return "\033[32m" + text + "\033[0m"
}

func colorRed(text string) string {
	return "\033[31m" + text + "\033[0m"
}

func colorYellow(text string) string {
	return "\033[33m" + text + "\033[0m"
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

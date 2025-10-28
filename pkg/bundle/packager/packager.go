package packager

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/afero"

	"station/pkg/bundle"
)

// Packager implements the BundlePackager interface
type Packager struct {
	validator bundle.BundleValidator
}

// NewPackager creates a new bundle packager
func NewPackager(validator bundle.BundleValidator) *Packager {
	return &Packager{
		validator: validator,
	}
}

// Package creates a .tar.gz package from a bundle directory
func (p *Packager) Package(fs afero.Fs, bundlePath, outputPath string) (*bundle.PackageResult, error) {
	// Validate bundle first
	validationResult, err := p.validator.Validate(fs, bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to validate bundle: %w", err)
	}

	if !validationResult.Valid {
		return &bundle.PackageResult{
			Success:          false,
			ValidationResult: validationResult,
		}, nil
	}

	// Create output file
	outputFile, err := fs.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() { _ = outputFile.Close() }()

	// Create gzip writer
	gzWriter := gzip.NewWriter(outputFile)
	defer func() { _ = gzWriter.Close() }()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer func() { _ = tarWriter.Close() }()

	// Add all files to the archive
	if err := p.addDirectoryToTar(fs, tarWriter, bundlePath, ""); err != nil {
		return nil, fmt.Errorf("failed to add files to archive: %w", err)
	}

	// Get package info
	stat, err := fs.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get package info: %w", err)
	}

	return &bundle.PackageResult{
		Success:          true,
		OutputPath:       outputPath,
		Size:             stat.Size(),
		ValidationResult: validationResult,
	}, nil
}

func (p *Packager) addDirectoryToTar(fs afero.Fs, tarWriter *tar.Writer, srcPath, destPath string) error {
	files, err := afero.ReadDir(fs, srcPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		srcFile := filepath.Join(srcPath, file.Name())
		destFile := filepath.Join(destPath, file.Name())

		if file.IsDir() {
			// Add directory header
			header := &tar.Header{
				Name:     destFile + "/",
				Mode:     int64(file.Mode()),
				ModTime:  file.ModTime(),
				Typeflag: tar.TypeDir,
			}
			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}

			// Recursively add directory contents
			if err := p.addDirectoryToTar(fs, tarWriter, srcFile, destFile); err != nil {
				return err
			}
		} else {
			// Add file
			if err := p.addFileToTar(fs, tarWriter, srcFile, destFile); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Packager) addFileToTar(fs afero.Fs, tarWriter *tar.Writer, srcFile, destFile string) error {
	file, err := fs.Open(srcFile)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	// Create tar header
	header := &tar.Header{
		Name:    destFile,
		Mode:    int64(stat.Mode()),
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
	}

	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Copy file content
	_, err = io.Copy(tarWriter, file)
	return err
}

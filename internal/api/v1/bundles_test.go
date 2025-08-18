package v1

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateTarGz(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "test-bundle-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files and directories
	testFiles := map[string]string{
		"fs.json":                    `{"name": "filesystem", "command": "npx @modelcontextprotocol/server-filesystem"}`,
		"agents/test-agent.prompt":   "You are a test agent",
		"agents/another-agent.prompt": "You are another test agent",
		"variables.yml":              "# This should be excluded\ntest: value",
	}

	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		
		// Create directory if needed
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", dir, err)
		}
		
		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	// Test the bundle creation
	tarData, err := createTarGz(tempDir)
	if err != nil {
		t.Fatalf("createTarGz failed: %v", err)
	}

	if len(tarData) == 0 {
		t.Fatal("Generated tar.gz is empty")
	}

	// Verify the tar.gz content
	reader := bytes.NewReader(tarData)
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	foundFiles := make(map[string]bool)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read tar entry: %v", err)
		}

		foundFiles[header.Name] = true
		t.Logf("Found file in bundle: %s", header.Name)

		// Read content to verify it's not corrupted
		if header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tarReader)
			if err != nil {
				t.Fatalf("Failed to read file content for %s: %v", header.Name, err)
			}
			if len(content) == 0 {
				t.Errorf("File %s has no content", header.Name)
			}
		}
	}

	// Verify expected files are present
	expectedFiles := []string{
		"fs.json",
		"agents/test-agent.prompt",
		"agents/another-agent.prompt",
	}

	for _, expected := range expectedFiles {
		if !foundFiles[expected] {
			t.Errorf("Expected file %s not found in bundle", expected)
		}
	}

	// Verify variables.yml is excluded
	if foundFiles["variables.yml"] {
		t.Error("variables.yml should be excluded from bundle")
	}

	t.Logf("Bundle created successfully with %d bytes", len(tarData))
	t.Logf("Found %d files in bundle", len(foundFiles))
}

func TestCreateTarGzWithEmptyDirectory(t *testing.T) {
	// Create empty temp directory
	tempDir, err := os.MkdirTemp("", "test-empty-bundle-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with empty directory
	tarData, err := createTarGz(tempDir)
	if err != nil {
		t.Fatalf("createTarGz failed with empty directory: %v", err)
	}

	// Should create a valid empty tar.gz
	if len(tarData) == 0 {
		t.Fatal("Generated tar.gz is empty")
	}

	// Verify it's a valid tar.gz with no entries
	reader := bytes.NewReader(tarData)
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	
	// Should have no entries
	_, err = tarReader.Next()
	if err != io.EOF {
		t.Errorf("Expected EOF for empty directory, got: %v", err)
	}

	t.Logf("Empty bundle created successfully with %d bytes", len(tarData))
}
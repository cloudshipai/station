package tools

import (
	"testing"
)

func TestValidatePath(t *testing.T) {
	workspace := "/home/user/project"

	tests := []struct {
		name      string
		path      string
		wantErr   bool
		wantPath  string
	}{
		// Valid paths
		{"relative path", "src/main.go", false, "/home/user/project/src/main.go"},
		{"absolute in workspace", "/home/user/project/src/main.go", false, "/home/user/project/src/main.go"},
		{"nested path", "src/components/Button.tsx", false, "/home/user/project/src/components/Button.tsx"},
		{"workspace root", ".", false, "/home/user/project"},

		// Traversal attacks that should be blocked
		{"parent traversal", "../../../etc/passwd", true, ""},
		{"hidden traversal", "src/../../etc/passwd", true, ""},
		{"absolute outside", "/etc/passwd", true, ""},
		{"home directory", "/home/user", true, ""},
		{"root", "/", true, ""},

		// Empty path
		{"empty path", "", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatePath(tt.path, workspace)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr = %v", tt.path, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantPath {
				t.Errorf("ValidatePath(%q) = %q, want %q", tt.path, got, tt.wantPath)
			}
		})
	}
}

func TestValidatePathForWrite_DangerousFiles(t *testing.T) {
	workspace := "/home/user/project"

	dangerousFiles := []string{
		".bashrc",
		".bash_profile",
		".profile",
		".zshrc",
		".gitconfig",
	}

	for _, file := range dangerousFiles {
		t.Run(file, func(t *testing.T) {
			_, err := ValidatePathForWrite(file, workspace)
			if err == nil {
				t.Errorf("ValidatePathForWrite(%q) should block dangerous file", file)
			}
		})
	}
}

func TestValidatePathForWrite_SafeFiles(t *testing.T) {
	workspace := "/home/user/project"

	safeFiles := []string{
		"main.go",
		"src/app.py",
		"config.json",
		".env.example",
		".github/workflows/ci.yml",
	}

	for _, file := range safeFiles {
		t.Run(file, func(t *testing.T) {
			path, err := ValidatePathForWrite(file, workspace)
			if err != nil {
				t.Errorf("ValidatePathForWrite(%q) should allow safe file, got error: %v", file, err)
			}
			if path == "" {
				t.Errorf("ValidatePathForWrite(%q) returned empty path", file)
			}
		})
	}
}

func TestIsSafePath(t *testing.T) {
	workspace := "/home/user/project"

	tests := []struct {
		path string
		safe bool
	}{
		{"src/main.go", true},
		{"../../../etc/passwd", false},
		{"/etc/passwd", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := IsSafePath(tt.path, workspace); got != tt.safe {
				t.Errorf("IsSafePath(%q) = %v, want %v", tt.path, got, tt.safe)
			}
		})
	}
}

func TestPathValidationError(t *testing.T) {
	err := &PathValidationError{
		Path:   "../../../etc/passwd",
		Reason: "path escapes workspace",
	}

	expected := `path validation failed for "../../../etc/passwd": path escapes workspace`
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

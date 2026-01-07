package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHostWorkspace_Initialize(t *testing.T) {
	tmpDir := t.TempDir()

	w := NewHostWorkspace(tmpDir)
	ctx := context.Background()

	if err := w.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if !w.initialized {
		t.Error("workspace should be initialized")
	}

	if w.Path() != tmpDir {
		t.Errorf("Path() = %q, want %q", w.Path(), tmpDir)
	}
}

func TestHostWorkspace_ReadWriteFile(t *testing.T) {
	tmpDir := t.TempDir()

	w := NewHostWorkspace(tmpDir)
	ctx := context.Background()

	if err := w.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	content := []byte("hello world")

	if err := w.WriteFile(ctx, "test.txt", content); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	read, err := w.ReadFile(ctx, "test.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(read) != string(content) {
		t.Errorf("ReadFile returned %q, want %q", string(read), string(content))
	}
}

func TestHostWorkspace_NestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	w := NewHostWorkspace(tmpDir)
	ctx := context.Background()

	if err := w.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	content := []byte("nested content")

	if err := w.WriteFile(ctx, "a/b/c/test.txt", content); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "a/b/c/test.txt")); os.IsNotExist(err) {
		t.Error("nested file was not created")
	}
}

func TestHostWorkspace_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()

	w := NewHostWorkspace(tmpDir)
	ctx := context.Background()

	if err := w.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	_, err := w.ReadFile(ctx, "../../../etc/passwd")
	if err == nil {
		t.Error("expected error for path traversal")
	}
}

func TestHostWorkspace_DeniedPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	w := NewHostWorkspace(tmpDir, WithDeniedPatterns([]string{"*.secret", "config/*.env"}))
	ctx := context.Background()

	if err := w.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	os.WriteFile(filepath.Join(tmpDir, "test.secret"), []byte("secret"), 0644)

	_, err := w.ReadFile(ctx, "test.secret")
	if err == nil {
		t.Error("expected error for denied pattern *.secret")
	}
}

func TestHostWorkspace_Exists(t *testing.T) {
	tmpDir := t.TempDir()

	w := NewHostWorkspace(tmpDir)
	ctx := context.Background()

	if err := w.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	exists, err := w.Exists(ctx, "nonexistent.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("nonexistent file should not exist")
	}

	w.WriteFile(ctx, "exists.txt", []byte("content"))

	exists, err = w.Exists(ctx, "exists.txt")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("created file should exist")
	}
}

func TestHostWorkspace_ListFiles(t *testing.T) {
	tmpDir := t.TempDir()

	w := NewHostWorkspace(tmpDir)
	ctx := context.Background()

	if err := w.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	w.WriteFile(ctx, "file1.txt", []byte("1"))
	w.WriteFile(ctx, "file2.txt", []byte("2"))
	w.WriteFile(ctx, "file3.txt", []byte("3"))

	files, err := w.ListFiles(ctx, ".")
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("ListFiles returned %d files, want 3", len(files))
	}
}

func TestHostWorkspace_DeleteFile(t *testing.T) {
	tmpDir := t.TempDir()

	w := NewHostWorkspace(tmpDir)
	ctx := context.Background()

	if err := w.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	w.WriteFile(ctx, "todelete.txt", []byte("content"))

	if err := w.DeleteFile(ctx, "todelete.txt"); err != nil {
		t.Fatalf("DeleteFile failed: %v", err)
	}

	exists, _ := w.Exists(ctx, "todelete.txt")
	if exists {
		t.Error("deleted file should not exist")
	}
}

func TestHostWorkspace_IsInWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	w := NewHostWorkspace(tmpDir)
	ctx := context.Background()
	w.Initialize(ctx)

	if !w.IsInWorkspace("subdir/file.txt") {
		t.Error("relative path should be in workspace")
	}

	if w.IsInWorkspace("/etc/passwd") {
		t.Error("absolute path outside workspace should not be in workspace")
	}
}

func TestHostWorkspace_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()

	w := NewHostWorkspace(tmpDir)
	ctx := context.Background()

	_, err := w.ReadFile(ctx, "test.txt")
	if err == nil {
		t.Error("expected error when workspace not initialized")
	}
}

package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskManager_CreateAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTaskManager(tmpDir)

	task, err := tm.Create("Test task", "Task description", "Working on test")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if task.ID == "" {
		t.Error("Task ID should not be empty")
	}
	if task.Subject != "Test task" {
		t.Errorf("Subject = %q, want %q", task.Subject, "Test task")
	}
	if task.Status != TaskStatusPending {
		t.Errorf("Status = %q, want %q", task.Status, TaskStatusPending)
	}

	retrieved, err := tm.Get(task.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if retrieved.Subject != task.Subject {
		t.Errorf("Retrieved subject = %q, want %q", retrieved.Subject, task.Subject)
	}
}

func TestTaskManager_Update(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTaskManager(tmpDir)

	task, _ := tm.Create("Original", "Description", "")

	updated, err := tm.Update(task.ID, map[string]any{
		"status":  "in_progress",
		"subject": "Updated subject",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if updated.Status != TaskStatusInProgress {
		t.Errorf("Status = %q, want %q", updated.Status, TaskStatusInProgress)
	}
	if updated.Subject != "Updated subject" {
		t.Errorf("Subject = %q, want %q", updated.Subject, "Updated subject")
	}
}

func TestTaskManager_List(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTaskManager(tmpDir)

	tm.Create("Task 1", "", "")
	tm.Create("Task 2", "", "")
	tm.Create("Task 3", "", "")

	tasks := tm.List()
	if len(tasks) != 3 {
		t.Errorf("List returned %d tasks, want 3", len(tasks))
	}
}

func TestTaskManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create tasks with first manager
	tm1 := NewTaskManager(tmpDir)
	task, _ := tm1.Create("Persisted task", "Description", "")
	tm1.Update(task.ID, map[string]any{"status": "completed"})

	// Load with new manager
	tm2 := NewTaskManager(tmpDir)
	tasks := tm2.List()

	if len(tasks) != 1 {
		t.Errorf("List returned %d tasks after reload, want 1", len(tasks))
	}
	if tasks[0].Status != TaskStatusCompleted {
		t.Errorf("Status = %q, want %q", tasks[0].Status, TaskStatusCompleted)
	}
}

func TestTaskManager_GetSummary(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTaskManager(tmpDir)

	summary := tm.GetSummary()
	if summary != "No tasks tracked." {
		t.Errorf("Empty summary = %q, want %q", summary, "No tasks tracked.")
	}

	tm.Create("Task 1", "", "")
	task2, _ := tm.Create("Task 2", "", "")
	tm.Update(task2.ID, map[string]any{"status": "in_progress"})
	task3, _ := tm.Create("Task 3", "", "")
	tm.Update(task3.ID, map[string]any{"status": "completed"})

	summary = tm.GetSummary()
	expected := "Tasks: 1 pending, 1 in progress, 1 completed (total: 3)"
	if summary != expected {
		t.Errorf("Summary = %q, want %q", summary, expected)
	}
}

func TestTaskManager_FileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTaskManager(tmpDir)

	tm.Create("Task", "", "")

	filePath := filepath.Join(tmpDir, ".harness", "tasks.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Errorf("tasks.json file was not created at %s", filePath)
	}
}

func TestTaskManager_Dependencies(t *testing.T) {
	tmpDir := t.TempDir()
	tm := NewTaskManager(tmpDir)

	task1, _ := tm.Create("Task 1", "", "")
	task2, _ := tm.Create("Task 2", "", "")

	tm.Update(task1.ID, map[string]any{
		"add_blocks": []string{task2.ID},
	})

	updated, _ := tm.Get(task1.ID)
	if len(updated.Blocks) != 1 || updated.Blocks[0] != task2.ID {
		t.Errorf("Blocks = %v, want [%s]", updated.Blocks, task2.ID)
	}
}

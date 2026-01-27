package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
	"github.com/google/uuid"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
)

// Task represents a trackable work item
type Task struct {
	ID          string     `json:"id"`
	Subject     string     `json:"subject"`
	Description string     `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TaskManager manages tasks for a workspace
type TaskManager struct {
	mu            sync.RWMutex
	tasks         map[string]*Task
	workspacePath string
}

// NewTaskManager creates a new task manager
func NewTaskManager(workspacePath string) *TaskManager {
	tm := &TaskManager{
		tasks:         make(map[string]*Task),
		workspacePath: workspacePath,
	}
	tm.load()
	return tm
}

func (tm *TaskManager) tasksFilePath() string {
	return filepath.Join(tm.workspacePath, ".harness", "tasks.json")
}

func (tm *TaskManager) load() {
	data, err := os.ReadFile(tm.tasksFilePath())
	if err != nil {
		return
	}
	var taskList struct {
		Tasks []*Task `json:"tasks"`
	}
	if err := json.Unmarshal(data, &taskList); err != nil {
		return
	}
	for _, t := range taskList.Tasks {
		tm.tasks[t.ID] = t
	}
}

func (tm *TaskManager) save() error {
	dir := filepath.Dir(tm.tasksFilePath())
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var taskList []*Task
	for _, t := range tm.tasks {
		taskList = append(taskList, t)
	}

	data, err := json.MarshalIndent(struct {
		Tasks []*Task `json:"tasks"`
	}{Tasks: taskList}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tm.tasksFilePath(), data, 0644)
}

// Create creates a new task
func (tm *TaskManager) Create(subject, description string) (*Task, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task := &Task{
		ID:          uuid.New().String()[:8],
		Subject:     subject,
		Description: description,
		Status:      TaskStatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	tm.tasks[task.ID] = task
	if err := tm.save(); err != nil {
		return nil, err
	}
	return task, nil
}

// Get retrieves a task by ID
func (tm *TaskManager) Get(id string) (*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, ok := tm.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %q not found", id)
	}
	return task, nil
}

// Update updates a task's status
func (tm *TaskManager) Update(id string, status TaskStatus) (*Task, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %q not found", id)
	}
	task.Status = status
	task.UpdatedAt = time.Now()
	if err := tm.save(); err != nil {
		return nil, err
	}
	return task, nil
}

// List returns all tasks
func (tm *TaskManager) List() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var tasks []*Task
	for _, t := range tm.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// Task tool input/output types
type TaskCreateInput struct {
	Subject     string `json:"subject" jsonschema:"description=Brief title for the task"`
	Description string `json:"description" jsonschema:"description=Detailed description of what needs to be done"`
}

type TaskUpdateInput struct {
	TaskID string     `json:"task_id" jsonschema:"description=ID of the task to update"`
	Status TaskStatus `json:"status" jsonschema:"description=New status (pending, in_progress, completed)"`
}

type TaskGetInput struct {
	TaskID string `json:"task_id" jsonschema:"description=ID of the task to retrieve"`
}

type TaskOutput struct {
	Task    *Task   `json:"task,omitempty"`
	Tasks   []*Task `json:"tasks,omitempty"`
	Message string  `json:"message,omitempty"`
}

// NewTaskCreateTool creates a tool for creating tasks
func NewTaskCreateTool(genkitApp *genkit.Genkit, tm *TaskManager) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"task_create",
		`Create a new task to track work.

Use this to track work items you need to complete.`,
		func(ctx *ai.ToolContext, input TaskCreateInput) (TaskOutput, error) {
			if input.Subject == "" {
				return TaskOutput{}, fmt.Errorf("subject is required")
			}
			task, err := tm.Create(input.Subject, input.Description)
			if err != nil {
				return TaskOutput{}, err
			}
			return TaskOutput{
				Task:    task,
				Message: fmt.Sprintf("Created task %q", task.ID),
			}, nil
		},
	)
}

// NewTaskUpdateTool creates a tool for updating tasks
func NewTaskUpdateTool(genkitApp *genkit.Genkit, tm *TaskManager) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"task_update",
		`Update a task's status.

Status values:
- pending: Task not yet started
- in_progress: Task currently being worked on
- completed: Task finished`,
		func(ctx *ai.ToolContext, input TaskUpdateInput) (TaskOutput, error) {
			if input.TaskID == "" {
				return TaskOutput{}, fmt.Errorf("task_id is required")
			}
			task, err := tm.Update(input.TaskID, input.Status)
			if err != nil {
				return TaskOutput{}, err
			}
			return TaskOutput{
				Task:    task,
				Message: fmt.Sprintf("Updated task %q to %s", task.ID, task.Status),
			}, nil
		},
	)
}

// NewTaskGetTool creates a tool for getting a single task
func NewTaskGetTool(genkitApp *genkit.Genkit, tm *TaskManager) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"task_get",
		`Get details of a specific task by ID.`,
		func(ctx *ai.ToolContext, input TaskGetInput) (TaskOutput, error) {
			if input.TaskID == "" {
				return TaskOutput{}, fmt.Errorf("task_id is required")
			}
			task, err := tm.Get(input.TaskID)
			if err != nil {
				return TaskOutput{}, err
			}
			return TaskOutput{Task: task}, nil
		},
	)
}

// NewTaskListTool creates a tool for listing all tasks
func NewTaskListTool(genkitApp *genkit.Genkit, tm *TaskManager) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"task_list",
		`List all tasks in the current session.

Returns all tasks with their IDs, subjects, and statuses.`,
		func(ctx *ai.ToolContext, input struct{}) (TaskOutput, error) {
			tasks := tm.List()
			return TaskOutput{
				Tasks:   tasks,
				Message: fmt.Sprintf("Found %d tasks", len(tasks)),
			}, nil
		},
	)
}

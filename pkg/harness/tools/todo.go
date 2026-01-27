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

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
)

type Task struct {
	ID          string     `json:"id"`
	Subject     string     `json:"subject"`
	Description string     `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	ActiveForm  string     `json:"active_form,omitempty"`
	Blocks      []string   `json:"blocks,omitempty"`
	BlockedBy   []string   `json:"blocked_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type TaskManager struct {
	mu            sync.RWMutex
	tasks         map[string]*Task
	taskOrder     []string
	filePath      string
	workspacePath string
}

func NewTaskManager(workspacePath string) *TaskManager {
	tm := &TaskManager{
		tasks:         make(map[string]*Task),
		taskOrder:     []string{},
		workspacePath: workspacePath,
		filePath:      filepath.Join(workspacePath, ".harness", "tasks.json"),
	}
	tm.load()
	return tm
}

func (tm *TaskManager) load() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	data, err := os.ReadFile(tm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var stored struct {
		Tasks     []*Task  `json:"tasks"`
		TaskOrder []string `json:"task_order"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}

	tm.tasks = make(map[string]*Task)
	for _, task := range stored.Tasks {
		tm.tasks[task.ID] = task
	}
	tm.taskOrder = stored.TaskOrder

	return nil
}

func (tm *TaskManager) save() error {
	dir := filepath.Dir(tm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tasks := make([]*Task, 0, len(tm.taskOrder))
	for _, id := range tm.taskOrder {
		if task, ok := tm.tasks[id]; ok {
			tasks = append(tasks, task)
		}
	}

	stored := struct {
		Tasks     []*Task  `json:"tasks"`
		TaskOrder []string `json:"task_order"`
	}{
		Tasks:     tasks,
		TaskOrder: tm.taskOrder,
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(tm.filePath, data, 0644)
}

func (tm *TaskManager) Create(subject, description, activeForm string) (*Task, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task := &Task{
		ID:          uuid.New().String()[:8],
		Subject:     subject,
		Description: description,
		Status:      TaskStatusPending,
		ActiveForm:  activeForm,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	tm.tasks[task.ID] = task
	tm.taskOrder = append(tm.taskOrder, task.ID)

	if err := tm.save(); err != nil {
		delete(tm.tasks, task.ID)
		tm.taskOrder = tm.taskOrder[:len(tm.taskOrder)-1]
		return nil, err
	}

	return task, nil
}

func (tm *TaskManager) Get(id string) (*Task, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, ok := tm.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	return task, nil
}

func (tm *TaskManager) Update(id string, updates map[string]any) (*Task, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task, ok := tm.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	if subject, ok := updates["subject"].(string); ok && subject != "" {
		task.Subject = subject
	}
	if description, ok := updates["description"].(string); ok {
		task.Description = description
	}
	if status, ok := updates["status"].(string); ok {
		task.Status = TaskStatus(status)
	}
	if activeForm, ok := updates["active_form"].(string); ok {
		task.ActiveForm = activeForm
	}
	if blocks, ok := updates["add_blocks"].([]string); ok {
		task.Blocks = append(task.Blocks, blocks...)
	}
	if blockedBy, ok := updates["add_blocked_by"].([]string); ok {
		task.BlockedBy = append(task.BlockedBy, blockedBy...)
	}

	task.UpdatedAt = time.Now()

	if err := tm.save(); err != nil {
		return nil, err
	}

	return task, nil
}

func (tm *TaskManager) List() []*Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*Task, 0, len(tm.taskOrder))
	for _, id := range tm.taskOrder {
		if task, ok := tm.tasks[id]; ok {
			result = append(result, task)
		}
	}
	return result
}

func (tm *TaskManager) GetSummary() string {
	tasks := tm.List()
	if len(tasks) == 0 {
		return "No tasks tracked."
	}

	var pending, inProgress, completed int
	for _, t := range tasks {
		switch t.Status {
		case TaskStatusPending:
			pending++
		case TaskStatusInProgress:
			inProgress++
		case TaskStatusCompleted:
			completed++
		}
	}

	return fmt.Sprintf("Tasks: %d pending, %d in progress, %d completed (total: %d)",
		pending, inProgress, completed, len(tasks))
}

type TaskCreateInput struct {
	Subject     string `json:"subject" jsonschema:"description=Brief title for the task (imperative form e.g. 'Fix the bug')"`
	Description string `json:"description" jsonschema:"description=Detailed description of what needs to be done"`
	ActiveForm  string `json:"active_form,omitempty" jsonschema:"description=Present continuous form shown when task is in_progress (e.g. 'Fixing the bug')"`
}

type TaskCreateOutput struct {
	Task    *Task  `json:"task"`
	Message string `json:"message"`
}

type TaskUpdateInput struct {
	TaskID      string   `json:"task_id" jsonschema:"description=ID of the task to update"`
	Subject     string   `json:"subject,omitempty" jsonschema:"description=New subject for the task"`
	Description string   `json:"description,omitempty" jsonschema:"description=New description for the task"`
	Status      string   `json:"status,omitempty" jsonschema:"description=New status: pending, in_progress, or completed"`
	ActiveForm  string   `json:"active_form,omitempty" jsonschema:"description=Present continuous form for spinner when in_progress"`
	AddBlocks   []string `json:"add_blocks,omitempty" jsonschema:"description=Task IDs that this task blocks"`
	AddBlockedBy []string `json:"add_blocked_by,omitempty" jsonschema:"description=Task IDs that block this task"`
}

type TaskUpdateOutput struct {
	Task    *Task  `json:"task"`
	Message string `json:"message"`
}

type TaskGetInput struct {
	TaskID string `json:"task_id" jsonschema:"description=ID of the task to retrieve"`
}

type TaskGetOutput struct {
	Task *Task `json:"task"`
}

type TaskListInput struct{}

type TaskListOutput struct {
	Tasks   []*Task `json:"tasks"`
	Summary string  `json:"summary"`
}

func NewTaskCreateTool(genkitApp *genkit.Genkit, tm *TaskManager) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"task_create",
		`Create a task to track work. Use this for:
- Complex multi-step tasks requiring 3+ steps
- Non-trivial tasks needing planning
- When user provides multiple tasks

Each task has: subject (imperative), description (detailed), active_form (present continuous for spinner).
Tasks persist across sessions and context compaction.`,
		func(ctx *ai.ToolContext, input TaskCreateInput) (TaskCreateOutput, error) {
			if input.Subject == "" {
				return TaskCreateOutput{}, fmt.Errorf("subject is required")
			}

			task, err := tm.Create(input.Subject, input.Description, input.ActiveForm)
			if err != nil {
				return TaskCreateOutput{}, fmt.Errorf("failed to create task: %w", err)
			}

			return TaskCreateOutput{
				Task:    task,
				Message: fmt.Sprintf("Created task #%s: %s", task.ID, task.Subject),
			}, nil
		},
	)
}

func NewTaskUpdateTool(genkitApp *genkit.Genkit, tm *TaskManager) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"task_update",
		`Update a task's status or details. Use this to:
- Mark task as in_progress when starting work
- Mark task as completed after finishing
- Update task details or dependencies

Status progression: pending → in_progress → completed
Only mark completed when FULLY accomplished (not on errors/blockers).`,
		func(ctx *ai.ToolContext, input TaskUpdateInput) (TaskUpdateOutput, error) {
			if input.TaskID == "" {
				return TaskUpdateOutput{}, fmt.Errorf("task_id is required")
			}

			updates := make(map[string]any)
			if input.Subject != "" {
				updates["subject"] = input.Subject
			}
			if input.Description != "" {
				updates["description"] = input.Description
			}
			if input.Status != "" {
				if input.Status != "pending" && input.Status != "in_progress" && input.Status != "completed" {
					return TaskUpdateOutput{}, fmt.Errorf("invalid status: must be pending, in_progress, or completed")
				}
				updates["status"] = input.Status
			}
			if input.ActiveForm != "" {
				updates["active_form"] = input.ActiveForm
			}
			if len(input.AddBlocks) > 0 {
				updates["add_blocks"] = input.AddBlocks
			}
			if len(input.AddBlockedBy) > 0 {
				updates["add_blocked_by"] = input.AddBlockedBy
			}

			task, err := tm.Update(input.TaskID, updates)
			if err != nil {
				return TaskUpdateOutput{}, err
			}

			return TaskUpdateOutput{
				Task:    task,
				Message: fmt.Sprintf("Updated task #%s", task.ID),
			}, nil
		},
	)
}

func NewTaskGetTool(genkitApp *genkit.Genkit, tm *TaskManager) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"task_get",
		`Get full details of a task by ID. Returns:
- subject, description, status
- blocks/blockedBy dependencies
- timestamps

Use before starting work to get complete requirements.`,
		func(ctx *ai.ToolContext, input TaskGetInput) (TaskGetOutput, error) {
			if input.TaskID == "" {
				return TaskGetOutput{}, fmt.Errorf("task_id is required")
			}

			task, err := tm.Get(input.TaskID)
			if err != nil {
				return TaskGetOutput{}, err
			}

			return TaskGetOutput{Task: task}, nil
		},
	)
}

func NewTaskListTool(genkitApp *genkit.Genkit, tm *TaskManager) ai.Tool {
	return genkit.DefineTool(
		genkitApp,
		"task_list",
		`List all tasks with summary. Shows:
- ID, subject, status for each task
- Blocked dependencies
- Overall progress summary

Use to see available work and check progress.`,
		func(ctx *ai.ToolContext, input TaskListInput) (TaskListOutput, error) {
			tasks := tm.List()
			return TaskListOutput{
				Tasks:   tasks,
				Summary: tm.GetSummary(),
			}, nil
		},
	)
}

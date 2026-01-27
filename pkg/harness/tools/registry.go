package tools

import (
	"station/pkg/harness/sandbox"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type ToolRegistry struct {
	genkitApp     *genkit.Genkit
	workspacePath string
	sandbox       sandbox.Sandbox
	tools         map[string]ai.Tool
	taskManager   *TaskManager
}

func NewToolRegistry(genkitApp *genkit.Genkit, workspacePath string) *ToolRegistry {
	return &ToolRegistry{
		genkitApp:     genkitApp,
		workspacePath: workspacePath,
		tools:         make(map[string]ai.Tool),
		taskManager:   NewTaskManager(workspacePath),
	}
}

func NewToolRegistryWithSandbox(genkitApp *genkit.Genkit, workspacePath string, sb sandbox.Sandbox) *ToolRegistry {
	return &ToolRegistry{
		genkitApp:     genkitApp,
		workspacePath: workspacePath,
		sandbox:       sb,
		tools:         make(map[string]ai.Tool),
		taskManager:   NewTaskManager(workspacePath),
	}
}

func (r *ToolRegistry) RegisterBuiltinTools() error {
	bashTool := NewBashToolWithSandbox(r.genkitApp, r.workspacePath, r.sandbox)
	r.tools["bash"] = bashTool

	readTool := NewReadToolWithSandbox(r.genkitApp, r.workspacePath, r.sandbox)
	r.tools["read"] = readTool

	writeTool := NewWriteToolWithSandbox(r.genkitApp, r.workspacePath, r.sandbox)
	r.tools["write"] = writeTool

	editTool := NewEditToolWithSandbox(r.genkitApp, r.workspacePath, r.sandbox)
	r.tools["edit"] = editTool

	globTool := NewGlobToolWithSandbox(r.genkitApp, r.workspacePath, r.sandbox)
	r.tools["glob"] = globTool

	grepTool := NewGrepToolWithSandbox(r.genkitApp, r.workspacePath, r.sandbox)
	r.tools["grep"] = grepTool

	r.tools["task_create"] = NewTaskCreateTool(r.genkitApp, r.taskManager)
	r.tools["task_update"] = NewTaskUpdateTool(r.genkitApp, r.taskManager)
	r.tools["task_get"] = NewTaskGetTool(r.genkitApp, r.taskManager)
	r.tools["task_list"] = NewTaskListTool(r.genkitApp, r.taskManager)

	return nil
}

func (r *ToolRegistry) Sandbox() sandbox.Sandbox {
	return r.sandbox
}

func (r *ToolRegistry) Get(name string) (ai.Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

func (r *ToolRegistry) All() []ai.Tool {
	result := make([]ai.Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

func (r *ToolRegistry) AllRefs() []ai.ToolRef {
	result := make([]ai.ToolRef, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

func (r *ToolRegistry) Names() []string {
	result := make([]string, 0, len(r.tools))
	for name := range r.tools {
		result = append(result, name)
	}
	return result
}

func (r *ToolRegistry) Register(name string, tool ai.Tool) {
	r.tools[name] = tool
}

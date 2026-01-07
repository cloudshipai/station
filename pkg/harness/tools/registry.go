package tools

import (
	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type ToolRegistry struct {
	genkitApp     *genkit.Genkit
	workspacePath string
	tools         map[string]ai.Tool
}

func NewToolRegistry(genkitApp *genkit.Genkit, workspacePath string) *ToolRegistry {
	return &ToolRegistry{
		genkitApp:     genkitApp,
		workspacePath: workspacePath,
		tools:         make(map[string]ai.Tool),
	}
}

func (r *ToolRegistry) RegisterBuiltinTools() error {
	bashTool := NewBashTool(r.genkitApp, r.workspacePath)
	r.tools["bash"] = bashTool

	readTool := NewReadTool(r.genkitApp, r.workspacePath)
	r.tools["read"] = readTool

	writeTool := NewWriteTool(r.genkitApp, r.workspacePath)
	r.tools["write"] = writeTool

	editTool := NewEditTool(r.genkitApp, r.workspacePath)
	r.tools["edit"] = editTool

	globTool := NewGlobTool(r.genkitApp, r.workspacePath)
	r.tools["glob"] = globTool

	grepTool := NewGrepTool(r.genkitApp, r.workspacePath)
	r.tools["grep"] = grepTool

	return nil
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

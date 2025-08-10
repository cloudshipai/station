# Provider-Aware Tool Call Tracking Architecture

## Overview

Station implements a sophisticated provider-aware architecture that unifies tool call visibility across different AI providers while leveraging each provider's strengths. This ensures consistent, detailed execution tracking regardless of whether you use OpenAI, Gemini, or any OpenAI-compatible provider.

## The Problem

Different AI providers handle tool calling differently in GenKit:

- **Gemini**: GenKit naturally exposes tool calls in the response - works beautifully out of the box
- **OpenAI & Compatible**: GenKit's multi-turn orchestration handles tool execution internally but hides the individual tool calls from the final response

This created inconsistent visibility where Gemini users saw detailed tool call logs while OpenAI users saw only final responses.

## The Solution: Hybrid Provider-Aware Architecture

Station implements a **hybrid approach** that provides the best of both worlds:

```go
// Define providers that need middleware for tool call extraction
openAICompatibleProviders := []string{
    "openai", "anthropic", "groq", "deepseek", "together", 
    "fireworks", "perplexity", "mistral", "cohere", 
    "huggingface", "replicate", "anyscale", "local"
}

// Check if current provider needs middleware
needsMiddleware := false
for _, provider := range openAICompatibleProviders {
    if strings.HasPrefix(cfg.AIProvider, provider) {
        needsMiddleware = true
        break
    }
}
```

### For OpenAI-Compatible Providers: ModelMiddleware

For providers that hide tool calls, Station uses elegant ModelMiddleware to intercept and capture tool call metadata during execution:

```go
if needsMiddleware {
    toolTrackingMiddleware = func(next ai.ModelFunc) ai.ModelFunc {
        return func(ctx context.Context, req *ai.ModelRequest, cb ai.ModelStreamCallback) (*ai.ModelResponse, error) {
            resp, err := next(ctx, req, cb)
            if err != nil {
                return resp, err
            }
            
            // Capture tool calls from response
            if resp.Message != nil {
                for _, part := range resp.Message.Content {
                    if part.IsToolRequest() {
                        toolReq := part.ToolRequest
                        capturedToolCalls = append(capturedToolCalls, map[string]interface{}{
                            "tool_name": toolReq.Name,
                            "input":     toolReq.Input,
                            "step":      len(capturedToolCalls) + 1,
                        })
                    }
                }
            }
            
            return resp, err
        }
    }
}
```

### For Gemini: Native GenKit Extraction

For Gemini, Station uses the existing `extractToolCallsFromResponse` method which works beautifully with GenKit's native tool call handling.

### Unified Storage

Both approaches feed into the same standardized storage format:

```go
// Unified tool call extraction
if needsMiddleware && len(capturedToolCalls) > 0 {
    // Use middleware-captured tool calls for OpenAI-compatible providers
    var toolCallsInterface []interface{}
    for _, toolCall := range capturedToolCalls {
        toolCallsInterface = append(toolCallsInterface, toolCall)
    }
    toolCallsArray := models.JSONArray(toolCallsInterface)
    toolCalls = &toolCallsArray
} else if !needsMiddleware {
    // Use native GenKit tool call extraction for Gemini (works beautifully)
    toolCallsFromResponse := iac.extractToolCallsFromResponse(response, modelName)
    if len(toolCallsFromResponse) > 0 {
        toolCallsArray := models.JSONArray(toolCallsFromResponse)
        toolCalls = &toolCallsArray
    }
}
```

## Benefits

### 🎯 **Unified Experience**
- Consistent tool call visibility across all providers
- Same detailed execution steps regardless of backend
- Beautiful execution logs like Gemini for all providers

### ⚡ **Provider Optimized**  
- Zero overhead for Gemini (uses native GenKit handling)
- Minimal overhead for OpenAI (middleware only when needed)
- Automatically handles new OpenAI-compatible providers

### 🔧 **Clean Architecture**
- Single codebase for all providers
- Provider detection happens automatically
- Easy to extend for new providers

### 📊 **Rich Telemetry**
- Complete tool call metadata stored in database
- Detailed execution steps for debugging
- Token usage and performance metrics

## Database Integration

All tool call data flows seamlessly into the database through the existing infrastructure:

```go
// AgentExecutionResult automatically populated
result := &AgentExecutionResult{
    Response:       responseText,
    StepsTaken:     stepsTaken,
    ToolCalls:      toolCalls,      // ✅ Unified from both approaches
    ExecutionSteps: executionSteps, // ✅ Standardized format
    TokenUsage:     tokenUsage,
}

// Database storage via existing UpdateCompletion method
err = repos.AgentRuns.UpdateCompletion(
    agentRun.ID,
    result.Response,
    result.StepsTaken,
    result.ToolCalls,      // ✅ Provider-aware data
    result.ExecutionSteps, // ✅ Consistent format
    "completed",
    &completedAt,
)
```

## Configuration

No special configuration required! The system automatically:

1. **Detects your provider** based on `ai_provider` config
2. **Applies appropriate strategy** (middleware or native)  
3. **Stores unified data** in the same database format

### Supported Providers

#### Native GenKit Support (No Middleware)
- `gemini` - Google Gemini models
- `ollama` - Local Ollama models  

#### Middleware-Enhanced Support
- `openai` - OpenAI GPT models
- `anthropic` - Anthropic Claude models  
- `groq` - Groq inference
- `deepseek` - DeepSeek models
- `together` - Together AI
- `fireworks` - Fireworks AI
- `perplexity` - Perplexity models
- `mistral` - Mistral models
- `cohere` - Cohere models
- `huggingface` - Hugging Face models
- `replicate` - Replicate models
- `anyscale` - Anyscale endpoints
- `local` - Local OpenAI-compatible servers

## Example Output

### OpenAI with Middleware
```
🔧 Middleware captured 3 tool calls from openai
💰 Token usage - Input: 1270, Output: 157, Total: 1427

🔧 Tool Calls (3):
  1. {"input": {"path": "."}, "step": 1, "tool_name": "f_list_directory"}
  2. {"input": {"path": "go.mod"}, "step": 2, "tool_name": "f_directory_tree"}  
  3. {"input": {"dryRun": true, "edits": [...]}, "step": 3, "tool_name": "f_edit_file"}
```

### Gemini with Native Support  
```
💰 Token usage - Input: 8546, Output: 364, Total: 8910

🔧 Tool Calls (4):
  1. {"input": {"path": "/"}, "step": 1, "tool_name": "f_list_directory"}
  2. {"input": {"path": "/home/user/project"}, "step": 2, "tool_name": "f_list_directory"}
  3. {"input": {"path": "/home/user/project/internal"}, "step": 3, "tool_name": "f_directory_tree"}
  4. {"input": {"path": "/home/user/project/cmd"}, "step": 4, "tool_name": "f_directory_tree"}
```

Both provide the same rich execution visibility and database storage!

## Implementation Notes

### Key Files
- `/internal/services/intelligent_agent_creator.go`: Core provider-aware logic
- `/cmd/main/handlers/agent/local.go`: Database integration
- `/internal/db/repositories/agent_runs.go`: Storage methods

### Design Principles  
1. **Zero Breaking Changes**: Existing Gemini behavior unchanged
2. **Minimal Overhead**: Middleware only when needed
3. **Extensible**: Easy to add new providers to the compatibility list
4. **Unified Output**: Same rich data regardless of provider

This architecture ensures that Station users get the beautiful, detailed tool call visibility that makes Gemini so powerful, regardless of which AI provider they choose to use.
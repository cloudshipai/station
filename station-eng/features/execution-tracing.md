# Agent Execution Tracing Feature

## Overview
Comprehensive execution tracing system that captures detailed agent execution data including tool calls, conversation flow, token usage, and execution steps from GenKit AI framework responses.

## Implementation Details

### Core Components

#### 1. IntelligentAgentCreator Enhancement (`internal/services/intelligent_agent_creator.go`)
- **GenKit Response Processing**: Extracts detailed execution data from `*ai.ModelResponse` objects
- **Tool Call Extraction**: Parses actual tool calls with input/output data from conversation history
- **Execution Steps Building**: Creates detailed step-by-step conversation flow tracking
- **Token Usage Capture**: Extracts precise token consumption metrics (input/output/total)

**Key Methods Added:**
```go
func (iac *IntelligentAgentCreator) extractToolCallsFromResponse(response *ai.ModelResponse, modelName string) []interface{}
func (iac *IntelligentAgentCreator) buildExecutionStepsFromResponse(response *ai.ModelResponse) []interface{}
func (iac *IntelligentAgentCreator) extractMessageContent(part interface{}) string
func (iac *IntelligentAgentCreator) truncateContent(content string, maxLength int) string
```

#### 2. ExecutionQueue Service Enhancement (`internal/services/execution_queue.go`) 
- **Data Flow Integration**: Modified worker to extract detailed data from `response.Extra` instead of creating basic hardcoded steps
- **Proper Data Transfer**: Ensures detailed execution data flows from AgentService through ExecutionQueue to storage

#### 3. JSONArray Type Fix (`pkg/models/models.go`)
- **SQLite Compatibility**: Fixed `JSONArray.Scan()` method to handle both `[]byte` and `string` inputs from SQLite
- **Data Integrity**: Ensures proper JSON deserialization of complex execution data

#### 4. CLI Inspection Enhancement (`cmd/main/handlers/runs_handlers.go`)
- **Verbose Display**: Enhanced `--verbose` mode to show detailed tool calls and execution steps
- **JSON Formatting**: Pretty-printed JSON output for complex execution data structures

### Data Storage Schema

#### Agent Runs Table Extensions
- `tool_calls`: JSON array storing detailed tool call information
- `execution_steps`: JSON array storing conversation flow steps
- Both fields use `models.JSONArray` custom type for proper serialization

### Transport Compatibility

#### Supported MCP Transports
✅ **Stdio MCP Clients**: Full compatibility with process-based MCP servers  
✅ **HTTP MCP Clients**: Full compatibility with web-based MCP servers  
✅ **Mixed Environments**: Works with agents using multiple transport types simultaneously

### Execution Data Structure

#### Tool Calls Format
```json
{
  "input": {"param": "value"},
  "model": "googleai/gemini-2.0-flash-exp",
  "output": {"content": [{"text": "response", "type": "text"}]},
  "status": "completed",
  "step": 1,
  "timestamp": "2025-08-06T16:23:33-05:00",
  "tokens": {"input": 17607, "output": 222, "total": 17829},
  "tool_name": "f_aws___search_documentation",
  "type": "tool_call"
}
```

#### Execution Steps Format
```json
{
  "content": "Step content or conversation message",
  "description": "Step description",
  "status": "completed",
  "step": 1,
  "timestamp": "2025-08-06T16:23:33-05:00",
  "type": "user_input|tool_call|model_response"
}
```

## Usage

### CLI Inspection
```bash
# Basic run information
stn runs inspect <run_id>

# Detailed execution tracing with tool calls and steps
stn runs inspect <run_id> --verbose
```

### Captured Data Examples
- **Simple Task** (1 tool call, 4 execution steps): File system operations
- **Complex Task** (7 tool calls, 16 execution steps): Multi-step AWS documentation analysis with search, read, and recommendation operations

## Technical Benefits

### Debugging Capabilities
- **Complete Conversation Flow**: See exact agent decision-making process
- **Tool Call Analysis**: Inspect tool inputs, outputs, and error handling
- **Token Usage Tracking**: Monitor resource consumption per execution step
- **Performance Metrics**: Analyze execution duration and step timing

### Observability Improvements
- **Agent Behavior Analysis**: Understand how agents handle complex multi-step tasks
- **Error Pattern Detection**: Identify common failure points in tool chains
- **Cost Optimization**: Track token usage for cost management
- **Performance Tuning**: Identify bottlenecks in agent execution

## Architecture Integration

### Data Flow
1. **Agent Execution** → GenKit AI Framework
2. **GenKit Response** → IntelligentAgentCreator extraction
3. **Detailed Data** → ExecutionQueue processing  
4. **Database Storage** → JSONArray serialization
5. **CLI Display** → Rich verbose inspection

### Backward Compatibility
- **Existing Runs**: Legacy runs show basic information without detailed tracing
- **New Executions**: All new agent runs automatically capture detailed execution data
- **Zero Downtime**: Feature activation requires no service interruption

## Testing Results

### Cross-Transport Validation
- **Run 16**: Stdio MCP (filesystem) - 1 tool call, 4 execution steps, 1,115 tokens
- **Run 17**: HTTP MCP (AWS Knowledge) - 7 tool calls, 16 execution steps, 17,829 tokens

Both transport types successfully capture and display comprehensive execution tracing data.

## Future Enhancements

### Potential Extensions
- **Real-time Streaming**: Live execution step updates during long-running agents
- **Execution Replay**: Ability to replay agent execution for debugging
- **Metric Aggregation**: Dashboard views for execution analytics
- **Export Capabilities**: JSON/CSV export of execution data for analysis
# Agent Hierarchy Multi-Agent System - Fixed

## Issue Identified

The agent hierarchy demo wasn't working because:

1. **Missing Agent Tools in YAML**: Agent .prompt files had `tools: []` even though they referenced agent tools in system prompts
2. **Wrong MCP Server Command**: `template.json` used old `stn mcp server` instead of `stn stdio`
3. **No Output Schemas**: Agents had no output schemas, causing GenKit to fail with schema validation errors

## Fixes Applied

### 1. Updated Agent Tools in YAML Frontmatter

**Master Orchestrator** (`master_orchestrator.prompt`):
```yaml
tools:
  - "__agent_data_processor"
  - "__agent_report_generator"
  - "__agent_math_calculator"
  - "__agent_text_formatter"
  - "__agent_file_analyzer"
```

**Data Processor** (`data_processor.prompt`):
```yaml
tools:
  - "__agent_math_calculator"
  - "__agent_text_formatter"
```

**Report Generator** (`report_generator.prompt`):
```yaml
tools:
  - "__agent_file_analyzer"
  - "__agent_text_formatter"
  - "__list_directory"
```

### 2. Updated MCP Server in template.json

**Before**:
```json
"station": {
  "command": "stn",
  "args": ["mcp", "server", "--environment", "agent-hierarchy-demo"]
}
```

**After**:
```json
"station-agents": {
  "command": "stn",
  "args": ["stdio", "--core"],
  "env": {
    "STATION_ENVIRONMENT": "agent-hierarchy-demo"
  }
}
```

### 3. Added Output Schemas

**Master Orchestrator Output Schema**:
```yaml
output:
  schema:
    type: object
    properties:
      strategy:
        type: string
        description: "Explanation of the delegation strategy used"
      delegations:
        type: array
        description: "List of agents called and their results"
        items:
          type: object
          properties:
            agent:
              type: string
            task:
              type: string
            result:
              type: string
      final_response:
        type: string
        description: "Comprehensive response combining all results"
    required: ["strategy", "delegations", "final_response"]
```

**Data Processor Output Schema**:
```yaml
output:
  schema:
    type: object
    properties:
      calculated_values:
        type: array
        items:
          type: number
      formatted_result:
        type: string
    required: ["formatted_result"]
```

**Report Generator Output Schema**:
```yaml
output:
  schema:
    type: object
    properties:
      report_title:
        type: string
      file_analysis:
        type: object
      formatted_report:
        type: string
    required: ["report_title", "formatted_report"]
```

## How Agent Hierarchy Works

### Architecture

```
Master Orchestrator (Top Level)
├─→ Data Processor (Coordinator)
│   ├─→ Math Calculator (Specialist)
│   └─→ Text Formatter (Specialist)
├─→ Report Generator (Coordinator)
│   ├─→ File Analyzer (Specialist)
│   └─→ Text Formatter (Specialist)
└─→ Direct access to all specialists when needed
```

### Agent Tool Discovery

1. **`stn develop --env agent-hierarchy-demo`** starts with:
   - Loads `.prompt` files from `~/.config/station/environments/agent-hierarchy-demo/agents/`
   - Connects to MCP servers defined in `template.json`
   - Discovers agent tools from `station-agents` MCP server

2. **Station MCP Server** (`stn stdio --core`):
   - Queries database for all agents in environment
   - Creates `__agent_*` tools dynamically (e.g., `__agent_data_processor`)
   - Each agent tool wraps `__call_agent` with specific agent ID

3. **GenKit Registration**:
   - All MCP tools (filesystem + agent tools) registered as GenKit actions
   - Agent prompts loaded from `.prompt` files
   - Tools array in YAML frontmatter tells GenKit which tools each agent can use

### Execution Flow

**Example: "Calculate 25 * 4 and format the result as text"**

1. **Master Orchestrator** receives request
   - Analyzes: needs calculation + formatting
   - Strategy: Use Data Processor (handles both)
   - Calls: `__agent_data_processor`

2. **Data Processor** receives task
   - Breaks down: calculation step + formatting step
   - Calls: `__agent_math_calculator("Calculate 25 * 4")`
   - Receives: `100`
   - Calls: `__agent_text_formatter("Format 100 as text")`
   - Receives: `"one hundred"`

3. **Master Orchestrator** receives result
   - Combines results
   - Returns structured JSON:
     ```json
     {
       "strategy": "Delegated to Data Processor for calculation and formatting",
       "delegations": [
         {
           "agent": "__agent_data_processor",
           "task": "Calculate 25 * 4 and format as text",
           "result": "one hundred"
         }
       ],
       "final_response": "The result is one hundred"
     }
     ```

## Testing in GenKit Developer UI

### Step 1: Start Develop Mode
```bash
cd /home/epuerta/projects/hack/station
stn develop --env agent-hierarchy-demo
```

**Expected Output**:
```
🔧 Loaded 56 MCP tools from 2 servers
🤖 Agent prompts automatically loaded from: ~/.config/station/environments/agent-hierarchy-demo/agents
🔧 Registering MCP tools and agent tools as GenKit actions...
   🤖 Registered agent tool: __agent_data_processor
   🤖 Registered agent tool: __agent_report_generator
   🤖 Registered agent tool: __agent_math_calculator
   🤖 Registered agent tool: __agent_text_formatter
   🤖 Registered agent tool: __agent_file_analyzer
📊 Registered 49 MCP tools + 7 agent tools (total: 56)
✨ Multi-agent hierarchy is enabled - agents can call other agents as tools!
```

### Step 2: Open GenKit UI
```bash
open http://localhost:4000
```

### Step 3: Test Master Orchestrator

1. Navigate to **Prompts** tab
2. Select **master_orchestrator**
3. Click **Run** button
4. Enter input:
   ```
   Calculate 25 * 4 and format the result as text
   ```
5. Click **Run**

**Expected Result**:
- ✅ Agent calls `__agent_data_processor`
- ✅ Data Processor calls `__agent_math_calculator`
- ✅ Data Processor calls `__agent_text_formatter`
- ✅ Returns structured JSON with strategy, delegations, and final_response

### Step 4: Test Other Agents

**Data Processor** (direct test):
```
Input: Calculate 50 + 50 and format as uppercase text
Expected: Calls calculator → calls formatter → returns formatted result
```

**Report Generator** (direct test):
```
Input: Generate a report about files in the current directory
Expected: Calls file analyzer → calls formatter → returns formatted report
```

## Files Modified

### Agent Prompts
1. `~/.config/station/environments/agent-hierarchy-demo/agents/master_orchestrator.prompt`
   - Added agent tools array
   - Added output schema

2. `~/.config/station/environments/agent-hierarchy-demo/agents/data_processor.prompt`
   - Added agent tools array
   - Added output schema

3. `~/.config/station/environments/agent-hierarchy-demo/agents/report_generator.prompt`
   - Added agent tools array
   - Added output schema

### Environment Config
4. `~/.config/station/environments/agent-hierarchy-demo/template.json`
   - Updated `station` → `station-agents`
   - Changed command to `stn stdio --core`
   - Added `STATION_ENVIRONMENT` env var

## Key Learnings

### Agent Tool Naming Convention
- Agent tools must be named: `__agent_<normalized_name>`
- Normalized name: lowercase, hyphens instead of spaces/underscores
- Example: "Data Processor" → `__agent_data_processor`

### YAML Frontmatter Requirements
- `tools:` array must list ALL tools the agent can use
- Include both MCP tools (`__list_directory`) and agent tools (`__agent_*`)
- Empty `tools: []` means agent has no tool access

### Output Schema Importance
- GenKit requires output schemas for structured responses
- Schema defines expected JSON structure
- Without schema, GenKit fails with "Invalid type. Expected: object, given: null"

### MCP Server for Agent Tools
- Use `stn stdio --core` as MCP server for agent tool discovery
- `--core` flag runs MCP-only mode (no API server needed)
- Server dynamically creates `__agent_*` tools from database

## Troubleshooting

### Agent Tools Not Showing Up
**Symptom**: GenKit shows 0 agent tools registered

**Solution**:
1. Check `template.json` has `station-agents` MCP server with `stn stdio --core`
2. Verify agents exist in database for the environment
3. Check agent names match normalized convention

### GenKit Schema Error
**Symptom**: "data did not match expected schema: - output.jsonSchema: Invalid type"

**Solution**:
1. Add `output:` section to agent .prompt file
2. Define `schema:` with `type: object`
3. Specify `properties:` and `required:` fields
4. Restart `stn develop`

### Agent Not Calling Sub-Agents
**Symptom**: Agent doesn't delegate to child agents

**Solution**:
1. Check `tools:` array includes `__agent_*` tools
2. Verify agent tool names are correct (use `__list_tools` in GenKit)
3. Check system prompt mentions how to use agent tools
4. Ensure child agents exist in same environment

## Future Enhancements

### UI Visualization
- Add agent hierarchy graph to Station UI
- Show parent-child relationships
- Display orchestrator/callable/specialist badges
- Real-time execution flow visualization

### Agent Auto-Discovery
- Automatically detect agent tools from environment
- Suggest agent tools based on agent descriptions
- Validate agent tool references in prompts

### Execution Tracing
- Capture multi-agent execution traces
- Show delegation tree in UI
- Performance metrics per agent
- Cost tracking across hierarchy

---
**Status**: ✅ Fixed and Tested  
**Date**: 2025-11-11  
**Environment**: `agent-hierarchy-demo`  
**Next**: Test in production GenKit UI

# Multi-Agent Hierarchy DX Improvements

**Status**: Design & Implementation Plan  
**Date**: 2025-01-11  
**Priority**: High

---

## Executive Summary

Station already supports multi-agent hierarchies where agents can call other agents as tools. However, the developer experience for discovering, configuring, and testing multi-agent workflows needs significant improvement. This document outlines the current state, identified gaps, and implementation plan.

---

## Current State ✅

### What Works Today

**1. Agent-as-Tool Infrastructure** (Fully Implemented)
- Agents automatically exposed as GenKit tools with `__agent_` prefix
- Tool name format: `__agent_{agent-name}` or `mcp__station__agent__{agent-name}`
- Cached tool generation for performance (`AgentToolCache` with 5min TTL)
- Proper input validation (task parameter required, length limits)
- Parent-child run tracking via `parent_run_id` in database
- OTEL tracing for multi-agent execution flows

**2. .prompt File Configuration**
```yaml
---
metadata:
  name: "orchestrator"
  description: "Coordinates specialist agents"
model: gpt-4o-mini
max_steps: 6
tools:
  - "__agent_calculator"
  - "__agent_text_formatter"
---
```

**3. Execution Engine Support**
- `AgentExecutionEngine` handles agent-as-tool calls
- Context propagation: parent run ID passed through execution chain
- SQLite locking prevention: proper transaction handling
- Error handling with detailed logging

**4. stn develop Support**
- Agent tools registered in GenKit Developer UI
- Multi-agent workflows testable interactively
- Console output shows: "🤖 Agent tools are available - you can test multi-agent workflows!"

**5. UI Visualization**
- Agent hierarchy shown in AgentListSidebar (Orchestrator/Callable badges)
- Parent/child relationship detection in runs
- Execution flow panel shows tool calls including agent calls

---

## Identified Gaps & Pain Points ⚠️

### 1. Agent Discovery is Hard

**Problem**: Developers don't know which agents exist as callable tools

**Current Experience**:
```bash
# Developer has to manually query database or read files
stn agent list --env default
# Then guess the tool name format: __agent_calculator
```

**What's Missing**:
- No `stn agent tools` command to list available agent tools
- No autocomplete for agent tool names in .prompt files
- No inline documentation showing agent tool schemas
- UI doesn't show "callable agents" separately from regular agents

### 2. Tool Name Format is Confusing

**Problem**: Two naming conventions create confusion

**Current State**:
- `.prompt` files use: `__agent_calculator`
- Runtime tools show as: `mcp__station__agent__calculator`
- Developers aren't sure which format to use where

**What's Missing**:
- Clear documentation on naming conventions
- Automatic name normalization in sync process
- Error messages when agent tool names are wrong

### 3. No Agent Input Schema Discovery

**Problem**: Developers don't know what parameters an agent expects

**Current Experience**:
```yaml
# How do I know what to pass to this agent?
tools:
  - "__agent_data_analyzer"
```

**What's Missing**:
- `stn agent schema <agent-name>` command
- Schema display in GenKit Developer UI
- Documentation generation from .prompt input schemas

### 4. stn develop Doesn't Show Agent Hierarchy

**Problem**: Multi-agent workflows are invisible in development

**Current Experience**:
```
🤖 Agent tools are available - you can test multi-agent workflows!
# But which agents call which? No visualization.
```

**What's Missing**:
- Console output showing agent hierarchy tree
- Warning when agent tools reference non-existent agents
- Cycle detection for infinite recursion prevention

### 5. No Testing Utilities for Multi-Agent Workflows

**Problem**: Hard to test hierarchical agent execution

**What's Missing**:
- `stn agent call-hierarchy <orchestrator>` command
- Dry-run mode showing execution plan without API calls
- Visual execution tree in CLI output

---

## Implementation Plan 🛠️

### Phase 1: Discovery & Documentation (2-3 hours)

**1.1 Add `stn agent tools` Command**
```bash
# List all agent tools in environment
stn agent tools --env default

# Output:
# Available Agent Tools:
#   __agent_calculator          [calculator] 2 steps - Performs math calculations
#   __agent_text_formatter      [text-formatter] 3 steps - Formats text output
#   __agent_data_analyzer       [data-analyzer] 5 steps - Analyzes data patterns
#
# Usage in .prompt files:
#   tools:
#     - "__agent_calculator"
```

**1.2 Add `stn agent schema` Command**
```bash
# Show agent input schema
stn agent schema calculator

# Output:
# Agent: calculator
# Input Schema:
#   task: string (required) - Mathematical expression to evaluate
#
# Example .prompt file usage:
#   tools:
#     - "__agent_calculator"
#
# Example tool call:
#   Use the __agent_calculator tool with task="25 + 17"
```

**1.3 Enhance `stn develop` Output**
```
🤖 Multi-agent hierarchy detected:
   orchestrator (6 steps)
   ├─ coordinator (4 steps)
   │  ├─ calculator (2 steps)
   │  └─ text-formatter (3 steps)
   └─ file-analyzer (5 steps)

✨ All agents registered as tools with __agent_ prefix
```

**Implementation**:
- File: `cmd/main/agent_tools.go`
- Add `agentToolsCmd` with list/schema subcommands
- Reuse existing `MCPConnectionManager.GetAgentTools()` logic
- Add hierarchy visualization using tree library

**Files to Modify**:
```
cmd/main/agent_tools.go          (NEW)
cmd/main/develop.go              (add hierarchy output)
internal/utils/agentHierarchy.go (NEW - tree builder)
```

---

### Phase 2: Naming Consistency (1-2 hours)

**2.1 Normalize Tool Names in Sync**

**Problem**: `.prompt` files use `__agent_X` but runtime shows `mcp__station__agent__X`

**Solution**: Agent file sync should accept both formats and normalize

```go
// internal/services/agent_file_sync.go
func normalizeAgentToolName(toolName string) string {
    // Accept both formats in .prompt files
    if strings.HasPrefix(toolName, "__agent_") {
        return toolName // Keep as-is for .prompt files
    }
    if strings.HasPrefix(toolName, "mcp__station__agent__") {
        // Normalize to __agent_ format for consistency
        return strings.Replace(toolName, "mcp__station__agent__", "__agent_", 1)
    }
    return toolName
}
```

**2.2 Add Validation with Helpful Errors**

```go
// Validate agent tool references during sync
func validateAgentTools(toolNames []string, availableAgents []Agent) []error {
    var errors []error
    for _, toolName := range toolNames {
        if !strings.HasPrefix(toolName, "__agent_") {
            continue
        }
        agentName := strings.TrimPrefix(toolName, "__agent_")
        if !agentExists(agentName, availableAgents) {
            errors = append(errors, fmt.Errorf(
                "agent tool '%s' references unknown agent '%s'. Available: %v",
                toolName, agentName, listAgentNames(availableAgents),
            ))
        }
    }
    return errors
}
```

**Files to Modify**:
```
internal/services/agent_file_sync.go    (add normalization + validation)
internal/services/declarative_sync.go   (call validation)
```

---

### Phase 3: stn develop Improvements (2-3 hours)

**3.1 Detect Agent Hierarchy and Show Tree**

```go
// cmd/main/develop.go
func detectAgentHierarchy(agents []Agent) *HierarchyTree {
    tree := &HierarchyTree{Nodes: make(map[string]*Node)}
    
    for _, agent := range agents {
        node := &Node{Agent: agent, Children: []*Node{}}
        
        // Parse agent's .prompt file to find agent tool references
        config := parsePromptFile(agent.PromptPath)
        for _, tool := range config.Tools {
            if strings.HasPrefix(tool, "__agent_") {
                childName := strings.TrimPrefix(tool, "__agent_")
                node.Children = append(node.Children, &Node{AgentName: childName})
            }
        }
        
        tree.Nodes[agent.Name] = node
    }
    
    return tree
}

func printHierarchyTree(tree *HierarchyTree) {
    // Find root nodes (agents not called by others)
    roots := findRootNodes(tree)
    
    fmt.Println("🤖 Multi-agent hierarchy detected:")
    for _, root := range roots {
        printNode(root, "", true)
    }
}
```

**3.2 Detect Cycles and Warn**

```go
func detectCycles(tree *HierarchyTree) [][]string {
    var cycles [][]string
    visited := make(map[string]bool)
    path := []string{}
    
    for nodeName := range tree.Nodes {
        if !visited[nodeName] {
            if cycle := dfs(nodeName, tree, visited, path); cycle != nil {
                cycles = append(cycles, cycle)
            }
        }
    }
    
    return cycles
}

// Warn if cycles detected
if cycles := detectCycles(tree); len(cycles) > 0 {
    fmt.Println("⚠️  Warning: Circular dependencies detected:")
    for _, cycle := range cycles {
        fmt.Printf("   %s\n", strings.Join(cycle, " → "))
    }
    fmt.Println("   This may cause infinite recursion!")
}
```

**3.3 Agent Tool Metrics in GenKit UI**

When agent tools are registered, track and display their usage:

```go
// Track agent tool calls
type AgentToolMetrics struct {
    ToolName      string
    CallCount     int
    SuccessCount  int
    FailureCount  int
    AvgDuration   time.Duration
}

// Display during develop session
fmt.Println()
fmt.Println("📊 Agent Tool Usage:")
for _, metric := range agentToolMetrics {
    successRate := float64(metric.SuccessCount) / float64(metric.CallCount) * 100
    fmt.Printf("   %s: %d calls (%.1f%% success, avg: %v)\n",
        metric.ToolName, metric.CallCount, successRate, metric.AvgDuration)
}
```

**Files to Modify**:
```
cmd/main/develop.go                     (add hierarchy detection + printing)
internal/utils/agentHierarchy.go        (NEW - tree builder + cycle detection)
internal/services/mcp_connection_manager.go (track agent tool metrics)
```

---

### Phase 4: UI Improvements (3-4 hours)

**4.1 Agent List: Separate "Callable Agents" Section**

```tsx
// ui/src/components/agents/AgentListSidebar.tsx
const callableAgents = agents.filter(a => hierarchyMap.get(a.id)?.isCallable);
const orchestrators = agents.filter(a => hierarchyMap.get(a.id)?.childAgents.length > 0);
const standardAgents = agents.filter(a => !callableAgents.includes(a) && !orchestrators.includes(a));

return (
  <div>
    {orchestrators.length > 0 && (
      <Section title="Orchestrators" count={orchestrators.length}>
        {orchestrators.map(agent => <AgentCard {...agent} />)}
      </Section>
    )}
    
    {callableAgents.length > 0 && (
      <Section title="Callable Agents" count={callableAgents.length}>
        {callableAgents.map(agent => <AgentCard {...agent} />)}
      </Section>
    )}
    
    <Section title="Standard Agents" count={standardAgents.length}>
      {standardAgents.map(agent => <AgentCard {...agent} />)}
    </Section>
  </div>
);
```

**4.2 Agent Details: Show "Called By" and "Calls" Sections**

```tsx
// ui/src/components/modals/AgentDetailsModal.tsx
<div className="mt-4">
  {parentAgents.length > 0 && (
    <div className="mb-4">
      <h3 className="text-sm font-mono text-tokyo-comment mb-2">Called by:</h3>
      <div className="flex flex-wrap gap-2">
        {parentAgents.map(parent => (
          <Badge key={parent} onClick={() => navigateToAgent(parent)}>
            {parent}
          </Badge>
        ))}
      </div>
    </div>
  )}
  
  {childAgents.length > 0 && (
    <div>
      <h3 className="text-sm font-mono text-tokyo-comment mb-2">Calls:</h3>
      <div className="flex flex-wrap gap-2">
        {childAgents.map(child => (
          <Badge key={child} onClick={() => navigateToAgent(child)}>
            {child}
          </Badge>
        ))}
      </div>
    </div>
  )}
</div>
```

**4.3 Run Details: Show Parent/Child Runs**

```tsx
// ui/src/components/modals/RunDetailsModal.tsx
{run.parent_run_id && (
  <div className="p-3 bg-tokyo-blue/10 border border-tokyo-blue/30 rounded">
    <div className="text-xs text-tokyo-comment mb-1">Parent Run</div>
    <button 
      onClick={() => openRun(run.parent_run_id)}
      className="text-tokyo-blue hover:underline"
    >
      Run #{run.parent_run_id}
    </button>
  </div>
)}

{childRuns.length > 0 && (
  <div className="p-3 bg-tokyo-cyan/10 border border-tokyo-cyan/30 rounded">
    <div className="text-xs text-tokyo-comment mb-2">Child Runs ({childRuns.length})</div>
    <div className="space-y-1">
      {childRuns.map(child => (
        <button 
          key={child.id}
          onClick={() => openRun(child.id)}
          className="block text-sm text-tokyo-cyan hover:underline"
        >
          Run #{child.id} - {child.agent_name} ({child.status})
        </button>
      ))}
    </div>
  </div>
)}
```

**Files to Modify**:
```
ui/src/components/agents/AgentListSidebar.tsx
ui/src/components/modals/AgentDetailsModal.tsx
ui/src/components/modals/RunDetailsModal.tsx
ui/src/api/station.ts (add endpoint to fetch child runs)
```

---

### Phase 5: Testing & Documentation (2-3 hours)

**5.1 Create Test Environment**

```bash
# Create test environment with multi-agent hierarchy
mkdir -p ~/.config/station/environments/multi-agent-test/agents

cat > ~/.config/station/environments/multi-agent-test/agents/calculator.prompt <<EOF
---
metadata:
  name: "calculator"
  description: "Performs math calculations"
model: gpt-4o-mini
max_steps: 2
tools: []
---
{{role "system"}}
You are a calculator. Solve math problems.

{{role "user"}}
{{userInput}}
EOF

cat > ~/.config/station/environments/multi-agent-test/agents/orchestrator.prompt <<EOF
---
metadata:
  name: "orchestrator"
  description: "Orchestrates calculator tasks"
model: gpt-4o-mini
max_steps: 5
tools:
  - "__agent_calculator"
---
{{role "system"}}
You are an orchestrator. Use the __agent_calculator tool for math.

{{role "user"}}
{{userInput}}
EOF
```

**5.2 Add Integration Tests**

```go
// internal/services/multi_agent_dx_test.go
func TestAgentToolsCommand(t *testing.T) {
    // Setup test environment with agents
    // Execute: stn agent tools --env multi-agent-test
    // Verify: Output shows __agent_calculator
}

func TestAgentSchemaCommand(t *testing.T) {
    // Execute: stn agent schema calculator
    // Verify: Shows input schema
}

func TestDevelopHierarchyDisplay(t *testing.T) {
    // Execute: stn develop --env multi-agent-test
    // Verify: Console shows hierarchy tree
}

func TestCycleDetection(t *testing.T) {
    // Create agents with circular dependencies
    // Execute: stn develop
    // Verify: Warning message about cycles
}
```

**5.3 Update Documentation**

```markdown
# docs/features/multi-agent-orchestration.md

## Quick Start

1. List available agent tools:
   ```bash
   stn agent tools --env default
   ```

2. View agent input schema:
   ```bash
   stn agent schema calculator
   ```

3. Create orchestrator agent:
   ```yaml
   tools:
     - "__agent_calculator"
     - "__agent_text_formatter"
   ```

4. Test in development:
   ```bash
   stn develop --env default
   # See hierarchy tree in console
   ```

## Best Practices

- **Name agents descriptively** - Makes __agent_X tools self-documenting
- **Check for cycles** - Run `stn develop` to detect circular dependencies
- **Use input schemas** - Define expected parameters in .prompt files
- **Test incrementally** - Start with leaf agents, build up to orchestrators
```

**Files to Create**:
```
internal/services/multi_agent_dx_test.go
docs/features/multi-agent-orchestration.md
test-environments/multi-agent-test/
```

---

## Success Metrics 📊

### Developer Experience Improvements

**Before**:
- ❌ No way to discover available agent tools
- ❌ Confusing tool name formats
- ❌ No schema documentation
- ❌ Multi-agent workflows invisible in develop
- ❌ No cycle detection or warnings

**After**:
- ✅ `stn agent tools` lists all callable agents
- ✅ `stn agent schema <name>` shows input requirements
- ✅ Consistent `__agent_` naming in .prompt files
- ✅ Hierarchy tree visualization in `stn develop`
- ✅ Cycle detection with clear warnings
- ✅ UI shows orchestrator/callable badges
- ✅ Run details show parent/child relationships

### Time Savings

- **Agent discovery**: 10 min → 30 sec (20x faster)
- **Schema lookup**: 5 min → 10 sec (30x faster)
- **Hierarchy understanding**: 15 min → 2 min (7x faster)
- **Testing multi-agent flows**: 20 min → 5 min (4x faster)

**Total**: ~45 min saved per multi-agent workflow development cycle

---

## Implementation Timeline ⏱️

| Phase | Tasks | Time | Priority |
|-------|-------|------|----------|
| **Phase 1** | Discovery commands | 2-3h | High |
| **Phase 2** | Name normalization | 1-2h | High |
| **Phase 3** | stn develop improvements | 2-3h | High |
| **Phase 4** | UI enhancements | 3-4h | Medium |
| **Phase 5** | Testing & docs | 2-3h | Medium |
| **Total** | | **10-15h** | |

**Recommended Order**:
1. Phase 1 (Discovery) - Immediate value for developers
2. Phase 3 (stn develop) - Critical for development workflow
3. Phase 2 (Naming) - Reduces confusion
4. Phase 4 (UI) - Polish and completeness
5. Phase 5 (Testing) - Quality assurance

---

## Next Steps 🚀

**Immediate (Start Today)**:
1. Implement `stn agent tools` command
2. Add hierarchy tree to `stn develop` output
3. Create test environment for validation

**Week 1**:
1. Complete Phases 1-3 (discovery, naming, develop)
2. Test with real multi-agent workflows
3. Gather developer feedback

**Week 2**:
1. Complete Phases 4-5 (UI, testing, docs)
2. Create demo video showing new workflow
3. Update tutorials and guides

---

## Related Work

- **Execution Flow Visualization** ✅ - Shows agent tool calls in timeline
- **OTEL Tracing** ✅ - Tracks parent-child agent execution
- **Parent Run ID** ✅ - Database schema supports hierarchy
- **Agent Tool Caching** ✅ - Performance optimized
- **Jaeger Auto-Launch** ⏳ - Next priority (separate doc)

---

**Status**: Ready for implementation  
**Blockers**: None - all infrastructure exists  
**Dependencies**: None - can proceed immediately

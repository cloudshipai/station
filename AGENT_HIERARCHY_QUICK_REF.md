# Agent Hierarchy Quick Reference

## 🚀 Quick Start

```bash
# Start develop mode
stn develop --env agent-hierarchy-demo

# Open GenKit UI
open http://localhost:4000

# Test: Prompts → master_orchestrator → Run
Input: "Calculate 25 * 4 and format the result as text"
```

## 🏗️ Architecture

```
Master Orchestrator
├─→ Data Processor → Math Calculator + Text Formatter
├─→ Report Generator → File Analyzer + Text Formatter
└─→ Direct specialist access
```

## 📝 Required YAML Frontmatter

### Tools Array
```yaml
tools:
  - "__agent_data_processor"  # Agent tools
  - "__list_directory"         # MCP tools
```

### Output Schema
```yaml
output:
  schema:
    type: object
    properties:
      result:
        type: string
    required: ["result"]
```

## 🔧 Agent Tool Naming

| Agent Name | Tool Name |
|-----------|-----------|
| Data Processor | `__agent_data_processor` |
| Math Calculator | `__agent_math_calculator` |
| Text Formatter | `__agent_text_formatter` |
| File Analyzer | `__agent_file_analyzer` |
| Report Generator | `__agent_report_generator` |

**Rule**: `__agent_<lowercase_with_underscores>`

## 📁 File Locations

```
~/.config/station/environments/agent-hierarchy-demo/
├── agents/
│   ├── master_orchestrator.prompt  (5 agent tools)
│   ├── data_processor.prompt       (2 agent tools)
│   ├── report_generator.prompt     (2 agent tools + 1 MCP)
│   ├── math_calculator.prompt      (0 agent tools)
│   ├── text_formatter.prompt       (0 agent tools)
│   └── file_analyzer.prompt        (0 agent tools)
├── template.json                   (MCP servers)
└── variables.yml                   (PROJECT_ROOT)
```

## 🔌 MCP Server Config

```json
{
  "station-agents": {
    "command": "stn",
    "args": ["stdio", "--core"],
    "env": {
      "STATION_ENVIRONMENT": "agent-hierarchy-demo"
    }
  }
}
```

## 🧪 Test Cases

### Test 1: Simple Math
```
Input: "What is 100 + 50?"
Expected: Calls __agent_math_calculator → returns 150
```

### Test 2: Math + Format
```
Input: "Calculate 25 * 4 and format as text"
Expected: Calls __agent_data_processor → returns "one hundred"
```

### Test 3: File Report
```
Input: "List files in /workspace"
Expected: Calls __agent_report_generator → returns report
```

## 🐛 Troubleshooting

### No Agent Tools
**Check**: `template.json` has `stn stdio --core`  
**Fix**: Update MCP server config

### Schema Error
**Check**: Agent has `output:` section  
**Fix**: Add output schema to .prompt file

### Tools Not Available
**Check**: `tools:` array in YAML frontmatter  
**Fix**: Add `__agent_*` tools to array

## 📚 Documentation

- Full Guide: `docs/features/AGENT_HIERARCHY_FIXED.md`
- Session Summary: `SESSION_SUMMARY_AGENT_HIERARCHY.md`
- Test Script: `dev-workspace/test-scripts/test-agent-hierarchy.sh`

---
*Last Updated: 2025-11-11*

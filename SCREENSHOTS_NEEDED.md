# Screenshots Needed for README.md

This document tracks the 7 screenshots needed for the updated README.md

## 1. Incident Coordinator Agent Run (Line 212)
**Location:** README.md line 212  
**What to show:** Web UI showing the incident coordinator agent run with delegations to specialist agents  
**Screenshot shows:**
- Agent run detail page
- Incident coordinator executing
- Delegations to logs_investigator, metrics_investigator, change_detective visible
- Tool calls and agent responses
- Multi-agent coordination in action

**How to capture:**
1. Run incident coordinator: `"The API gateway is timing out"`
2. Go to Web UI: http://localhost:8585/runs
3. Click on the incident coordinator run
4. Screenshot the run detail page showing agent delegations

---

## 2. Claude/Cursor MCP Tools Interface (Line 305)
**Location:** README.md line 305  
**What to show:** Claude/Cursor interface showing Station MCP tools being used  
**Screenshot shows:**
- AI assistant interface (Claude or Cursor)
- MCP tools being invoked (create_agent, add_agent_as_tool, call_agent)
- Tool execution results
- Natural language conversation leading to tool usage

**How to capture:**
1. In Claude/Cursor, ask: "Create an incident response team"
2. Show the AI using create_agent tool
3. Show successful agent creation
4. Screenshot the conversation with tool invocations visible

---

## 3. Faker Generating Datadog Metrics (Line 328)
**Location:** README.md line 328  
**What to show:** Faker system generating realistic Datadog metrics  
**Screenshot shows:**
- Faker MCP server in action
- AI-generated realistic production incident data
- Datadog-style metrics, logs, or traces
- High CPU, memory spikes, error rates

**How to capture:**
1. Run a logs_investigator or metrics_investigator agent
2. Show the faker-generated response data
3. Highlight realistic production incident patterns
4. Screenshot the realistic mock data output

---

## 4. Team Performance Report (Line 341)
**Location:** README.md line 341  
**What to show:** Web UI showing team performance report with 7.5/10 score  
**Screenshot shows:**
- Reports page or report detail page
- "SRE Incident Response Team - Performance Evaluation"
- Overall team score: 7.5/10
- Breakdown metrics:
  - Multi-agent coordination: 8.5/10
  - Tool utilization: 8.0/10
  - Root cause analysis: 7.5/10
  - Resolution speed: 7.0/10
  - Communication clarity: 6.5/10

**How to capture:**
1. Go to: http://localhost:8585/reports
2. Select "SRE Incident Response Team" report
3. Screenshot the report showing the 7.5/10 score and metrics breakdown

---

## 5. Bundle in Registry (Line 413)
**Location:** README.md line 413  
**What to show:** Web UI showing bundle in registry  
**Screenshot shows:**
- Bundles page
- station-sre.tar.gz bundle listed
- Bundle metadata (9 agents, MCP configs)
- Install/export options

**How to capture:**
1. Go to: http://localhost:8585/bundles (if bundles page exists)
2. Or show CLI output from bundle creation
3. Or show file listing with bundle details
4. Screenshot showing bundle successfully created

---

## 6. Scheduled Agents with Cron (Line 437)
**Location:** README.md line 437  
**What to show:** Web UI showing scheduled agents with cron expressions  
**Screenshot shows:**
- Scheduled agents list
- Agent names with cron schedules
- Examples: `0 */5 * * * *` (every 5 min), `0 0 9 * * *` (daily 9am)
- Next run times
- Enable/disable toggles

**How to capture:**
1. Create scheduled agent via MCP: "Schedule the incident coordinator to run every 5 minutes"
2. Go to Web UI schedules page
3. Screenshot showing agents with cron expressions and next run times

---

## 7. Jaeger Multi-Agent Trace (Line 491)
**Location:** README.md line 491  
**What to show:** Jaeger UI showing multi-agent trace  
**Screenshot shows:**
- Jaeger trace timeline
- incident_coordinator parent span (18.2s)
- Child spans for delegations:
  - assess_severity (0.5s)
  - delegate_logs_investigator (4.1s)
    - __get_logs (3.2s)
  - delegate_metrics_investigator (3.8s)
    - __query_time_series (2.9s)
  - delegate_change_detective (2.4s)
  - synthesize_findings (1.2s)

**How to capture:**
1. Ensure Jaeger is running: http://localhost:16686
2. Run incident coordinator agent
3. Go to Jaeger UI
4. Search for traces from incident_coordinator
5. Open the trace showing multi-agent delegation
6. Screenshot the trace timeline showing parent/child spans

---

## Screenshot Priorities

**High Priority (Critical for understanding):**
1. Team Performance Report (Line 341) - Shows LLM-as-judge capability
2. Claude/Cursor MCP Tools (Line 305) - Shows MCP-driven interaction
3. Jaeger Multi-Agent Trace (Line 491) - Shows observability

**Medium Priority (Nice to have):**
4. Incident Coordinator Run (Line 212) - Shows multi-agent coordination
5. Faker Generating Data (Line 328) - Shows faker system

**Low Priority (Can be text/CLI):**
6. Bundle in Registry (Line 413) - Can show CLI output instead
7. Scheduled Agents (Line 437) - Can show CLI output instead

---

## Notes

- All screenshots should use consistent theme (Tokyo Night if possible)
- Redact any sensitive information (API keys, tokens)
- Use high resolution (at least 1920x1080)
- Crop to relevant area, no excessive white space
- Add subtle border or shadow for visual polish

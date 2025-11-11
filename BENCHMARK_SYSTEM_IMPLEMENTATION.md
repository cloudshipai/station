# Task-Based Benchmark System - Implementation Guide

## 📄 Quick Reference

**Full PRD:** `docs/features/BENCHMARK_SYSTEM_PRD.md`

## 🎯 What We're Building

Transform reports from **abstract metrics** → **production readiness benchmarks**

**Before:**
- "Effectiveness: 7.3/10" (what does this mean?)
- No task completion tracking
- No competitive comparison

**After:**
- "Detected 47/50 SQL injections (94%)" (concrete results)
- Task-based pass/fail criteria
- Champion rankings (🥇 iac-security-scanner beats 🥈 terraform-checker)
- Production readiness: GO/NO-GO decision

## 🏗️ Implementation Phases

### ✅ Phase 1: Data Foundation (Days 1-2)
**Goal:** Leverage ALL existing data (runs, Jaeger, inputs, outputs, tool calls)

**Tasks:**
1. Extend database schema for benchmark tasks
2. Create BenchmarkAnalyzer service
3. Implement evidence extraction from runs + Jaeger traces

### 🔄 Phase 2: Task System (Days 3-4)
**Goal:** Replace abstract criteria with concrete tasks

**Tasks:**
1. Update LLM prompts for task evaluation
2. Build Task Builder UI
3. Create task templates

### 🏆 Phase 3: Competitive Ranking (Days 5-6)
**Goal:** Show which agents win at each task

**Tasks:**
1. Ranking algorithm
2. Champion badges
3. Redesigned report page

### 🧪 Phase 4: Faker Integration (Optional)
**Goal:** Benchmark testing environments

### 📄 Phase 5: PDF Export (Optional)
**Goal:** Executive reports

## 🚀 Current Status

**Completed:**
- ✅ Reports system foundation
- ✅ LLM-as-judge evaluation
- ✅ Basic UI with score display
- ✅ Comprehensive PRD

**In Progress:**
- 🔄 Phase 1.1: Data model updates

**Next Up:**
- ⏳ Phase 1.2: BenchmarkAnalyzer service
- ⏳ Phase 2.1: LLM prompt updates

## 📊 Data We Have Access To

From **agent_runs** table:
- `task` - User's original request
- `final_response` - Agent's output
- `execution_steps` - JSON array of all steps
- `tool_calls` - JSON of tool invocations
- `status`, `duration_seconds`, `input_tokens`, `output_tokens`, `cost`

From **Jaeger traces** (via API):
- Span hierarchy (parent/child relationships)
- Tool call timings
- Error patterns
- Execution flow

From **agent configurations**:
- Prompts
- Max steps
- Tools available

## 🔑 Key Decisions

1. **Keep existing reports table** - extend it, don't replace
2. **Add benchmark_tasks as JSON** - flexible schema
3. **Evidence = run IDs + trace IDs** - link to actual data
4. **LLM analyzes task completion** - not abstract metrics
5. **Competitive ranking** - agents compete per task

## 📝 Example Benchmark Task

```json
{
  "name": "Detect SQL Injection Vulnerabilities",
  "category": "security",
  "scenario": "Scan codebase containing 20 SQL injection vulnerabilities",
  "success_criteria": {
    "detection_rate": {
      "operator": ">=",
      "value": 0.90,
      "description": "Must find at least 90% of vulnerabilities"
    },
    "false_positive_rate": {
      "operator": "<=",
      "value": 0.10,
      "description": "False positives must be under 10%"
    },
    "completion_time": {
      "operator": "<=",
      "value": 180,
      "description": "Must complete within 3 minutes"
    }
  },
  "weight": 0.40,
  "test_data": "benchmarks/security/sql-injection-samples"
}
```

## 💡 Implementation Tips

1. **Start small:** Get one task working end-to-end
2. **Use existing data:** Don't create new test runs, analyze existing ones
3. **LLM does the analysis:** We provide evidence, LLM judges completion
4. **Evidence is key:** Every score must link to actual runs/traces
5. **Faker later:** Phase 1-3 work with existing production data

---

**Next Step:** Extend database schema for benchmark tasks
**File to edit:** `internal/db/migrations/031_add_benchmark_tasks.sql`

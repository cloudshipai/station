# Agent Evaluation & Testing

Station's built-in evaluation system uses LLM-as-judge to automatically test agents, generate performance reports, and ensure production readiness. This guide shows you how to test individual agents, evaluate multi-agent teams, and interpret quality metrics.

## Why Built-In Evaluation?

**Traditional Agent Testing:**
- ❌ Manual testing with ad-hoc prompts
- ❌ Subjective "does this look good?" quality checks
- ❌ No baseline for comparing iterations
- ❌ Can't measure team performance vs. business goals

**Station's Evaluation System:**
- ✅ Automated test scenario generation (100+ scenarios per agent)
- ✅ LLM-as-judge scoring (objective quality metrics)
- ✅ Business-focused team reports (MTTR, accuracy, cost reduction)
- ✅ Full execution traces with Jaeger integration
- ✅ Genkit-compatible datasets for external analysis

---

## Evaluation Workflow

### 1. Test Individual Agents

**Generate test scenarios and execute:**
```typescript
opencode-station_generate_and_test_agent({
  agent_id: "42",
  scenario_count: 100,
  variation_strategy: "comprehensive",
  max_concurrent: 10,
  jaeger_url: "http://localhost:16686"
})
```

**What happens:**
1. **AI generates 100 test scenarios** based on agent's purpose
2. **Executes all scenarios** with full trace capture
3. **Saves results** to timestamped dataset directory
4. **Returns task ID** for progress monitoring

**Parameters:**
- `scenario_count`: Number of test scenarios (default: 100)
- `variation_strategy`: 
  - `"comprehensive"` - Wide range of scenarios (default)
  - `"edge_cases"` - Unusual and boundary conditions
  - `"common"` - Typical real-world use cases
- `max_concurrent`: Parallel executions (default: 10, max: 20)
- `jaeger_url`: Jaeger endpoint for trace collection

**Output:**
```json
{
  "task_id": "test-42-20251119-103045",
  "status": "running",
  "dataset_path": "/workspace/environments/default/datasets/agent-42-20251119-103045"
}
```

---

### 2. Evaluate Dataset Quality

**Run LLM-as-judge evaluation:**
```typescript
opencode-station_evaluate_dataset({
  dataset_path: "/workspace/environments/default/datasets/agent-42-20251119-103045"
})
```

**What happens:**
1. **Loads all test runs** from dataset
2. **LLM-as-judge analyzes each run** for quality, accuracy, completeness
3. **Aggregates scores** across all scenarios
4. **Generates production readiness assessment**

**Output:**
```json
{
  "dataset_path": "/workspace/environments/default/datasets/agent-42-20251119-103045",
  "total_runs": 100,
  "evaluated": 100,
  "aggregate_scores": {
    "quality": 8.2,
    "accuracy": 0.89,
    "completeness": 0.85,
    "relevance": 0.87
  },
  "tool_effectiveness": {
    "__get_cost_and_usage": 0.92,
    "__list_cost_allocation_tags": 0.88,
    "__describe_ec2_instances": 0.75
  },
  "production_ready": true,
  "recommendations": [
    "Agent performs well on typical cost analysis scenarios",
    "Consider adding more context for multi-region cost breakdowns",
    "Tool effectiveness is high for core FinOps operations"
  ],
  "evaluation_completed_at": "2025-11-19T11:00:00Z"
}
```

---

### 3. Create Team Performance Reports

**Define business goals:**
```typescript
opencode-station_create_report({
  name: "SRE Team Q4 Performance",
  environment_id: "3",
  description: "Quarterly evaluation of SRE incident response team",
  team_criteria: JSON.stringify({
    goal: "Minimize incident MTTR and prevent recurring issues",
    criteria: {
      mttr_reduction: {
        weight: 0.4,
        description: "Reduce mean time to resolution by 30%",
        threshold: 0.7
      },
      root_cause_accuracy: {
        weight: 0.3,
        description: "Correctly identify root causes in 90% of incidents",
        threshold: 0.9
      },
      prevention_rate: {
        weight: 0.3,
        description: "Prevent 50% of similar incidents from recurring",
        threshold: 0.5
      }
    }
  }),
  agent_criteria: JSON.stringify({
    "Incident Coordinator": {
      delegation_quality: { 
        weight: 0.5, 
        threshold: 0.8,
        description: "Routes incidents to correct specialists"
      },
      coordination_speed: { 
        weight: 0.5, 
        threshold: 0.7,
        description: "Quickly synthesizes specialist findings"
      }
    },
    "Kubernetes Expert": {
      diagnostic_accuracy: { 
        weight: 0.6, 
        threshold: 0.9,
        description: "Correctly diagnoses K8s issues"
      },
      remediation_quality: { 
        weight: 0.4, 
        threshold: 0.8,
        description: "Provides actionable kubectl commands"
      }
    }
  })
})
```

**Generate report:**
```typescript
opencode-station_generate_report({ report_id: "5" })
```

**What happens:**
1. **Generates test scenarios** for each agent
2. **Executes comprehensive testing** (100+ scenarios per agent)
3. **LLM-as-judge evaluates** against business criteria
4. **Calculates team score** (weighted average)
5. **Produces PDF report** with detailed breakdown

---

### 4. Review Results

**Get report details:**
```typescript
opencode-station_get_report({ report_id: "5" })
```

**Output:**
```json
{
  "id": "5",
  "name": "SRE Team Q4 Performance",
  "environment_id": "3",
  "status": "completed",
  "team_score": 7.5,
  "team_breakdown": {
    "mttr_reduction": 0.72,
    "root_cause_accuracy": 0.88,
    "prevention_rate": 0.58
  },
  "agent_scores": {
    "Incident Coordinator": 8.2,
    "Kubernetes Expert": 7.8,
    "Log Analyzer": 7.1,
    "Metrics Analyzer": 7.5,
    "Network Diagnostics": 6.9,
    "Database Troubleshooter": 7.8,
    "Application Performance Analyzer": 7.4,
    "Security Incident Responder": 7.2,
    "Remediation Executor": 6.8
  },
  "generated_at": "2025-11-19T12:00:00Z",
  "pdf_path": "/workspace/reports/sre-team-q4-performance.pdf"
}
```

---

## Understanding Scores

### Quality Metrics

**Overall Quality Score (0-10)**
- **9-10**: Production-ready, excellent performance
- **7-8**: Good performance, minor improvements needed
- **5-6**: Acceptable, needs refinement
- **<5**: Not production-ready, significant issues

**Component Metrics (0-1.0)**
- **Accuracy**: Correctness of technical analysis
- **Completeness**: Thoroughness of investigation
- **Relevance**: Focus on actual problem vs. tangents
- **Efficiency**: Steps taken vs. optimal path

---

### Team Criteria Examples

#### FinOps Team
```json
{
  "goal": "Maximize cloud cost savings and forecast accuracy",
  "criteria": {
    "cost_savings_identified": {
      "weight": 0.4,
      "description": "Identify $100K+ annual savings opportunities",
      "threshold": 0.8
    },
    "forecast_accuracy": {
      "weight": 0.3,
      "description": "Forecast within 5% of actual spend",
      "threshold": 0.95
    },
    "execution_cost": {
      "weight": 0.3,
      "description": "Keep AI/API costs under $500/month",
      "threshold": 0.9
    }
  }
}
```

---

#### Security Team
```json
{
  "goal": "Detect vulnerabilities and maintain compliance",
  "criteria": {
    "vulnerability_detection": {
      "weight": 0.5,
      "description": "Detect 95% of OWASP Top 10 issues",
      "threshold": 0.95
    },
    "false_positive_rate": {
      "weight": 0.3,
      "description": "Keep false positives under 10%",
      "threshold": 0.9
    },
    "compliance_coverage": {
      "weight": 0.2,
      "description": "Cover 100% of CIS benchmarks",
      "threshold": 1.0
    }
  }
}
```

---

#### DevOps Team
```json
{
  "goal": "Improve deployment reliability and speed",
  "criteria": {
    "deployment_insights": {
      "weight": 0.4,
      "description": "Detect 90% of deployment risks pre-production",
      "threshold": 0.9
    },
    "failure_prediction": {
      "weight": 0.3,
      "description": "Predict 80% of deployment failures",
      "threshold": 0.8
    },
    "remediation_speed": {
      "weight": 0.3,
      "description": "Reduce rollback time by 50%",
      "threshold": 0.5
    }
  }
}
```

---

## Testing Strategies

### Strategy 1: Comprehensive Testing

**Goal:** Validate agent across all scenarios

**Approach:**
```typescript
opencode-station_generate_and_test_agent({
  agent_id: "42",
  scenario_count: 100,
  variation_strategy: "comprehensive"
})
```

**Scenarios Generated:**
- 30% typical use cases
- 40% edge cases
- 20% error conditions
- 10% boundary conditions

**Use Case:** Pre-production validation

---

### Strategy 2: Edge Case Testing

**Goal:** Find agent weaknesses

**Approach:**
```typescript
opencode-station_generate_and_test_agent({
  agent_id: "42",
  scenario_count: 50,
  variation_strategy: "edge_cases"
})
```

**Scenarios Generated:**
- Unusual data patterns
- Missing information
- Ambiguous queries
- Conflicting requirements
- Resource limits

**Use Case:** Hardening before production

---

### Strategy 3: Common Case Testing

**Goal:** Validate 80% use cases work

**Approach:**
```typescript
opencode-station_generate_and_test_agent({
  agent_id: "42",
  scenario_count: 30,
  variation_strategy: "common"
})
```

**Scenarios Generated:**
- Most frequent user requests
- Standard workflows
- Typical data patterns

**Use Case:** Rapid iteration during development

---

### Strategy 4: Batch Model Comparison

**Goal:** Compare GPT-4o vs GPT-4o-mini performance

**Approach:**
```typescript
// Create two versions of same agent with different models
const agent_gpt4o = opencode-station_create_agent({
  name: "Cost Analyzer (GPT-4o)",
  prompt: "...",
  // Uses gpt-4o from dotprompt
})

const agent_gpt4o_mini = opencode-station_create_agent({
  name: "Cost Analyzer (GPT-4o-mini)",
  prompt: "...",
  // Uses gpt-4o-mini from dotprompt
})

// Test both in parallel
opencode-station_batch_execute_agents({
  tasks: JSON.stringify([
    {agent_id: agent_gpt4o.id, task: "Analyze costs"},
    {agent_id: agent_gpt4o_mini.id, task: "Analyze costs"}
  ]),
  iterations: 50,
  max_concurrent: 10
})

// Compare results
const gpt4o_runs = opencode-station_list_runs_by_model({
  model_name: "openai/gpt-4o"
})

const gpt4o_mini_runs = opencode-station_list_runs_by_model({
  model_name: "openai/gpt-4o-mini"
})
```

**Metrics to Compare:**
- Quality score difference
- Execution time (GPT-4o is slower)
- Token usage (GPT-4o is more expensive)
- Accuracy on complex scenarios

---

## LLM-as-Judge Evaluation

### How It Works

**Input to Judge:**
```json
{
  "agent_task": "Analyze AWS costs for last 30 days",
  "agent_output": "Cost Analysis:\n1. EC2: $45,231 (35%)\n2. RDS: $32,100 (25%)...",
  "tool_calls": [
    {"tool": "__get_cost_and_usage", "result": "{...}"},
    {"tool": "__list_cost_allocation_tags", "result": "{...}"}
  ],
  "execution_steps": [
    {"step": 1, "action": "Retrieved cost data"},
    {"step": 2, "action": "Analyzed cost trends"}
  ]
}
```

**Judge Prompt:**
```
Evaluate the agent's performance on this cost analysis task.

CRITERIA:
1. Accuracy: Did the agent correctly interpret the cost data?
2. Completeness: Did it analyze all major cost drivers?
3. Relevance: Did it focus on actionable insights?
4. Efficiency: Were the tool calls necessary?

Rate each criterion 0-10 and provide overall quality score.
```

**Judge Output:**
```json
{
  "quality_score": 8.2,
  "accuracy": 9.0,
  "completeness": 8.5,
  "relevance": 8.0,
  "efficiency": 7.5,
  "feedback": "Agent correctly identified top cost drivers. Good use of cost allocation tags. Could improve by comparing against previous periods."
}
```

---

### Customizing Judge Criteria

**Example: Security Scanner Agent**
```typescript
opencode-station_create_report({
  name: "Security Scanner Evaluation",
  environment_id: "4",
  team_criteria: JSON.stringify({
    goal: "Comprehensive security scanning with low false positives",
    criteria: {
      vulnerability_coverage: {
        weight: 0.4,
        description: "Detect all OWASP Top 10 vulnerabilities",
        threshold: 0.95,
        judge_prompt: "Did the agent scan for SQL injection, XSS, CSRF, etc.?"
      },
      false_positive_rate: {
        weight: 0.3,
        description: "Minimize false positives",
        threshold: 0.9,
        judge_prompt: "Are the reported vulnerabilities real issues?"
      },
      remediation_quality: {
        weight: 0.3,
        description: "Provide actionable fix recommendations",
        threshold: 0.8,
        judge_prompt: "Are the remediation steps clear and correct?"
      }
    }
  })
})
```

---

## Observability

### Jaeger Traces

Every test execution produces Jaeger traces showing:
- **Agent execution span**: Total runtime
- **Tool call spans**: Each tool invocation
- **LLM call spans**: AI model inference time
- **Multi-agent spans**: Coordinator → specialist delegations

**Start Jaeger:**
```bash
make jaeger
# Navigate to http://localhost:16686
```

**Search for traces:**
- Service: `station-agent-<agent-name>`
- Operation: `execute_agent`
- Tags: `agent.id`, `run.id`, `test.scenario`

**Example Trace:**
```
[AWS Cost Analyzer] 12.5s
  ├── [LLM Inference] 2.1s
  ├── [__get_cost_and_usage] 3.2s
  ├── [LLM Inference] 1.8s
  ├── [__list_cost_allocation_tags] 1.1s
  ├── [LLM Inference] 2.0s
  └── [Final Response] 0.3s
```

**Insights:**
- Which tool calls are bottlenecks?
- How many LLM calls per execution?
- What's the total latency?

---

### Detailed Run Inspection

**Inspect single run:**
```typescript
opencode-station_inspect_run({
  run_id: "1234",
  verbose: true
})
```

**Shows:**
```json
{
  "id": "1234",
  "agent_id": "42",
  "status": "success",
  "tool_calls": [
    {
      "tool": "__get_cost_and_usage",
      "arguments": {"time_period": "LAST_30_DAYS"},
      "result": "{\"costs\": [...]}",
      "duration_ms": 3200,
      "timestamp": "2025-11-19T10:00:01Z"
    }
  ],
  "execution_steps": [
    {"step": 1, "action": "Called __get_cost_and_usage"},
    {"step": 2, "action": "Analyzed cost breakdown"},
    {"step": 3, "action": "Generated recommendations"}
  ],
  "token_usage": {
    "input_tokens": 1250,
    "output_tokens": 890,
    "total_tokens": 2140
  },
  "llm_calls": [
    {
      "model": "gpt-4o-mini",
      "input_tokens": 800,
      "output_tokens": 450,
      "duration_ms": 2100
    }
  ]
}
```

---

## Dataset Export

### Export for External Analysis

**Export runs to Genkit format:**
```typescript
opencode-station_export_dataset({
  filter_agent_id: "42",
  limit: 100,
  output_dir: "/workspace/exports"
})
```

**Output:**
```
/workspace/exports/
└── dataset-20251119-120000.json  # Genkit-compatible format
```

**Use Cases:**
- Import to Genkit for advanced analysis
- Share datasets with team for review
- Archive test results for compliance
- Export to data warehouse for BI

---

## Continuous Evaluation

### Scheduled Testing

**Run nightly evaluations:**
```typescript
// Create evaluation agent
const eval_agent = opencode-station_create_agent({
  name: "Nightly Agent Evaluator",
  description: "Runs comprehensive testing on all production agents",
  prompt: `Test all agents in production environment and report quality scores`,
  environment_id: "1"
})

// Schedule nightly at 2 AM
opencode-station_set_schedule({
  agent_id: eval_agent.id,
  cron_schedule: "0 0 2 * * *",
  enabled: true
})
```

---

### Regression Detection

**Compare new version vs. baseline:**
```typescript
// Test baseline (current production agent)
const baseline = opencode-station_generate_and_test_agent({
  agent_id: "42",  // v1.0
  scenario_count: 50
})

// Update agent prompt (new version)
opencode-station_update_agent_prompt({
  agent_id: "42",
  prompt: "Enhanced prompt with new capabilities..."
})

// Test new version
const new_version = opencode-station_generate_and_test_agent({
  agent_id: "42",  // v2.0
  scenario_count: 50
})

// Compare scores
baseline_eval = opencode-station_evaluate_dataset({
  dataset_path: baseline.dataset_path
})

new_eval = opencode-station_evaluate_dataset({
  dataset_path: new_version.dataset_path
})

// Alert if regression
if (new_eval.quality < baseline.quality - 0.5) {
  alert("Quality regression detected!")
}
```

---

## Best Practices

### 1. Test Early and Often

**During development:**
- Test with 10-20 scenarios after each prompt change
- Use `variation_strategy: "common"` for fast feedback
- Iterate on prompts until quality > 7.0

**Before production:**
- Run comprehensive testing (100+ scenarios)
- Use `variation_strategy: "comprehensive"`
- Ensure quality > 8.0 for critical agents

---

### 2. Define Business-Focused Criteria

**❌ Avoid technical-only criteria:**
```json
{
  "criteria": {
    "uses_correct_tools": { "weight": 0.5 },
    "follows_prompt_instructions": { "weight": 0.5 }
  }
}
```

**✅ Use business-focused criteria:**
```json
{
  "criteria": {
    "cost_savings_identified": { 
      "weight": 0.6,
      "description": "Identifies real cost optimization opportunities"
    },
    "time_to_resolution": { 
      "weight": 0.4,
      "description": "Provides answers within 30 seconds"
    }
  }
}
```

---

### 3. Monitor Production Performance

**Track real-world metrics:**
```typescript
// Weekly production report
opencode-station_create_report({
  name: "Production Performance - Week 47",
  environment_id: "prod",
  team_criteria: JSON.stringify({
    goal: "Maintain SLOs and user satisfaction",
    criteria: {
      response_quality: { weight: 0.5, threshold: 0.8 },
      response_time: { weight: 0.3, threshold: 0.9 },
      user_satisfaction: { weight: 0.2, threshold: 0.85 }
    }
  })
})
```

---

### 4. Use Jaeger for Debugging

**When quality score is low:**
1. Export dataset runs
2. Find failing scenarios in Jaeger
3. Inspect tool calls and LLM responses
4. Identify patterns in failures
5. Update prompt to address issues

---

### 5. Compare Models with Real Data

**Don't assume GPT-4o is always better:**
```typescript
// Test both models on same scenarios
opencode-station_batch_execute_agents({
  tasks: JSON.stringify([
    {agent_id: "agent-gpt4o", task: "scenario1"},
    {agent_id: "agent-gpt4o-mini", task: "scenario1"},
    {agent_id: "agent-gpt4o", task: "scenario2"},
    {agent_id: "agent-gpt4o-mini", task: "scenario2"}
  ]),
  iterations: 50
})

// Compare cost vs. quality
// GPT-4o-mini might be 70% cheaper with only 5% quality drop
```

---

## Troubleshooting

### Issue: Low Quality Scores (<6.0)

**Debug Process:**
1. **Inspect failing runs**: `opencode-station_inspect_run({ run_id: "...", verbose: true })`
2. **Check tool effectiveness**: Are tools returning useful data?
3. **Review LLM-as-judge feedback**: What specific issues were identified?
4. **Look at Jaeger traces**: Are there timeouts or errors?
5. **Test specialist isolation**: For multi-agent teams, test specialists individually

**Common Causes:**
- Prompt is too vague or generic
- Tools are misconfigured or returning errors
- Agent attempting tasks beyond its expertise
- Insufficient context provided in prompts

---

### Issue: Inconsistent Scores

**Symptoms:** Same scenario gets 8.0, then 5.0, then 9.0

**Causes:**
- LLM-as-judge is non-deterministic
- Scenarios are poorly defined
- Agent has non-deterministic tool usage

**Solutions:**
- Run multiple iterations per scenario (5-10)
- Use temperature=0 for more consistent LLM calls
- Add scenario validation to ensure test quality

---

### Issue: Expensive Evaluation

**Symptoms:** Testing costs $50+ per agent

**Causes:**
- Too many scenarios (100+ with GPT-4o)
- Scenarios trigger many tool calls
- LLM-as-judge uses expensive model

**Solutions:**
- Use GPT-4o-mini for evaluation (70% cheaper)
- Reduce scenario count for development testing
- Use `variation_strategy: "common"` during iteration

---

## Next Steps

- [Multi-Agent Teams](./multi-agent-teams.md) - Build and test coordinator hierarchies
- [Station MCP Tools](./station-mcp-tools.md) - Full evaluation tool reference
- [Agent Development](./agent-development.md) - Writing testable prompts
- [Examples](./examples.md) - Real-world evaluation examples

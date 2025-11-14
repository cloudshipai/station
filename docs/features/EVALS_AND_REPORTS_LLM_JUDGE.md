# Station Evals & Reports with LLM-as-Judge

**Status**: Design Document  
**Date**: 2025-01-11  
**Priority**: High  
**Goal**: Automated agent evaluation with LLM judges and comprehensive reporting

---

## Executive Summary

Station needs an evaluation framework to test agent quality, track regressions, and provide confidence in agent behavior. This system uses **LLMs as judges** to evaluate agent outputs against expected criteria, generating structured reports with scores, rationale, and improvement suggestions.

---

## Problem Statement

### Current Gaps

**No Quality Assurance**:
- Agents can break silently after prompt changes
- No way to validate agent outputs meet requirements
- Manual testing is time-consuming and inconsistent
- Can't detect regressions when updating agents

**No Performance Tracking**:
- Don't know if agents are getting better or worse over time
- Can't compare different prompt versions
- No metrics for agent quality

**No Systematic Testing**:
- Each agent tested ad-hoc
- No reusable test scenarios
- Can't run comprehensive test suites

---

## Solution: LLM-as-Judge Evaluation Framework

### Core Concept

Use a **separate LLM as a judge** to evaluate agent outputs:

```
┌─────────────────┐
│  Agent Under    │  "Analyze AWS cost spike"
│  Test           │  ─────────────────────────>  Agent Output
└─────────────────┘                                    │
                                                       │
                                                       v
┌─────────────────┐                            ┌────────────┐
│  LLM Judge      │  <─────────────────────── │  Eval       │
│  (GPT-4 etc)    │   Evaluate output          │  Criteria   │
└─────────────────┘                            └────────────┘
        │
        v
┌─────────────────┐
│  Structured     │  { score: 8.5, pass: true, reasoning: "..." }
│  Eval Result    │
└─────────────────┘
```

### Key Features

1. **Automated Evaluation**: Run evals on-demand or in CICD
2. **Structured Scoring**: Numerical scores (0-10) with pass/fail thresholds
3. **Detailed Reasoning**: LLM explains why it scored the way it did
4. **Multiple Criteria**: Evaluate accuracy, completeness, relevance, safety
5. **Historical Tracking**: Compare eval results over time
6. **Report Generation**: Beautiful HTML/PDF reports with charts

---

## Architecture

### Database Schema

```sql
-- Evaluation scenarios (test cases for agents)
CREATE TABLE eval_scenarios (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    agent_id INTEGER,
    input_prompt TEXT NOT NULL,          -- What to send to agent
    expected_behavior TEXT,              -- What agent should do
    eval_criteria JSONB NOT NULL,        -- Criteria for judging
    tags TEXT,                           -- comma-separated tags
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (agent_id) REFERENCES agents(id)
);

-- Evaluation runs (execution of eval scenarios)
CREATE TABLE eval_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    scenario_id INTEGER NOT NULL,
    agent_id INTEGER NOT NULL,
    agent_run_id INTEGER,               -- Link to actual agent execution
    status TEXT NOT NULL,               -- 'pending', 'running', 'completed', 'failed'
    
    -- Agent output
    agent_output TEXT,
    agent_duration_seconds REAL,
    agent_tokens INTEGER,
    agent_cost REAL,
    agent_error TEXT,
    
    -- Judge evaluation
    judge_model TEXT,                   -- Which LLM was the judge
    judge_score REAL,                   -- 0-10 score
    judge_passed BOOLEAN,               -- Did it pass threshold?
    judge_reasoning TEXT,               -- Why did judge score this way?
    judge_criteria_scores JSONB,        -- Individual criterion scores
    judge_suggestions TEXT,             -- Improvement suggestions
    judge_duration_seconds REAL,
    judge_tokens INTEGER,
    judge_cost REAL,
    
    -- Metadata
    eval_batch_id TEXT,                 -- Group related eval runs
    git_commit TEXT,                    -- Track which code version
    environment TEXT,                   -- Which environment
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (scenario_id) REFERENCES eval_scenarios(id),
    FOREIGN KEY (agent_id) REFERENCES agents(id),
    FOREIGN KEY (agent_run_id) REFERENCES agent_runs(id)
);

-- Evaluation criteria templates
CREATE TABLE eval_criteria_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    criteria JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Example criteria JSON structure:
{
  "accuracy": {
    "weight": 0.4,
    "description": "Does the output contain correct information?",
    "threshold": 7.0
  },
  "completeness": {
    "weight": 0.3,
    "description": "Does it address all aspects of the question?",
    "threshold": 6.0
  },
  "relevance": {
    "weight": 0.2,
    "description": "Is the output focused on the user's request?",
    "threshold": 7.0
  },
  "safety": {
    "weight": 0.1,
    "description": "Does it avoid harmful or sensitive information?",
    "threshold": 9.0
  }
}
```

---

## Evaluation Criteria System

### Built-in Criteria Templates

**1. General Quality (Default)**
```yaml
accuracy:
  weight: 0.4
  description: "Factual correctness of information"
  threshold: 7.0
  
completeness:
  weight: 0.3
  description: "Addresses all aspects of the request"
  threshold: 6.0
  
relevance:
  weight: 0.2
  description: "Stays focused on user's needs"
  threshold: 7.0
  
clarity:
  weight: 0.1
  description: "Clear and understandable output"
  threshold: 6.0
```

**2. AWS Cost Analysis**
```yaml
cost_identification:
  weight: 0.3
  description: "Correctly identifies cost drivers"
  threshold: 8.0
  
data_accuracy:
  weight: 0.3
  description: "Accurate cost numbers and calculations"
  threshold: 9.0
  
recommendations:
  weight: 0.25
  description: "Actionable cost optimization suggestions"
  threshold: 7.0
  
compliance:
  weight: 0.15
  description: "Follows FinOps best practices"
  threshold: 7.0
```

**3. Security Analysis**
```yaml
vulnerability_detection:
  weight: 0.4
  description: "Finds actual security issues"
  threshold: 8.0
  
false_positives:
  weight: 0.2
  description: "Minimizes false alarms (10 - false_positive_rate)"
  threshold: 7.0
  
severity_assessment:
  weight: 0.2
  description: "Correctly prioritizes findings"
  threshold: 7.0
  
remediation:
  weight: 0.2
  description: "Provides actionable fixes"
  threshold: 6.0
```

**4. Code Generation**
```yaml
correctness:
  weight: 0.4
  description: "Code works as intended"
  threshold: 9.0
  
style:
  weight: 0.2
  description: "Follows best practices and conventions"
  threshold: 7.0
  
documentation:
  weight: 0.2
  description: "Includes helpful comments"
  threshold: 6.0
  
efficiency:
  weight: 0.2
  description: "Optimal algorithm and resource usage"
  threshold: 6.0
```

---

## LLM Judge System

### Judge Prompt Template

```
You are an expert evaluator assessing AI agent outputs. Your task is to objectively evaluate the quality of an agent's response.

**Agent Task:**
{agent_input}

**Agent Output:**
{agent_output}

**Expected Behavior:**
{expected_behavior}

**Evaluation Criteria:**
{criteria_descriptions}

**Instructions:**
1. Evaluate the agent's output against each criterion
2. Assign a score from 0-10 for each criterion (0=terrible, 10=perfect)
3. Provide detailed reasoning for each score
4. Calculate weighted overall score
5. Determine if output meets minimum thresholds
6. Suggest specific improvements

**Output Format (JSON):**
{
  "overall_score": <float>,
  "passed": <boolean>,
  "criteria_scores": {
    "criterion_name": {
      "score": <float>,
      "reasoning": "<detailed explanation>",
      "examples": ["<specific example from output>"]
    }
  },
  "strengths": ["<what agent did well>"],
  "weaknesses": ["<what agent could improve>"],
  "suggestions": ["<actionable improvement>"],
  "verdict": "<brief summary>"
}

Be objective, specific, and constructive in your evaluation.
```

### Judge Model Selection

**Recommended Models**:
1. **GPT-4o** - Best quality, most expensive
2. **GPT-4o-mini** - Good balance of quality/cost
3. **Claude 3.5 Sonnet** - Excellent reasoning
4. **Gemini 2.0 Flash** - Fast and cost-effective

**Configuration**:
```yaml
judge:
  model: "gpt-4o-mini"
  temperature: 0.1  # Low temperature for consistency
  max_tokens: 2000
  timeout: 30s
```

---

## CLI Commands

### Create Eval Scenario

```bash
# Create from template
stn eval create \
  --agent "cost-spike-investigator" \
  --name "aws-cost-spike-detection" \
  --input "Analyze the cost spike in production on Nov 10" \
  --criteria aws-cost-analysis

# Create with custom criteria
stn eval create \
  --agent "security-scanner" \
  --name "terraform-security-scan" \
  --input "Scan terraform/ for security issues" \
  --criteria-file ./custom-security-criteria.yaml
```

### Run Evaluation

```bash
# Run single eval
stn eval run aws-cost-spike-detection

# Run all evals for agent
stn eval run --agent cost-spike-investigator

# Run eval batch (for CICD)
stn eval batch \
  --agents "cost-spike-investigator,security-scanner" \
  --git-commit $GIT_COMMIT \
  --output report.html

# Run with custom judge model
stn eval run aws-cost-spike-detection --judge gpt-4o
```

### View Results

```bash
# List eval runs
stn eval list

# Show specific eval result
stn eval show <eval-run-id>

# Compare eval runs
stn eval compare <run-id-1> <run-id-2>

# Generate report
stn eval report --batch <batch-id> --format html --output ./report.html
```

### Manage Scenarios

```bash
# List scenarios
stn eval scenarios

# Edit scenario
stn eval edit aws-cost-spike-detection

# Delete scenario
stn eval delete aws-cost-spike-detection

# Export scenarios
stn eval export --agent cost-spike-investigator --output scenarios.yaml
```

---

## Service Implementation

### EvalRunner Service

```go
// internal/services/eval_runner.go
package services

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
)

type EvalRunner struct {
    agentService    *AgentService
    judgeModel      string
    judgeAPIKey     string
    repos           *repositories.Repositories
}

type EvalScenario struct {
    ID               int64
    Name             string
    AgentID          int64
    InputPrompt      string
    ExpectedBehavior string
    Criteria         map[string]EvalCriterion
    Tags             []string
}

type EvalCriterion struct {
    Weight      float64 `json:"weight"`
    Description string  `json:"description"`
    Threshold   float64 `json:"threshold"`
}

type EvalResult struct {
    ScenarioID       int64
    AgentRunID       int64
    OverallScore     float64
    Passed           bool
    CriteriaScores   map[string]CriterionScore
    Strengths        []string
    Weaknesses       []string
    Suggestions      []string
    Verdict          string
    JudgeModel       string
    JudgeDuration    time.Duration
    JudgeTokens      int
    JudgeCost        float64
}

type CriterionScore struct {
    Score     float64  `json:"score"`
    Reasoning string   `json:"reasoning"`
    Examples  []string `json:"examples"`
}

// RunEvaluation executes an eval scenario
func (e *EvalRunner) RunEvaluation(ctx context.Context, scenarioID int64) (*EvalResult, error) {
    // 1. Load scenario
    scenario, err := e.repos.EvalScenarios.GetByID(scenarioID)
    if err != nil {
        return nil, fmt.Errorf("failed to load scenario: %w", err)
    }
    
    // 2. Execute agent
    agentOutput, runID, err := e.executeAgent(ctx, scenario)
    if err != nil {
        return nil, fmt.Errorf("agent execution failed: %w", err)
    }
    
    // 3. Call LLM judge
    judgeResult, err := e.callJudge(ctx, scenario, agentOutput)
    if err != nil {
        return nil, fmt.Errorf("judge evaluation failed: %w", err)
    }
    
    // 4. Save eval run
    evalRun := &models.EvalRun{
        ScenarioID:       scenarioID,
        AgentID:          scenario.AgentID,
        AgentRunID:       runID,
        Status:           "completed",
        AgentOutput:      agentOutput.Text,
        AgentDuration:    agentOutput.Duration,
        AgentTokens:      agentOutput.Tokens,
        AgentCost:        agentOutput.Cost,
        JudgeModel:       e.judgeModel,
        JudgeScore:       judgeResult.OverallScore,
        JudgePassed:      judgeResult.Passed,
        JudgeReasoning:   judgeResult.Verdict,
        JudgeCriteriaScores: judgeResult.CriteriaScores,
        JudgeSuggestions: strings.Join(judgeResult.Suggestions, "\n"),
        JudgeDuration:    judgeResult.JudgeDuration.Seconds(),
        JudgeTokens:      judgeResult.JudgeTokens,
        JudgeCost:        judgeResult.JudgeCost,
    }
    
    if err := e.repos.EvalRuns.Create(evalRun); err != nil {
        return nil, fmt.Errorf("failed to save eval run: %w", err)
    }
    
    return judgeResult, nil
}

// callJudge uses LLM to evaluate agent output
func (e *EvalRunner) callJudge(ctx context.Context, scenario *EvalScenario, agentOutput *AgentOutput) (*EvalResult, error) {
    // Build judge prompt
    judgePrompt := e.buildJudgePrompt(scenario, agentOutput)
    
    // Call LLM (using GenKit or direct API)
    startTime := time.Now()
    response, err := e.callLLM(ctx, judgePrompt)
    judgeDuration := time.Since(startTime)
    
    if err != nil {
        return nil, err
    }
    
    // Parse JSON response
    var result EvalResult
    if err := json.Unmarshal([]byte(response.Text), &result); err != nil {
        return nil, fmt.Errorf("failed to parse judge response: %w", err)
    }
    
    // Add metadata
    result.JudgeModel = e.judgeModel
    result.JudgeDuration = judgeDuration
    result.JudgeTokens = response.Tokens
    result.JudgeCost = e.calculateCost(response.Tokens)
    
    return &result, nil
}

// buildJudgePrompt constructs the evaluation prompt
func (e *EvalRunner) buildJudgePrompt(scenario *EvalScenario, output *AgentOutput) string {
    criteriaDesc := ""
    for name, criterion := range scenario.Criteria {
        criteriaDesc += fmt.Sprintf("- **%s** (weight: %.1f, threshold: %.1f): %s\n",
            name, criterion.Weight, criterion.Threshold, criterion.Description)
    }
    
    return fmt.Sprintf(`You are an expert evaluator assessing AI agent outputs.

**Agent Task:**
%s

**Agent Output:**
%s

**Expected Behavior:**
%s

**Evaluation Criteria:**
%s

Evaluate the agent's output against each criterion and provide structured feedback in JSON format.`,
        scenario.InputPrompt,
        output.Text,
        scenario.ExpectedBehavior,
        criteriaDesc,
    )
}
```

---

## Report Generation

### HTML Report Template

```html
<!DOCTYPE html>
<html>
<head>
    <title>Agent Evaluation Report</title>
    <style>
        body { font-family: system-ui; max-width: 1200px; margin: 40px auto; }
        .score { font-size: 48px; font-weight: bold; }
        .pass { color: #10b981; }
        .fail { color: #ef4444; }
        .criterion { border-left: 4px solid #3b82f6; padding-left: 16px; margin: 16px 0; }
        .chart { width: 100%; height: 300px; }
    </style>
</head>
<body>
    <h1>Agent Evaluation Report</h1>
    
    <div class="summary">
        <h2>Overall Score: <span class="score {{if .Passed}}pass{{else}}fail{{end}}">{{.OverallScore}}/10</span></h2>
        <p><strong>Status:</strong> {{if .Passed}}✅ PASSED{{else}}❌ FAILED{{end}}</p>
        <p><strong>Agent:</strong> {{.AgentName}}</p>
        <p><strong>Scenario:</strong> {{.ScenarioName}}</p>
        <p><strong>Judge:</strong> {{.JudgeModel}}</p>
        <p><strong>Date:</strong> {{.Timestamp}}</p>
    </div>
    
    <div class="criteria-scores">
        <h3>Criteria Breakdown</h3>
        {{range .CriteriaScores}}
        <div class="criterion">
            <h4>{{.Name}}: {{.Score}}/10 (weight: {{.Weight}})</h4>
            <p>{{.Reasoning}}</p>
            {{if .Examples}}
            <ul>
                {{range .Examples}}<li>{{.}}</li>{{end}}
            </ul>
            {{end}}
        </div>
        {{end}}
    </div>
    
    <div class="feedback">
        <h3>Strengths</h3>
        <ul>{{range .Strengths}}<li>{{.}}</li>{{end}}</ul>
        
        <h3>Areas for Improvement</h3>
        <ul>{{range .Weaknesses}}<li>{{.}}</li>{{end}}</ul>
        
        <h3>Suggestions</h3>
        <ul>{{range .Suggestions}}<li>{{.}}</li>{{end}}</ul>
    </div>
    
    <div class="details">
        <h3>Execution Details</h3>
        <table>
            <tr><td>Agent Duration:</td><td>{{.AgentDuration}}</td></tr>
            <tr><td>Agent Tokens:</td><td>{{.AgentTokens}}</td></tr>
            <tr><td>Agent Cost:</td><td>${{.AgentCost}}</td></tr>
            <tr><td>Judge Duration:</td><td>{{.JudgeDuration}}</td></tr>
            <tr><td>Judge Tokens:</td><td>{{.JudgeTokens}}</td></tr>
            <tr><td>Judge Cost:</td><td>${{.JudgeCost}}</td></tr>
        </table>
    </div>
</body>
</html>
```

---

## UI Components

### Evals Page

```
┌─────────────────────────────────────────────────────┐
│ Evaluations                             [+ New Eval] │
├─────────────────────────────────────────────────────┤
│ Scenarios | Runs | Reports                           │
├─────────────────────────────────────────────────────┤
│                                                       │
│ ┌──────────────────────────────────────────────┐   │
│ │ aws-cost-spike-detection                 ✅  │   │
│ │ Tests cost spike analysis accuracy           │   │
│ │ Last run: 2 hours ago | Score: 8.5/10        │   │
│ │ [Run] [Edit] [View Results]                  │   │
│ └──────────────────────────────────────────────┘   │
│                                                       │
│ ┌──────────────────────────────────────────────┐   │
│ │ terraform-security-scan                  ❌  │   │
│ │ Validates security issue detection           │   │
│ │ Last run: 1 day ago | Score: 6.2/10          │   │
│ │ [Run] [Edit] [View Results]                  │   │
│ └──────────────────────────────────────────────┘   │
│                                                       │
└─────────────────────────────────────────────────────┘
```

### Eval Run Details

```
┌─────────────────────────────────────────────────────┐
│ Eval Run #123                                        │
│ aws-cost-spike-detection | cost-spike-investigator  │
├─────────────────────────────────────────────────────┤
│                                                       │
│ Overall Score: 8.5/10 ✅ PASSED                     │
│                                                       │
│ Criteria Breakdown:                                  │
│ ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  │
│ Cost Identification: 9.0/10 ████████████████░░      │
│ Data Accuracy: 8.5/10      ██████████████░░░░      │
│ Recommendations: 8.0/10    ████████████░░░░░░      │
│ Compliance: 8.5/10         ██████████████░░░░      │
│                                                       │
│ 💪 Strengths:                                        │
│ • Correctly identified S3 storage cost increase      │
│ • Accurate cost calculations                         │
│ • Clear, actionable recommendations                  │
│                                                       │
│ ⚠️ Areas for Improvement:                           │
│ • Could include more cost optimization options       │
│ • Missing historical trend analysis                  │
│                                                       │
│ 💡 Suggestions:                                      │
│ • Add comparison with previous month's costs         │
│ • Include forecasted costs if current trend continues│
│                                                       │
│ [View Agent Output] [Re-run Eval] [Export Report]   │
│                                                       │
└─────────────────────────────────────────────────────┘
```

---

## CICD Integration

### GitHub Actions Example

```yaml
name: Agent Evaluation

on:
  push:
    paths:
      - 'agents/**'
  pull_request:
    paths:
      - 'agents/**'

jobs:
  eval:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install Station
        run: curl -sSL https://install.station.dev | bash
      
      - name: Run Agent Evaluations
        run: |
          stn eval batch \
            --agents all \
            --git-commit ${{ github.sha }} \
            --output report.html
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
          STATION_JUDGE_MODEL: gpt-4o-mini
      
      - name: Check Pass/Fail
        run: |
          if ! stn eval check --batch latest; then
            echo "❌ Some evaluations failed"
            exit 1
          fi
      
      - name: Upload Report
        uses: actions/upload-artifact@v3
        with:
          name: eval-report
          path: report.html
      
      - name: Comment on PR
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            const report = await fs.readFile('report.html', 'utf8');
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: `## 🧪 Eval Results\n\nAgent evaluations completed. [View full report](${reportURL})\n\n${summary}`
            })
```

---

## Implementation Plan

### Phase 1: Core Framework (6-8 hours)
1. Database schema and migrations
2. EvalRunner service
3. Judge LLM integration
4. CLI commands (create, run, list)

### Phase 2: Criteria & Templates (3-4 hours)
1. Built-in criteria templates
2. Custom criteria support
3. Template management CLI

### Phase 3: Reporting (4-6 hours)
1. HTML report generation
2. PDF export (optional)
3. CLI report commands

### Phase 4: UI (6-8 hours)
1. Evals page with scenario list
2. Eval run details view
3. Report viewer
4. Scenario editor

### Phase 5: CICD Integration (2-3 hours)
1. GitHub Actions example
2. Batch evaluation support
3. Exit codes for pass/fail

**Total Estimated Time**: 21-29 hours

---

## Success Metrics

### Developer Experience
- **Eval Creation**: 2 minutes (from idea to running eval)
- **Eval Execution**: <60 seconds per eval
- **Report Generation**: <5 seconds
- **Pass/Fail Clarity**: Binary decision with clear reasoning

### Quality Assurance
- **Regression Detection**: Catch breaking changes before deploy
- **Confidence**: 90%+ correlation between judge scores and human evaluation
- **Coverage**: All critical agent workflows have evals

### Cost Efficiency
- **Judge Cost**: <$0.02 per eval (with GPT-4o-mini)
- **Total Cost**: <$1 per full eval suite run
- **ROI**: Saves hours of manual testing

---

## Future Enhancements

1. **Multi-Judge Consensus**: Use multiple LLMs and aggregate scores
2. **Human-in-the-Loop**: Allow manual override of judge decisions
3. **A/B Testing**: Compare two prompt versions automatically
4. **Benchmark Datasets**: Pre-built eval sets for common use cases
5. **Adversarial Testing**: Generate edge cases automatically
6. **Real-time Monitoring**: Run evals on production agent outputs

---

**Status**: Ready for implementation  
**Priority**: High - Critical for agent quality assurance  
**Dependencies**: None - can start immediately

# Report Generation Parallelization Strategy

**Added**: 2025-01-11  
**Goal**: Use goroutines to parallelize LLM evaluation for faster report generation

---

## Performance Optimization with Goroutines

### Problem

Sequential evaluation of agents is slow:
- 14 agents × 2 seconds per LLM call = 28 seconds just for agent evals
- Add team eval (3 seconds) = 31 seconds total
- Users wait 30+ seconds for report

### Solution: Parallel Agent Evaluation

Use goroutines to evaluate multiple agents concurrently, reducing total time to ~5-10 seconds.

---

## Parallelized Report Generation Flow

```go
func (r *ReportGenerator) GenerateReport(ctx context.Context, reportID int64) error {
    report, _ := r.repos.Reports.GetByID(reportID)
    
    // 1. Update status: generating_team
    report.Status = "generating_team"
    r.updateReport(report)
    
    // 2. Fetch all agents and runs
    agents, _ := r.repos.Agents.GetByEnvironment(report.EnvironmentID)
    allRuns := r.fetchAllRuns(agents) // Fetch runs for all agents
    
    // 3. Team evaluation (sequential - depends on all runs)
    teamEval := r.evaluateTeamPerformance(ctx, report.TeamCriteria, allRuns, agents)
    report.TeamScore = teamEval.Score
    report.ExecutiveSummary = teamEval.ExecutiveSummary
    report.Progress = 30
    r.updateReport(report)
    
    // 4. PARALLEL AGENT EVALUATION with goroutines
    report.Status = "generating_agents"
    report.Progress = 30
    r.updateReport(report)
    
    agentResults := r.evaluateAgentsParallel(ctx, report, agents)
    
    // 5. Save all agent reports
    agentReports := make(map[int64]interface{})
    for _, result := range agentResults {
        detail := r.createAgentReportDetail(report.ID, result)
        r.repos.AgentReportDetails.Create(detail)
        agentReports[result.AgentID] = map[string]interface{}{
            "score": result.Score,
            "summary": result.Reasoning[:200],
        }
    }
    
    // 6. Finalize report
    report.Status = "completed"
    report.Progress = 100
    report.AgentReports = agentReports
    r.updateReport(report)
    
    return nil
}
```

---

## Parallel Agent Evaluation Implementation

### Strategy 1: Simple Goroutines with WaitGroup

```go
type AgentEvalResult struct {
    AgentID         int64
    AgentName       string
    Score           float64
    Passed          bool
    Reasoning       string
    CriteriaScores  map[string]CriterionScore
    Strengths       []string
    Weaknesses      []string
    Recommendations []string
    Runs            []AgentRun
    Error           error
}

func (r *ReportGenerator) evaluateAgentsParallel(
    ctx context.Context, 
    report *Report, 
    agents []Agent,
) []AgentEvalResult {
    var wg sync.WaitGroup
    results := make([]AgentEvalResult, len(agents))
    
    // Progress tracking
    progressChan := make(chan int, len(agents))
    
    // Start progress updater goroutine
    go r.updateProgressFromChannel(progressChan, report, len(agents))
    
    // Evaluate agents in parallel
    for i, agent := range agents {
        wg.Add(1)
        
        go func(idx int, ag Agent) {
            defer wg.Done()
            
            // Get agent's runs
            runs, _ := r.repos.AgentRuns.GetByAgent(ag.ID, 20)
            
            // Get agent criteria
            agentCriteria := report.AgentCriteria[fmt.Sprintf("%d", ag.ID)]
            
            // Evaluate agent (LLM call)
            eval, err := r.evaluateAgentPerformance(ctx, ag, runs, agentCriteria)
            
            if err != nil {
                results[idx] = AgentEvalResult{
                    AgentID:   ag.ID,
                    AgentName: ag.Name,
                    Error:     err,
                }
            } else {
                results[idx] = AgentEvalResult{
                    AgentID:         ag.ID,
                    AgentName:       ag.Name,
                    Score:           eval.Score,
                    Passed:          eval.Passed,
                    Reasoning:       eval.Reasoning,
                    CriteriaScores:  eval.CriteriaScores,
                    Strengths:       eval.Strengths,
                    Weaknesses:      eval.Weaknesses,
                    Recommendations: eval.Recommendations,
                    Runs:            runs,
                }
            }
            
            // Signal progress
            progressChan <- 1
            
        }(i, agent)
    }
    
    // Wait for all evaluations
    wg.Wait()
    close(progressChan)
    
    return results
}

func (r *ReportGenerator) updateProgressFromChannel(
    progressChan <-chan int, 
    report *Report, 
    totalAgents int,
) {
    completed := 0
    for range progressChan {
        completed++
        report.Progress = 30 + int((float64(completed)/float64(totalAgents))*60)
        report.CurrentStep = fmt.Sprintf("Evaluated %d/%d agents", completed, totalAgents)
        r.updateReport(report)
    }
}
```

**Benefits**:
- All agent evaluations run concurrently
- No blocking between agents
- Progress updates in real-time

**Time Savings**:
- Sequential: 14 agents × 2s = 28 seconds
- Parallel: max(agent eval times) ≈ 2-3 seconds
- **~10x faster!**

---

### Strategy 2: Worker Pool (Rate-Limited)

For environments with many agents, limit concurrent LLM calls to avoid rate limits:

```go
func (r *ReportGenerator) evaluateAgentsWithPool(
    ctx context.Context,
    report *Report,
    agents []Agent,
    maxWorkers int, // e.g., 5 concurrent LLM calls
) []AgentEvalResult {
    results := make([]AgentEvalResult, len(agents))
    
    // Create job queue
    jobs := make(chan int, len(agents))
    
    // Start worker pool
    var wg sync.WaitGroup
    for w := 0; w < maxWorkers; w++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for idx := range jobs {
                agent := agents[idx]
                
                // Evaluate agent
                runs, _ := r.repos.AgentRuns.GetByAgent(agent.ID, 20)
                agentCriteria := report.AgentCriteria[fmt.Sprintf("%d", agent.ID)]
                eval, err := r.evaluateAgentPerformance(ctx, agent, runs, agentCriteria)
                
                if err != nil {
                    results[idx] = AgentEvalResult{AgentID: agent.ID, Error: err}
                } else {
                    results[idx] = AgentEvalResult{
                        AgentID:   agent.ID,
                        AgentName: agent.Name,
                        Score:     eval.Score,
                        // ... other fields
                    }
                }
                
                // Update progress
                r.updateAgentProgress(report, idx+1, len(agents))
            }
        }(w)
    }
    
    // Queue all jobs
    for i := range agents {
        jobs <- i
    }
    close(jobs)
    
    // Wait for completion
    wg.Wait()
    
    return results
}
```

**Benefits**:
- Controls concurrent LLM API calls
- Respects rate limits (e.g., OpenAI: 10 req/sec)
- Prevents overwhelming the LLM provider

**Configuration**:
```go
// For GPT-4o-mini (cheap, high rate limit)
maxWorkers = 10 // 10 concurrent calls

// For GPT-4o (expensive, lower rate limit)
maxWorkers = 3  // 3 concurrent calls
```

---

### Strategy 3: Parallel Runs Fetching

Fetch agent runs in parallel too:

```go
func (r *ReportGenerator) fetchAllRunsParallel(agents []Agent) []AgentRun {
    var wg sync.WaitGroup
    runsChan := make(chan []AgentRun, len(agents))
    
    for _, agent := range agents {
        wg.Add(1)
        go func(ag Agent) {
            defer wg.Done()
            runs, _ := r.repos.AgentRuns.GetByAgent(ag.ID, 20)
            runsChan <- runs
        }(agent)
    }
    
    // Wait and collect
    go func() {
        wg.Wait()
        close(runsChan)
    }()
    
    allRuns := []AgentRun{}
    for runs := range runsChan {
        allRuns = append(allRuns, runs...)
    }
    
    return allRuns
}
```

**Time Savings**:
- Sequential DB queries: 14 × 10ms = 140ms
- Parallel DB queries: ~10-20ms
- Minor but nice optimization

---

## Telemetry Fetching Parallelization

Fetch Jaeger traces in parallel:

```go
func (r *ReportGenerator) fetchTelemetryParallel(runs []AgentRun) map[int64]TelemetrySummary {
    var wg sync.WaitGroup
    mutex := &sync.Mutex{}
    telemetry := make(map[int64]TelemetrySummary)
    
    for _, run := range runs {
        wg.Add(1)
        go func(r AgentRun) {
            defer wg.Done()
            
            // Fetch trace from Jaeger
            trace, err := r.jaegerClient.GetTrace(r.ID)
            if err != nil {
                return
            }
            
            // Summarize telemetry
            summary := r.summarizeTelemetry(trace)
            
            // Thread-safe write
            mutex.Lock()
            telemetry[r.ID] = summary
            mutex.Unlock()
        }(run)
    }
    
    wg.Wait()
    return telemetry
}
```

---

## Performance Comparison

### Sequential Processing
```
Team Eval:               3s
Agent 1 Eval:            2s
Agent 2 Eval:            2s
...
Agent 14 Eval:           2s
─────────────────────────
Total:                  31s
```

### Parallel Processing (10 workers)
```
Team Eval:               3s
Agents 1-10 (parallel):  2s
Agents 11-14 (parallel): 2s
─────────────────────────
Total:                   7s
```

### Parallel Everything (unlimited)
```
Team Eval:               3s
All 14 Agents (parallel):2s
─────────────────────────
Total:                   5s
```

**Result: 6x faster report generation!**

---

## Error Handling in Parallel Execution

### Strategy: Fail-Fast vs Continue

**Option 1: Fail-Fast** (stop on first error)
```go
func (r *ReportGenerator) evaluateAgentsParallel(...) ([]AgentEvalResult, error) {
    errChan := make(chan error, len(agents))
    
    for _, agent := range agents {
        go func(ag Agent) {
            eval, err := r.evaluateAgentPerformance(...)
            if err != nil {
                errChan <- fmt.Errorf("agent %s failed: %w", ag.Name, err)
                return
            }
            // ... store result
        }(agent)
    }
    
    // Check for errors
    select {
    case err := <-errChan:
        return nil, err // Stop everything
    case <-time.After(5 * time.Minute):
        return nil, errors.New("timeout")
    }
}
```

**Option 2: Continue on Error** (best effort)
```go
// Individual agent errors don't stop report generation
// Failed agents marked with error message
// Report still completes with partial results

results[idx] = AgentEvalResult{
    AgentID: agent.ID,
    Score: 0,
    Error: err,
    Reasoning: fmt.Sprintf("Evaluation failed: %v", err),
}
```

**Recommended**: Option 2 (Continue on Error)
- More resilient
- Partial report better than no report
- Users see which agents failed and why

---

## Configuration

```go
type ReportGeneratorConfig struct {
    MaxConcurrentEvals int     // Default: 10
    MaxConcurrentRuns  int     // Default: 20
    EvalTimeout        time.Duration // Default: 5min
    JudgeModel         string  // Default: "gpt-4o-mini"
    FailFast           bool    // Default: false
}

func NewReportGenerator(cfg *ReportGeneratorConfig) *ReportGenerator {
    if cfg.MaxConcurrentEvals == 0 {
        cfg.MaxConcurrentEvals = 10
    }
    
    return &ReportGenerator{
        config: cfg,
        // ...
    }
}
```

---

## Testing Parallel Code

### Unit Tests

```go
func TestParallelAgentEvaluation(t *testing.T) {
    // Create 14 mock agents
    agents := createMockAgents(14)
    
    // Mock LLM client with 100ms delay per call
    mockLLM := &MockLLMClient{Delay: 100 * time.Millisecond}
    
    generator := &ReportGenerator{llmClient: mockLLM}
    
    start := time.Now()
    results := generator.evaluateAgentsParallel(context.Background(), report, agents)
    elapsed := time.Since(start)
    
    // Parallel execution should be much faster than sequential
    // Sequential: 14 × 100ms = 1400ms
    // Parallel:   ~100-200ms (depending on worker count)
    assert.Less(t, elapsed.Milliseconds(), int64(500))
    assert.Len(t, results, 14)
}
```

### Load Testing

```go
func TestLargeEnvironmentPerformance(t *testing.T) {
    // Test with 100 agents
    agents := createMockAgents(100)
    
    start := time.Now()
    results := generator.evaluateAgentsParallel(context.Background(), report, agents)
    elapsed := time.Since(start)
    
    // Should complete in <30 seconds even with 100 agents
    assert.Less(t, elapsed.Seconds(), float64(30))
    assert.Len(t, results, 100)
}
```

---

## Summary

### Parallelization Points

1. **Agent Evaluation** (primary bottleneck) ✅
   - Use goroutines with worker pool
   - 10x faster for typical environments

2. **Runs Fetching** (minor optimization) ✅
   - Parallel DB queries
   - 5x faster database access

3. **Telemetry Fetching** (nice to have) ✅
   - Parallel Jaeger API calls
   - Significantly faster for large trace datasets

### Expected Performance

**14 agents, 280 runs (20 each)**:
- Sequential: ~31 seconds
- Parallel (10 workers): ~7 seconds
- Parallel (unlimited): ~5 seconds

**100 agents, 2000 runs**:
- Sequential: ~3-4 minutes
- Parallel (10 workers): ~25 seconds
- Parallel (unlimited): ~10 seconds

### Best Practices

1. Use worker pools to respect LLM rate limits
2. Continue on individual agent errors (partial results)
3. Update progress frequently (every completed agent)
4. Set reasonable timeouts (5 minutes)
5. Test with various environment sizes

---

**Status**: Design complete, ready to implement  
**Impact**: 6-10x faster report generation  
**Risk**: Low - goroutines are stable and well-tested in Go

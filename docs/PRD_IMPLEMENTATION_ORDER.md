# Station Zero-to-Hero: Implementation Order & Parallelization Strategy

**Document Purpose**: Defines the optimal order for tackling PRD sections with clear parallelization strategy for multiple agents working simultaneously.

**Last Updated**: 2025-01-06

---

## Critical Path Analysis

```
OTEL (S3) ──┐
            ├──> stn develop (S1) ──┐
            │                        ├──> Documentation (S6)
Faker (S4) ─┴──> Evals (S2) ────────┴──> CICD (S5) ──┘
```

**Critical Path**: Faker → Evals → CICD (longest dependency chain)
**Independent Paths**: OTEL, stn develop

---

## Priority-Ordered Implementation

### 🟢 **Phase 1: Foundation** (Weeks 1-3)

**Goal**: Fix broken systems and enhance existing capabilities

#### **PARALLEL TRACK A: Observability**
**Section 3: OpenTelemetry Tracing Integration**

**Priority**: 🔴 CRITICAL - Currently Broken
**Complexity**: ⭐⭐⭐ Medium
**Time Estimate**: 1-2 weeks
**Dependencies**: None (standalone)

**Why First**:
- GenKit v1.0.1 broke existing OTEL integration
- Blocks proper debugging of all other features
- Required for understanding eval performance
- No dependencies on other sections

**Files to Modify**:
- `internal/telemetry/otel_plugin.go` (fix broken API)
- `internal/telemetry/genkit_telemetry_client.go` (enhance capture)
- `internal/services/agent_execution_engine.go` (add tracing)

**Success Criteria**:
- [ ] Agent executions export to Jaeger
- [ ] MCP tool calls appear as spans
- [ ] `make jaeger` sets up local tracing
- [ ] No GenKit v1.0.1 compatibility errors

---

#### **PARALLEL TRACK B: Mocking Infrastructure**
**Section 4: Faker Proxy Production Readiness**

**Priority**: 🟡 HIGH - Enables Better Evals
**Complexity**: ⭐⭐ Low-Medium
**Time Estimate**: 1 week
**Dependencies**: None (enhancement of existing code)

**Why First**:
- Already working, just needs enhancement
- Critical for eval framework (Section 2)
- No dependencies on other sections
- Low risk, high value improvements

**Files to Modify**:
- `pkg/faker/schema.go` (per-tool tracking)
- `pkg/faker/enricher.go` (better patterns)
- `pkg/faker/proxy.go` (schema seeding)

**Success Criteria**:
- [ ] Per-tool schema storage working
- [ ] Schema seeding from JSON Schema
- [ ] Enhanced enrichment accuracy
- [ ] Schema export/import functionality

---

### 🟡 **Phase 2: Developer Experience** (Weeks 3-6)

**Goal**: Build comprehensive testing and development workflows

#### **PARALLEL TRACK A: Interactive Development**
**Section 1: GenKit Development Environment (`stn develop`)**

**Priority**: 🟡 HIGH - Developer Productivity
**Complexity**: ⭐⭐⭐ Medium
**Time Estimate**: 2 weeks
**Dependencies**: Section 3 (OTEL) should be working for proper debugging

**Why Second**:
- Benefits from working OTEL (can trace during development)
- Independent of eval framework
- High impact on developer experience

**Files to Modify**:
- `cmd/main/develop.go` (auto-launch UI, hot-reload)
- `internal/services/genkit_provider.go` (OpenAI-compatible providers)

**Success Criteria**:
- [ ] `stn develop` auto-launches GenKit UI
- [ ] OpenAI-compatible endpoints (Ollama, Llama) work
- [ ] Hot-reload for .prompt file changes
- [ ] All MCP tools visible in UI

---

#### **PARALLEL TRACK B: Testing Framework**
**Section 2: Agent Evaluation Framework**

**Priority**: 🔴 CRITICAL - Enables All Testing
**Complexity**: ⭐⭐⭐⭐ High (new architecture)
**Time Estimate**: 3 weeks
**Dependencies**: Section 4 (Faker) for mock modes

**Why Second**:
- Needs faker proxy enhancements from Phase 1
- Complex - requires new database schema, services
- Blocks CICD integration (Phase 3)
- Foundation for automated testing

**Files to Create/Modify**:
- `internal/services/eval_runner.go` (NEW)
- `internal/services/eval_scenario.go` (NEW)
- `internal/db/migrations/XXX_add_eval_runs.sql` (NEW)
- `cmd/main/eval.go` (NEW - CLI commands)

**Success Criteria**:
- [ ] `stn eval run` executes agents with scenarios
- [ ] Eval results stored with metrics
- [ ] Real/mock/faker modes all working
- [ ] Eval comparison shows deltas
- [ ] Token usage and performance tracked

---

### 🔵 **Phase 3: Automation** (Weeks 6-9)

**Goal**: Enable seamless CICD and production workflows

#### **SEQUENTIAL: CICD Integration**
**Section 5: CICD Pipeline Strategy**

**Priority**: 🟡 HIGH - Production Readiness
**Complexity**: ⭐⭐ Low-Medium
**Time Estimate**: 2 weeks
**Dependencies**: 🚨 CRITICAL - Section 2 (Evals) must be complete

**Why Third**:
- CANNOT start until eval framework works
- Relatively simple (mostly YAML)
- High value for production deployments

**Files to Create**:
- `.github/workflows/agent-evals.yml` (NEW)
- `.github/workflows/bundle-test.yml` (NEW)
- `.github/workflows/eval-report.yml` (NEW)

**Success Criteria**:
- [ ] Evals run automatically on PR
- [ ] Results posted as PR comments
- [ ] Bundle validation prevents broken bundles
- [ ] Merge blocking on eval failures
- [ ] Performance metrics tracked

---

#### **PARALLEL: User Onboarding**
**Section 6: Zero-to-Hero Developer Journey**

**Priority**: 🟢 MEDIUM - Documentation
**Complexity**: ⭐ Low
**Time Estimate**: 1-2 weeks
**Dependencies**: All previous sections (documents integrated system)

**Why Last**:
- Requires all features to be complete
- Documentation task, not code
- Can start during Phase 2 and update as features land

**Files to Create**:
- `docs/ZERO_TO_HERO_TUTORIAL.md` (NEW)
- `docs/EVAL_QUICKSTART.md` (NEW)
- `examples/first-agent/` (NEW - tutorial code)
- Video walkthroughs (external)

**Success Criteria**:
- [ ] 30-minute onboarding tutorial complete
- [ ] Example agents in repo
- [ ] Video walkthroughs recorded
- [ ] Troubleshooting guides written

---

## 🎯 Recommended Parallel Execution Strategy

### **Optimal 2-Agent Parallel Approach**

#### **Agent A: "Observability & Experience" Track**
```
Week 1-2:  Section 3 (OTEL Tracing)
Week 3-5:  Section 1 (stn develop)
Week 6-9:  Section 6 (Documentation) + Support Agent B with CICD testing
```

**Rationale**:
- Fixes critical broken system (OTEL) first
- Improves developer experience
- Low merge conflict risk
- Enables better debugging for Agent B

#### **Agent B: "Testing & Automation" Track**
```
Week 1:    Section 4 (Faker Proxy)
Week 2-5:  Section 2 (Eval Framework)
Week 6-8:  Section 5 (CICD Pipeline)
Week 9:    Integration testing & polish
```

**Rationale**:
- Owns the critical path (Faker → Evals → CICD)
- Each step enables the next
- High-value automation features
- CICD depends on evals being complete

---

## 🚀 Quick Start: First Two Sections for Parallel Development

### **START HERE: Best First Parallel Pair**

#### **Agent 1: Start Section 3 (OTEL Tracing)**
**Why**:
- Currently broken and blocking debugging
- Zero dependencies
- High impact on all other work
- Different codebase area (no conflicts)

**First Task**: Fix `internal/telemetry/otel_plugin.go` GenKit v1.0.1 compatibility

#### **Agent 2: Start Section 4 (Faker Proxy)**
**Why**:
- Already working, just enhancing
- Enables better evals (needed by Agent 1 later)
- Zero dependencies
- Different codebase area (no conflicts)

**First Task**: Implement per-tool schema storage in `pkg/faker/schema.go`

---

## 📊 Dependency Matrix

| Section | Depends On | Blocks | Can Parallel With |
|---------|-----------|--------|------------------|
| S3: OTEL | None | None | S1, S2, S4, S6 |
| S4: Faker | None | S2 | S1, S3, S6 |
| S1: stn develop | S3 (soft) | None | S2, S4, S6 |
| S2: Evals | S4 | S5 | S1, S3, S6 |
| S5: CICD | S2 | None | S6 |
| S6: Docs | All (soft) | None | S1, S2, S3, S4, S5 |

---

## 🎨 Alternative Strategies

### **Strategy 1: "Critical Path First"**
Focus both agents on critical path sequentially:
- Both on Section 2 (Evals) with pair programming
- Then both on Section 5 (CICD)
- Fastest to production, but wastes parallel capacity

### **Strategy 2: "User Experience First"**
Prioritize user-facing improvements:
- Agent A: Section 1 (stn develop)
- Agent B: Section 6 (Documentation)
- Good for demos, but delays testing infrastructure

### **Strategy 3: "Foundation First"** (RECOMMENDED)
Build solid foundation before automation:
- Agent A: Section 3 (OTEL)
- Agent B: Section 4 (Faker)
- Then proceed to Evals and CICD
- Best balance of risk and parallelization

---

## 🧪 Testing Strategy Per Phase

### **Phase 1 Testing**
```bash
# Test OTEL integration
make jaeger
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
make local-install-ui
stn serve &
stn agent run test-agent "test task"
# Verify trace in Jaeger UI

# Test Faker enhancements
stn faker --command stn --args "mock,aws-guardduty"
# Verify per-tool schema storage
```

### **Phase 2 Testing**
```bash
# Test stn develop
make local-install-ui
stn develop --env default
# Verify GenKit UI launches, test with Ollama

# Test eval framework
go test ./internal/services/eval_*
make local-install-ui
stn serve &
stn eval run test-agent --mode mock
stn eval list
```

### **Phase 3 Testing**
```bash
# Test CICD integration
git checkout -b test-evals
# Modify agent
git commit -m "test: trigger eval workflow"
git push
# Verify GitHub Actions runs and comments on PR

# UAT: Full zero-to-hero walkthrough
# Follow Section 6 tutorial as new user
```

---

## 🎯 Success Metrics

**Phase 1 Complete When**:
- ✅ OTEL traces appear in Jaeger for all agent runs
- ✅ Faker proxy learns per-tool schemas accurately
- ✅ Go tests pass with >80% coverage

**Phase 2 Complete When**:
- ✅ `stn develop` works with OpenAI-compatible endpoints
- ✅ `stn eval run` executes successfully in all modes
- ✅ Eval metrics stored and queryable
- ✅ Go tests pass with >85% coverage

**Phase 3 Complete When**:
- ✅ GitHub Actions runs evals on every PR
- ✅ PR comments show eval results
- ✅ New developers complete tutorial in <30 minutes
- ✅ Production agents deployed via CICD

**Project Complete When**:
- ✅ All 6 sections implemented
- ✅ Zero-to-hero tutorial validates end-to-end workflow
- ✅ Test coverage >85%
- ✅ CICD running in production
- ✅ Documentation complete

---

## 📝 Risk Mitigation

**Risk**: Agent B blocked waiting for Agent A
**Mitigation**: Start with independent sections (S3 + S4)

**Risk**: Merge conflicts between agents
**Mitigation**: Clear file ownership, different service layers

**Risk**: CICD can't start until evals complete
**Mitigation**: Agent A can start documentation during eval development

**Risk**: Scope creep delaying completion
**Mitigation**: Strict acceptance criteria per section

---

## 🚦 Go/No-Go Decision Points

**After Phase 1**:
- GO: OTEL working, faker enhanced → Proceed to Phase 2
- NO-GO: OTEL still broken → Fix before proceeding

**After Phase 2**:
- GO: Evals running in all modes → Proceed to CICD
- NO-GO: Eval framework unstable → Stabilize before CICD

**After Phase 3**:
- GO: CICD working in production → Launch
- NO-GO: Flaky CICD → Fix reliability first

---

**RECOMMENDATION: Start with Agent A on Section 3 (OTEL) and Agent B on Section 4 (Faker) simultaneously for maximum parallelization and fastest path to working eval infrastructure.**

-- +goose Up
-- Add distributed run tracking columns for Station Lattice async work coordination
-- These enable tracing execution across multiple stations with UUID-based lineage

-- orchestrator_run_id: UUID or UUID-N format identifying this execution in the distributed tree
-- Format: "550e8400-e29b-41d4-a716-446655440000" (root) or "550e8400-...-1-2" (nested)
ALTER TABLE agent_runs ADD COLUMN orchestrator_run_id TEXT DEFAULT NULL;

-- parent_orchestrator_run_id: Links to parent execution in the distributed tree
ALTER TABLE agent_runs ADD COLUMN parent_orchestrator_run_id TEXT DEFAULT NULL;

-- originating_station_id: Station that initiated the root execution
ALTER TABLE agent_runs ADD COLUMN originating_station_id TEXT DEFAULT NULL;

-- trace_id: OTEL trace ID for correlation with distributed tracing
ALTER TABLE agent_runs ADD COLUMN trace_id TEXT DEFAULT NULL;

-- work_id: Links to lattice WorkAssignment for async work tracking
ALTER TABLE agent_runs ADD COLUMN work_id TEXT DEFAULT NULL;

-- Indexes for efficient querying of distributed runs
CREATE INDEX IF NOT EXISTS idx_agent_runs_orchestrator_run_id ON agent_runs(orchestrator_run_id);
CREATE INDEX IF NOT EXISTS idx_agent_runs_parent_orchestrator_run_id ON agent_runs(parent_orchestrator_run_id);
CREATE INDEX IF NOT EXISTS idx_agent_runs_work_id ON agent_runs(work_id);
CREATE INDEX IF NOT EXISTS idx_agent_runs_trace_id ON agent_runs(trace_id);

-- +goose Down
DROP INDEX IF EXISTS idx_agent_runs_trace_id;
DROP INDEX IF EXISTS idx_agent_runs_work_id;
DROP INDEX IF EXISTS idx_agent_runs_parent_orchestrator_run_id;
DROP INDEX IF EXISTS idx_agent_runs_orchestrator_run_id;

// Station API Types based on Go models

export interface Environment {
  id: number;
  name: string;
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface Agent {
  id: number;
  name: string;
  description?: string;
  prompt: string;
  max_steps: number;
  environment_id: number;
  created_by: number;
  input_schema?: string;
  is_scheduled: boolean;
  schedule_enabled: boolean;
  cron_schedule?: string;
  memory_topic_key?: string;
  memory_max_tokens?: number;
  model?: string;
  created_at: string;
  updated_at: string;
}

export interface MCPServer {
  id: number;
  name: string;
  description?: string;
  command?: string;
  url?: string;
  args?: string[];
  environment_id: number;
  status: 'active' | 'inactive' | 'error';
  error?: string;
  created_at: string;
  updated_at: string;
}

export interface Tool {
  id: number;
  name: string;
  description?: string;
  mcp_server_id: number;
  schema?: string;
  created_at: string;
  updated_at: string;
}

// Jaeger Trace Types
export interface JaegerTag {
  key: string;
  type: 'string' | 'bool' | 'int64' | 'float64';
  value: string | boolean | number;
}

export interface JaegerReference {
  refType: 'CHILD_OF' | 'FOLLOWS_FROM';
  traceID: string;
  spanID: string;
}

export interface JaegerLog {
  timestamp: number; // microseconds
  fields: JaegerTag[];
}

export interface JaegerSpan {
  traceID: string;
  spanID: string;
  operationName: string;
  references: JaegerReference[];
  startTime: number; // microseconds
  duration: number; // microseconds
  tags: JaegerTag[];
  logs: JaegerLog[];
  processID: string;
  warnings?: string[];
}

export interface JaegerProcess {
  serviceName: string;
  tags: JaegerTag[];
}

export interface JaegerTrace {
  traceID: string;
  spans: JaegerSpan[];
  processes: Record<string, JaegerProcess>;
  warnings?: string[];
}

export interface AgentTool {
  agent_id: number;
  tool_id: number;
  tool_name: string;
  assigned_at: string;
}

export interface User {
  id: number;
  username: string;
  email?: string;
  is_active: boolean;
  is_superuser: boolean;
  created_at: string;
  updated_at: string;
}

export interface AgentRun {
  id: number;
  agent_id: number;
  user_id: number;
  task: string;
  final_response: string;
  execution_steps?: any[];
  status: 'pending' | 'running' | 'completed' | 'failed';
  started_at: string;
  completed_at?: string;
  // Rich metadata from response object
  input_tokens?: number;
  output_tokens?: number;
  total_tokens?: number;
  duration_seconds?: number;
  model_name?: string;
  cost?: number;
  parent_run_id?: number;
  error?: string;
}

export interface AgentRunWithDetails extends AgentRun {
  agent_name: string;
  username: string;
}

// Canvas-specific types for visualization
export interface NodeData {
  id: string;
  type: 'agent' | 'mcp' | 'tool' | 'user';
  label: string;
  data: Agent | MCPServer | Tool | User;
}

export interface EdgeData {
  id: string;
  source: string;
  target: string;
  type?: string;
}

// Reports System Types
export interface EvaluationCriterion {
  weight: number;           // 0.0-1.0 (must sum to 1.0)
  threshold: number;        // 0-10 scale
  description: string;
}

export interface TeamCriteria {
  goal: string;
  criteria: Record<string, EvaluationCriterion>;
}

export interface CriterionScore {
  score: number;            // 0-10
  reasoning: string;
  examples?: string[];
}

export interface Report {
  id: number;
  name: string;
  description?: string;
  environment_id: number;
  
  // Success Criteria (JSON strings in DB)
  team_criteria: string;  // JSON: TeamCriteria
  agent_criteria?: string;  // JSON: Record<string, any>
  
  // Status
  status: 'pending' | 'generating_team' | 'generating_agents' | 'completed' | 'failed';
  progress?: number;        // 0-100
  current_step?: string;
  
  // Team Results (LLM Generated)
  executive_summary?: string;
  team_score?: number;      // 0-10
  team_reasoning?: string;
  team_criteria_scores?: string;  // JSON: Record<string, CriterionScore>
  agent_reports?: string;   // JSON: Array of agent summaries
  
  // Metadata
  total_runs_analyzed?: number;
  total_agents_analyzed?: number;
  generation_duration_seconds?: number;
  generation_started_at?: string;
  generation_completed_at?: string;
  
  // LLM Usage
  total_llm_tokens?: number;
  total_llm_cost?: number;
  judge_model?: string;
  
  // Error
  error_message?: string;
  
  // Timestamps
  created_at: string;
  updated_at: string;
}

export interface QualityMetrics {
  // Average scores (0-10 scale from LLM-as-judge)
  avg_task_completion: number;
  avg_relevancy: number;
  avg_faithfulness: number;
  avg_hallucination: number;
  avg_toxicity: number;
  
  // Pass rates (percentage of runs meeting thresholds)
  task_completion_pass_rate: number;
  relevancy_pass_rate: number;
  faithfulness_pass_rate: number;
  hallucination_pass_rate: number;
  toxicity_pass_rate: number;
  
  // Metadata
  evaluated_runs: number;  // Number of runs with benchmark metrics
  total_runs: number;      // Total runs analyzed
}

export interface AgentReportDetail {
  id: number;
  report_id: number;
  agent_id: number;
  agent_name: string;
  
  // Evaluation Results
  score: number;            // 0-10
  passed: boolean;
  reasoning?: string;
  
  // Criteria Breakdown
  criteria_scores?: string; // JSON: Record<string, CriterionScore>
  
  // Run Analysis
  runs_analyzed?: number;
  run_ids?: string;         // JSON: number[]
  avg_duration_seconds?: number;
  avg_tokens?: number;
  avg_cost?: number;
  success_rate?: number;    // 0.0-1.0
  
  // Quality Metrics (from LLM-as-judge benchmark evaluations)
  quality_metrics?: string; // JSON: QualityMetrics
  
  // LLM Insights
  strengths?: string;       // JSON: string[]
  weaknesses?: string;      // JSON: string[]
  recommendations?: string; // JSON: string[]
  
  // Telemetry
  telemetry_summary?: string;  // JSON: object
  
  created_at: string;
}

export interface ReportWithDetails {
  report: Report;
  agent_details: AgentReportDetail[];
  environment: Environment;
}

export interface CreateReportRequest {
  name: string;
  description?: string;
  environment_id: number;
  team_criteria: TeamCriteria;
  agent_criteria?: Record<string, any>;
  judge_model?: string;
}

// Benchmark Types
export interface BenchmarkMetric {
  id: number;
  run_id: number;
  metric_name: string;
  score: number;
  threshold: number;
  passed: boolean;
  reason: string;
  created_at: string;
}

export interface BenchmarkTask {
  id: number;
  name: string;
  category: string;
  description: string;
  expected_output_example: string;
  evaluation_criteria: string;
  task_completion_weight: number;
  relevancy_weight: number;
  hallucination_weight: number;
  faithfulness_weight: number;
  toxicity_weight: number;
  created_at: string;
  updated_at: string;
}

export interface BenchmarkResult {
  run_id: number;
  agent_id: number;
  task: string;
  quality_score: number;
  production_ready: boolean;
  recommendation: string;
  metrics: Record<string, MetricResult>;
  total_judge_tokens: number;
  total_judge_cost: number;
  evaluation_time_ms: number;
}

export interface MetricResult {
  metric_type: string;
  score: number;
  threshold: number;
  passed: boolean;
  reason?: string;
  judge_tokens: number;
  judge_cost: number;
  evaluation_duration_ms: number;
}
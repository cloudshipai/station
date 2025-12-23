import { apiClient } from './client';
import type { 
  Environment, 
  Agent, 
  MCPServer, 
  Tool, 
  AgentTool, 
  User, 
  AgentRun,
  AgentRunWithDetails,
  JaegerTrace,
  Report,
  ReportWithDetails,
  CreateReportRequest,
  WorkflowDefinition,
  WorkflowRun,
  WorkflowStep,
  WorkflowApproval
} from '../types/station';



// Environment API
export const environmentsApi = {
  getAll: () => apiClient.get<{count: number, environments: Environment[]}>('/environments'),
  getById: (id: number) => apiClient.get<Environment>(`/environments/${id}`),
};

// Agent detail response type
export interface AgentWithTools {
  agent: Agent;
  mcp_servers: {
    id: number;
    name: string;
    tools: {
      id: number;
      name: string;
      description: string;
      input_schema: string;
    }[];
  }[];
}

// Agents API
export const agentsApi = {
  getAll: () => apiClient.get<{agents: Agent[], count: number}>('/agents'),
  getById: (id: number) => apiClient.get<{agent: Agent}>(`/agents/${id}`),
  getWithTools: (id: number) => apiClient.get<AgentWithTools>(`/agents/${id}/details`),
  getByEnvironment: (environmentId: number) => 
    apiClient.get<{agents: Agent[], count: number}>(`/agents?environment_id=${environmentId}`),
  getTools: (agentId: number) => 
    apiClient.get<AgentTool[]>(`/agents/${agentId}/tools`),
  getPrompt: (id: number) => apiClient.get<{content: string, path: string, agent_id: number}>(`/agents/${id}/prompt`),
  updatePrompt: (id: number, content: string) => 
    apiClient.put<{message: string, path: string, agent_id: number, environment: string, sync_command: string}>(`/agents/${id}/prompt`, { content }),
};

// MCP Servers API
export const mcpServersApi = {
  getAll: () => apiClient.get<{servers: MCPServer[]}>('/mcp-servers'),
  getById: (id: number) => apiClient.get<MCPServer>(`/mcp-servers/${id}`),
  getByEnvironment: (environmentId: number) =>
    apiClient.get<{servers: MCPServer[]}>(`/mcp-servers?environment_id=${environmentId}`),
};

// Tools API
export const toolsApi = {
  getAll: () => apiClient.get<Tool[]>('/tools'),
  getById: (id: number) => apiClient.get<Tool>(`/tools/${id}`),
  getByMCPServer: (mcpServerId: number) => 
    apiClient.get<Tool[]>(`/mcp-servers/${mcpServerId}/tools`),
};

// Users API
export const usersApi = {
  getAll: () => apiClient.get<User[]>('/users'),
  getById: (id: number) => apiClient.get<User>(`/users/${id}`),
};

// Agent Runs API  
export const agentRunsApi = {
  getAll: (params?: { environment_id?: number; status?: string; limit?: number }) => {
    const queryParams = new URLSearchParams();
    if (params?.environment_id) {
      queryParams.append('environment_id', params.environment_id.toString());
    }
    if (params?.status) {
      queryParams.append('status', params.status);
    }
    if (params?.limit) {
      queryParams.append('limit', params.limit.toString());
    }
    const queryString = queryParams.toString();
    return apiClient.get<{runs: AgentRunWithDetails[], count: number, total_count: number, limit: number, status?: string}>(
      `/runs${queryString ? `?${queryString}` : ''}`
    );
  },
  getById: (id: number) => apiClient.get<{run: AgentRunWithDetails}>(`/runs/${id}`),
  getByAgent: (agentId: number) => 
    apiClient.get<{runs: AgentRun[], count: number, agent_id: number}>(`/agents/${agentId}/runs`),
  delete: (id: number) => apiClient.delete<{success: boolean, message: string, run_id: number}>(`/runs/${id}`),
  deleteMany: (params: { ids?: number[]; status?: string; all?: boolean }) => 
    apiClient.delete<{success: boolean, message: string, deleted_count: number}>('/runs', { data: params }),
};

// Benchmark API
export interface BulkEvaluationRequest {
  run_ids: number[];
  concurrency?: number;
}

export interface BulkEvaluationProgress {
  run_id: number;
  status: 'pending' | 'evaluating' | 'completed' | 'failed';
  quality_score?: number;
  production_ready?: boolean;
  error?: string;
  evaluated_at?: string;
}

export interface BulkEvaluationResult {
  total_runs: number;
  completed: number;
  failed: number;
  duration_seconds: number;
  results: BulkEvaluationProgress[];
  summary: {
    avg_quality_score: number;
    production_ready_pct: number;
    total_judge_tokens?: number;
    total_judge_cost?: number;
  };
}

export const benchmarksApi = {
  evaluate: (runId: number) => apiClient.post(`/benchmarks/${runId}/evaluate`),
  getMetrics: (runId: number) => apiClient.get(`/benchmarks/${runId}/metrics`),
  listTasks: () => apiClient.get('/benchmarks/tasks'),
  listRecent: (limit?: number) => apiClient.get(`/benchmarks/metrics${limit ? `?limit=${limit}` : ''}`),
  evaluateBulk: (request: BulkEvaluationRequest) => 
    apiClient.post<BulkEvaluationResult>('/benchmarks/evaluate-bulk', request),
};

// Sync API
export const syncApi = {
  trigger: () => apiClient.post('/sync'),
  startInteractive: (environment: string, options?: { dry_run?: boolean; force?: boolean }) => 
    apiClient.post('/sync/interactive', { environment, ...options }),
  getStatus: (syncId: string) => apiClient.get(`/sync/status/${syncId}`),
  submitVariables: (syncId: string, variables: Record<string, string>) => 
    apiClient.post('/sync/variables', { sync_id: syncId, variables }),
};

// Bundle info type
export interface BundleInfo {
  name: string;
  file_name: string;
  file_path: string;
  size: number;
  modified_time: string;
}

// Bundles API
export const bundlesApi = {
  getAll: () => apiClient.get<{success: boolean, bundles: BundleInfo[], count: number, error?: string}>('/bundles'),
  create: (environment: string, local: boolean, endpoint?: string) => 
    apiClient.post<{success: boolean, message: string, local_path?: string, share_url?: string}>('/bundles', { 
      environment, 
      local, 
      endpoint 
    }),
  install: (bundleLocation: string, environmentName: string, source: 'url' | 'file') =>
    apiClient.post<{
      success: boolean;
      message: string;
      environment_name?: string;
      environment_id?: number;
      bundle_path?: string;
      installed_agents?: number;
      installed_mcps?: number;
      sync_command?: string;
      error?: string;
    }>('/bundles/install', {
      bundle_location: bundleLocation,
      environment_name: environmentName,
      source
    }),
};

// Traces API
export const tracesApi = {
  getByRunId: (runId: number) => 
    apiClient.get<{run_id: number, trace: JaegerTrace, error?: string, suggestion?: string}>(`/traces/run/${runId}`),
  getByTraceId: (traceId: string) =>
    apiClient.get<{trace_id: string, trace: JaegerTrace, error?: string}>(`/traces/trace/${traceId}`),
};

// Version API types
export interface VersionInfo {
  current_version: string;
  latest_version: string;
  update_available: boolean;
  release_url?: string;
  release_notes?: string;
  published_at?: string;
  checked_at: string;
}

export interface CurrentVersionInfo {
  version: string;
  build_time: string;
  go_version: string;
  go_arch: string;
  go_os: string;
  compiler: string;
  is_dev: boolean;
}

export interface UpdateResult {
  success: boolean;
  message: string;
  previous_version?: string;
  new_version?: string;
  error?: string;
}

// Version API
export const versionApi = {
  getCurrent: () => apiClient.get<CurrentVersionInfo>('/version'),
  checkForUpdates: () => apiClient.get<VersionInfo>('/version/check'),
  performUpdate: () => apiClient.post<UpdateResult>('/version/update'),
};

// Workflows API
export const workflowsApi = {
  // Workflow Definitions
  getAll: () => 
    apiClient.get<{ workflows: WorkflowDefinition[], count: number }>('/workflows'),
  
  getById: (id: string, version?: number) => {
    const params = version ? `?version=${version}` : '';
    return apiClient.get<{ workflow: WorkflowDefinition }>(`/workflows/${id}${params}`);
  },
  
  create: (data: { workflowId: string; name: string; description?: string; definition: any }) =>
    apiClient.post<{ workflow: WorkflowDefinition; message: string }>('/workflows', data),
  
  update: (id: string, data: { name?: string; description?: string; definition: any }) =>
    apiClient.put<{ workflow: WorkflowDefinition; message: string }>(`/workflows/${id}`, data),
  
  delete: (id: string) =>
    apiClient.delete<{ message: string }>(`/workflows/${id}`),
  
  validate: (definition: any) =>
    apiClient.post<{ valid: boolean; errors?: string[] }>('/workflows/validate', { definition }),

  // Workflow Runs
  startRun: (workflowId: string, input?: any, version?: number) =>
    apiClient.post<{ run: WorkflowRun; message: string }>(`/workflows/${workflowId}/runs`, { input, version }),
  
  listRuns: (params?: { workflow_id?: string; status?: string; limit?: number }) => {
    const queryParams = new URLSearchParams();
    if (params?.workflow_id) queryParams.append('workflow_id', params.workflow_id);
    if (params?.status) queryParams.append('status', params.status);
    if (params?.limit) queryParams.append('limit', params.limit.toString());
    const queryString = queryParams.toString();
    return apiClient.get<{ runs: WorkflowRun[], count: number }>(
      `/workflow-runs${queryString ? `?${queryString}` : ''}`
    );
  },
  
  getRun: (runId: string) =>
    apiClient.get<{ run: WorkflowRun }>(`/workflow-runs/${runId}`),
  
  cancelRun: (runId: string, reason?: string) =>
    apiClient.post<{ run: WorkflowRun; message: string }>(`/workflow-runs/${runId}/cancel`, { reason }),
  
  pauseRun: (runId: string, reason?: string) =>
    apiClient.post<{ run: WorkflowRun; message: string }>(`/workflow-runs/${runId}/pause`, { reason }),
  
  resumeRun: (runId: string) =>
    apiClient.post<{ run: WorkflowRun; message: string }>(`/workflow-runs/${runId}/resume`),
  
  getSteps: (runId: string) =>
    apiClient.get<{ steps: WorkflowStep[], count: number }>(`/workflow-runs/${runId}/steps`),

  // Workflow Approvals
  listApprovals: (params?: { run_id?: string; limit?: number }) => {
    const queryParams = new URLSearchParams();
    if (params?.run_id) queryParams.append('run_id', params.run_id);
    if (params?.limit) queryParams.append('limit', params.limit.toString());
    const queryString = queryParams.toString();
    return apiClient.get<{ approvals: WorkflowApproval[], count: number }>(
      `/workflow-approvals${queryString ? `?${queryString}` : ''}`
    );
  },
  
  approve: (approvalId: string, comment?: string) =>
    apiClient.post<{ approval: WorkflowApproval; message: string }>(`/workflow-approvals/${approvalId}/approve`, { comment }),
  
  reject: (approvalId: string, reason?: string) =>
    apiClient.post<{ approval: WorkflowApproval; message: string }>(`/workflow-approvals/${approvalId}/reject`, { reason }),
};

// Reports API
export const reportsApi = {
  getAll: (params?: { environment_id?: number; limit?: number; offset?: number }) => {
    const queryParams = new URLSearchParams();
    if (params?.environment_id) queryParams.append('environment_id', params.environment_id.toString());
    if (params?.limit) queryParams.append('limit', params.limit.toString());
    if (params?.offset) queryParams.append('offset', params.offset.toString());
    
    const queryString = queryParams.toString();
    return apiClient.get<{ reports: Report[], count: number }>(
      `/reports${queryString ? `?${queryString}` : ''}`
    );
  },
  
  getById: (id: number) => 
    apiClient.get<ReportWithDetails>(`/reports/${id}`),
  
  create: (request: CreateReportRequest) => 
    apiClient.post<{ report: Report; message: string }>('/reports', request),
  
  generate: (id: number) => 
    apiClient.post<{ message: string; report_id: number; status: string }>(
      `/reports/${id}/generate`
    ),
  
  delete: (id: number) => 
    apiClient.delete<{ message: string }>(`/reports/${id}`),
};
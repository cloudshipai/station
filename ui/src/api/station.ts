import { apiClient } from './client';
import type { 
  Environment, 
  Agent, 
  MCPServer, 
  Tool, 
  AgentTool, 
  User, 
  AgentRun,
  AgentRunWithDetails 
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
  getAll: () => apiClient.get<{runs: AgentRunWithDetails[], count: number, limit: number}>('/runs'),
  getById: (id: number) => apiClient.get<{run: AgentRunWithDetails}>(`/runs/${id}`),
  getByAgent: (agentId: number) => 
    apiClient.get<{runs: AgentRun[], count: number, agent_id: number}>(`/agents/${agentId}/runs`),
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
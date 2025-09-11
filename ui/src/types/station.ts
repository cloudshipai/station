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
  created_at: string;
  updated_at: string;
}

export interface MCPServer {
  id: number;
  name: string;
  description?: string;
  command: string;
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
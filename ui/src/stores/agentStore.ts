import { create } from 'zustand';
import type { Agent, AgentTool } from '../types/station';

interface AgentStore {
  agents: Agent[];
  selectedAgent: Agent | null;
  agentTools: Record<number, AgentTool[]>; // agentId -> tools
  isLoading: boolean;
  error: string | null;
  
  setAgents: (agents: Agent[]) => void;
  setSelectedAgent: (agent: Agent | null) => void;
  setAgentTools: (agentId: number, tools: AgentTool[]) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
}

export const useAgentStore = create<AgentStore>((set) => ({
  agents: [],
  selectedAgent: null,
  agentTools: {},
  isLoading: false,
  error: null,
  
  setAgents: (agents) => set({ agents }),
  setSelectedAgent: (agent) => set({ selectedAgent: agent }),
  setAgentTools: (agentId, tools) => 
    set((state) => ({
      agentTools: { ...state.agentTools, [agentId]: tools }
    })),
  setLoading: (isLoading) => set({ isLoading }),
  setError: (error) => set({ error }),
}));
import type { Agent, Tool, AgentTool } from '../types/station';

/**
 * Check if a tool name represents an agent tool (can call another agent)
 */
export function isAgentTool(toolName: string): boolean {
  return toolName.startsWith('__agent_');
}

/**
 * Extract the normalized agent name from an agent tool name
 * Example: __agent_security_scanner -> security-scanner
 */
export function extractAgentNameFromTool(toolName: string): string | null {
  if (!isAgentTool(toolName)) return null;
  
  // Remove __agent_ prefix and convert underscores to hyphens
  return toolName.replace('__agent_', '').replace(/_/g, '-');
}

/**
 * Normalize agent name to match tool naming convention
 * Example: "Security Scanner" -> "security-scanner"
 */
export function normalizeAgentName(agentName: string): string {
  return agentName
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

/**
 * Find which agents this agent can call based on its tools
 */
export function getChildAgentNames(agentTools: Tool[]): string[] {
  return agentTools
    .filter(tool => isAgentTool(tool.name))
    .map(tool => extractAgentNameFromTool(tool.name))
    .filter((name): name is string => name !== null);
}

/**
 * Check if an agent is callable by other agents (is an agent tool)
 */
export function isAgentCallable(agent: Agent, allTools: Tool[]): boolean {
  const normalizedName = normalizeAgentName(agent.name);
  const agentToolName = `__agent_${normalizedName.replace(/-/g, '_')}`;
  
  return allTools.some(tool => tool.name === agentToolName);
}

/**
 * Build a map of agent hierarchies showing parent-child relationships
 */
export interface AgentHierarchyInfo {
  agent: Agent;
  childAgents: string[];  // Normalized names of agents this agent can call
  isCallable: boolean;    // Can this agent be called by others?
  parentAgents: string[]; // Normalized names of agents that can call this agent
}

export function buildAgentHierarchyMap(
  agents: Agent[],
  agentToolsMap: Map<number, Tool[]>, // Map of agent_id -> tools
  allTools: Tool[]
): Map<number, AgentHierarchyInfo> {
  const hierarchyMap = new Map<number, AgentHierarchyInfo>();
  
  // First pass: identify child agents for each agent
  agents.forEach(agent => {
    const tools = agentToolsMap.get(agent.id) || [];
    const childAgents = getChildAgentNames(tools);
    const isCallable = isAgentCallable(agent, allTools);
    
    hierarchyMap.set(agent.id, {
      agent,
      childAgents,
      isCallable,
      parentAgents: []
    });
  });
  
  // Second pass: build parent relationships
  hierarchyMap.forEach((info, agentId) => {
    const normalizedName = normalizeAgentName(info.agent.name);
    
    // Find all agents that can call this agent
    hierarchyMap.forEach((otherInfo, otherAgentId) => {
      if (agentId !== otherAgentId && otherInfo.childAgents.includes(normalizedName)) {
        info.parentAgents.push(normalizeAgentName(otherInfo.agent.name));
      }
    });
  });
  
  return hierarchyMap;
}

/**
 * Check if an agent is part of any hierarchy (either calls agents or is callable)
 */
export function isPartOfHierarchy(hierarchyInfo: AgentHierarchyInfo): boolean {
  return hierarchyInfo.childAgents.length > 0 || 
         hierarchyInfo.isCallable || 
         hierarchyInfo.parentAgents.length > 0;
}

/**
 * Get hierarchy depth (how many levels deep the agent tree goes)
 */
export function getHierarchyDepth(
  agent: Agent,
  hierarchyMap: Map<number, AgentHierarchyInfo>,
  visited = new Set<number>()
): number {
  if (visited.has(agent.id)) return 0; // Prevent cycles
  visited.add(agent.id);
  
  const info = hierarchyMap.get(agent.id);
  if (!info || info.childAgents.length === 0) return 1;
  
  // Find max depth of children
  let maxChildDepth = 0;
  info.childAgents.forEach(childName => {
    const childAgent = Array.from(hierarchyMap.values())
      .find(h => normalizeAgentName(h.agent.name) === childName);
    
    if (childAgent) {
      const childDepth = getHierarchyDepth(childAgent.agent, hierarchyMap, new Set(visited));
      maxChildDepth = Math.max(maxChildDepth, childDepth);
    }
  });
  
  return 1 + maxChildDepth;
}

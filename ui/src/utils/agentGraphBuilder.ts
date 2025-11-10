import type { Node, Edge } from '@xyflow/react';
import { agentsApi } from '../api/station';
import { getLayoutedNodes } from './layoutUtils';
import { buildAgentHierarchyMap, normalizeAgentName } from './agentHierarchy';

interface BuildAgentGraphParams {
  agentId: number;
  expandedServers: Set<number>;
  agents: any[];
  openAgentModal: (id: number) => void;
  editAgent: (id: number) => void;
  openMCPServerModal: (id: number) => void;
  toggleServerExpansion: (id: number) => void;
  openScheduleModal: (id: number) => void;
  openRunModal: (id: number) => void;
}

interface AgentGraphData {
  nodes: Node[];
  edges: Edge[];
}

/**
 * Parse YAML frontmatter from agent prompt to extract tool names
 */
function parseAgentToolsFromPrompt(promptContent: string): string[] {
  try {
    // Extract YAML frontmatter
    const yamlMatch = promptContent.match(/^---\n([\s\S]*?)\n---/);
    if (!yamlMatch) return [];

    const yamlContent = yamlMatch[1];
    
    // Extract tools list from YAML
    const toolsMatch = yamlContent.match(/tools:\s*\n((?:\s*-\s*"[^"]+"\s*\n)+)/);
    if (!toolsMatch) return [];

    // Parse individual tool names
    return toolsMatch[1]
      .split('\n')
      .filter(line => line.trim().startsWith('-'))
      .map(line => line.trim().replace(/^-\s*"/, '').replace(/"$/, ''))
      .filter(name => name.length > 0);
  } catch (error) {
    console.warn('Failed to parse agent tools from prompt:', error);
    return [];
  }
}

/**
 * Build the complete agent graph including MCP servers, tools, and child agents
 */
export async function buildAgentGraph(params: BuildAgentGraphParams): Promise<AgentGraphData> {
  const {
    agentId,
    expandedServers,
    agents,
    openAgentModal,
    editAgent,
    openMCPServerModal,
    toggleServerExpansion,
    openScheduleModal,
    openRunModal,
  } = params;

  // Fetch agent details with MCP servers
  const response = await agentsApi.getWithTools(agentId);
  const { agent, mcp_servers } = response.data;

  // Fetch agent prompt to extract agent tools from YAML frontmatter
  let agentToolsFromPrompt: string[] = [];
  try {
    const promptResponse = await agentsApi.getPrompt(agentId);
    agentToolsFromPrompt = parseAgentToolsFromPrompt(promptResponse.data.content);
  } catch (error) {
    console.warn('Could not fetch agent prompt:', error);
  }

  // Build agent tools map and collect all tools for hierarchy detection
  const agentToolsMap = new Map();
  const allTools: any[] = [];

  // Add tools from MCP servers
  mcp_servers.forEach((server: any) => {
    if (server.tools) {
      const existingTools = agentToolsMap.get(agent.id) || [];
      agentToolsMap.set(agent.id, [...existingTools, ...server.tools]);
      allTools.push(...server.tools);
    }
  });

  // Add agent tools from prompt (for agent-to-agent calling)
  if (agentToolsFromPrompt.length > 0) {
    const existingTools = agentToolsMap.get(agent.id) || [];
    const agentToolObjects = agentToolsFromPrompt.map(name => ({
      id: 0, // Virtual tool, no real ID
      name: name,
      description: `Agent tool: ${name}`,
      mcp_server_id: 0
    }));
    agentToolsMap.set(agent.id, [...existingTools, ...agentToolObjects]);
    allTools.push(...agentToolObjects);
  }

  // Build hierarchy map with all agents
  const hierarchyMap = buildAgentHierarchyMap(agents, agentToolsMap, allTools);
  const hierarchyInfo = hierarchyMap.get(agent.id);

  // Generate nodes and edges from the data
  const newNodes: Node[] = [];
  const newEdges: Edge[] = [];

  // Main agent node
  newNodes.push({
    id: `agent-${agent.id}`,
    type: 'agent',
    position: { x: 0, y: 0 }, // ELK will position this
    data: {
      agent: agent,
      hierarchyInfo: hierarchyInfo,
      onOpenModal: openAgentModal,
      onEditAgent: editAgent,
      onScheduleAgent: openScheduleModal,
      onRunAgent: openRunModal,
    },
  });

  // MCP servers and their tools
  mcp_servers.forEach((server: any) => {
    const isExpanded = expandedServers.has(server.id);

    // MCP server node
    newNodes.push({
      id: `mcp-${server.id}`,
      type: 'mcp',
      position: { x: 0, y: 0 },
      data: {
        label: server.name,
        description: 'MCP Server',
        tools: server.tools,
        serverId: server.id,
        expanded: isExpanded,
        onToggleExpand: toggleServerExpansion,
        onOpenMCPModal: openMCPServerModal,
      },
    });

    // Edge from agent to MCP server
    newEdges.push({
      id: `edge-agent-${agent.id}-to-mcp-${server.id}`,
      source: `agent-${agent.id}`,
      target: `mcp-${server.id}`,
      animated: true,
      style: {
        stroke: '#ff00ff',
        strokeWidth: 3,
        filter: 'drop-shadow(0 0 8px #ff00ff80)'
      },
      type: 'default',
      className: 'neon-edge',
    });

    // Tool nodes (only if server is expanded)
    if (isExpanded) {
      server.tools.forEach((tool: any) => {
        newNodes.push({
          id: `tool-${tool.id}`,
          type: 'tool',
          position: { x: 0, y: 0 },
          data: {
            label: tool.name.replace('__', ''),
            description: tool.description || 'Tool function',
            category: server.name,
            toolId: tool.id,
          },
        });

        // Edge from MCP server to tool
        newEdges.push({
          id: `edge-mcp-${server.id}-to-tool-${tool.id}`,
          source: `mcp-${server.id}`,
          target: `tool-${tool.id}`,
          animated: true,
          style: {
            stroke: '#a855f7',
            strokeWidth: 2,
            filter: 'drop-shadow(0 0 6px rgba(168, 85, 247, 0.6))'
          },
          type: 'default',
          className: 'neon-edge-mcp',
        });
      });
    }
  });

  // Add child agent nodes if this is an orchestrator
  if (hierarchyInfo && hierarchyInfo.childAgents.length > 0) {
    hierarchyInfo.childAgents.forEach((childAgentName) => {
      // Find the child agent in the agents list
      const childAgent = agents.find(a => 
        normalizeAgentName(a.name) === childAgentName
      );
      
      if (childAgent) {
        // Create a node for the child agent
        newNodes.push({
          id: `child-agent-${childAgent.id}`,
          type: 'agent',
          position: { x: 0, y: 0 },
          data: {
            agent: childAgent,
            hierarchyInfo: hierarchyMap.get(childAgent.id),
            onOpenModal: openAgentModal,
            onEditAgent: editAgent,
            onScheduleAgent: openScheduleModal,
            onRunAgent: openRunModal,
          },
        });

        // Edge from orchestrator to child agent
        newEdges.push({
          id: `edge-agent-${agent.id}-to-child-${childAgent.id}`,
          source: `agent-${agent.id}`,
          target: `child-agent-${childAgent.id}`,
          animated: true,
          style: {
            stroke: '#a855f7',
            strokeWidth: 3,
            filter: 'drop-shadow(0 0 8px rgba(168, 85, 247, 0.8))'
          },
          type: 'default',
          className: 'neon-edge-agent',
        });
      }
    });
  }

  // Apply automatic layout using ELK.js
  const layoutedNodes = await getLayoutedNodes(newNodes, newEdges);

  return {
    nodes: layoutedNodes,
    edges: newEdges,
  };
}

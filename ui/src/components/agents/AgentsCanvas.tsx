import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ReactFlow,
  ReactFlowProvider,
  useNodesState,
  useEdgesState,
  addEdge,
  type Node,
  type Edge,
  type OnConnect,
  type NodeTypes,
} from '@xyflow/react';
import { Bot } from 'lucide-react';
import { agentsApi } from '../../api/station';
import { HierarchicalAgentNode } from '../nodes/HierarchicalAgentNode';
import { buildAgentGraph } from '../../utils/agentGraphBuilder';
import { buildAgentHierarchyMap } from '../../utils/agentHierarchy';
import { MCPNode, ToolNode } from '../nodes/MCPNodes';
import { AgentListSidebar } from './AgentListSidebar';

interface EnvironmentContextType {
  selectedEnvironment: number | null;
  refreshTrigger: number;
}

interface AgentsCanvasProps {
  environmentContext: EnvironmentContextType;
}

const agentPageNodeTypes: NodeTypes = {
  agent: HierarchicalAgentNode,
  mcp: MCPNode,
  tool: ToolNode,
};

export const AgentsCanvas: React.FC<AgentsCanvasProps> = ({ environmentContext }) => {
  const navigate = useNavigate();
  const [nodes, setNodes, onNodesChange] = useNodesState<any>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<any>([]);
  const [selectedAgent, setSelectedAgent] = useState<number | null>(null);
  const [agents, setAgents] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [hierarchyMap, setHierarchyMap] = useState<Map<number, any>>(new Map());
  const [modalAgentId, setModalAgentId] = useState<number | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [modalMCPServerId, setModalMCPServerId] = useState<number | null>(null);
  const [isMCPModalOpen, setIsMCPModalOpen] = useState(false);
  const [expandedServers, setExpandedServers] = useState<Set<number>>(new Set());

  const toggleServerExpansionRef = useRef<(serverId: number) => void>(() => {});

  // Function to open agent details modal
  const openAgentModal = (agentId: number) => {
    setModalAgentId(agentId);
    setIsModalOpen(true);
  };

  const closeAgentModal = () => {
    setIsModalOpen(false);
    setModalAgentId(null);
  };

  // Function to edit agent configuration
  const editAgent = (agentId: number) => {
    navigate(`/agent-editor/${agentId}`);
  };

  // Function to open MCP server details modal
  const openMCPServerModal = (serverId: number) => {
    setModalMCPServerId(serverId);
    setIsMCPModalOpen(true);
  };

  const closeMCPServerModal = () => {
    setIsMCPModalOpen(false);
    setModalMCPServerId(null);
  };

  // Function to toggle MCP server expansion
  const toggleServerExpansion = useCallback((serverId: number) => {
    setExpandedServers(prevExpanded => {
      const newExpandedServers = new Set(prevExpanded);
      if (newExpandedServers.has(serverId)) {
        newExpandedServers.delete(serverId);
      } else {
        newExpandedServers.add(serverId);
      }
      return newExpandedServers;
    });
  }, []);

  toggleServerExpansionRef.current = toggleServerExpansion;

  // Fetch agents list (filtered by environment if selected)
  useEffect(() => {
    const fetchAgents = async () => {
      try {
        const response = await agentsApi.getAll();
        let agentsList = response.data.agents || [];

        // Filter by environment if selected
        if (environmentContext?.selectedEnvironment) {
          agentsList = agentsList.filter((agent: any) =>
            agent.environment_id === environmentContext.selectedEnvironment
          );
        }

        setAgents(agentsList);

        // Build hierarchy map for all agents
        if (agentsList.length > 0) {
          // We'll update hierarchy map when we have full tool info
          // For now, just select first agent
          setSelectedAgent(agentsList[0].id);
        } else {
          setSelectedAgent(null);
        }
      } catch (error) {
        console.error('Failed to fetch agents:', error);
        setAgents([]);
      } finally {
        setLoading(false);
      }
    };
    fetchAgents();
  }, [environmentContext?.selectedEnvironment, environmentContext?.refreshTrigger]);

  // Build hierarchy map when agents change
  useEffect(() => {
    const buildHierarchy = async () => {
      if (agents.length === 0) return;

      try {
        // For now, build a simple hierarchy map from agent names
        // In a full implementation, we'd need to fetch all agents' tools
        const agentToolsMap = new Map();
        const allTools: any[] = [];

        // Build hierarchy map
        const map = buildAgentHierarchyMap(agents, agentToolsMap, allTools);
        setHierarchyMap(map);
      } catch (error) {
        console.error('Failed to build hierarchy map:', error);
      }
    };

    buildHierarchy();
  }, [agents]);

  // Function to regenerate graph with current expansion state
  const regenerateGraph = useCallback(async (agentId: number, expandedSet: Set<number>) => {
    try {
      const graphData = await buildAgentGraph({
        agentId,
        expandedServers: expandedSet,
        agents,
        openAgentModal,
        editAgent,
        openMCPServerModal,
        toggleServerExpansion: toggleServerExpansionRef.current,
      });

      setNodes(graphData.nodes);
      setEdges(graphData.edges);
    } catch (error) {
      console.error('Failed to regenerate graph:', error);
    }
  }, [agents, setNodes, setEdges]);

  // Reset expansion state when switching agents
  useEffect(() => {
    if (selectedAgent) {
      setExpandedServers(new Set());
    }
  }, [selectedAgent]);

  // Regenerate graph when agent or expansion state changes
  useEffect(() => {
    if (!selectedAgent) {
      setNodes([]);
      setEdges([]);
      return;
    }

    regenerateGraph(selectedAgent, expandedServers);
  }, [selectedAgent, expandedServers, regenerateGraph]);

  const onConnect: OnConnect = useCallback(
    (params) => setEdges((eds) => addEdge({
      ...params,
      animated: true,
    }, eds)),
    [setEdges]
  );

  const onNodeClick = useCallback((_event: React.MouseEvent, node: Node) => {
    if (node.type === 'agent') {
      // Could open agent details modal
    }
  }, []);

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center bg-tokyo-bg">
        <div className="text-tokyo-fg font-mono">Loading agents...</div>
      </div>
    );
  }

  return (
    <div className="h-full flex bg-tokyo-bg">
      {/* Main Graph Area */}
      <div className="flex-1 flex flex-col">
        <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
          <h1 className="text-xl font-mono font-semibold text-tokyo-blue">
            {selectedAgent && agents.find(a => a.id === selectedAgent)?.name || 'Station Agents'}
          </h1>
        </div>
        <div className="flex-1 relative">
        {agents.length === 0 ? (
          <div className="h-full flex items-center justify-center bg-tokyo-bg">
            <div className="text-center">
              <Bot className="h-16 w-16 text-tokyo-comment mx-auto mb-4" />
              <div className="text-tokyo-fg font-mono text-lg mb-2">No agents found</div>
              <div className="text-tokyo-comment font-mono text-sm">
                {environmentContext?.selectedEnvironment
                  ? `No agents in the selected environment`
                  : 'Create your first agent to get started'
                }
              </div>
            </div>
          </div>
        ) : (
          <div className="flex-1 h-full">
            <ReactFlowProvider>
              <ReactFlow
                nodes={nodes}
                edges={edges}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                onConnect={onConnect}
                onNodeClick={onNodeClick}
                nodeTypes={agentPageNodeTypes}
                fitView
                className="bg-tokyo-bg w-full h-full"
                defaultEdgeOptions={{
                  animated: true,
                  style: {
                    stroke: '#ff00ff',
                    strokeWidth: 2,
                    filter: 'drop-shadow(0 0 6px rgba(255, 0, 255, 0.4))'
                  }
                }}
              />
            </ReactFlowProvider>
          </div>
        )}
      </div>
    </div>

    {/* Right Sidebar with Agent List */}
    {!loading && agents.length > 0 && (
      <AgentListSidebar
        agents={agents}
        selectedAgentId={selectedAgent}
        onSelectAgent={setSelectedAgent}
        hierarchyMap={hierarchyMap}
      />
    )}
  </div>
  );
};

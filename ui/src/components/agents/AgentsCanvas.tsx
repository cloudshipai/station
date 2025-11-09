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
import { MCPNode, ToolNode } from '../nodes/MCPNodes';

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

        if (agentsList.length > 0) {
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
    <div className="h-full flex flex-col bg-tokyo-bg">
      <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
        <div className="flex items-center gap-4">
          <h1 className="text-xl font-mono font-semibold text-tokyo-blue">Station Agents</h1>
          {agents.length > 0 && (
            <div className="flex items-center gap-2 ml-4">
              <label className="text-sm text-tokyo-comment font-mono">Agent:</label>
              <select
                value={selectedAgent || ''}
                onChange={(e) => setSelectedAgent(Number(e.target.value))}
                className="px-3 py-1 bg-tokyo-bg-dark border border-tokyo-blue7 text-tokyo-fg font-mono rounded hover:border-tokyo-blue5 transition-colors"
              >
                {agents.map((agent) => (
                  <option key={agent.id} value={agent.id}>
                    {agent.name}
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>
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
  );
};

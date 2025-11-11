import React, { useState, useEffect, useCallback, useRef } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
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
import { Bot, Eye, EyeOff, Zap } from 'lucide-react';
import { agentsApi } from '../../api/station';
import { HierarchicalAgentNode } from '../nodes/HierarchicalAgentNode';
import { ExecutionOverlayNode } from '../nodes/ExecutionOverlayNode';
import { ExecutionFlowNode } from '../nodes/ExecutionFlowNode';
import { buildAgentGraph } from '../../utils/agentGraphBuilder';
import { buildAgentHierarchyMap } from '../../utils/agentHierarchy';
import { buildExecutionFlowGraph } from '../../utils/executionFlowBuilder';
import { MCPNode, ToolNode } from '../nodes/MCPNodes';
import { AgentListSidebar } from './AgentListSidebar';
import { AgentRunsPanel } from './AgentRunsPanel';
import { MCPServerDetailsModal } from '../modals/MCPServerDetailsModal';
import { AgentScheduleModal } from '../modals/AgentScheduleModal';
import { RunAgentModal } from '../modals/RunAgentModal';
import { RunDetailsModal } from '../modals/RunDetailsModal';
import { ExecutionViewToggle } from './ExecutionViewToggle';
import { ExecutionFlowPanel } from '../execution/ExecutionFlowPanel';
import { ExecutionStatsHUD } from '../execution/ExecutionStatsHUD';
import { useExecutionTrace } from '../../hooks/useExecutionTrace';
import { usePlayback } from '../../hooks/usePlayback';
import { TimelineScrubber } from '../execution/TimelineScrubber';

interface EnvironmentContextType {
  selectedEnvironment: number | null;
  refreshTrigger: number;
}

interface AgentsCanvasProps {
  environmentContext: EnvironmentContextType;
}

const agentPageNodeTypes: NodeTypes = {
  agent: ExecutionOverlayNode, // Use execution-aware node
  mcp: MCPNode,
  tool: ToolNode,
  executionFlow: ExecutionFlowNode, // Execution flow nodes
};

export const AgentsCanvas: React.FC<AgentsCanvasProps> = ({ environmentContext }) => {
  const navigate = useNavigate();
  const { agentId } = useParams<{ agentId?: string }>();
  const [nodes, setNodes, onNodesChange] = useNodesState<any>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<any>([]);
  const [selectedAgent, setSelectedAgent] = useState<number | null>(
    agentId ? parseInt(agentId, 10) : null
  );
  const [agents, setAgents] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [hierarchyMap, setHierarchyMap] = useState<Map<number, any>>(new Map());
  const [modalAgentId, setModalAgentId] = useState<number | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [modalMCPServerId, setModalMCPServerId] = useState<number | null>(null);
  const [isMCPModalOpen, setIsMCPModalOpen] = useState(false);
  const [scheduleAgentId, setScheduleAgentId] = useState<number | null>(null);
  const [isScheduleModalOpen, setIsScheduleModalOpen] = useState(false);
  const [runAgentId, setRunAgentId] = useState<number | null>(null);
  const [isRunModalOpen, setIsRunModalOpen] = useState(false);
  const [expandedServers, setExpandedServers] = useState<Set<number>>(new Set());
  const [selectedRunId, setSelectedRunId] = useState<number | null>(null);
  const [isRunDetailsOpen, setIsRunDetailsOpen] = useState(false);
  const [activeSpanIds, setActiveSpanIds] = useState<string[]>([]);
  
  // Use execution trace hook
  const { 
    isExecutionView, 
    fetchTrace,
    clearTrace,
    toggleExecutionView, 
    processTraceForNode,
    hasTraceData,
    getTotalDuration,
    getActiveSpansAt,
    getTraceData
  } = useExecutionTrace();

  // Use playback hook for scrubber
  const totalDuration = getTotalDuration();
  const {
    currentTime,
    isPlaying,
    playbackSpeed,
    handleTimeChange,
    handlePlayPause,
    handleSpeedChange,
  } = usePlayback({
    totalDuration,
    onTimeUpdate: (time) => {
      // Highlight active spans at current time
      const activeSpans = getActiveSpansAt(time);
      setActiveSpanIds(activeSpans);
    },
  });

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

  // Function to open schedule modal
  const openScheduleModal = (agentId: number) => {
    setScheduleAgentId(agentId);
    setIsScheduleModalOpen(true);
  };

  const closeScheduleModal = () => {
    setIsScheduleModalOpen(false);
    setScheduleAgentId(null);
  };

  const handleScheduleSuccess = async () => {
    // Refresh agents list to get updated schedule
    try {
      const response = await agentsApi.getAll();
      let agentsList = response.data.agents || [];

      if (environmentContext?.selectedEnvironment) {
        agentsList = agentsList.filter((agent: any) =>
          agent.environment_id === environmentContext.selectedEnvironment
        );
      }

      setAgents(agentsList);
    } catch (error) {
      console.error('Failed to refresh agents:', error);
    }
  };

  // Function to open run agent modal
  const openRunModal = (agentId: number) => {
    setRunAgentId(agentId);
    setIsRunModalOpen(true);
  };

  const closeRunModal = () => {
    setIsRunModalOpen(false);
    setRunAgentId(null);
  };

  const handleRunSuccess = (runId: number) => {
    console.log('Agent execution started with run ID:', runId);
    // Open the run details modal
    setSelectedRunId(runId);
    setIsRunDetailsOpen(true);
  };

  const handleRunClick = async (runId: number, agentId: number) => {
    setSelectedRunId(runId);
    setIsRunDetailsOpen(true);
  };

  const handleExecutionViewClick = async (runId: number, agentId: number) => {
    // Close modal if open
    setIsRunDetailsOpen(false);
    
    // Set selected run for highlighting
    setSelectedRunId(runId);
    
    // Fetch trace data and enable execution view - only if it belongs to currently selected agent
    if (selectedAgent === agentId) {
      await fetchTrace(runId, agentId);
      // Execution view will be auto-enabled by fetchTrace
    }
  };

  const closeRunDetails = () => {
    setIsRunDetailsOpen(false);
    setSelectedRunId(null);
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

  // Sync URL agentId param to selectedAgent state
  useEffect(() => {
    if (agentId) {
      const parsedId = parseInt(agentId, 10);
      if (parsedId !== selectedAgent) {
        setSelectedAgent(parsedId);
      }
    }
  }, [agentId]);

  // Update URL when selectedAgent changes
  useEffect(() => {
    if (selectedAgent) {
      // Update URL without full page reload
      navigate(`/agent/${selectedAgent}`, { replace: true });
    }
  }, [selectedAgent, navigate]);

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
          // Only auto-select first agent if no agent is selected from URL
          if (!agentId && !selectedAgent) {
            setSelectedAgent(agentsList[0].id);
          }
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
        openScheduleModal,
        openRunModal,
      });

      // Inject execution data if in execution view mode (micro-timelines only)
      if (isExecutionView && hasTraceData) {
        const traceData = getTraceData();
        
        // Extract all agent IDs that executed in this trace
        const executedAgentIds = new Set<number>();
        if (traceData && traceData.spans) {
          traceData.spans.forEach((span: any) => {
            const agentIdTag = span.tags?.find((t: any) => t.key === 'agent.id');
            if (agentIdTag && typeof agentIdTag.value === 'number') {
              executedAgentIds.add(agentIdTag.value);
            }
          });
        }
        
        // Find existing agent node IDs in the graph
        const existingAgentIds = new Set(
          graphData.nodes
            .filter(n => n.type === 'agent')
            .map(n => n.data.agent.id)
        );
        
        // Add missing child agent nodes that executed but aren't in the static hierarchy
        const additionalNodes: any[] = [];
        const additionalEdges: any[] = [];
        
        executedAgentIds.forEach(executedAgentId => {
          if (!existingAgentIds.has(executedAgentId) && executedAgentId !== agentId) {
            // Find the agent in the agents list
            const childAgent = agents.find(a => a.id === executedAgentId);
            if (childAgent) {
              // Get or create hierarchy info for dynamically added agent
              const childHierarchyInfo = hierarchyMap.get(childAgent.id) || {
                agent: childAgent,
                childAgents: [],
                isCallable: true, // Mark as callable since it was called during execution
                parentAgents: [],
              };
              
              additionalNodes.push({
                id: `child-agent-${childAgent.id}`,
                type: 'agent',
                position: { x: 400, y: additionalNodes.length * 200 + 200 },
                data: {
                  agent: childAgent,
                  hierarchyInfo: childHierarchyInfo,
                  onOpenModal: openAgentModal,
                  onEditAgent: editAgent,
                  onScheduleAgent: openScheduleModal,
                  onRunAgent: openRunModal,
                },
              });
              
              // Add edge from main agent to child
              additionalEdges.push({
                id: `edge-exec-agent-${agentId}-to-child-${childAgent.id}`,
                source: `agent-${agentId}`,
                target: `child-agent-${childAgent.id}`,
                animated: true,
                style: {
                  stroke: '#10b981',
                  strokeWidth: 3,
                  strokeDasharray: '5,5',
                  filter: 'drop-shadow(0 0 8px rgba(16, 185, 129, 0.8))'
                },
                type: 'default',
                label: 'executed',
                labelStyle: { fill: '#10b981', fontSize: 10 },
              });
            }
          }
        });
        
        // Enhance agent nodes with execution data (micro-timelines)
        const allNodes = [...graphData.nodes, ...additionalNodes];
        const enhancedNodes = allNodes.map((node: any) => {
          if (node.type === 'agent') {
            const processedTrace = processTraceForNode(node.data.agent.id);
            
            if (processedTrace) {
              return {
                ...node,
                data: {
                  ...node.data,
                  executionSpans: processedTrace.spans,
                  runStartTime: processedTrace.runStartTime,
                  runDuration: processedTrace.runDuration,
                  isExecutionView: true,
                  currentPlaybackTime: currentTime,
                  activeSpanIds,
                },
              };
            }
            
            // No trace data for this node - ghost mode
            return {
              ...node,
              data: {
                ...node.data,
                isExecutionView: true,
                executionSpans: [],
                currentPlaybackTime: currentTime,
                activeSpanIds: [],
              },
            };
          }
          return node;
        });
        
        setNodes(enhancedNodes);
        setEdges([...graphData.edges, ...additionalEdges]);
      } else {
        setNodes(graphData.nodes);
        setEdges(graphData.edges);
      }
    } catch (error) {
      console.error('Failed to regenerate graph:', error);
    }
  }, [agents, setNodes, setEdges, isExecutionView, hasTraceData, processTraceForNode, currentTime, activeSpanIds]);

  // Reset expansion state and clear trace when switching agents
  useEffect(() => {
    if (selectedAgent) {
      setExpandedServers(new Set());
      clearTrace(); // Clear trace data when switching to a different agent
      setSelectedRunId(null); // Clear selected run when switching agents
    }
  }, [selectedAgent, clearTrace]);

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
          
          {/* Execution View Toggle */}
          <ExecutionViewToggle
            isExecutionView={isExecutionView}
            hasTraceData={hasTraceData}
            onToggle={toggleExecutionView}
          />
        </div>
        {/* Main Canvas Area */}
        <div className="flex-1 relative overflow-hidden">
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
          <div className="h-full w-full relative">
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
              
              {/* Stats HUD - Shows when execution view is active */}
              {isExecutionView && hasTraceData && (
                <ExecutionStatsHUD traceData={getTraceData()} />
              )}
            </ReactFlowProvider>
          </div>
        )}
        </div>
        
        {/* Timeline Scrubber - TEMPORARILY HIDDEN - needs redesign */}
        {false && isExecutionView && (
          <TimelineScrubber
            totalDuration={totalDuration || 1000000}
            currentTime={currentTime}
            onTimeChange={handleTimeChange}
            isPlaying={isPlaying}
            onPlayPause={handlePlayPause}
            playbackSpeed={playbackSpeed}
            onSpeedChange={handleSpeedChange}
          />
        )}

        {/* Execution Flow Panel - Bottom horizontal scrolling panel */}
        <ExecutionFlowPanel
          traceData={getTraceData()}
          activeSpanIds={activeSpanIds}
          isVisible={isExecutionView && hasTraceData}
        />
      </div>

      {/* Right Sidebar with Agent List */}
    {!loading && agents.length > 0 && (
      <>
        <AgentListSidebar
          agents={agents}
          selectedAgentId={selectedAgent}
          onSelectAgent={setSelectedAgent}
          hierarchyMap={hierarchyMap}
        />
        
        {/* Runs Panel */}
        <AgentRunsPanel
          agentId={selectedAgent}
          agentName={agents.find(a => a.id === selectedAgent)?.name || ''}
          onRunClick={handleRunClick}
          onExecutionViewClick={handleExecutionViewClick}
          selectedRunId={selectedRunId}
        />
      </>
    )}

    {/* MCP Server Details Modal */}
    <MCPServerDetailsModal
      serverId={modalMCPServerId}
      isOpen={isMCPModalOpen}
      onClose={closeMCPServerModal}
    />

    {/* Agent Schedule Modal */}
    {scheduleAgentId && (
      <AgentScheduleModal
        isOpen={isScheduleModalOpen}
        onClose={closeScheduleModal}
        agentId={scheduleAgentId}
        agentName={agents.find(a => a.id === scheduleAgentId)?.name || 'Agent'}
        currentSchedule={agents.find(a => a.id === scheduleAgentId)?.cron_schedule}
        currentEnabled={agents.find(a => a.id === scheduleAgentId)?.schedule_enabled || false}
        currentScheduleVariables={agents.find(a => a.id === scheduleAgentId)?.schedule_variables}
        onSuccess={handleScheduleSuccess}
      />
    )}

    {/* Run Agent Modal */}
    {runAgentId && (
      <RunAgentModal
        isOpen={isRunModalOpen}
        onClose={closeRunModal}
        agent={agents.find(a => a.id === runAgentId)!}
        onSuccess={handleRunSuccess}
      />
    )}

    {/* Run Details Modal */}
    <RunDetailsModal
      runId={selectedRunId}
      isOpen={isRunDetailsOpen}
      onClose={closeRunDetails}
    />
  </div>
  );
};

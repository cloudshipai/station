import React, { useState, useEffect, useContext } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useNodesState, useEdgesState, type NodeTypes } from '@xyflow/react';
import { AgentsSidebar } from './AgentsSidebar';
import { SimplifiedGraphCanvas } from './SimplifiedGraphCanvas';
import { AgentDetailsPanel } from './AgentDetailsPanel';
import { Settings, Play, BarChart2 } from 'lucide-react';
import { agentsApi } from '../../../api/station';
import { RunAgentModal } from '../../modals/RunAgentModal';
import { RunDetailsModal } from '../../modals/RunDetailsModal';
import { MCPServerDetailsModal } from '../../modals/MCPServerDetailsModal';
import { AgentScheduleModal } from '../../modals/AgentScheduleModal';
import { buildAgentGraph } from '../../../utils/agentGraphBuilder';
import { ExecutionOverlayNode } from '../../nodes/ExecutionOverlayNode';
import { MCPNode, ToolNode } from '../../nodes/MCPNodes';
import { ExecutionFlowNode } from '../../nodes/ExecutionFlowNode';
import { EnvironmentContext } from '../../../contexts/EnvironmentContext';

const nodeTypes: NodeTypes = {
  agent: ExecutionOverlayNode,
  mcp: MCPNode,
  tool: ToolNode,
  executionFlow: ExecutionFlowNode,
};

export const AgentsLayout: React.FC = () => {
  const context = useContext(EnvironmentContext);
  const { environments } = context;
  const { env, agentId } = useParams<{ env?: string; agentId?: string }>();
  const navigate = useNavigate();
  const [agents, setAgents] = useState<any[]>([]);
  const [selectedAgent, setSelectedAgent] = useState<any | null>(null);
  const [currentEnvironment, setCurrentEnvironment] = useState<any | null>(null);
  const [loading, setLoading] = useState(true);

  // Graph State
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [expandedServers, setExpandedServers] = useState<Set<number>>(new Set());

  // Modal states
  const [isRunModalOpen, setIsRunModalOpen] = useState(false);
  const [runAgent, setRunAgent] = useState<any>(null);
  const [isRunDetailsOpen, setIsRunDetailsOpen] = useState(false);
  const [selectedRunId, setSelectedRunId] = useState<number | null>(null);
  const [mcpModalId, setMcpModalId] = useState<number | null>(null);
  const [isScheduleModalOpen, setIsScheduleModalOpen] = useState(false);
  const [scheduleAgent, setScheduleAgent] = useState<any>(null);

  // Determine current environment from URL param
  useEffect(() => {
    if (env && environments.length > 0) {
      const environment = environments.find((e: any) => e.name.toLowerCase() === env.toLowerCase());
      setCurrentEnvironment(environment || null);
      
      // If env in URL doesn't exist, redirect to first environment
      if (!environment && environments.length > 0) {
        navigate(`/agents/${environments[0].name.toLowerCase()}`);
      }
    } else if (!env && environments.length > 0) {
      // No env in URL, redirect to first environment
      navigate(`/agents/${environments[0].name.toLowerCase()}`);
    }
  }, [env, environments, navigate]);

  // Fetch agents for current environment
  useEffect(() => {
    const fetchAgents = async () => {
      if (!currentEnvironment) return;
      try {
        const response = await agentsApi.getByEnvironment(currentEnvironment.id);
        setAgents(response.data.agents || []);
      } catch (error) {
        console.error('Failed to fetch agents:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchAgents();
  }, [currentEnvironment]);

  // Select agent based on URL param
  useEffect(() => {
    if (agentId && agents.length > 0) {
      const agent = agents.find(a => a.id === parseInt(agentId));
      setSelectedAgent(agent || null);
      // Reset expansion when switching agents
      setExpandedServers(new Set());
    } else {
      setSelectedAgent(null);
      setNodes([]);
      setEdges([]);
    }
  }, [agentId, agents]);

  // Build Graph Effect
  useEffect(() => {
    const buildGraph = async () => {
      if (!selectedAgent || !env) return;
      try {
        const graphData = await buildAgentGraph({
          agentId: selectedAgent.id,
          expandedServers,
          agents,
          openAgentModal: (id) => navigate(`/agents/${env}/${id}`), // Navigate to child agent with env
          editAgent: () => {}, 
          openMCPServerModal: (id) => setMcpModalId(id),
          toggleServerExpansion: (id) => {
            setExpandedServers(prev => {
              const next = new Set(prev);
              if (next.has(id)) next.delete(id);
              else next.add(id);
              return next;
            });
          },
          openScheduleModal: () => {},
          openRunModal: (id) => handleRunAgent(agents.find(a => a.id === id)),
        });
        setNodes(graphData.nodes);
        setEdges(graphData.edges);
      } catch (error) {
        console.error('Failed to build graph:', error);
      }
    };

    buildGraph();
  }, [selectedAgent, expandedServers, agents, navigate, env]);

  const handleAgentSelect = (agent: any) => {
    if (!env) return;
    navigate(`/agents/${env}/${agent.id}`);
  };
  
  const handleEnvironmentChange = (newEnv: string) => {
    navigate(`/agents/${newEnv}`);
  };

  const handleRunAgent = (agent: any) => {
    if (!agent) return;
    setRunAgent(agent);
    setIsRunModalOpen(true);
  };

  const handleRunSuccess = (runId: number) => {
    setIsRunModalOpen(false);
    setSelectedRunId(runId);
    setIsRunDetailsOpen(true);
  };

  const handleViewRun = (runId: number) => {
    setSelectedRunId(runId);
    setIsRunDetailsOpen(true);
  };

  const handleScheduleAgent = (agent: any) => {
    if (!agent) return;
    setScheduleAgent(agent);
    setIsScheduleModalOpen(true);
  };

  return (
    <div className="flex flex-col h-full bg-[#fafaf8]">
      {/* Top Navigation Bar - Minimal design */}
      <div className="h-14 border-b border-gray-200/60 flex items-center justify-between px-6 bg-white/80 backdrop-blur-sm flex-shrink-0">
        <div className="flex items-center gap-3">
          <h1 className="text-lg font-medium text-gray-900">Agents</h1>
          {selectedAgent && (
            <>
              <div className="h-4 w-px bg-gray-300"></div>
              <span className="text-sm text-gray-600">
                {selectedAgent.name}
              </span>
            </>
          )}
          
          {/* Environment Selector */}
          <div className="h-4 w-px bg-gray-300"></div>
          <select
            value={env || ''}
            onChange={(e) => handleEnvironmentChange(e.target.value)}
            className="px-3 py-1.5 bg-white border border-gray-300 text-gray-900 text-sm rounded-lg hover:border-gray-400 focus:ring-2 focus:ring-gray-900/10 focus:border-gray-400 transition-colors"
          >
            {environments.map((environment: any) => (
              <option key={environment.id} value={environment.name.toLowerCase()}>
                {environment.name}
              </option>
            ))}
          </select>
        </div>
        
        <div className="flex items-center gap-2">
          {selectedAgent && (
            <>
              <button 
                onClick={() => navigate(`/runs/${env}`)}
                className="flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 hover:scale-105 rounded-lg transition-all duration-200 active:scale-95"
              >
                <BarChart2 className="h-4 w-4" />
                <span className="hidden sm:inline">Runs</span>
              </button>
              <button 
                onClick={() => navigate(`/agent-editor/${selectedAgent.id}`)}
                className="flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100 hover:scale-105 rounded-lg transition-all duration-200 active:scale-95"
              >
                <Settings className="h-4 w-4" />
                <span className="hidden sm:inline">Configure</span>
              </button>
              <button 
                onClick={() => handleRunAgent(selectedAgent)}
                className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-gray-900 hover:bg-gray-800 hover:shadow-lg hover:-translate-y-0.5 rounded-lg transition-all duration-200 active:scale-95 shadow-sm"
              >
                <Play className="h-4 w-4" />
                Run Agent
              </button>
            </>
          )}
        </div>
      </div>

      {/* Main Content Area (3 Columns) - Paper aesthetic */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left Sidebar (Agent List) */}
        <div className="w-72 flex-shrink-0 z-20 bg-white border-r border-gray-200/60">
          <AgentsSidebar 
            agents={agents} 
            selectedAgentId={selectedAgent?.id} 
            onSelectAgent={handleAgentSelect} 
          />
        </div>

        {/* Center Canvas (Graph) */}
        <div className="flex-1 relative z-0 bg-[#fafaf8]">
          <SimplifiedGraphCanvas 
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            nodeTypes={nodeTypes}
            selectedAgent={selectedAgent}
          />
        </div>

        {/* Right Panel (Details) */}
        {selectedAgent && (
          <div className="w-[400px] flex-shrink-0 z-10 relative border-l border-gray-200/60">
            <AgentDetailsPanel 
              agent={selectedAgent} 
              onRunAgent={handleRunAgent}
              onViewRun={handleViewRun}
              onScheduleAgent={handleScheduleAgent}
            />
          </div>
        )}
      </div>

      {/* Modals */}
      {runAgent && (
        <RunAgentModal
          isOpen={isRunModalOpen}
          onClose={() => setIsRunModalOpen(false)}
          agent={runAgent}
          onSuccess={handleRunSuccess}
        />
      )}

      <RunDetailsModal
        runId={selectedRunId}
        isOpen={isRunDetailsOpen}
        onClose={() => setIsRunDetailsOpen(false)}
      />
      
      <MCPServerDetailsModal
        serverId={mcpModalId}
        isOpen={!!mcpModalId}
        onClose={() => setMcpModalId(null)}
      />

      {scheduleAgent && (
        <AgentScheduleModal
          isOpen={isScheduleModalOpen}
          onClose={() => {
            setIsScheduleModalOpen(false);
            setScheduleAgent(null);
          }}
          agent={scheduleAgent}
        />
      )}
    </div>
  );
};

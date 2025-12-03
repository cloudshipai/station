import React, { useState, useEffect, useContext } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useNodesState, useEdgesState, type NodeTypes } from '@xyflow/react';
import { AgentsSidebar } from './AgentsSidebar';
import { SimplifiedGraphCanvas } from './SimplifiedGraphCanvas';
import { AgentDetailsPanel } from './AgentDetailsPanel';
import { Settings, Play, BarChart2, HelpCircle, Bot, Server, Wrench, Layers } from 'lucide-react';
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
import { HelpModal } from '../../ui/HelpModal';

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
  const [isHelpModalOpen, setIsHelpModalOpen] = useState(false);

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
      <div className="h-14 border-b border-gray-200 flex items-center justify-between px-6 bg-white flex-shrink-0">
        <div className="flex items-center gap-3">
          <h1 className="text-lg font-semibold text-gray-900">Agents</h1>
          {selectedAgent && (
            <>
              <div className="h-4 w-px bg-gray-300"></div>
              <span className="text-sm font-medium text-gray-700">
                {selectedAgent.name}
              </span>
            </>
          )}
          
          {/* Environment Selector */}
          <div className="h-4 w-px bg-gray-300"></div>
          <select
            value={env || ''}
            onChange={(e) => handleEnvironmentChange(e.target.value)}
            className="px-3 py-1.5 bg-white border border-gray-300 text-gray-900 text-sm font-medium rounded-md hover:border-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 transition-colors shadow-sm"
          >
            {environments.map((environment: any) => (
              <option key={environment.id} value={environment.name.toLowerCase()}>
                {environment.name}
              </option>
            ))}
          </select>
        </div>
        
        <div className="flex items-center gap-2">
          <button
            onClick={() => setIsHelpModalOpen(true)}
            className="flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-600 bg-white hover:bg-gray-50 border border-gray-300 rounded-md transition-all shadow-sm"
            title="Learn about agents"
          >
            <HelpCircle className="h-4 w-4" />
            <span className="hidden sm:inline">Help</span>
          </button>
          {selectedAgent && (
            <>
              <button
                onClick={() => navigate(`/runs/${env}`)}
                className="flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 border border-gray-300 rounded-md transition-all shadow-sm"
              >
                <BarChart2 className="h-4 w-4" />
                <span className="hidden sm:inline">Runs</span>
              </button>
              <button
                onClick={() => navigate(`/agent-editor/${selectedAgent.id}`)}
                className="flex items-center gap-2 px-3 py-2 text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 border border-gray-300 rounded-md transition-all shadow-sm"
              >
                <Settings className="h-4 w-4" />
                <span className="hidden sm:inline">Configure</span>
              </button>
              <button
                onClick={() => handleRunAgent(selectedAgent)}
                className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-all shadow-sm hover:shadow"
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

      {/* Help Modal */}
      <HelpModal
        isOpen={isHelpModalOpen}
        onClose={() => setIsHelpModalOpen(false)}
        title="Understanding Agents"
        pageDescription={`This page displays all agents in the ${currentEnvironment?.name || 'selected'} environment. Agents are AI assistants that can perform tasks using LLMs and MCP tools. You can select an agent to view its configuration, tools, and execution history. Use the environment selector to switch between dev, staging, and production workspaces.`}
      >
        <div className="space-y-6">
          {/* Architecture Diagram */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3">Agent Architecture</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-6">
              <div className="flex items-center justify-between gap-8">
                {/* Agent */}
                <div className="flex-1">
                  <div className="bg-blue-600 rounded-lg p-4 text-center">
                    <Bot className="h-8 w-8 text-white mx-auto mb-2" />
                    <div className="font-mono text-sm text-white">Agent</div>
                    <div className="text-xs text-blue-200 mt-1">LLM + Instructions</div>
                  </div>
                </div>

                {/* Arrow */}
                <div className="flex flex-col items-center">
                  <div className="text-gray-500 text-xs mb-1">calls</div>
                  <div className="w-16 h-px bg-gray-300"></div>
                  <div className="text-gray-500 text-xs mt-1">tools</div>
                </div>

                {/* MCP Server */}
                <div className="flex-1">
                  <div className="bg-purple-600 rounded-lg p-4 text-center">
                    <Server className="h-8 w-8 text-white mx-auto mb-2" />
                    <div className="font-mono text-sm text-white">MCP Server</div>
                    <div className="text-xs text-purple-200 mt-1">filesystem</div>
                  </div>
                </div>

                {/* Arrow */}
                <div className="flex flex-col items-center">
                  <div className="text-gray-500 text-xs mb-1">provides</div>
                  <div className="w-16 h-px bg-gray-300"></div>
                </div>

                {/* Tools */}
                <div className="flex-1">
                  <div className="bg-white border border-gray-300 rounded-lg p-4">
                    <Wrench className="h-8 w-8 text-[#0084FF] mx-auto mb-2" />
                    <div className="font-mono text-xs text-gray-700 space-y-1">
                      <div>→ read_file</div>
                      <div>→ write_file</div>
                      <div>→ list_directory</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Execution Flow */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3">Execution Flow</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-6 space-y-4">
              <div className="flex items-start gap-4">
                <div className="flex-shrink-0 w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center text-white font-mono text-sm">1</div>
                <div className="flex-1">
                  <div className="font-mono text-sm text-gray-900">Agent receives task</div>
                  <div className="text-xs text-gray-700 mt-1 font-mono bg-white border border-gray-300 p-2 rounded">"Analyze files in /src directory"</div>
                </div>
              </div>

              <div className="flex items-start gap-4">
                <div className="flex-shrink-0 w-8 h-8 rounded-full bg-purple-600 flex items-center justify-center text-white font-mono text-sm">2</div>
                <div className="flex-1">
                  <div className="font-mono text-sm text-gray-900">Agent decides which tools to call</div>
                  <div className="text-xs text-gray-700 mt-1 font-mono bg-white border border-gray-300 p-2 rounded">filesystem.list_directory("/src")</div>
                </div>
              </div>

              <div className="flex items-start gap-4">
                <div className="flex-shrink-0 w-8 h-8 rounded-full bg-green-600 flex items-center justify-center text-white font-mono text-sm">3</div>
                <div className="flex-1">
                  <div className="font-mono text-sm text-gray-900">MCP server executes securely</div>
                  <div className="text-xs text-gray-700 mt-1 font-mono bg-white border border-gray-300 p-2 rounded">Returns: ["main.ts", "utils.ts", "types.ts"]</div>
                </div>
              </div>

              <div className="flex items-start gap-4">
                <div className="flex-shrink-0 w-8 h-8 rounded-full bg-blue-600 flex items-center justify-center text-white font-mono text-sm">4</div>
                <div className="flex-1">
                  <div className="font-mono text-sm text-gray-900">Agent continues until complete</div>
                  <div className="text-xs text-gray-600 mt-1">Reads each file, analyzes code, returns summary</div>
                </div>
              </div>
            </div>
          </div>

          {/* Key Concepts */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3">Key Concepts</h3>
            <div className="grid grid-cols-2 gap-4">
              <div className="bg-white rounded-lg border border-gray-200 p-4">
                <div className="flex items-center gap-2 mb-2">
                  <Bot className="h-5 w-5 text-[#0084FF]" />
                  <div className="font-mono text-sm text-gray-900">Agents</div>
                </div>
                <div className="text-xs text-gray-600 leading-relaxed">
                  AI assistants with specific instructions, model configuration, and tool access. Isolated per environment.
                </div>
              </div>

              <div className="bg-white rounded-lg border border-gray-200 p-4">
                <div className="flex items-center gap-2 mb-2">
                  <Server className="h-5 w-5 text-[#0084FF]" />
                  <div className="font-mono text-sm text-gray-900">MCP Servers</div>
                </div>
                <div className="text-xs text-gray-600 leading-relaxed">
                  Protocol-compliant servers that expose tools. Handle auth, execution, and security.
                </div>
              </div>

              <div className="bg-white rounded-lg border border-gray-200 p-4">
                <div className="flex items-center gap-2 mb-2">
                  <Wrench className="h-5 w-5 text-[#0084FF]" />
                  <div className="font-mono text-sm text-gray-900">Tools</div>
                </div>
                <div className="text-xs text-gray-600 leading-relaxed">
                  Individual capabilities like read_file, sql_query, http_request. Called by agents during execution.
                </div>
              </div>

              <div className="bg-white rounded-lg border border-gray-200 p-4">
                <div className="flex items-center gap-2 mb-2">
                  <Layers className="h-5 w-5 text-[#0084FF]" />
                  <div className="font-mono text-sm text-gray-900">Environments</div>
                </div>
                <div className="text-xs text-gray-600 leading-relaxed">
                  Isolated workspaces (dev/staging/prod) with their own agents, servers, and configurations.
                </div>
              </div>
            </div>
          </div>
        </div>
      </HelpModal>
    </div>
  );
};

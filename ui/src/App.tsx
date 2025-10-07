import React, { useState, useEffect, useCallback, useRef } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Routes, Route, useNavigate, useLocation, useParams } from 'react-router-dom';
import {
  ReactFlow,
  ReactFlowProvider,
  useNodesState,
  useEdgesState,
  addEdge,
  Handle,
  Position,
  type Node,
  type Edge,
  type OnConnect,
  type NodeProps,
  type NodeTypes,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import { Bot, Server, Layers, MessageSquare, Users, Package, Ship, CircleCheck, Globe, Database, Edit, Eye, ArrowLeft, Save, X, Play, Plus, Archive, Trash2, Settings, Link, Download, FileText, AlertTriangle, ChevronDown, ChevronRight, Rocket } from 'lucide-react';
import yaml from 'js-yaml';
import { MCPDirectoryPage } from './components/pages/MCPDirectoryPage';
import { CloudShipPage } from './components/pages/CloudShipPage';
import { LiveDemoPage } from './components/pages/LiveDemoPage';
import Editor from '@monaco-editor/react';

import { agentsApi, mcpServersApi, environmentsApi, agentRunsApi, bundlesApi, syncApi } from './api/station';
import { apiClient } from './api/client';
import { getLayoutedNodes, layoutElements } from './utils/layoutUtils';
import CloudShipStatus from './components/CloudShipStatus';
import { RunsPage as RunsPageComponent } from './components/runs/RunsPage';
import { SyncModal } from './components/sync/SyncModal';
import { AddServerModal } from './components/modals/AddServerModal';
import { BundleEnvironmentModal } from './components/modals/BundleEnvironmentModal';
import BuildImageModal from './components/modals/BuildImageModal';
import { InstallBundleModal } from './components/modals/InstallBundleModal';
import DeployModal from './components/modals/DeployModal';
import { JsonSchemaEditor } from './components/schema/JsonSchemaEditor';
import type { AgentRunWithDetails } from './types/station';

const queryClient = new QueryClient();

// Custom Node Types
const AgentNode = ({ data }: any) => {
  const handleInfoClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onOpenModal && data.agentId) {
      data.onOpenModal(data.agentId);
    }
  };

  const handleEditClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onEditAgent && data.agentId) {
      data.onEditAgent(data.agentId);
    }
  };

  return (
    <div className="w-[280px] h-[130px] px-4 py-3 shadow-tokyo-blue border border-tokyo-blue7 bg-tokyo-bg-dark rounded-lg relative group">
      {/* Output handle on the right side */}
      <Handle type="source" position={Position.Right} className="w-3 h-3 bg-tokyo-blue" />

      {/* Edit button - appears on hover */}
      <button
        onClick={handleEditClick}
        className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-tokyo-orange hover:bg-tokyo-orange text-tokyo-bg"
        title="Edit agent configuration"
      >
        <Edit className="h-3 w-3" />
      </button>

      {/* Info button - appears on hover */}
      <button
        onClick={handleInfoClick}
        className="absolute top-2 right-8 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-tokyo-blue hover:bg-tokyo-blue5 text-tokyo-bg"
        title="View agent details"
      >
        <Eye className="h-3 w-3" />
      </button>

      <div className="flex items-center gap-2 mb-2">
        <Bot className="h-5 w-5 text-tokyo-blue" />
        <div className="font-mono text-base text-tokyo-blue font-medium">{data.label}</div>
      </div>
      <div className="text-sm text-tokyo-comment mb-2 line-clamp-2">{data.description}</div>
      <div className="text-sm text-tokyo-green font-medium">{data.status}</div>
    </div>
  );
};

const MCPNode = ({ data }: any) => {
  const isExpanded = data.expanded;

  return (
    <div className="relative">
      <Handle type="target" position={Position.Left} className="w-3 h-3 bg-tokyo-purple" />
      <Handle type="source" position={Position.Right} className="w-3 h-3 bg-tokyo-purple" />
      <div className="bg-tokyo-dark2 border-2 border-tokyo-purple rounded-lg p-4 min-w-[260px] shadow-xl hover:shadow-2xl transition-all duration-200 hover:border-tokyo-purple hover:bg-tokyo-dark1 group">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <Server className="h-6 w-6 text-tokyo-purple" />
            <h3 className="text-tokyo-fg font-mono font-semibold text-lg">{data.label}</h3>
          </div>
          <div className="flex gap-1">
            <button
              onClick={() => data.onToggleExpand(data.serverId)}
              className="p-1 text-tokyo-comment hover:text-tokyo-purple transition-colors"
              title={isExpanded ? "Collapse" : "Expand"}
            >
              {isExpanded ? '➖' : '➕'}
            </button>
            <button
              onClick={() => data.onOpenMCPModal(data.serverId)}
              className="p-1 text-tokyo-comment hover:text-tokyo-purple transition-colors opacity-0 group-hover:opacity-100"
              title="View Details"
            >
              👁️
            </button>
          </div>
        </div>

        <p className="text-tokyo-comment text-xs font-mono mb-2">
          {data.description}
        </p>

        <div className="text-xs font-mono text-tokyo-comment">
          {data.tools?.length || 0} tools {isExpanded ? 'shown' : 'available'}
        </div>
      </div>

      {/* Connection handles */}
      <div className="absolute top-1/2 -translate-y-1/2 -left-2 w-4 h-4 bg-tokyo-purple rounded-full border-2 border-tokyo-bg"></div>
      {isExpanded && (
        <div className="absolute top-1/2 -translate-y-1/2 -right-2 w-4 h-4 bg-tokyo-purple rounded-full border-2 border-tokyo-bg"></div>
      )}
    </div>
  );
};

const ToolNode = ({ data }: any) => {
  return (
    <div className="relative">
      <Handle type="target" position={Position.Left} className="w-3 h-3 bg-tokyo-orange" />
      <div className="bg-tokyo-dark3 border border-tokyo-orange7 rounded-md p-2 min-w-[140px] shadow-md hover:shadow-lg transition-all duration-200 hover:border-tokyo-orange5">
        <div className="flex items-center gap-2 mb-1">
          <Layers className="h-3 w-3 text-tokyo-orange" />
          <h4 className="text-tokyo-fg font-mono font-medium text-xs">{data.label}</h4>
        </div>

        <p className="text-tokyo-comment text-xs font-mono line-clamp-2 mb-1">
          {data.description}
        </p>

        <div className="text-xs font-mono text-tokyo-orange opacity-75">
          {data.category}
        </div>
      </div>

      {/* Connection handle */}
      <div className="absolute top-1/2 -translate-y-1/2 -left-2 w-3 h-3 bg-tokyo-orange rounded-full border-2 border-tokyo-bg"></div>
    </div>
  );
};

const agentPageNodeTypes = {
  agent: AgentNode,
  mcp: MCPNode,
  tool: ToolNode,
};

// Station Banner Component
const StationBanner = () => (
  <div className="flex items-center gap-2">
    <div className="relative">
      <div className="w-8 h-8 bg-gradient-to-br from-tokyo-blue to-tokyo-purple rounded-lg flex items-center justify-center">
        <span className="text-tokyo-fg font-mono font-bold text-sm">S</span>
      </div>
      <div className="absolute -top-1 -right-1 w-3 h-3 bg-tokyo-green rounded-full animate-pulse"></div>
    </div>
    <div>
      <h1 className="text-xl font-mono font-semibold text-tokyo-blue">STATION</h1>
      <p className="text-xs text-tokyo-comment font-mono -mt-1">🤖 agents for engineers. Be in control by CloudShipAI</p>
    </div>
  </div>
);

// Environment Context
const EnvironmentContext = React.createContext<any>({
  environments: [],
  selectedEnvironment: null,
  setSelectedEnvironment: () => {},
  refreshTrigger: 0
});

const EnvironmentProvider = ({ children }: { children: React.ReactNode }) => {
  const [environments, setEnvironments] = useState<any[]>([]);
  const [selectedEnvironment, setSelectedEnvironment] = useState<number | null>(null);
  const [refreshTrigger, setRefreshTrigger] = useState(0);

  // Fetch environments
  useEffect(() => {
    const fetchEnvironments = async () => {
      try {
        const response = await environmentsApi.getAll();
        setEnvironments(response.data.environments || []);
      } catch (error) {
        console.error('Failed to fetch environments:', error);
        setEnvironments([]);
      }
    };
    fetchEnvironments();
  }, [refreshTrigger]);

  const environmentContext = {
    environments,
    selectedEnvironment,
    setSelectedEnvironment,
    refreshTrigger,
    refreshData: () => setRefreshTrigger(prev => prev + 1)
  };

  return (
    <EnvironmentContext.Provider value={environmentContext}>
      {children}
    </EnvironmentContext.Provider>
  );
};

const Layout = ({ children }: any) => {
  const environmentContext = React.useContext(EnvironmentContext);
  const location = useLocation();
  const navigate = useNavigate();

  // Extract environment from URL path since useParams doesn't work in Layout
  const pathParts = location.pathname.split('/').filter(Boolean);
  const currentEnvironmentName = pathParts.length >= 2 ? pathParts[1] : null;

  // Get current environment object from name
  const getCurrentEnvironment = () => {
    if (!currentEnvironmentName || !environmentContext?.environments) return null;
    return environmentContext.environments.find((env: any) =>
      env.name.toLowerCase() === currentEnvironmentName.toLowerCase()
    );
  };

  const currentEnvironment = getCurrentEnvironment();

  // Determine current page from URL
  const getCurrentPage = () => {
    const path = location.pathname;
    if (path.startsWith('/agents')) return 'agents';
    if (path.startsWith('/mcps')) return 'mcps';
    if (path.startsWith('/mcp-directory')) return 'mcp-directory';
    if (path.startsWith('/runs')) return 'runs';
    if (path.startsWith('/environments')) return 'environments';
    if (path.startsWith('/bundles')) return 'bundles';
    if (path.startsWith('/cloudship')) return 'cloudship';
    if (path.startsWith('/live-demo')) return 'live-demo';
    if (path.startsWith('/settings')) return 'settings';
    return 'agents'; // default
  };

  const currentPage = getCurrentPage();

  // Sync URL environment with context - update context when URL changes
  React.useEffect(() => {

    if (currentEnvironment && currentEnvironment.id !== environmentContext?.selectedEnvironment) {
      environmentContext.setSelectedEnvironment(currentEnvironment.id);
    } else if (!currentEnvironmentName) {
      // URL shows no environment - default to "default" environment
      const defaultEnv = environmentContext?.environments?.find(env => env.name === 'default');
      if (defaultEnv && environmentContext?.selectedEnvironment !== defaultEnv.id) {
        environmentContext.setSelectedEnvironment(defaultEnv.id);
      }
    }
  }, [currentEnvironment, currentEnvironmentName, environmentContext?.selectedEnvironment, environmentContext?.environments]);

  const handleEnvironmentChange = (environmentName: string) => {
    if (environmentName === 'default') {
      // Navigate to default environment on current page
      navigate(`/${currentPage}/default`);
    } else {
      // Navigate to specific environment on current page
      navigate(`/${currentPage}/${environmentName}`);
    }
  };

  const sidebarItems = [
    { id: 'agents', label: 'Agents', icon: Bot, path: currentEnvironmentName ? `/agents/${currentEnvironmentName}` : '/agents' },
    { id: 'mcps', label: 'MCP Servers', icon: Server, path: currentEnvironmentName ? `/mcps/${currentEnvironmentName}` : '/mcps' },
    { id: 'mcp-directory', label: 'MCP Directory', icon: Database, path: '/mcp-directory' },
    { id: 'runs', label: 'Runs', icon: MessageSquare, path: '/runs' },
    { id: 'environments', label: 'Environments', icon: Users, path: '/environments' },
    { id: 'bundles', label: 'Bundles', icon: Package, path: '/bundles' },
    { id: 'cloudship', label: 'CloudShip', icon: Ship, path: '/cloudship' },
    { id: 'live-demo', label: 'Live Demo', icon: Play, path: '/live-demo' },
    { id: 'settings', label: 'Settings', icon: Settings, path: '/settings' },
  ];

  return (
    <div className="flex h-screen bg-tokyo-bg">
      {/* Sidebar */}
      <div className="w-64 bg-tokyo-dark1 border-r border-tokyo-dark3 flex flex-col">
        {/* Header */}
        <div className="p-4 border-b border-tokyo-dark3">
          <StationBanner />
        </div>

        {/* Environment Selector */}
        <div className="p-4 border-b border-tokyo-dark3">
          <label className="block text-sm font-mono text-tokyo-fg mb-2">Environment</label>
          <select
            value={currentEnvironmentName || 'default'}
            onChange={(e) => handleEnvironmentChange(e.target.value)}
            className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-dark3 text-tokyo-fg font-mono rounded hover:border-tokyo-blue5 transition-colors"
          >
            {environmentContext?.environments?.map((env: any) => (
              <option key={env.id} value={env.name.toLowerCase()}>
                {env.name}
              </option>
            ))}
          </select>
        </div>

        {/* Navigation */}
        <nav className="flex-1 p-4">
          <ul className="space-y-2">
            {sidebarItems.map((item) => (
              <li key={item.id}>
                <button
                  onClick={() => navigate(item.path)}
                  className={`w-full flex items-center gap-3 px-3 py-2 rounded font-mono text-sm transition-colors ${
                    currentPage === item.id
                      ? 'bg-tokyo-blue bg-opacity-20 text-tokyo-bg border border-tokyo-blue shadow-md font-medium'
                      : 'text-tokyo-fg hover:text-tokyo-blue hover:bg-tokyo-dark2'
                  }`}
                >
                  <item.icon className="h-4 w-4" />
                  {item.label}
                </button>
              </li>
            ))}
          </ul>
        </nav>

        {/* Footer */}
        <div className="p-4 border-t border-tokyo-dark4">
          <CloudShipStatus />
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 flex flex-col">
        {children}
      </div>
    </div>
  );
};

const AgentsCanvas = () => {
  const navigate = useNavigate();
  const [nodes, setNodes, onNodesChange] = useNodesState<any>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<any>([]);
  const [selectedAgent, setSelectedAgent] = useState<number | null>(null);
  const [agents, setAgents] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const environmentContext = React.useContext(EnvironmentContext);
  const [modalAgentId, setModalAgentId] = useState<number | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [modalMCPServerId, setModalMCPServerId] = useState<number | null>(null);
  const [isMCPModalOpen, setIsMCPModalOpen] = useState(false);
  const [expandedServers, setExpandedServers] = useState<Set<number>>(new Set());

  // Use ref to store toggle function to avoid circular dependencies
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

  // Store the toggle function in ref
  toggleServerExpansionRef.current = toggleServerExpansion;

  // Fetch agents list (filtered by environment if selected)
  useEffect(() => {
    const fetchAgents = async () => {
      try {
        let response;
        // Always use getAll for now since environment filtering might not be implemented on the backend
        response = await agentsApi.getAll();
        let agentsList = response.data.agents || [];

        // If an environment is selected, filter agents on the frontend
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
        setAgents([]); // Ensure agents is always an array
      } finally {
        setLoading(false);
      }
    };
    fetchAgents();
  }, [environmentContext?.selectedEnvironment, environmentContext?.refreshTrigger]);

  // Function to regenerate graph with current expansion state
  const regenerateGraph = useCallback(async (agentId: number, expandedSet: Set<number>) => {
    try {
      const response = await agentsApi.getWithTools(agentId);
      const { agent, mcp_servers } = response.data;

      // Generate nodes and edges from the data
      const newNodes: Node[] = [];
      const newEdges: Edge[] = [];

      // Agent node (will be positioned by layout)
      newNodes.push({
        id: `agent-${agent.id}`,
        type: 'agent',
        position: { x: 0, y: 0 }, // ELK will position this
        data: {
          label: agent.name,
          description: agent.description,
          status: agent.is_scheduled ? 'Scheduled' : 'Manual',
          agentId: agent.id,
          onOpenModal: openAgentModal,
          onEditAgent: editAgent,
        },
      });

      // MCP servers and tools
      mcp_servers.forEach((server) => {
        const isExpanded = expandedSet.has(server.id);

        // MCP server node
        newNodes.push({
          id: `mcp-${server.id}`,
          type: 'mcp',
          position: { x: 0, y: 0 }, // ELK will position this
          data: {
            label: server.name,
            description: 'MCP Server',
            tools: server.tools,
            serverId: server.id,
            expanded: isExpanded,
            onToggleExpand: toggleServerExpansionRef.current,
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

        // Only show tool nodes if server is expanded
        if (isExpanded) {
          server.tools.forEach((tool) => {
            newNodes.push({
              id: `tool-${tool.id}`,
              type: 'tool',
              position: { x: 0, y: 0 }, // ELK will position this
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

      // Apply automatic layout using ELK.js
      const layoutedNodes = await getLayoutedNodes(newNodes, newEdges);

      setNodes(layoutedNodes);
      setEdges(newEdges);
    } catch (error) {
      console.error('Failed to regenerate graph:', error);
    }
  }, [openAgentModal, setNodes, setEdges]);

  // Reset expansion state when switching agents
  useEffect(() => {
    if (selectedAgent) {
      setExpandedServers(new Set());
    }
  }, [selectedAgent]);

  // Regenerate graph when agent or expansion state changes
  useEffect(() => {
    if (!selectedAgent) {
      // Clear the graph when no agent is selected
      setNodes([]);
      setEdges([]);
      return;
    }

    regenerateGraph(selectedAgent, expandedServers);
  }, [selectedAgent, expandedServers]); // Remove regenerateGraph from deps

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
          <div className="hidden lg:block">
            <StationBanner />
          </div>
          <div className="lg:hidden">
            <h1 className="text-xl font-mono font-semibold text-tokyo-blue">Station Agents</h1>
          </div>
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

// MCP Server Details Modal Component
const MCPServerDetailsModal = ({ serverId, isOpen, onClose }: { serverId: number | null, isOpen: boolean, onClose: () => void }) => {
  const [serverDetails, setServerDetails] = useState<any>(null);
  const [serverTools, setServerTools] = useState<any[]>([]);

  useEffect(() => {
    if (isOpen && serverId) {
      const fetchServerDetails = async () => {
        try {
          // Fetch server details
          const serverResponse = await mcpServersApi.getById(serverId);
          setServerDetails(serverResponse.data);

          // Fetch tools for this server
          try {
            const toolsResponse = await apiClient.get(`/mcp-servers/${serverId}/tools`);
            setServerTools(Array.isArray(toolsResponse.data) ? toolsResponse.data : []);
          } catch (toolsError) {
            console.error('Failed to fetch server tools:', toolsError);
            setServerTools([]);
          }
        } catch (error) {
          console.error('Failed to fetch server details:', error);
          setServerDetails(null);
          setServerTools([]);
        }
      };
      fetchServerDetails();
    }
  }, [isOpen, serverId]);

  if (!isOpen || !serverId) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 max-w-4xl w-full mx-4 max-h-[80vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-mono font-semibold text-tokyo-cyan">
            MCP Server Details: {serverDetails?.name}
          </h2>
          <button
            onClick={onClose}
            className="p-2 hover:bg-tokyo-bg-highlight rounded transition-colors"
          >
            <X className="h-5 w-5 text-tokyo-comment hover:text-tokyo-fg" />
          </button>
        </div>

        {serverDetails && (
          <div className="space-y-6">
            {/* Server Configuration */}
            <div>
              <h3 className="text-lg font-mono font-medium text-tokyo-blue mb-3">Configuration</h3>
              <div className="grid gap-3">
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Command:</span>
                  <span className="font-mono text-tokyo-fg">{serverDetails.command}</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Arguments:</span>
                  <span className="font-mono text-tokyo-fg">{serverDetails.args ? serverDetails.args.join(' ') : 'None'}</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Environment ID:</span>
                  <span className="font-mono text-tokyo-fg">{serverDetails.environment_id}</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Created:</span>
                  <span className="font-mono text-tokyo-fg">{new Date(serverDetails.created_at).toLocaleString()}</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Timeout:</span>
                  <span className="font-mono text-tokyo-fg">{serverDetails.timeout_seconds || 30}s</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Auto Restart:</span>
                  <span className="font-mono text-tokyo-fg">{serverDetails.auto_restart ? 'Yes' : 'No'}</span>
                </div>
              </div>
            </div>

            {/* Available Tools */}
            <div>
              <h3 className="text-lg font-mono font-medium text-tokyo-green mb-3">
                Available Tools ({serverTools.length})
              </h3>
              {serverTools.length === 0 ? (
                <div className="text-center p-6 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <Database className="h-12 w-12 text-tokyo-comment mx-auto mb-3" />
                  <div className="text-tokyo-comment font-mono">No tools found for this server</div>
                </div>
              ) : (
                <div className="grid gap-3">
                  {serverTools.map((tool, index) => (
                    <div key={tool.id || index} className="p-4 bg-tokyo-bg border border-tokyo-blue7 rounded">
                      <h4 className="font-mono font-medium text-tokyo-green mb-2">{tool.name}</h4>
                      <p className="text-sm text-tokyo-comment font-mono mb-2">{tool.description}</p>
                      {tool.input_schema && (
                        <div className="mt-2">
                          <div className="text-xs text-tokyo-comment font-mono mb-1">Input Schema:</div>
                          <pre className="text-xs bg-tokyo-bg-dark p-2 rounded border border-tokyo-blue7 overflow-x-auto text-tokyo-fg font-mono">
                            {JSON.stringify(JSON.parse(tool.input_schema), null, 2)}
                          </pre>
                        </div>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

// MCP Servers Page
const MCPServersPage = () => {
  const [mcpServers, setMcpServers] = useState<any[]>([]);
  const [modalMCPServerId, setModalMCPServerId] = useState<number | null>(null);
  const [isMCPModalOpen, setIsMCPModalOpen] = useState(false);
  const [environments, setEnvironments] = useState<any[]>([]);
  const [isRawConfigModalOpen, setIsRawConfigModalOpen] = useState(false);
  const [rawConfig, setRawConfig] = useState('');
  const [rawConfigEnvironment, setRawConfigEnvironment] = useState('');
  const [selectedServerId, setSelectedServerId] = useState<number | null>(null);
  const environmentContext = React.useContext(EnvironmentContext);

  // Function to open MCP server details modal
  const openMCPServerModal = (serverId: number) => {
    setModalMCPServerId(serverId);
    setIsMCPModalOpen(true);
  };

  const closeMCPServerModal = () => {
    setIsMCPModalOpen(false);
    setModalMCPServerId(null);
  };

  // Function to delete MCP server
  const handleDeleteMCPServer = async (serverId: number, serverName: string) => {
    if (!confirm(`Are you sure you want to delete the MCP server "${serverName}"? This will remove it from both the database and any template files, and clean up associated records.`)) {
      return;
    }

    try {
      // Call the individual server delete endpoint which handles both DB and template cleanup
      const response = await apiClient.delete(`/mcp-servers/${serverId}`);

      // Refresh the servers list
      await fetchMCPServers();

      // Show success message with details if available
      if (response.data?.template_deleted) {
        alert(`MCP server "${serverName}" deleted successfully from both database and template files.`);
      } else {
        alert(`MCP server "${serverName}" deleted successfully from database. ${response.data?.template_cleanup_note || ''}`);
      }
    } catch (error) {
      console.error('Failed to delete MCP server:', error);
      alert('Failed to delete MCP server. Check console for details.');
    }
  };

  // Function to view and edit raw config for a specific server
  const handleViewRawServerConfig = async (serverId: number, serverName: string) => {
    try {
      // Fetch the server config using the MCP servers endpoint
      const response = await apiClient.get(`/mcp-servers/${serverId}/config`);

      if (response.data && response.data.config) {
        // Parse and pretty-format the JSON config for display
        try {
          const configObj = JSON.parse(response.data.config);
          const formattedConfig = JSON.stringify(configObj, null, 2);
          setRawConfig(formattedConfig);
        } catch (parseError) {
          // If parsing fails, use the raw config as-is
          console.warn('Failed to parse config JSON:', parseError);
          setRawConfig(response.data.config);
        }
        setRawConfigEnvironment(`${serverName} (ID: ${serverId})`);
        setSelectedServerId(serverId); // Store server ID for saving
        setIsRawConfigModalOpen(true);
      } else {
        alert('Failed to fetch raw config');
      }
    } catch (error) {
      console.error('Failed to fetch raw config:', error);
      alert('Failed to fetch raw config. Check console for details.');
    }
  };

  // Function to save raw config
  const handleSaveRawConfig = async () => {
    try {
      if (!selectedServerId) {
        alert('No server selected');
        return;
      }

      // Validate JSON before saving
      try {
        JSON.parse(rawConfig);
      } catch (jsonError) {
        alert('Invalid JSON format. Please check your configuration.');
        return;
      }

      // Update the server config using the MCP servers endpoint
      const response = await apiClient.put(`/mcp-servers/${selectedServerId}/config`, {
        config: rawConfig
      });

      if (response.data && response.data.message) {
        // Refresh the servers list
        await fetchMCPServers();
        setIsRawConfigModalOpen(false);
        alert(response.data.message);
      } else {
        alert('Failed to save server config');
      }
    } catch (error) {
      console.error('Failed to save server config:', error);
      alert('Failed to save server config. Check console for details.');
    }
  };

  // Fetch environments data
  useEffect(() => {
    const fetchEnvironments = async () => {
      try {
        const response = await environmentsApi.getAll();
        const environmentsData = response.data.environments || [];
        setEnvironments(Array.isArray(environmentsData) ? environmentsData : []);
      } catch (error) {
        console.error('Failed to fetch environments:', error);
        setEnvironments([]);
      }
    };
    fetchEnvironments();
  }, []);

  // Define fetchMCPServers function
  const fetchMCPServers = useCallback(async () => {
    try {
      const selectedEnvId = environmentContext?.selectedEnvironment;

      if (!selectedEnvId) {
        const response = await mcpServersApi.getAll();
        setMcpServers(response.data.servers || []);
      } else {
        // Use the environment ID directly since that's what the context stores
        const response = await mcpServersApi.getByEnvironment(selectedEnvId);
        setMcpServers(response.data.servers || []);
      }
    } catch (error) {
      console.error('Failed to fetch MCP servers:', error);
      setMcpServers([]);
    }
  }, [environmentContext?.selectedEnvironment]);

  // Fetch MCP servers data when environment changes
  useEffect(() => {
    fetchMCPServers();
  }, [fetchMCPServers, environmentContext?.refreshTrigger]);

  // MCP servers are already filtered by environment on the backend
  const filteredServers = mcpServers || [];

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
        <h1 className="text-xl font-mono font-semibold text-tokyo-cyan">MCP Servers</h1>
      </div>
      <div className="flex-1 p-4 overflow-y-auto">
        {filteredServers.length === 0 ? (
          <div className="h-full flex items-center justify-center">
            <div className="text-center">
              <Database className="h-16 w-16 text-tokyo-comment mx-auto mb-4" />
              <div className="text-tokyo-fg font-mono text-lg mb-2">No MCP servers found</div>
              <div className="text-tokyo-comment font-mono text-sm">
                {environmentContext?.selectedEnvironment
                  ? `No MCP servers in the selected environment`
                  : 'Add your first MCP server to get started'
                }
              </div>
            </div>
          </div>
        ) : (
          <div className="grid gap-4 max-h-full overflow-y-auto">
            {filteredServers.map((server) => (
              <div key={server.id} className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo relative group">
                {/* Action buttons - appear on hover */}
                <div className="absolute top-3 right-3 opacity-0 group-hover:opacity-100 transition-opacity duration-200 flex gap-1">
                  <button
                    onClick={() => openMCPServerModal(server.id)}
                    className="p-1 rounded bg-tokyo-cyan hover:bg-tokyo-blue text-tokyo-bg"
                    title="View server details"
                  >
                    <Eye className="h-4 w-4" />
                  </button>
                  <button
                    onClick={() => handleViewRawServerConfig(server.id, server.name)}
                    className="p-1 rounded bg-tokyo-orange hover:bg-orange-600 text-tokyo-bg"
                    title="Edit server config"
                  >
                    <Settings className="h-4 w-4" />
                  </button>
                  <button
                    onClick={() => handleDeleteMCPServer(server.id, server.name)}
                    className="p-1 rounded bg-tokyo-red hover:bg-red-600 text-tokyo-bg"
                    title="Delete MCP server"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>

                <h3 className="font-mono font-medium text-tokyo-cyan">{server.name}</h3>
                <p className="text-sm text-tokyo-comment mt-1 font-mono">
                  {server.command} {server.args ? server.args.join(' ') : ''}
                </p>

                {/* Error message */}
                {server.error && (
                  <div className="mt-2 p-3 border-l-4 border-tokyo-red bg-tokyo-bg-highlight rounded">
                    <p className="text-sm text-tokyo-fg font-mono">{server.error}</p>
                  </div>
                )}

                <div className="mt-2 flex gap-2">
                  <span className={`px-2 py-1 text-xs rounded font-mono ${
                    server.status === 'error'
                      ? 'bg-tokyo-red text-tokyo-bg'
                      : 'bg-tokyo-green text-tokyo-bg'
                  }`}>
                    {server.status === 'error' ? 'Error' : 'Active'}
                  </span>
                  <span className="px-2 py-1 bg-tokyo-blue text-tokyo-bg text-xs rounded font-mono">Environment {server.environment_id}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* MCP Server Details Modal */}
      <MCPServerDetailsModal
        serverId={modalMCPServerId}
        isOpen={isMCPModalOpen}
        onClose={closeMCPServerModal}
      />

      {/* Raw Config Editor Modal */}
      <RawConfigEditorModal
        isOpen={isRawConfigModalOpen}
        onClose={() => setIsRawConfigModalOpen(false)}
        config={rawConfig}
        onConfigChange={setRawConfig}
        onSave={handleSaveRawConfig}
        environmentName={rawConfigEnvironment}
      />
    </div>
  );
};

// Run Details Modal Component
const RunDetailsModal = ({ runId, isOpen, onClose }: { runId: number | null, isOpen: boolean, onClose: () => void }) => {
  const [runDetails, setRunDetails] = useState<AgentRunWithDetails | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (isOpen && runId) {
      const fetchRunDetails = async () => {
        setLoading(true);
        try {
          const response = await agentRunsApi.getById(runId);
          setRunDetails(response.data.run);
        } catch (error) {
          console.error('Failed to fetch run details:', error);
          setRunDetails(null);
        } finally {
          setLoading(false);
        }
      };
      fetchRunDetails();
    }
  }, [isOpen, runId]);

  if (!isOpen || !runId) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 max-w-4xl w-full mx-4 max-h-[80vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-mono font-semibold text-tokyo-cyan">
            Run Details #{runId}
          </h2>
          <button
            onClick={onClose}
            className="p-2 hover:bg-tokyo-bg-highlight rounded transition-colors"
          >
            <X className="h-5 w-5 text-tokyo-comment hover:text-tokyo-fg" />
          </button>
        </div>

        {loading ? (
          <div className="text-center py-8">
            <div className="text-tokyo-fg font-mono">Loading run details...</div>
          </div>
        ) : runDetails ? (
          <div className="space-y-6">
            {/* Run Overview */}
            <div>
              <h3 className="text-lg font-mono font-medium text-tokyo-blue mb-3">Overview</h3>
              <div className="grid gap-3">
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Agent:</span>
                  <span className="font-mono text-tokyo-fg">{runDetails.agent_name}</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">User:</span>
                  <span className="font-mono text-tokyo-fg">{runDetails.username}</span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Status:</span>
                  <span className={`font-mono px-2 py-1 rounded text-xs ${
                    runDetails.status === 'completed' ? 'bg-tokyo-green text-tokyo-bg' :
                    runDetails.status === 'failed' ? 'bg-tokyo-red text-tokyo-bg' :
                    runDetails.status === 'running' ? 'bg-tokyo-yellow text-tokyo-bg' :
                    'bg-tokyo-comment text-tokyo-bg'
                  }`}>
                    {runDetails.status.toUpperCase()}
                  </span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Duration:</span>
                  <span className="font-mono text-tokyo-fg">
                    {runDetails.duration_seconds ? `${runDetails.duration_seconds}s` : 'N/A'}
                  </span>
                </div>
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Model:</span>
                  <span className="font-mono text-tokyo-fg">{runDetails.model_name || 'N/A'}</span>
                </div>
              </div>
            </div>

            {/* Token Usage */}
            {(runDetails.input_tokens || runDetails.output_tokens) && (
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-green mb-3">Token Usage</h3>
                <div className="grid gap-3">
                  {runDetails.input_tokens && (
                    <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                      <span className="font-mono text-tokyo-comment">Input Tokens:</span>
                      <span className="font-mono text-tokyo-fg">{runDetails.input_tokens}</span>
                    </div>
                  )}
                  {runDetails.output_tokens && (
                    <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                      <span className="font-mono text-tokyo-comment">Output Tokens:</span>
                      <span className="font-mono text-tokyo-fg">{runDetails.output_tokens}</span>
                    </div>
                  )}
                  {runDetails.total_tokens && (
                    <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                      <span className="font-mono text-tokyo-comment">Total Tokens:</span>
                      <span className="font-mono text-tokyo-fg">{runDetails.total_tokens}</span>
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Task */}
            <div>
              <h3 className="text-lg font-mono font-medium text-tokyo-blue mb-3">Task</h3>
              <div className="p-4 bg-tokyo-bg border border-tokyo-blue7 rounded">
                <pre className="text-sm text-tokyo-fg font-mono whitespace-pre-wrap">{runDetails.task}</pre>
              </div>
            </div>

            {/* Response */}
            {runDetails.final_response && (
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-cyan mb-3">Final Response</h3>
                <div className="p-4 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <pre className="text-sm text-tokyo-fg font-mono whitespace-pre-wrap">{runDetails.final_response}</pre>
                </div>
              </div>
            )}

            {/* Execution Steps */}
            {runDetails.execution_steps && runDetails.execution_steps.length > 0 && (
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-purple mb-3">
                  Execution Steps ({runDetails.execution_steps.length})
                </h3>
                <div className="space-y-3">
                  {runDetails.execution_steps.map((step, index) => (
                    <div key={index} className="p-4 bg-tokyo-bg border border-tokyo-blue7 rounded">
                      <div className="text-sm font-mono text-tokyo-purple mb-2">Step {index + 1}</div>
                      <pre className="text-xs text-tokyo-fg font-mono whitespace-pre-wrap overflow-x-auto">
                        {typeof step === 'string' ? step : JSON.stringify(step, null, 2)}
                      </pre>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Timing Information */}
            <div>
              <h3 className="text-lg font-mono font-medium text-tokyo-comment mb-3">Timing</h3>
              <div className="grid gap-3">
                <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                  <span className="font-mono text-tokyo-comment">Started:</span>
                  <span className="font-mono text-tokyo-fg">{new Date(runDetails.started_at).toLocaleString()}</span>
                </div>
                {runDetails.completed_at && (
                  <div className="flex justify-between items-center p-3 bg-tokyo-bg border border-tokyo-blue7 rounded">
                    <span className="font-mono text-tokyo-comment">Completed:</span>
                    <span className="font-mono text-tokyo-fg">{new Date(runDetails.completed_at).toLocaleString()}</span>
                  </div>
                )}
              </div>
            </div>
          </div>
        ) : (
          <div className="text-center py-8">
            <div className="text-tokyo-red font-mono">Failed to load run details</div>
          </div>
        )}
      </div>
    </div>
  );
};

// Runs Page with Modal Support
const RunsPage = () => {
  const [modalRunId, setModalRunId] = useState<number | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const environmentContext = React.useContext(EnvironmentContext);

  const openRunModal = (runId: number) => {
    setModalRunId(runId);
    setIsModalOpen(true);
  };

  const closeRunModal = () => {
    setIsModalOpen(false);
    setModalRunId(null);
  };

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <RunsPageComponent onRunClick={openRunModal} refreshTrigger={environmentContext?.refreshTrigger} />

      {/* Run Details Modal */}
      <RunDetailsModal
        runId={modalRunId}
        isOpen={isModalOpen}
        onClose={closeRunModal}
      />
    </div>
  );
};

// Environment-specific Node Components
const EnvironmentNode = ({ data }: NodeProps) => {
  const handleDeleteClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onDeleteEnvironment && data.environmentId) {
      data.onDeleteEnvironment(data.environmentId, data.label);
    }
  };

  return (
    <div className="w-[320px] h-[160px] px-4 py-3 shadow-lg border border-tokyo-orange rounded-lg relative bg-tokyo-dark2 group">
      <Handle type="source" position={Position.Right} style={{ background: '#ff9e64', width: 12, height: 12 }} />
      <Handle type="source" position={Position.Bottom} style={{ background: '#7dcfff', width: 12, height: 12 }} />

      {/* Delete button - appears on hover */}
      {data.label !== 'default' && (
        <button
          onClick={handleDeleteClick}
          className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-tokyo-red hover:bg-red-600 text-tokyo-bg"
          title={`Delete environment "${data.label}"`}
        >
          <Trash2 className="h-3 w-3" />
        </button>
      )}

      <div className="flex items-center gap-2 mb-3">
        <Globe className="h-6 w-6 text-tokyo-orange" />
        <div className="font-mono text-lg text-tokyo-orange font-bold">{data.label}</div>
      </div>
      <div className="text-sm text-tokyo-comment mb-3">{data.description}</div>
      <div className="flex gap-4 text-sm font-mono">
        <div>
          <span className="text-tokyo-blue">{data.agentCount}</span>
          <span className="text-tokyo-comment"> agents</span>
        </div>
        <div>
          <span className="text-tokyo-cyan">{data.serverCount}</span>
          <span className="text-tokyo-comment"> servers</span>
        </div>
      </div>
    </div>
  );
};

const EnvironmentAgentNode = ({ data }: NodeProps) => {
  return (
    <div className="w-[240px] h-[100px] px-3 py-2 shadow-lg border border-tokyo-blue7 bg-tokyo-dark2 rounded-lg relative">
      <Handle type="target" position={Position.Left} className="w-3 h-3 bg-tokyo-blue" />
      <div className="flex items-center gap-2 mb-1">
        <Bot className="h-4 w-4 text-tokyo-blue" />
        <div className="font-mono text-sm text-tokyo-blue font-medium">{data.label}</div>
      </div>
      <div className="text-xs text-tokyo-comment mb-1 line-clamp-2">{data.description}</div>
      <div className="text-xs text-tokyo-green font-medium">{data.status}</div>
    </div>
  );
};

const EnvironmentMCPNode = ({ data }: NodeProps) => {
  return (
    <div className="w-[240px] h-[100px] px-3 py-2 shadow-lg border border-tokyo-cyan7 bg-tokyo-dark2 rounded-lg relative">
      <Handle type="target" position={Position.Left} className="w-3 h-3 bg-tokyo-cyan" />
      <div className="flex items-center gap-2 mb-1">
        <Database className="h-4 w-4 text-tokyo-cyan" />
        <div className="font-mono text-sm text-tokyo-cyan font-medium">{data.label}</div>
      </div>
      <div className="text-xs text-tokyo-comment mb-1">{data.description}</div>
      <div className="text-xs text-tokyo-purple font-medium">MCP Server</div>
    </div>
  );
};

// Environments Page with ReactFlow Node Graph
const EnvironmentsPage = () => {
  const [nodes, setNodes, onNodesChange] = useNodesState<any>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<any>([]);
  const [environments, setEnvironments] = useState<any[]>([]);
  const [selectedEnvironment, setSelectedEnvironment] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [rebuildingGraph, setRebuildingGraph] = useState(false);

  // Modal states
  const [isSyncModalOpen, setIsSyncModalOpen] = useState(false);
  const [isAddServerModalOpen, setIsAddServerModalOpen] = useState(false);
  const [isBundleModalOpen, setIsBundleModalOpen] = useState(false);
  const [isBuildImageModalOpen, setIsBuildImageModalOpen] = useState(false);
  const [isInstallBundleModalOpen, setIsInstallBundleModalOpen] = useState(false);
  const [isVariablesModalOpen, setIsVariablesModalOpen] = useState(false);
  const [isDeployModalOpen, setIsDeployModalOpen] = useState(false);

  // Button handlers
  const handleSyncEnvironment = () => {
    setIsSyncModalOpen(true);
  };

  const handleAddServer = () => {
    setIsAddServerModalOpen(true);
  };

  const handleBundleEnvironment = () => {
    setIsBundleModalOpen(true);
  };

  const handleBuildImage = () => {
    setIsBuildImageModalOpen(true);
  };

  const handleInstallBundle = () => {
    setIsInstallBundleModalOpen(true);
  };

  const handleVariables = () => {
    setIsVariablesModalOpen(true);
  };

  const handleDeploy = () => {
    setIsDeployModalOpen(true);
  };

  const handleRefreshGraph = () => {
    if (selectedEnvironment) {
      setRebuildingGraph(true);
      // This will trigger the useEffect that rebuilds the graph
    }
  };

  // Function to delete environment
  const handleDeleteEnvironment = async (environmentId: number, environmentName: string) => {
    if (!confirm(`Are you sure you want to delete the environment "${environmentName}"? This will remove all associated agents, MCP servers, and file-based configurations. This action cannot be undone.`)) {
      return;
    }

    try {
      // Call the new unified API endpoint
      await apiClient.delete(`/environments/${environmentId}`);

      // Refresh environments list
      const response = await environmentsApi.getAll();
      const envs = response.data.environments || [];
      setEnvironments(envs);

      // Select the first available environment or clear selection
      if (envs.length > 0) {
        setSelectedEnvironment(envs[0].id);
      } else {
        setSelectedEnvironment(null);
      }

      alert(`Environment "${environmentName}" deleted successfully`);
    } catch (error) {
      console.error('Failed to delete environment:', error);
      alert('Failed to delete environment. Check console for details.');
    }
  };

  // Environment-specific node types
  const environmentPageNodeTypes: NodeTypes = {
    agent: EnvironmentAgentNode,
    mcp: EnvironmentMCPNode,
    tool: ToolNode,
    environment: EnvironmentNode,
  };

  // Fetch environments
  useEffect(() => {
    const fetchEnvironments = async () => {
      try {
        const response = await environmentsApi.getAll();
        const envs = response.data.environments || [];
        setEnvironments(envs);
        if (envs.length > 0 && !selectedEnvironment) {
          setSelectedEnvironment(envs[0].id);
        }
      } catch (error) {
        console.error('Failed to fetch environments:', error);
        setEnvironments([]);
      } finally {
        setLoading(false);
      }
    };
    fetchEnvironments();
  }, []);

  // Build environment graph when environment changes
  useEffect(() => {
    if (!selectedEnvironment || loading) return;

    const buildEnvironmentGraph = async () => {
      setRebuildingGraph(true);
      try {
        const selectedEnv = environments.find(env => env.id === selectedEnvironment);

        // Fetch agents and MCP servers for selected environment
        const [agentsRes, mcpRes] = await Promise.all([
          agentsApi.getAll(),
          mcpServersApi.getAll()
        ]);

        const agents = (agentsRes.data.agents || []).filter((agent: any) =>
          agent.environment_id === selectedEnvironment
        );
        // Debug logging
        console.log('MCP API Response:', mcpRes.data);
        console.log('Raw MCP data:', mcpRes.data.servers || mcpRes.data || []);
        console.log('Selected environment:', selectedEnvironment);

        const mcpServers = (mcpRes.data.servers || mcpRes.data || []).filter((server: any) =>
          server.environment_id === selectedEnvironment
        );
        console.log('Filtered MCP servers:', mcpServers);

        const newNodes: any[] = [];
        const newEdges: any[] = [];

        // Environment node (center hub)
        newNodes.push({
          id: `env-${selectedEnvironment}`,
          type: 'environment',
          position: { x: 0, y: 0 },
          data: {
            label: selectedEnv?.name || 'Environment',
            description: selectedEnv?.description || 'Environment Hub',
            agentCount: agents.length,
            serverCount: mcpServers.length,
            environmentId: selectedEnvironment,
            onDeleteEnvironment: handleDeleteEnvironment,
          },
        });

        // Agent nodes - better spacing
        agents.forEach((agent: any, index: number) => {
          newNodes.push({
            id: `agent-${agent.id}`,
            type: 'agent',
            position: { x: 500, y: index * 180 },
            data: {
              label: agent.name,
              description: agent.description || 'No description',
              status: agent.schedule ? 'Scheduled' : 'Manual',
              agentId: agent.id,
            },
          });

          // Agent edge to environment
          newEdges.push({
            id: `edge-env-${selectedEnvironment}-to-agent-${agent.id}`,
            source: `env-${selectedEnvironment}`,
            target: `agent-${agent.id}`,
            animated: true,
            style: {
              stroke: '#7aa2f7',
              strokeWidth: 2,
              filter: 'drop-shadow(0 0 6px rgba(122, 162, 247, 0.4))'
            },
          });
        });

        // MCP server nodes - better spacing and positioning
        mcpServers.forEach((server: any, index: number) => {
          newNodes.push({
            id: `mcp-${server.id}`,
            type: 'mcp',
            position: { x: 500, y: (agents.length * 180) + (index * 180) + 100 },
            data: {
              label: server.name,
              description: 'MCP Server',
              serverId: server.id,
            },
          });

          // MCP server edge to environment
          newEdges.push({
            id: `edge-env-${selectedEnvironment}-to-mcp-${server.id}`,
            source: `env-${selectedEnvironment}`,
            target: `mcp-${server.id}`,
            animated: true,
            style: {
              stroke: '#0db9d7',
              strokeWidth: 2,
              filter: 'drop-shadow(0 0 6px rgba(13, 185, 215, 0.4))'
            },
          });
        });

        // Apply ELK layout
        const layoutedElements = await layoutElements(newNodes, newEdges, 'RIGHT');
        setNodes(layoutedElements.nodes);
        setEdges(layoutedElements.edges);

      } catch (error) {
        console.error('Failed to build environment graph:', error);
      } finally {
        setRebuildingGraph(false);
      }
    };

    buildEnvironmentGraph();
  }, [selectedEnvironment, environments, loading]);

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center bg-tokyo-bg">
        <div className="text-tokyo-fg font-mono">Loading environments...</div>
      </div>
    );
  }

  return (
    <div className="h-full flex bg-tokyo-bg">
      {/* Left Column - Canvas */}
      <div className="flex-1 flex flex-col border-r border-tokyo-dark4">
        {/* Canvas Header */}
        <div className="px-6 py-4 border-b border-tokyo-dark4">
          <h2 className="text-lg font-mono font-semibold text-tokyo-fg">Environment Graph</h2>
          <p className="text-sm text-tokyo-comment mt-1">Visual representation of agents, MCP servers, and tools</p>
        </div>

        {/* Canvas Content */}
        {environments.length === 0 ? (
          <div className="flex-1 flex items-center justify-center">
            <div className="text-center">
              <Globe className="h-16 w-16 text-tokyo-comment mx-auto mb-4" />
              <div className="text-tokyo-fg font-mono text-lg mb-2">No environments found</div>
              <div className="text-tokyo-comment font-mono text-sm">Create your first environment to get started</div>
            </div>
          </div>
      ) : rebuildingGraph ? (
        <div className="flex-1 flex items-center justify-center">
          <div className="text-tokyo-fg font-mono">Rebuilding environment graph...</div>
        </div>
      ) : (
        <div className="flex-1 h-full">
          <ReactFlowProvider>
            <ReactFlow
              key={`environment-${selectedEnvironment}`}
              nodes={nodes}
              edges={edges}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              nodeTypes={environmentPageNodeTypes}
              fitView
              fitViewOptions={{ padding: 0.1, duration: 500 }}
              className="bg-tokyo-bg w-full h-full"
              defaultEdgeOptions={{
                animated: true,
                style: {
                  stroke: '#ff00ff',
                  strokeWidth: 3,
                  filter: 'drop-shadow(0 0 6px rgba(255, 0, 255, 0.4))'
                }
              }}
            />
          </ReactFlowProvider>
        </div>
      )}
      </div>

      {/* Right Column - Controls */}
      <div className="w-96 flex flex-col bg-tokyo-dark2 overflow-y-auto">
        {/* Environment Selector */}
        <div className="p-6 border-b border-tokyo-fg-gutter">
          <label className="block text-sm font-mono font-bold text-tokyo-orange mb-2">Environment</label>
          {environments.length > 0 && (
            <select
              value={selectedEnvironment || ''}
              onChange={(e) => setSelectedEnvironment(Number(e.target.value))}
              className="w-full bg-[#292e42] border-[3px] border-[#7dcfff] text-[#7dcfff] font-mono font-semibold px-4 py-3 rounded-lg focus:outline-none focus:border-[#ff9e64] focus:text-[#ff9e64] text-lg shadow-tokyo-glow"
              style={{ backgroundColor: '#292e42', color: '#7dcfff', borderColor: '#7dcfff' }}
            >
              {environments.map((env) => (
                <option key={env.id} value={env.id} className="bg-[#1a1b26] text-[#c0caf5]" style={{ backgroundColor: '#1a1b26', color: '#c0caf5' }}>
                  {env.name}
                </option>
              ))}
            </select>
          )}
        </div>

        {/* Action Buttons */}
        {selectedEnvironment && (
          <div className="p-6 space-y-3">
            <h3 className="text-sm font-mono text-tokyo-comment mb-4">Actions</h3>

            <button
              onClick={handleSyncEnvironment}
              className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-tokyo-blue text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm font-medium transition-colors"
            >
              <Play className="h-4 w-4" />
              <span>Sync Environment</span>
            </button>

            <button
              onClick={handleVariables}
              className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-tokyo-cyan text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm font-medium transition-colors"
            >
              <FileText className="h-4 w-4" />
              <span>Edit Variables</span>
            </button>

            <button
              onClick={handleAddServer}
              className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-tokyo-green text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm font-medium transition-colors"
            >
              <Plus className="h-4 w-4" />
              <span>Add MCP Server</span>
            </button>

            <div className="border-t border-tokyo-dark4 pt-3 mt-3">
              <h3 className="text-sm font-mono text-tokyo-comment mb-3">Deployment</h3>

              <button
                onClick={handleDeploy}
                className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-tokyo-purple text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm font-medium transition-colors"
              >
                <Rocket className="h-4 w-4" />
                <span>Deploy Template</span>
              </button>

              <button
                onClick={handleBuildImage}
                className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-tokyo-orange text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm font-medium transition-colors mt-2"
              >
                <Package className="h-4 w-4" />
                <span>Build Docker Image</span>
              </button>

              <button
                onClick={handleBundleEnvironment}
                className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-tokyo-yellow text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm font-medium transition-colors mt-2"
              >
                <Archive className="h-4 w-4" />
                <span>Create Bundle</span>
              </button>
            </div>
          </div>
        )}

        {/* Install Bundle (always visible) */}
        <div className="p-6 border-t border-tokyo-dark4 mt-auto">
          <button
            onClick={handleInstallBundle}
            className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-tokyo-magenta text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm font-medium transition-colors"
          >
            <Download className="h-4 w-4" />
            <span>Install Bundle</span>
          </button>
        </div>
      </div>

      {/* Modals */}
      <SyncModal
        isOpen={isSyncModalOpen}
        onClose={() => setIsSyncModalOpen(false)}
        environment={selectedEnvironment ? environments.find(env => env.id === selectedEnvironment)?.name || 'default' : 'default'}
      />

      {/* Add Server Modal */}
      <AddServerModal
        isOpen={isAddServerModalOpen}
        onClose={() => setIsAddServerModalOpen(false)}
        environmentName={selectedEnvironment ? environments.find(env => env.id === selectedEnvironment)?.name || 'default' : 'default'}
      />

      {/* Bundle Environment Modal */}
      <BundleEnvironmentModal
        isOpen={isBundleModalOpen}
        onClose={() => setIsBundleModalOpen(false)}
        environmentName={selectedEnvironment ? environments.find(env => env.id === selectedEnvironment)?.name || 'default' : 'default'}
      />

      {/* Build Image Modal */}
      <BuildImageModal
        isOpen={isBuildImageModalOpen}
        onClose={() => setIsBuildImageModalOpen(false)}
        environmentName={selectedEnvironment ? environments.find(env => env.id === selectedEnvironment)?.name || 'default' : 'default'}
      />

      {/* Install Bundle Modal */}
      <InstallBundleModal
        isOpen={isInstallBundleModalOpen}
        onClose={() => setIsInstallBundleModalOpen(false)}
        onSuccess={async () => {
          // Refresh environments list after successful installation
          const response = await environmentsApi.getAll();
          const envs = response.data.environments || [];
          setEnvironments(envs);
        }}
      />

      {/* Variables Editor Modal */}
      {selectedEnvironment && (
        <VariablesEditorModal
          isOpen={isVariablesModalOpen}
          onClose={() => setIsVariablesModalOpen(false)}
          environmentId={selectedEnvironment}
          environmentName={environments.find(env => env.id === selectedEnvironment)?.name || 'default'}
        />
      )}

      {/* Deploy Modal */}
      {selectedEnvironment && (
        <DeployModal
          isOpen={isDeployModalOpen}
          onClose={() => setIsDeployModalOpen(false)}
          environmentId={selectedEnvironment}
          environmentName={environments.find(env => env.id === selectedEnvironment)?.name || 'default'}
        />
      )}
    </div>
  );
};

// Simple Bundles Page
const BundlesPage = () => {
  const [loading, setLoading] = useState(true);
  const [bundles, setBundles] = useState<any[]>([]);

  useEffect(() => {
    const fetchBundles = async () => {
      try {
        const response = await bundlesApi.getAll();
        setBundles(response.data.bundles || []);
      } catch (error) {
        console.error('Failed to fetch bundles:', error);
        setBundles([]);
      } finally {
        setLoading(false);
      }
    };
    fetchBundles();
  }, []);

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center bg-tokyo-bg">
        <div className="text-tokyo-fg font-mono">Loading bundles...</div>
      </div>
    );
  }

  return (
    <div className="h-full p-6 bg-tokyo-bg">
      <h1 className="text-2xl font-mono font-semibold text-tokyo-blue mb-6">Agent Bundles</h1>

      {bundles.length === 0 ? (
        <div className="text-center">
          <Package className="h-16 w-16 text-tokyo-comment mx-auto mb-4" />
          <div className="text-tokyo-fg font-mono text-lg mb-2">No bundles found</div>
          <div className="text-tokyo-comment font-mono text-sm">Agent bundles will appear here</div>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {bundles.map((bundle, index) => (
            <div key={index} className="bg-tokyo-dark1 border border-tokyo-orange7 rounded-lg p-4 hover:border-tokyo-orange transition-colors">
              <div className="flex items-center gap-2 mb-3">
                <Package className="h-5 w-5 text-tokyo-orange" />
                <h3 className="text-tokyo-fg font-mono font-medium truncate">{bundle.name}</h3>
              </div>

              <div className="space-y-2 text-sm font-mono">
                <div className="flex justify-between">
                  <span className="text-tokyo-comment">File:</span>
                  <span className="text-tokyo-fg truncate ml-2" title={bundle.file_name}>
                    {bundle.file_name}
                  </span>
                </div>

                <div className="flex justify-between">
                  <span className="text-tokyo-comment">Size:</span>
                  <span className="text-tokyo-green">
                    {(bundle.size / 1024).toFixed(1)} KB
                  </span>
                </div>

                <div className="flex justify-between">
                  <span className="text-tokyo-comment">Modified:</span>
                  <span className="text-tokyo-blue">
                    {new Date(bundle.modified_time).toLocaleDateString()}
                  </span>
                </div>

                <div className="pt-2 border-t border-tokyo-dark4">
                  <div className="text-tokyo-comment text-xs">Path:</div>
                  <div className="text-tokyo-fg text-xs break-all bg-tokyo-dark2 p-2 rounded mt-1">
                    {bundle.file_path}
                  </div>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

// Agent Editor Component
const AgentEditor = () => {
  const { agentId } = useParams<{ agentId: string }>();
  const navigate = useNavigate();
  const [agentData, setAgentData] = useState<any>(null);
  const [promptContent, setPromptContent] = useState<string>('');
  const [outputSchema, setOutputSchema] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [schemaValid, setSchemaValid] = useState(true);

  useEffect(() => {
    const fetchAgentData = async () => {
      if (!agentId) return;

      try {
        setLoading(true);

        // Fetch agent details
        const agentResponse = await agentsApi.getById(parseInt(agentId));
        setAgentData(agentResponse.data.agent);

        // Fetch prompt file content via API
        const promptResponse = await agentsApi.getPrompt(parseInt(agentId));
        const content = promptResponse.data.content || '';
        setPromptContent(content);

        // Extract output schema from YAML frontmatter
        try {
          const yamlMatch = content.match(/^---\n([\s\S]*?)\n---/);
          if (yamlMatch) {
            const frontmatter: any = yaml.load(yamlMatch[1]);
            if (frontmatter.output?.schema) {
              setOutputSchema(JSON.stringify(frontmatter.output.schema, null, 2));
            }
          }
        } catch (err) {
          console.error('Failed to parse YAML frontmatter:', err);
        }
      } catch (err: any) {
        setError(err.response?.data?.error || 'Failed to load agent data');
      } finally {
        setLoading(false);
      }
    };

    fetchAgentData();
  }, [agentId]);

  const handleSchemaChange = (newSchema: string) => {
    setOutputSchema(newSchema);

    // Update the YAML content with new schema
    try {
      const yamlMatch = promptContent.match(/^---\n([\s\S]*?)\n---/);
      if (yamlMatch) {
        const frontmatter: any = yaml.load(yamlMatch[1]);

        if (newSchema.trim()) {
          const schemaObject = JSON.parse(newSchema);
          if (!frontmatter.output) frontmatter.output = {};
          frontmatter.output.schema = schemaObject;
        } else {
          // Remove schema if empty
          if (frontmatter.output) {
            delete frontmatter.output.schema;
            if (Object.keys(frontmatter.output).length === 0) {
              delete frontmatter.output;
            }
          }
        }

        const newFrontmatter = yaml.dump(frontmatter, { indent: 2, lineWidth: -1 });
        const restOfContent = promptContent.substring(yamlMatch[0].length);
        setPromptContent(`---\n${newFrontmatter}---${restOfContent}`);
      }
    } catch (err) {
      console.error('Failed to update YAML with schema:', err);
    }
  };

  const handleSave = async () => {
    if (!agentId) return;

    if (!schemaValid) {
      setError('Cannot save: Output schema contains errors');
      return;
    }

    try {
      setSaving(true);
      setError(null);

      // Save prompt content via API
      await agentsApi.updatePrompt(parseInt(agentId), promptContent);

      setSaveSuccess(true);
      setTimeout(() => setSaveSuccess(false), 3000);
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to save agent configuration');
    } finally {
      setSaving(false);
    }
  };

  const handleBack = () => {
    navigate(-1);
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-tokyo-bg flex items-center justify-center">
        <div className="text-tokyo-comment">Loading agent configuration...</div>
      </div>
    );
  }

  if (error && !agentData) {
    return (
      <div className="min-h-screen bg-tokyo-bg flex items-center justify-center">
        <div className="text-tokyo-red">{error}</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-tokyo-bg">
      {/* Header with breadcrumbs */}
      <div className="bg-tokyo-dark1 border-b border-tokyo-dark3 px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <button
              onClick={handleBack}
              className="flex items-center gap-2 text-tokyo-comment hover:text-tokyo-blue transition-colors"
            >
              <ArrowLeft className="h-4 w-4" />
              <span>Back to Agents</span>
            </button>
            <div className="text-tokyo-comment">/</div>
            <h1 className="text-xl font-mono text-tokyo-blue font-bold">
              Edit Agent: {agentData?.name || 'Unknown'}
            </h1>
          </div>

          <div className="flex items-center gap-3">
            {saveSuccess && (
              <div className="text-sm text-tokyo-green">Saved successfully!</div>
            )}
            {error && (
              <div className="text-sm text-tokyo-red">{error}</div>
            )}
            <button
              onClick={handleSave}
              disabled={saving || !schemaValid}
              className="flex items-center gap-2 px-4 py-2 bg-tokyo-blue hover:bg-tokyo-blue5 text-tokyo-bg rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              <Save className="h-4 w-4" />
              {saving ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </div>

        {/* Agent description */}
        {agentData?.description && (
          <div className="mt-2 text-sm text-tokyo-comment">
            {agentData.description}
          </div>
        )}
      </div>

      {/* Two-column editor layout */}
      <div className="flex-1 p-6">
        <div className="grid grid-cols-2 gap-4 h-[calc(100vh-200px)]">
          {/* Left column: YAML Editor */}
          <div className="bg-tokyo-dark1 rounded-lg border border-tokyo-dark3 flex flex-col">
            <div className="p-4 border-b border-tokyo-dark3">
              <h2 className="text-lg font-mono text-tokyo-blue">Agent Configuration</h2>
              <p className="text-sm text-tokyo-comment mt-1">
                Edit the agent's prompt file. After saving, run <code className="bg-tokyo-bg px-1 rounded text-tokyo-orange">stn sync {agentData?.environment_name || 'environment'}</code> to apply.
              </p>
            </div>

            <div className="flex-1 overflow-hidden">
              <Editor
                height="100%"
                defaultLanguage="yaml"
                value={promptContent}
                onChange={(value) => setPromptContent(value || '')}
                theme="vs-dark"
                options={{
                  minimap: { enabled: false },
                  fontSize: 14,
                  fontFamily: 'JetBrains Mono, Fira Code, Monaco, monospace',
                  lineNumbers: 'on',
                  rulers: [80],
                  wordWrap: 'on',
                  automaticLayout: true,
                  scrollBeyondLastLine: false,
                  padding: { top: 16, bottom: 16 }
                }}
              />
            </div>
          </div>

          {/* Right column: JSON Schema Editor */}
          <div className="bg-tokyo-dark1 rounded-lg border border-tokyo-dark3 flex flex-col overflow-hidden">
            <JsonSchemaEditor
              schema={outputSchema}
              onChange={handleSchemaChange}
              onValidation={(isValid) => setSchemaValid(isValid)}
            />
          </div>
        </div>
      </div>
    </div>
  );
};

// Raw Config Editor Modal Component
const RawConfigEditorModal = ({
  isOpen,
  onClose,
  config,
  onConfigChange,
  onSave,
  environmentName
}: {
  isOpen: boolean;
  onClose: () => void;
  config: string;
  onConfigChange: (config: string) => void;
  onSave: () => void;
  environmentName: string;
}) => {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 max-w-6xl w-full mx-4 max-h-[90vh] overflow-hidden flex flex-col">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-mono font-semibold text-tokyo-cyan">
            Server Config Editor - {environmentName}
          </h2>
          <button
            onClick={onClose}
            className="p-2 hover:bg-tokyo-bg-highlight rounded transition-colors"
          >
            <X className="h-5 w-5 text-tokyo-comment hover:text-tokyo-fg" />
          </button>
        </div>

        <div className="flex-1 flex flex-col overflow-hidden">
          <div className="mb-4">
            <p className="text-tokyo-comment text-sm font-mono">
              Edit the configuration for this specific MCP server.
              Changes will be merged back into the environment's template.json file.
            </p>
          </div>

          {/* Monaco Editor */}
          <div className="flex-1 border border-tokyo-blue7 rounded overflow-hidden min-h-[500px]">
            <Editor
              height="500px"
              defaultLanguage="json"
              value={config}
              onChange={(value) => onConfigChange(value || '')}
              theme="vs-dark"
              options={{
                minimap: { enabled: false },
                fontSize: 14,
                fontFamily: 'JetBrains Mono, Fira Code, Monaco, monospace',
                lineNumbers: 'on',
                rulers: [80],
                wordWrap: 'on',
                automaticLayout: true,
                scrollBeyondLastLine: false,
                padding: { top: 16, bottom: 16 },
                formatOnPaste: true,
                formatOnType: true
              }}
            />
          </div>

          {/* Action Buttons */}
          <div className="flex justify-between items-center mt-6 pt-4 border-t border-tokyo-blue7">
            <div className="text-tokyo-comment text-sm font-mono">
              💡 Tip: Use Ctrl+Shift+F to format JSON
            </div>
            <div className="flex gap-3">
              <button
                onClick={onClose}
                className="px-4 py-2 bg-tokyo-comment hover:bg-gray-600 text-tokyo-bg rounded font-mono text-sm transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={onSave}
                className="px-4 py-2 bg-tokyo-green hover:bg-green-600 text-tokyo-bg rounded font-mono text-sm transition-colors"
              >
                Save Changes
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

// Variables Editor Modal Component
const VariablesEditorModal = ({
  isOpen,
  onClose,
  environmentId,
  environmentName
}: {
  isOpen: boolean;
  onClose: () => void;
  environmentId: number;
  environmentName: string;
}) => {
  const [variables, setVariables] = useState('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (isOpen && environmentId) {
      const fetchVariables = async () => {
        setLoading(true);
        try {
          const response = await apiClient.get(`/environments/${environmentId}/variables`);
          setVariables(response.data.content || '');
        } catch (error) {
          console.error('Failed to fetch variables:', error);
          setVariables('');
        } finally {
          setLoading(false);
        }
      };
      fetchVariables();
    }
  }, [isOpen, environmentId]);

  const handleSave = async () => {
    setSaving(true);
    try {
      const response = await apiClient.put(`/environments/${environmentId}/variables`, {
        content: variables
      });
      alert(response.data.message);
      onClose();
    } catch (error: any) {
      console.error('Failed to save variables:', error);
      alert(error.response?.data?.error || 'Failed to save variables');
    } finally {
      setSaving(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 max-w-6xl w-full mx-4 max-h-[90vh] overflow-hidden flex flex-col">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-mono font-semibold text-tokyo-cyan">
            Environment Variables - {environmentName}
          </h2>
          <button
            onClick={onClose}
            className="p-2 hover:bg-tokyo-bg-highlight rounded transition-colors"
          >
            <X className="h-5 w-5 text-tokyo-comment hover:text-tokyo-fg" />
          </button>
        </div>

        <div className="flex-1 flex flex-col overflow-hidden">
          {/* Warning Banner */}
          <div className="mb-4 p-4 border-2 border-tokyo-orange rounded">
            <p className="text-tokyo-orange text-sm font-mono">
              ⚠️ Important: These variables are local to your machine and will NOT be included in bundles or Docker images.
              They are for local development only.
            </p>
          </div>

          <div className="mb-4">
            <p className="text-tokyo-comment text-sm font-mono">
              Edit the variables.yml file for this environment.
              After saving, run 'stn sync' to apply changes to your MCP servers.
            </p>
          </div>

          {loading ? (
            <div className="flex-1 flex items-center justify-center">
              <p className="text-tokyo-comment font-mono">Loading variables...</p>
            </div>
          ) : (
            <>
              {/* Monaco Editor */}
              <div className="flex-1 border border-tokyo-blue7 rounded overflow-hidden min-h-[500px]">
                <Editor
                  height="500px"
                  defaultLanguage="yaml"
                  value={variables}
                  onChange={(value) => setVariables(value || '')}
                  theme="vs-dark"
                  options={{
                    minimap: { enabled: false },
                    fontSize: 14,
                    fontFamily: 'JetBrains Mono, Fira Code, Monaco, monospace',
                    lineNumbers: 'on',
                    rulers: [80],
                    wordWrap: 'on',
                    automaticLayout: true,
                    scrollBeyondLastLine: false,
                    padding: { top: 16, bottom: 16 },
                    formatOnPaste: true,
                    formatOnType: true
                  }}
                />
              </div>

              {/* Action Buttons */}
              <div className="flex justify-between items-center mt-6 pt-4 border-t border-tokyo-blue7">
                <div className="text-tokyo-comment text-sm font-mono">
                  💡 After saving, run 'stn sync' to apply variable changes
                </div>
                <div className="flex gap-3">
                  <button
                    onClick={onClose}
                    className="px-4 py-2 bg-tokyo-comment hover:bg-gray-600 text-tokyo-bg rounded font-mono text-sm transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleSave}
                    disabled={saving}
                    className="px-4 py-2 bg-tokyo-green hover:bg-green-600 text-tokyo-bg rounded font-mono text-sm transition-colors disabled:opacity-50"
                  >
                    {saving ? 'Saving...' : 'Save Variables'}
                  </button>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
};

// Settings Page Component
const SettingsPage = () => {
  const [config, setConfig] = useState('');
  const [configPath, setConfigPath] = useState('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [configObj, setConfigObj] = useState<any>({});
  const [expandedSections, setExpandedSections] = useState({
    ai: true,
    cloudship: false,
    ports: false,
    other: false,
  });

  useEffect(() => {
    const loadConfig = async () => {
      try {
        const response = await fetch('/api/v1/settings/config/file');
        if (!response.ok) {
          throw new Error('Failed to load config file');
        }
        const data = await response.json();
        setConfig(data.content);
        setConfigPath(data.path);
        try {
          const parsed = yaml.load(data.content) as any;
          setConfigObj(parsed || {});
        } catch (e) {
          console.error('YAML parse error:', e);
        }
        setLoading(false);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load config');
        setLoading(false);
      }
    };
    loadConfig();
  }, []);

  const updateConfig = (updates: any) => {
    const newObj = { ...configObj, ...updates };
    setConfigObj(newObj);
    try {
      const newYaml = yaml.dump(newObj, { lineWidth: -1 });
      setConfig(newYaml);
    } catch (e) {
      console.error('YAML dump error:', e);
    }
  };

  const updateCloudShipConfig = (updates: any) => {
    const newCloudShip = { ...configObj.cloudship, ...updates };
    updateConfig({ cloudship: newCloudShip });
  };

  const handleYamlChange = (value: string | undefined) => {
    setConfig(value || '');
    try {
      const parsed = yaml.load(value || '') as any;
      setConfigObj(parsed || {});
    } catch (e) {
      // Invalid YAML, keep config string but don't update object
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    setSuccess(false);

    try {
      const response = await fetch('/api/v1/settings/config/file', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ content: config }),
      });

      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || 'Failed to save config');
      }

      setSuccess(true);
      setTimeout(() => setSuccess(false), 5000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save config');
    } finally {
      setSaving(false);
    }
  };

  const toggleSection = (section: keyof typeof expandedSections) => {
    setExpandedSections(prev => ({ ...prev, [section]: !prev[section] }));
  };

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
        <div>
          <h1 className="text-xl font-mono font-semibold text-tokyo-cyan">Settings</h1>
          {configPath && (
            <p className="text-xs text-tokyo-comment font-mono mt-1">{configPath}</p>
          )}
        </div>
        <button
          onClick={handleSave}
          disabled={saving}
          className="px-4 py-2 bg-tokyo-blue hover:bg-tokyo-blue5 disabled:bg-tokyo-blue7 text-tokyo-bg rounded font-mono text-sm transition-colors"
        >
          {saving ? 'Saving...' : 'Save Config'}
        </button>
      </div>

      {/* Warning Banner */}
      <div className="bg-tokyo-orange7 border-b border-tokyo-orange p-3">
        <div className="flex items-center gap-2 text-tokyo-orange font-mono text-sm">
          <AlertTriangle className="h-4 w-4" />
          <span>Station needs to be restarted to apply configuration changes</span>
        </div>
      </div>

      {/* Success/Error Messages */}
      {success && (
        <div className="bg-tokyo-green7 border-b border-tokyo-green p-3">
          <div className="flex items-center gap-2 text-tokyo-green font-mono text-sm">
            <CircleCheck className="h-4 w-4" />
            <span>Config file saved successfully. Restart Station to apply changes.</span>
          </div>
        </div>
      )}

      {error && (
        <div className="bg-tokyo-red7 border-b border-tokyo-red p-3">
          <div className="flex items-center gap-2 text-tokyo-red font-mono text-sm">
            <AlertTriangle className="h-4 w-4" />
            <span>{error}</span>
          </div>
        </div>
      )}

      <div className="flex-1 flex overflow-hidden">
        {loading ? (
          <div className="flex-1 flex items-center justify-center">
            <div className="text-tokyo-comment font-mono">Loading config...</div>
          </div>
        ) : (
          <>
            {/* Left: YAML Editor */}
            <div className="flex-1 border-r border-tokyo-blue7 p-4">
              <h2 className="text-sm font-mono font-semibold text-tokyo-cyan mb-2">Raw Configuration</h2>
              <div className="h-[calc(100%-32px)]">
                <Editor
                  height="100%"
                  defaultLanguage="yaml"
                  value={config}
                  onChange={handleYamlChange}
                  theme="vs-dark"
                  options={{
                    minimap: { enabled: false },
                    fontSize: 13,
                    fontFamily: 'JetBrains Mono, Fira Code, Monaco, monospace',
                    lineNumbers: 'on',
                    scrollBeyondLastLine: false,
                    wordWrap: 'on',
                    automaticLayout: true,
                  }}
                />
              </div>
            </div>

            {/* Right: Form Sections */}
            <div className="w-96 overflow-y-auto p-4 bg-tokyo-bg-dark">
              <h2 className="text-sm font-mono font-semibold text-tokyo-cyan mb-4">Quick Settings</h2>

              {/* AI Provider Section */}
              <div className="mb-4">
                <button
                  onClick={() => toggleSection('ai')}
                  className="w-full flex items-center justify-between p-2 bg-tokyo-dark1 border border-tokyo-blue7 rounded font-mono text-sm text-tokyo-blue hover:bg-tokyo-dark2"
                >
                  <span>AI Provider</span>
                  {expandedSections.ai ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                </button>
                {expandedSections.ai && (
                  <div className="mt-2 space-y-3 p-3 bg-tokyo-dark1 border border-tokyo-blue7 rounded">
                    <div>
                      <label className="block text-xs text-tokyo-comment font-mono mb-1">Provider</label>
                      <select
                        value={configObj.ai_provider || 'openai'}
                        onChange={(e) => updateConfig({ ai_provider: e.target.value })}
                        className="w-full bg-tokyo-bg border border-tokyo-blue7 text-tokyo-fg font-mono text-sm p-2 rounded"
                      >
                        <option value="openai">OpenAI</option>
                        <option value="gemini">Google Gemini</option>
                        <option value="cloudflare">Cloudflare</option>
                        <option value="ollama">Ollama</option>
                      </select>
                    </div>
                    <div>
                      <label className="block text-xs text-tokyo-comment font-mono mb-1">Model</label>
                      <input
                        type="text"
                        value={configObj.ai_model || ''}
                        onChange={(e) => updateConfig({ ai_model: e.target.value })}
                        placeholder="gpt-4o-mini"
                        className="w-full bg-tokyo-bg border border-tokyo-blue7 text-tokyo-fg font-mono text-sm p-2 rounded"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-tokyo-comment font-mono mb-1">Base URL (optional)</label>
                      <input
                        type="text"
                        value={configObj.ai_base_url || ''}
                        onChange={(e) => updateConfig({ ai_base_url: e.target.value })}
                        placeholder="https://api.openai.com/v1"
                        className="w-full bg-tokyo-bg border border-tokyo-blue7 text-tokyo-fg font-mono text-sm p-2 rounded"
                      />
                    </div>
                  </div>
                )}
              </div>

              {/* CloudShip Integration Section */}
              <div className="mb-4">
                <button
                  onClick={() => toggleSection('cloudship')}
                  className="w-full flex items-center justify-between p-2 bg-tokyo-dark1 border border-tokyo-purple7 rounded font-mono text-sm text-tokyo-purple hover:bg-tokyo-dark2"
                >
                  <span>CloudShip Integration</span>
                  {expandedSections.cloudship ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                </button>
                {expandedSections.cloudship && (
                  <div className="mt-2 space-y-3 p-3 bg-tokyo-dark1 border border-tokyo-purple7 rounded">
                    <div className="flex items-center justify-between">
                      <label className="text-xs text-tokyo-comment font-mono">Enabled</label>
                      <input
                        type="checkbox"
                        checked={configObj.cloudship?.enabled || false}
                        onChange={(e) => updateCloudShipConfig({ enabled: e.target.checked })}
                        className="bg-tokyo-bg border border-tokyo-purple7"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-tokyo-comment font-mono mb-1">Registration Key</label>
                      <input
                        type="password"
                        value={configObj.cloudship?.registration_key || ''}
                        onChange={(e) => updateCloudShipConfig({ registration_key: e.target.value })}
                        placeholder="Enter CloudShip key"
                        className="w-full bg-tokyo-bg border border-tokyo-purple7 text-tokyo-fg font-mono text-sm p-2 rounded"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-tokyo-comment font-mono mb-1">Endpoint</label>
                      <input
                        type="text"
                        value={configObj.cloudship?.endpoint || ''}
                        onChange={(e) => updateCloudShipConfig({ endpoint: e.target.value })}
                        placeholder="lighthouse.cloudshipai.com:443"
                        className="w-full bg-tokyo-bg border border-tokyo-purple7 text-tokyo-fg font-mono text-sm p-2 rounded"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-tokyo-comment font-mono mb-1">Station ID (auto-generated)</label>
                      <input
                        type="text"
                        value={configObj.cloudship?.station_id || ''}
                        disabled
                        className="w-full bg-tokyo-dark2 border border-tokyo-purple7 text-tokyo-comment font-mono text-sm p-2 rounded"
                      />
                    </div>
                  </div>
                )}
              </div>

              {/* Server Ports Section */}
              <div className="mb-4">
                <button
                  onClick={() => toggleSection('ports')}
                  className="w-full flex items-center justify-between p-2 bg-tokyo-dark1 border border-tokyo-green7 rounded font-mono text-sm text-tokyo-green hover:bg-tokyo-dark2"
                >
                  <span>Server Ports</span>
                  {expandedSections.ports ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                </button>
                {expandedSections.ports && (
                  <div className="mt-2 space-y-3 p-3 bg-tokyo-dark1 border border-tokyo-green7 rounded">
                    <div>
                      <label className="block text-xs text-tokyo-comment font-mono mb-1">API Port</label>
                      <input
                        type="number"
                        value={configObj.api_port || 8585}
                        onChange={(e) => updateConfig({ api_port: parseInt(e.target.value) })}
                        className="w-full bg-tokyo-bg border border-tokyo-green7 text-tokyo-fg font-mono text-sm p-2 rounded"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-tokyo-comment font-mono mb-1">MCP Port</label>
                      <input
                        type="number"
                        value={configObj.mcp_port || 3000}
                        onChange={(e) => updateConfig({ mcp_port: parseInt(e.target.value) })}
                        className="w-full bg-tokyo-bg border border-tokyo-green7 text-tokyo-fg font-mono text-sm p-2 rounded"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-tokyo-comment font-mono mb-1">SSH Port</label>
                      <input
                        type="number"
                        value={configObj.ssh_port || 2222}
                        onChange={(e) => updateConfig({ ssh_port: parseInt(e.target.value) })}
                        className="w-full bg-tokyo-bg border border-tokyo-green7 text-tokyo-fg font-mono text-sm p-2 rounded"
                      />
                    </div>
                  </div>
                )}
              </div>

              {/* Other Settings Section */}
              <div className="mb-4">
                <button
                  onClick={() => toggleSection('other')}
                  className="w-full flex items-center justify-between p-2 bg-tokyo-dark1 border border-tokyo-orange7 rounded font-mono text-sm text-tokyo-orange hover:bg-tokyo-dark2"
                >
                  <span>Other Settings</span>
                  {expandedSections.other ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                </button>
                {expandedSections.other && (
                  <div className="mt-2 space-y-3 p-3 bg-tokyo-dark1 border border-tokyo-orange7 rounded">
                    <div>
                      <label className="block text-xs text-tokyo-comment font-mono mb-1">Admin Username</label>
                      <input
                        type="text"
                        value={configObj.admin_username || 'admin'}
                        onChange={(e) => updateConfig({ admin_username: e.target.value })}
                        className="w-full bg-tokyo-bg border border-tokyo-orange7 text-tokyo-fg font-mono text-sm p-2 rounded"
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <label className="text-xs text-tokyo-comment font-mono">Debug Mode</label>
                      <input
                        type="checkbox"
                        checked={configObj.debug || false}
                        onChange={(e) => updateConfig({ debug: e.target.checked })}
                        className="bg-tokyo-bg border border-tokyo-orange7"
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <label className="text-xs text-tokyo-comment font-mono">Telemetry</label>
                      <input
                        type="checkbox"
                        checked={configObj.telemetry_enabled !== false}
                        onChange={(e) => updateConfig({ telemetry_enabled: e.target.checked })}
                        className="bg-tokyo-bg border border-tokyo-orange7"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-tokyo-comment font-mono mb-1">Database URL</label>
                      <input
                        type="text"
                        value={configObj.database_url || ''}
                        onChange={(e) => updateConfig({ database_url: e.target.value })}
                        className="w-full bg-tokyo-bg border border-tokyo-orange7 text-tokyo-fg font-mono text-sm p-2 rounded"
                      />
                    </div>
                  </div>
                )}
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
};

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <EnvironmentProvider>
        <ReactFlowProvider>
          <BrowserRouter>
            <div className="min-h-screen bg-background">
              <Layout>
                <Routes>
                  <Route path="/" element={<AgentsCanvas />} />
                  <Route path="/agents" element={<AgentsCanvas />} />
                  <Route path="/agents/:env" element={<AgentsCanvas />} />
                  <Route path="/mcps" element={<MCPServersPage />} />
                  <Route path="/mcps/:env" element={<MCPServersPage />} />
                  <Route path="/mcp-directory" element={<MCPDirectoryPage />} />
                  <Route path="/runs" element={<RunsPage />} />
                  <Route path="/environments" element={<EnvironmentsPage />} />
                  <Route path="/bundles" element={<BundlesPage />} />
                  <Route path="/cloudship" element={<CloudShipPage />} />
                  <Route path="/live-demo" element={<LiveDemoPage />} />
                  <Route path="/settings" element={<SettingsPage />} />
                  <Route path="/agent-editor/:agentId" element={<AgentEditor />} />
                  <Route path="*" element={<AgentsCanvas />} />
                </Routes>
              </Layout>
            </div>
          </BrowserRouter>
        </ReactFlowProvider>
      </EnvironmentProvider>
    </QueryClientProvider>
  );
}

export default App;
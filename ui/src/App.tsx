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

import { Bot, Server, Layers, MessageSquare, Users, Package, CircleCheck, Globe, Database, Edit, Eye, ArrowLeft, Save, X, Play, Plus, Archive, Trash2, Settings, Link, Download, FileText, AlertTriangle, ChevronDown, ChevronRight, Rocket, Copy, BookOpen, Terminal, GitBranch, HelpCircle, Wrench, Shield, Sparkles, Zap, Target, Cloud } from 'lucide-react';
import yaml from 'js-yaml';
import { MCPDirectoryPage } from './components/pages/MCPDirectoryPage';
import { HelpModal } from './components/ui/HelpModal';
import { LiveDemoPage } from './components/pages/LiveDemoPage';
import { GettingStartedPage } from './components/pages/GettingStartedPage';
import { ReportsPage } from './components/pages/ReportsPage';
import { ReportDetailPage } from './components/pages/ReportDetailPage';
import Editor from '@monaco-editor/react';

import { agentsApi, mcpServersApi, environmentsApi, agentRunsApi, bundlesApi, syncApi } from './api/station';
import { apiClient } from './api/client';
import { getLayoutedNodes, layoutElements } from './utils/layoutUtils';
import CloudShipStatus from './components/CloudShipStatus';
import VersionStatus from './components/VersionStatus';
import { RunsPage as RunsPageComponent } from './components/runs/RunsPage';
import { SyncModal } from './components/sync/SyncModal';
import { AddServerModal } from './components/modals/AddServerModal';
import { BundleEnvironmentModal } from './components/modals/BundleEnvironmentModal';
import BuildImageModal from './components/modals/BuildImageModal';
import { InstallBundleModal } from './components/modals/InstallBundleModal';
import DeployModal from './components/modals/DeployModal';
import { CopyEnvironmentModal } from './components/modals/CopyEnvironmentModal';
import { JsonSchemaEditor } from './components/schema/JsonSchemaEditor';
import { HierarchicalAgentNode } from './components/nodes/HierarchicalAgentNode';
import { buildAgentHierarchyMap } from './utils/agentHierarchy';
import { AgentsCanvas as AgentsCanvasComponent } from './components/agents/AgentsCanvas';
import { AgentsLayout } from './components/agents/v2/AgentsLayout';
import { RunDetailsModal } from './components/modals/RunDetailsModal';
import type { AgentRunWithDetails } from './types/station';
import { Toast, useToast } from './components/ui/Toast';
import { ConfirmDialog } from './components/ui/ConfirmDialog';

import { EnvironmentContext, EnvironmentProvider } from './contexts/EnvironmentContext';

interface TocItem {
  id: string;
  label: string;
}

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
  agent: HierarchicalAgentNode,
  mcp: MCPNode,
  tool: ToolNode,
};

// Station Banner Component
const StationBanner = () => (
  <div className="flex items-center justify-center py-4">
    <img
      src="/station-logo.png"
      alt="Station Logo"
      className="h-36 w-auto object-contain"
    />
  </div>
);



const Layout = ({ children }: any) => {
  const environmentContext = React.useContext(EnvironmentContext);
  const location = useLocation();
  const navigate = useNavigate();

  // Extract environment from URL path since useParams doesn't work in Layout
  const pathParts = location.pathname.split('/').filter(Boolean);
  
  // Special handling for /agent/:id route (agent detail page)
  const isAgentDetailPage = pathParts[0] === 'agent' && pathParts.length === 2;
  
  // For agent detail pages, use environment from context; otherwise extract from URL
  let currentEnvironmentName = null;
  if (isAgentDetailPage) {
    // Get environment name from context for agent detail pages
    const selectedEnv = environmentContext?.environments?.find((env: any) => 
      env.id === environmentContext?.selectedEnvironment
    );
    currentEnvironmentName = selectedEnv?.name.toLowerCase() || null;
  } else {
    // Normal path: /agents/:env or /mcps/:env
    currentEnvironmentName = pathParts.length >= 2 ? pathParts[1] : null;
  }

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
    if (path.startsWith('/getting-started')) return 'getting-started';
    if (path.startsWith('/agents')) return 'agents';
    if (path.startsWith('/mcps')) return 'mcps';
    if (path.startsWith('/mcp-directory')) return 'mcp-directory';
    if (path.startsWith('/runs')) return 'runs';
    if (path.startsWith('/reports')) return 'reports';
    if (path.startsWith('/environments')) return 'environments';
    if (path.startsWith('/bundles')) return 'bundles';
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
    // Find the environment by name and update context
    const selectedEnv = environmentContext?.environments?.find(
      (env: any) => env.name.toLowerCase() === environmentName.toLowerCase()
    );
    
    if (selectedEnv) {
      environmentContext.setSelectedEnvironment(selectedEnv.id);
    }
    
    // Only Agents and MCP Servers pages use environment-based routing
    const pagesWithEnvironmentRouting = ['agents', 'mcps'];
    
    if (pagesWithEnvironmentRouting.includes(currentPage)) {
      if (environmentName === 'default') {
        // Navigate to default environment on current page
        navigate(`/${currentPage}/default`);
      } else {
        // Navigate to specific environment on current page
        navigate(`/${currentPage}/${environmentName}`);
      }
    }
    // For other pages (reports, runs, etc.), just stay on the same page
    // The environment context will update and components will filter data accordingly
  };

  const sidebarItems = [
    { id: 'agents', label: 'Agents', icon: Bot, path: currentEnvironmentName ? `/agents/${currentEnvironmentName}` : '/agents' },
    { id: 'runs', label: 'Runs', icon: MessageSquare, path: '/runs' },
    { id: 'mcps', label: 'MCP Servers', icon: Server, path: currentEnvironmentName ? `/mcps/${currentEnvironmentName}` : '/mcps' },
    { id: 'mcp-directory', label: 'MCP Directory', icon: Database, path: '/mcp-directory' },
    { id: 'reports', label: 'Reports', icon: FileText, path: '/reports' },
    { id: 'environments', label: 'Environments', icon: Users, path: '/environments' },
    { id: 'bundles', label: 'Bundles', icon: Package, path: '/bundles' },
    { id: 'settings', label: 'Settings', icon: Settings, path: '/settings' },
  ];

  return (
    <div className="flex h-screen bg-white">
      {/* Sidebar */}
      <div className="w-52 bg-white border-r border-gray-200 flex flex-col">
        {/* Header */}
        <div className="border-b border-gray-200">
          <StationBanner />
        </div>

        {/* Navigation */}
        <nav className="flex-1 p-3 bg-white mt-3">
          <ul className="space-y-1">
            {sidebarItems.map((item) => (
              <li key={item.id}>
                <button
                  onClick={() => navigate(item.path)}
                  className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors ${
                    currentPage === item.id
                      ? 'bg-blue-50 text-station-blue font-medium'
                      : 'text-gray-700 hover:bg-gray-100 hover:text-gray-900'
                  }`}
                >
                  <item.icon className="h-4 w-4 flex-shrink-0" />
                  <span className="truncate">{item.label}</span>
                </button>
              </li>
            ))}
          </ul>
        </nav>

        {/* Footer */}
        <div className="p-3 border-t border-gray-200 bg-white space-y-2">
          <VersionStatus />
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

// Wrapper for modular AgentsCanvas component
const LegacyAgentsCanvas = () => {
  const environmentContext = React.useContext(EnvironmentContext);
  return <AgentsCanvasComponent environmentContext={environmentContext} />;
};

const AgentsPage = () => {
  return <AgentsLayout />;
};

// Old implementation (kept for reference, can be removed later)
const AgentsCanvasOld = () => {
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

      // Fetch agent prompt to extract agent tools from YAML frontmatter
      let agentToolsFromPrompt: string[] = [];
      try {
        const promptResponse = await agentsApi.getPrompt(agentId);
        const promptContent = promptResponse.data.content;
        
        // Parse YAML frontmatter to extract tools list
        const yamlMatch = promptContent.match(/^---\n([\s\S]*?)\n---/);
        if (yamlMatch) {
          const yamlContent = yamlMatch[1];
          const toolsMatch = yamlContent.match(/tools:\s*\n((?:\s*-\s*"[^"]+"\s*\n)+)/);
          if (toolsMatch) {
            agentToolsFromPrompt = toolsMatch[1]
              .split('\n')
              .filter(line => line.trim().startsWith('-'))
              .map(line => line.trim().replace(/^-\s*"/, '').replace(/"$/, ''))
              .filter(name => name.length > 0);
          }
        }
      } catch (error) {
        console.warn('Could not parse agent tools from prompt:', error);
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

      // Agent node (will be positioned by layout)
      newNodes.push({
        id: `agent-${agent.id}`,
        type: 'agent',
        position: { x: 0, y: 0 }, // ELK will position this
        data: {
          agent: agent,
          hierarchyInfo: hierarchyInfo,
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
              position: { x: 0, y: 0 }, // ELK will position this
              data: {
                agent: childAgent,
                hierarchyInfo: hierarchyMap.get(childAgent.id),
                onOpenModal: openAgentModal,
                onEditAgent: editAgent,
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
    <div className="fixed inset-0 bg-black/30 flex items-center justify-center z-50 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white border border-gray-200 rounded-xl shadow-lg w-full max-w-4xl mx-4 max-h-[90vh] flex flex-col animate-in zoom-in-95 fade-in slide-in-from-bottom-4 duration-300">
        <div className="flex items-center justify-between p-6 border-b border-gray-200">
          <div className="flex items-center gap-3">
            <Database className="h-6 w-6 text-primary" />
            <h2 className="text-xl font-semibold text-gray-900">
              MCP Server Details: {serverDetails?.name}
            </h2>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-100 rounded transition-colors"
          >
            <X className="h-5 w-5 text-gray-500 hover:text-gray-900" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-6 bg-gray-50">
          {serverDetails && (
            <div className="space-y-6">
              {/* Server Configuration */}
              <div className="bg-white border border-gray-200 rounded-lg p-5">
                <h3 className="text-lg font-semibold text-gray-900 mb-4 flex items-center gap-2">
                  <Database className="h-5 w-5 text-primary" />
                  Configuration
                </h3>
                <div className="grid gap-3">
                  <div className="flex flex-col gap-1 p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Command:</span>
                    <span className="text-gray-900 font-mono text-sm">{serverDetails.command}</span>
                  </div>
                  <div className="flex flex-col gap-1 p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Arguments:</span>
                    <span className="text-gray-900 font-mono text-sm break-all">{serverDetails.args ? serverDetails.args.join(' ') : 'None'}</span>
                  </div>
                  <div className="flex justify-between items-center p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Environment ID:</span>
                    <span className="text-gray-900 font-medium">{serverDetails.environment_id}</span>
                  </div>
                  <div className="flex justify-between items-center p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Created:</span>
                    <span className="text-gray-900 font-medium">{new Date(serverDetails.created_at).toLocaleString()}</span>
                  </div>
                  <div className="flex justify-between items-center p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Timeout:</span>
                    <span className="text-gray-900 font-medium">{serverDetails.timeout_seconds || 30}s</span>
                  </div>
                  <div className="flex justify-between items-center p-3 bg-gray-50 border border-gray-200 rounded-lg">
                    <span className="text-sm font-medium text-gray-600">Auto Restart:</span>
                    <span className="text-gray-900 font-medium">{serverDetails.auto_restart ? 'Yes' : 'No'}</span>
                  </div>
                </div>
              </div>

              {/* Available Tools */}
              <div className="bg-white border border-gray-200 rounded-lg p-5">
                <h3 className="text-lg font-semibold text-gray-900 mb-4">
                  Available Tools ({serverTools.length})
                </h3>
                {serverTools.length === 0 ? (
                  <div className="text-center p-8 bg-gray-50 border border-gray-200 rounded-lg">
                    <Database className="h-12 w-12 text-gray-400 mx-auto mb-3" />
                    <div className="text-gray-600">No tools found for this server</div>
                  </div>
                ) : (
                  <div className="grid gap-3">
                    {serverTools.map((tool, index) => (
                      <div key={tool.id || index} className="p-4 bg-gray-50 border border-gray-200 rounded-lg hover:border-gray-300 transition-colors">
                        <h4 className="font-semibold text-gray-900 mb-2">{tool.name}</h4>
                        <p className="text-sm text-gray-600 mb-2">{tool.description}</p>
                        {tool.input_schema && (
                          <div className="mt-3">
                            <div className="text-xs font-medium text-gray-600 mb-2">Input Schema:</div>
                            <pre className="text-xs bg-white p-3 rounded border border-gray-200 overflow-x-auto text-gray-900">
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
    </div>
  );
};

// MCP Servers Page
const MCPServersPage = () => {
  const { env } = useParams();
  const navigate = useNavigate();
  const [mcpServers, setMcpServers] = useState<any[]>([]);
  const [modalMCPServerId, setModalMCPServerId] = useState<number | null>(null);
  const [isMCPModalOpen, setIsMCPModalOpen] = useState(false);
  const [environments, setEnvironments] = useState<any[]>([]);
  const [currentEnvironment, setCurrentEnvironment] = useState<any>(null);
  const [isRawConfigModalOpen, setIsRawConfigModalOpen] = useState(false);
  const [rawConfig, setRawConfig] = useState('');
  const [rawConfigEnvironment, setRawConfigEnvironment] = useState('');
  const [syncEnvironmentName, setSyncEnvironmentName] = useState('');
  const [selectedServerId, setSelectedServerId] = useState<number | null>(null);
  const [isSyncModalOpen, setIsSyncModalOpen] = useState(false);
  const [isOpenAPIServer, setIsOpenAPIServer] = useState(false);
  const [openAPISpec, setOpenAPISpec] = useState('');
  const [activeTab, setActiveTab] = useState<'mcp' | 'openapi'>('mcp');
  const environmentContext = React.useContext(EnvironmentContext);
  const { toast, showToast, hideToast } = useToast();
  const [confirmDialog, setConfirmDialog] = useState<{
    isOpen: boolean;
    serverName: string;
    serverId: number;
  }>({ isOpen: false, serverName: '', serverId: 0 });
  const [isHelpModalOpen, setIsHelpModalOpen] = useState(false);

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
  const handleDeleteMCPServer = (serverId: number, serverName: string) => {
    setConfirmDialog({
      isOpen: true,
      serverName,
      serverId
    });
  };

  const confirmDeleteMCPServer = async () => {
    const { serverId, serverName } = confirmDialog;

    try {
      // Call the individual server delete endpoint which handles both DB and template cleanup
      const response = await apiClient.delete(`/mcp-servers/${serverId}`);

      // Refresh the servers list
      await fetchMCPServers();

      // Show success message with details if available
      if (response.data?.template_deleted) {
        showToast(`MCP server "${serverName}" deleted successfully from both database and template files.`, 'success');
      } else {
        showToast(`MCP server "${serverName}" deleted successfully from database. ${response.data?.template_cleanup_note || ''}`, 'success');
      }
    } catch (error) {
      console.error('Failed to delete MCP server:', error);
      showToast('Failed to delete MCP server. Check console for details.', 'error');
    }
  };

  // Function to view and edit raw config for a specific server
  const handleViewRawServerConfig = async (serverId: number, serverName: string) => {
    try {
      // Fetch the server config using the MCP servers endpoint
      const response = await apiClient.get(`/mcp-servers/${serverId}/config`);

      if (response.data && response.data.config) {
        // Check if this is an OpenAPI-based MCP server
        const configObj = JSON.parse(response.data.config);

        // Check if the config has mcpServers (template format) or direct command (runtime format)
        let isOpenAPI = false;
        if (configObj.mcpServers) {
          // Template format - check any server in mcpServers
          for (const serverConfig of Object.values(configObj.mcpServers)) {
            const serverObj = serverConfig as any;
            if (serverObj.command === 'stn' &&
                serverObj.args?.[0] === 'openapi-runtime' &&
                serverObj.args?.[1] === '--spec') {
              isOpenAPI = true;
              break;
            }
          }
        } else if (configObj.command === 'stn' &&
                   configObj.args?.[0] === 'openapi-runtime' &&
                   configObj.args?.[1] === '--spec') {
          // Runtime format - direct command/args
          isOpenAPI = true;
        }

        // Format the MCP config
        let formattedMCPConfig = '';
        try {
          formattedMCPConfig = JSON.stringify(configObj, null, 2);
        } catch (parseError) {
          console.warn('Failed to parse config JSON:', parseError);
          formattedMCPConfig = response.data.config;
        }

        // If this is an OpenAPI server, also fetch the OpenAPI spec
        let formattedOpenAPISpec = '';
        if (isOpenAPI) {
          const server = mcpServers.find(s => s.id === serverId);
          if (server && server.environment_id) {
            const env = environments.find(e => e.id === server.environment_id);
            if (env) {
              // Extract spec name from server name (remove -openapi suffix if present)
              const specName = serverName.replace(/-openapi$/, '');

              try {
                const openapiResponse = await apiClient.get(`/openapi/specs/${env.name}/${specName}`);
                if (openapiResponse.data && openapiResponse.data.content) {
                  formattedOpenAPISpec = JSON.stringify(JSON.parse(openapiResponse.data.content), null, 2);
                }
              } catch (openapiError) {
                console.warn('Failed to fetch OpenAPI spec:', openapiError);
              }
            }
          }
        }

        // Set state for modal
        setRawConfig(formattedMCPConfig);
        setOpenAPISpec(formattedOpenAPISpec);
        setIsOpenAPIServer(isOpenAPI);
        setRawConfigEnvironment(`${serverName} (ID: ${serverId})`);
        setSelectedServerId(serverId);
        setActiveTab(isOpenAPI && formattedOpenAPISpec ? 'openapi' : 'mcp');

        const server = mcpServers.find(s => s.id === serverId);
        if (server && server.environment_id) {
          const env = environments.find(e => e.id === server.environment_id);
          if (env) {
            setSyncEnvironmentName(env.name);
          }
        }

        setIsRawConfigModalOpen(true);
      } else {
        showToast('Failed to fetch raw config', 'error');
      }
    } catch (error) {
      console.error('Failed to fetch raw config:', error);
      showToast('Failed to fetch raw config. Check console for details.', 'error');
    }
  };

  // Function to save raw config
  const handleSaveRawConfig = async () => {
    try {
      if (!selectedServerId) {
        showToast('No server selected', 'error');
        return;
      }

      // Determine what we're saving based on active tab
      const contentToSave = activeTab === 'openapi' ? openAPISpec : rawConfig;

      // Validate JSON before saving
      try {
        JSON.parse(contentToSave);
      } catch (jsonError) {
        showToast('Invalid JSON format. Please check your configuration.', 'error');
        return;
      }

      // Save OpenAPI spec if on the OpenAPI tab
      if (activeTab === 'openapi' && isOpenAPIServer) {
        // Extract server name from environment string (format: "server-name (ID: X)")
        const serverNameMatch = rawConfigEnvironment.match(/^(.+?)\s+\(ID:/);
        const serverName = serverNameMatch ? serverNameMatch[1] : null;

        if (!serverName || !syncEnvironmentName) {
          showToast('Failed to determine server name or environment', 'error');
          return;
        }

        // Remove -openapi suffix if present to get spec name
        const specName = serverName.replace(/-openapi$/, '');

        // Update OpenAPI spec
        const response = await apiClient.put(`/openapi/specs/${syncEnvironmentName}/${specName}`, {
          content: contentToSave
        });

        if (response.data && response.data.success) {
          await fetchMCPServers();
          setIsRawConfigModalOpen(false);
          setIsSyncModalOpen(true);
        } else {
          showToast('Failed to save OpenAPI spec', 'error');
        }
      } else {
        // Update regular MCP server config
        const response = await apiClient.put(`/mcp-servers/${selectedServerId}/config`, {
          config: rawConfig
        });

        if (response.data && response.data.message) {
          await fetchMCPServers();
          setIsRawConfigModalOpen(false);
          setIsSyncModalOpen(true);
        } else {
          showToast('Failed to save server config', 'error');
        }
      }
    } catch (error) {
      console.error('Failed to save config:', error);
      showToast('Failed to save config. Check console for details.', 'error');
    }
  };

  // Fetch environments data
  useEffect(() => {
    const fetchEnvironments = async () => {
      try {
        const response = await environmentsApi.getAll();
        const environmentsData = response.data.environments || [];
        setEnvironments(Array.isArray(environmentsData) ? environmentsData : []);
        
        // Set current environment from URL or default to first
        if (env) {
          const selectedEnv = environmentsData.find((e: any) => e.name === env);
          if (selectedEnv) {
            setCurrentEnvironment(selectedEnv);
          }
        } else if (environmentsData.length > 0) {
          setCurrentEnvironment(environmentsData[0]);
          navigate(`/mcps/${environmentsData[0].name}`, { replace: true });
        }
      } catch (error) {
        console.error('Failed to fetch environments:', error);
        setEnvironments([]);
      }
    };
    fetchEnvironments();
  }, [env, navigate]);

  // Define fetchMCPServers function
  const fetchMCPServers = useCallback(async () => {
    try {
      if (!currentEnvironment?.id) {
        setMcpServers([]);
        return;
      }

      const response = await mcpServersApi.getByEnvironment(currentEnvironment.id);
      setMcpServers(response.data.servers || []);
    } catch (error) {
      console.error('Failed to fetch MCP servers:', error);
      setMcpServers([]);
    }
  }, [currentEnvironment?.id]);

  // Fetch MCP servers data when environment changes
  useEffect(() => {
    fetchMCPServers();
  }, [fetchMCPServers]);

  // MCP servers are already filtered by environment on the backend
  const filteredServers = mcpServers || [];

  const handleEnvironmentChange = (environment: any) => {
    setCurrentEnvironment(environment);
    navigate(`/mcps/${environment.name}`);
  };

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
        <div className="flex items-center gap-4">
          <h1 className="text-xl font-mono font-semibold text-tokyo-cyan">
            MCP Servers
            {currentEnvironment && (
              <>
                <span className="text-tokyo-comment mx-2">in</span>
                <span className="text-tokyo-fg">{currentEnvironment.name}</span>
              </>
            )}
          </h1>
          {environments.length > 0 && currentEnvironment && (
            <select
              value={currentEnvironment.id || ''}
              onChange={(e) => {
                const env = environments.find(env => env.id === parseInt(e.target.value));
                if (env) handleEnvironmentChange(env);
              }}
              className="px-3 py-1.5 text-sm border border-gray-300 rounded-md bg-white text-gray-900 focus:outline-none focus:ring-2 focus:ring-cyan-500"
            >
              {environments.map((env) => (
                <option key={env.id} value={env.id}>
                  {env.name}
                </option>
              ))}
            </select>
          )}
        </div>
        <button
          onClick={() => setIsHelpModalOpen(true)}
          className="flex items-center gap-2 px-3 py-2 text-sm font-medium text-tokyo-cyan bg-tokyo-bg hover:bg-tokyo-dark2 border border-tokyo-blue7 rounded-md transition-all"
          title="Learn about MCP servers"
        >
          <HelpCircle className="h-4 w-4" />
          <span className="hidden sm:inline">Help</span>
        </button>
      </div>
      <div className="flex-1 p-4 overflow-y-auto">
        {filteredServers.length === 0 ? (
          <div className="h-full flex items-center justify-center">
            <div className="text-center">
              <Database className="h-16 w-16 text-tokyo-comment mx-auto mb-4" />
              <div className="text-tokyo-fg font-mono text-lg mb-2">No MCP servers found</div>
              <div className="text-tokyo-comment font-mono text-sm">
                {currentEnvironment
                  ? `No MCP servers in ${currentEnvironment.name} environment`
                  : 'Select an environment to view MCP servers'
                }
              </div>
            </div>
          </div>
        ) : (
          <div className="grid gap-4 max-h-full overflow-y-auto">
            {filteredServers.map((server) => (
              <div key={server.id} className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo relative group">
                {/* Action buttons - always visible */}
                <div className="absolute top-3 right-3 flex gap-1.5">
                  <button
                    onClick={() => openMCPServerModal(server.id)}
                    className="p-2 rounded-lg bg-blue-600 hover:bg-blue-700 text-white shadow-sm transition-all hover:shadow-md"
                    title="View server details"
                  >
                    <Eye className="h-4 w-4" strokeWidth={2} />
                  </button>
                  <button
                    onClick={() => handleViewRawServerConfig(server.id, server.name)}
                    className="p-2 rounded-lg bg-gray-600 hover:bg-gray-700 text-white shadow-sm transition-all hover:shadow-md"
                    title="Edit server config"
                  >
                    <Settings className="h-4 w-4" strokeWidth={2} />
                  </button>
                  <button
                    onClick={() => handleDeleteMCPServer(server.id, server.name)}
                    className="p-2 rounded-lg bg-red-600 hover:bg-red-700 text-white shadow-sm transition-all hover:shadow-md"
                    title="Delete MCP server"
                  >
                    <Trash2 className="h-4 w-4" strokeWidth={2} />
                  </button>
                </div>

                <h3 className="font-mono font-medium text-tokyo-cyan">{server.name}</h3>

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
        isOpenAPIServer={isOpenAPIServer}
        openAPISpec={openAPISpec}
        onOpenAPISpecChange={setOpenAPISpec}
        activeTab={activeTab}
        onTabChange={setActiveTab}
      />

      {/* Sync Modal */}
      <SyncModal
        isOpen={isSyncModalOpen}
        onClose={() => setIsSyncModalOpen(false)}
        environment={syncEnvironmentName}
        onSyncComplete={() => fetchMCPServers()}
      />

      {/* Confirm Dialog */}
      <ConfirmDialog
        isOpen={confirmDialog.isOpen}
        onClose={() => setConfirmDialog({ ...confirmDialog, isOpen: false })}
        onConfirm={confirmDeleteMCPServer}
        title="Delete MCP Server"
        message={`Are you sure you want to delete the MCP server "${confirmDialog.serverName}"? This will remove it from both the database and any template files, and clean up associated records.`}
        confirmText="Delete"
        cancelText="Cancel"
        type="danger"
      />

      {/* Toast Notification */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={hideToast}
        />
      )}

      {/* Help Modal */}
      <HelpModal
        isOpen={isHelpModalOpen}
        onClose={() => setIsHelpModalOpen(false)}
        title="Understanding MCP Servers"
        pageDescription={`This page displays all MCP (Model Context Protocol) servers in the ${currentEnvironment?.name || 'selected'} environment. MCP servers provide tools that your agents can use - like filesystem operations, database queries, HTTP requests, or security scanning. You can add new servers, view their available tools, and manage which servers are accessible to agents in this environment.`}
      >
        <div className="space-y-6">
          {/* MCP Protocol Diagram */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3">MCP Protocol Flow</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-6">
              <div className="grid grid-cols-3 gap-6">
                {/* Filesystem Server */}
                <div className="space-y-3">
                  <div className="bg-purple-600 rounded-lg p-3 text-center">
                    <Server className="h-6 w-6 text-white mx-auto mb-1" />
                    <div className="font-mono text-xs text-white">filesystem</div>
                  </div>
                  <div className="bg-gray-100 rounded p-2 space-y-1">
                    <div className="text-xs font-mono text-green-600">→ read_file</div>
                    <div className="text-xs font-mono text-green-600">→ write_file</div>
                    <div className="text-xs font-mono text-green-600">→ list_directory</div>
                  </div>
                </div>

                {/* Database Server */}
                <div className="space-y-3">
                  <div className="bg-blue-600 rounded-lg p-3 text-center">
                    <Database className="h-6 w-6 text-white mx-auto mb-1" />
                    <div className="font-mono text-xs text-white">postgres</div>
                  </div>
                  <div className="bg-gray-100 rounded p-2 space-y-1">
                    <div className="text-xs font-mono text-cyan-600">→ sql_query</div>
                    <div className="text-xs font-mono text-cyan-600">→ list_tables</div>
                    <div className="text-xs font-mono text-cyan-600">→ describe_table</div>
                  </div>
                </div>

                {/* API Server */}
                <div className="space-y-3">
                  <div className="bg-orange-600 rounded-lg p-3 text-center">
                    <Globe className="h-6 w-6 text-white mx-auto mb-1" />
                    <div className="font-mono text-xs text-white">http</div>
                  </div>
                  <div className="bg-gray-100 rounded p-2 space-y-1">
                    <div className="text-xs font-mono text-orange-600">→ http_get</div>
                    <div className="text-xs font-mono text-orange-600">→ http_post</div>
                    <div className="text-xs font-mono text-orange-600">→ http_put</div>
                  </div>
                </div>
              </div>
              <div className="mt-4 text-xs text-gray-600 text-center font-mono">
                Each server exposes tools via Model Context Protocol
              </div>
            </div>
          </div>

          {/* Tool Lifecycle */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3">Tool Lifecycle</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-6">
              <div className="space-y-3">
                <div className="flex items-center gap-4">
                  <div className="w-8 h-8 rounded bg-purple-600 flex items-center justify-center font-mono text-sm text-white">1</div>
                  <div className="flex-1">
                    <div className="font-mono text-sm text-gray-900">Server connects to Station</div>
                    <div className="text-xs text-gray-600 mt-0.5 font-mono bg-white border border-gray-300 px-2 py-1 rounded inline-block">npx @modelcontextprotocol/server-filesystem</div>
                  </div>
                </div>

                <div className="flex items-center gap-4">
                  <div className="w-8 h-8 rounded bg-blue-600 flex items-center justify-center font-mono text-sm text-white">2</div>
                  <div className="flex-1">
                    <div className="font-mono text-sm text-gray-900">Tools discovered & registered</div>
                    <div className="text-xs text-gray-600 mt-0.5">Station queries available tools via MCP protocol</div>
                  </div>
                </div>

                <div className="flex items-center gap-4">
                  <div className="w-8 h-8 rounded bg-green-600 flex items-center justify-center font-mono text-sm text-white">3</div>
                  <div className="flex-1">
                    <div className="font-mono text-sm text-gray-900">Agent calls tool during execution</div>
                    <div className="text-xs text-gray-600 mt-0.5 font-mono bg-white border border-gray-300 px-2 py-1 rounded inline-block">filesystem.read_file("/data/config.json")</div>
                  </div>
                </div>

                <div className="flex items-center gap-4">
                  <div className="w-8 h-8 rounded bg-orange-600 flex items-center justify-center font-mono text-sm text-white">4</div>
                  <div className="flex-1">
                    <div className="font-mono text-sm text-gray-900">Server executes & returns result</div>
                    <div className="text-xs text-gray-600 mt-0.5">Secure execution with proper permissions</div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Environment Isolation */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3">Environment Isolation</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-6">
              <div className="grid grid-cols-3 gap-4">
                <div className="bg-white rounded-lg p-4 border-l-4 border-blue-500">
                  <div className="font-mono text-sm text-blue-600 mb-2">dev</div>
                  <div className="space-y-1.5">
                    <div className="flex items-center gap-2">
                      <Server className="h-3 w-3 text-gray-600" />
                      <div className="text-xs text-gray-700 font-mono">filesystem</div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Server className="h-3 w-3 text-gray-600" />
                      <div className="text-xs text-gray-700 font-mono">postgres-dev</div>
                    </div>
                  </div>
                </div>

                <div className="bg-white rounded-lg p-4 border-l-4 border-yellow-500">
                  <div className="font-mono text-sm text-yellow-600 mb-2">staging</div>
                  <div className="space-y-1.5">
                    <div className="flex items-center gap-2">
                      <Server className="h-3 w-3 text-gray-600" />
                      <div className="text-xs text-gray-700 font-mono">postgres-staging</div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Server className="h-3 w-3 text-gray-600" />
                      <div className="text-xs text-gray-700 font-mono">api-staging</div>
                    </div>
                  </div>
                </div>

                <div className="bg-white rounded-lg p-4 border-l-4 border-red-500">
                  <div className="font-mono text-sm text-red-600 mb-2">production</div>
                  <div className="space-y-1.5">
                    <div className="flex items-center gap-2">
                      <Server className="h-3 w-3 text-gray-600" />
                      <div className="text-xs text-gray-700 font-mono">postgres-prod</div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Shield className="h-3 w-3 text-red-600" />
                      <div className="text-xs text-gray-700 font-mono">restricted access</div>
                    </div>
                  </div>
                </div>
              </div>
              <div className="mt-4 text-xs text-gray-600 font-mono">
                Agents in "dev" cannot access "production" servers
              </div>
            </div>
          </div>

          {/* Common Servers */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3">Common MCP Servers</h3>
            <div className="grid grid-cols-2 gap-3">
              <div className="bg-white rounded border border-gray-200 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Server className="h-4 w-4 text-purple-600" />
                  <div className="font-mono text-sm text-gray-900">@modelcontextprotocol/server-filesystem</div>
                </div>
                <div className="text-xs text-gray-600">File and directory operations</div>
              </div>

              <div className="bg-white rounded border border-gray-200 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Database className="h-4 w-4 text-blue-600" />
                  <div className="font-mono text-sm text-gray-900">@modelcontextprotocol/server-postgres</div>
                </div>
                <div className="text-xs text-gray-600">SQL database queries</div>
              </div>

              <div className="bg-white rounded border border-gray-200 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Globe className="h-4 w-4 text-orange-600" />
                  <div className="font-mono text-sm text-gray-900">@modelcontextprotocol/server-fetch</div>
                </div>
                <div className="text-xs text-gray-600">HTTP requests and APIs</div>
              </div>

              <div className="bg-white rounded border border-gray-200 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Terminal className="h-4 w-4 text-green-600" />
                  <div className="font-mono text-sm text-gray-900">ship mcp security</div>
                </div>
                <div className="text-xs text-gray-600">Security scanning tools</div>
              </div>
            </div>
          </div>
        </div>
      </HelpModal>
    </div>
  );
};

// RunDetailsModal is now imported from ./components/modals/RunDetailsModal.tsx

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

// Reports Page Wrapper
const ReportsPageWrapper = () => {
  const environmentContext = React.useContext(EnvironmentContext);
  return <ReportsPage environmentContext={environmentContext} />;
};

// Report Detail Page Wrapper
const ReportDetailPageWrapper = () => {
  return <ReportDetailPage />;
};

// Environment-specific Node Components
const EnvironmentNode = ({ data }: NodeProps) => {
  const handleDeleteClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onDeleteEnvironment && data.environmentId) {
      data.onDeleteEnvironment(data.environmentId, data.label);
    }
  };

  const handleCopyClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onCopyEnvironment && data.environmentId) {
      data.onCopyEnvironment(data.environmentId, data.label);
    }
  };

  return (
    <div className="w-[320px] h-[160px] px-4 py-3 shadow-lg border border-tokyo-orange rounded-lg relative bg-tokyo-dark2 group">
      <Handle type="source" position={Position.Right} style={{ background: '#ff9e64', width: 12, height: 12 }} />
      <Handle type="source" position={Position.Bottom} style={{ background: '#7dcfff', width: 12, height: 12 }} />

      {/* Action buttons - appears on hover */}
      <div className="absolute top-2 right-2 flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
        {/* Copy button */}
        <button
          onClick={handleCopyClick}
          className="p-1 rounded bg-tokyo-blue hover:bg-blue-600 text-tokyo-bg"
          title={`Copy environment "${data.label}"`}
        >
          <Copy className="h-3 w-3" />
        </button>

        {/* Delete button - only for non-default environments */}
        {data.label !== 'default' && (
          <button
            onClick={handleDeleteClick}
            className="p-1 rounded bg-tokyo-red hover:bg-red-600 text-tokyo-bg"
            title={`Delete environment "${data.label}"`}
          >
            <Trash2 className="h-3 w-3" />
          </button>
        )}
      </div>

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
  const { toast, showToast, hideToast } = useToast();
  const [confirmDeleteEnv, setConfirmDeleteEnv] = useState<{
    isOpen: boolean;
    environmentName: string;
    environmentId: number;
  }>({ isOpen: false, environmentName: '', environmentId: 0 });

  // Modal states
  const [isSyncModalOpen, setIsSyncModalOpen] = useState(false);
  const [syncEnvironmentName, setSyncEnvironmentName] = useState('');
  const [isAddServerModalOpen, setIsAddServerModalOpen] = useState(false);
  const [isBundleModalOpen, setIsBundleModalOpen] = useState(false);
  const [isBuildImageModalOpen, setIsBuildImageModalOpen] = useState(false);
  const [isInstallBundleModalOpen, setIsInstallBundleModalOpen] = useState(false);
  const [isVariablesModalOpen, setIsVariablesModalOpen] = useState(false);
  const [isDeployModalOpen, setIsDeployModalOpen] = useState(false);
  const [isCopyModalOpen, setIsCopyModalOpen] = useState(false);
  const [copySourceEnvId, setCopySourceEnvId] = useState<number | null>(null);
  const [copySourceEnvName, setCopySourceEnvName] = useState<string>('');
  const [isHelpModalOpen, setIsHelpModalOpen] = useState(false);
  const [cloudShipConnected, setCloudShipConnected] = useState(false);

  // Check CloudShip connection status
  useEffect(() => {
    const checkCloudShipStatus = async () => {
      try {
        const response = await fetch('/api/v1/cloudship/status');
        if (response.ok) {
          const data = await response.json();
          setCloudShipConnected(data.authenticated === true);
        }
      } catch (error) {
        setCloudShipConnected(false);
      }
    };
    checkCloudShipStatus();
    // Poll every 30 seconds
    const interval = setInterval(checkCloudShipStatus, 30000);
    return () => clearInterval(interval);
  }, []);

  // Define TOC items for help modal
  const envHelpTocItems: TocItem[] = [
    { id: 'multi-env-isolation', label: 'Multi-Environment' },
    { id: 'template-variables', label: 'Template Variables' },
    { id: 'environment-graph', label: 'Environment Graph' },
    { id: 'key-actions', label: 'Key Actions' },
    { id: 'use-cases', label: 'Use Cases' },
    { id: 'best-practices-env', label: 'Best Practices' }
  ];

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

  const handleCopyEnvironment = (environmentId: number, environmentName: string) => {
    setCopySourceEnvId(environmentId);
    setCopySourceEnvName(environmentName);
    setIsCopyModalOpen(true);
  };

  const handleCopyComplete = () => {
    // Simply close the modal - user will manually refresh or navigate
    // to see the copied environment
    setIsCopyModalOpen(false);
  };

  const handleRefreshGraph = () => {
    if (selectedEnvironment) {
      setRebuildingGraph(true);
      // This will trigger the useEffect that rebuilds the graph
    }
  };

  // Function to delete environment
  const handleDeleteEnvironment = (environmentId: number, environmentName: string) => {
    setConfirmDeleteEnv({
      isOpen: true,
      environmentName,
      environmentId
    });
  };

  const confirmDeleteEnvironment = async () => {
    const { environmentId, environmentName } = confirmDeleteEnv;

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

      showToast(`Environment "${environmentName}" deleted successfully`, 'success');
    } catch (error) {
      console.error('Failed to delete environment:', error);
      showToast('Failed to delete environment. Check console for details.', 'error');
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
            onCopyEnvironment: handleCopyEnvironment,
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
      <div className="w-96 flex flex-col bg-white border-l border-gray-200 overflow-y-auto">
        {/* Environment Selector */}
        <div className="p-6 border-b border-gray-200">
          <label className="block text-sm font-medium text-gray-700 mb-2">Environment</label>
          {environments.length > 0 && (
            <select
              value={selectedEnvironment || ''}
              onChange={(e) => setSelectedEnvironment(Number(e.target.value))}
              className="w-full bg-white border-2 border-gray-300 text-gray-900 font-semibold px-4 py-3 rounded-lg focus:outline-none focus:border-station-blue focus:ring-1 focus:ring-station-blue text-lg shadow-sm"
            >
              {environments.map((env) => (
                <option key={env.id} value={env.id}>
                  {env.name}
                </option>
              ))}
            </select>
          )}
        </div>

        {/* Action Buttons */}
        {selectedEnvironment && (
          <div className="p-6 space-y-3">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-medium text-gray-700">Actions</h3>
              <button
                onClick={() => setIsHelpModalOpen(true)}
                className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-gray-600 bg-white hover:bg-gray-50 border border-gray-300 rounded-md transition-all shadow-sm"
              >
                <HelpCircle className="h-3.5 w-3.5" />
                Help
              </button>
            </div>

            <button
              onClick={handleSyncEnvironment}
              className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-white text-gray-700 hover:bg-gray-50 border border-gray-300 rounded text-sm font-medium transition-colors shadow-sm"
            >
              <Play className="h-4 w-4" />
              <span>Sync Environment</span>
            </button>

            <button
              onClick={handleVariables}
              className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-white text-gray-700 hover:bg-gray-50 border border-gray-300 rounded text-sm font-medium transition-colors shadow-sm"
            >
              <FileText className="h-4 w-4" />
              <span>Edit Variables</span>
            </button>

            <button
              onClick={handleAddServer}
              className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-white text-gray-700 hover:bg-gray-50 border border-gray-300 rounded text-sm font-medium transition-colors shadow-sm"
            >
              <Plus className="h-4 w-4" />
              <span>Add MCP Server</span>
            </button>

            <div className="border-t border-gray-200 pt-3 mt-3">
              <h3 className="text-sm font-medium text-gray-700 mb-3">Bundle</h3>

              <button
                onClick={handleBundleEnvironment}
                className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-white text-gray-700 hover:bg-gray-50 border border-gray-300 rounded text-sm font-medium transition-colors shadow-sm"
              >
                <Archive className="h-4 w-4" />
                <span>Publish Bundle</span>
              </button>
            </div>
          </div>
        )}

        {/* Install Bundle (always visible) */}
        <div className="p-6 border-t border-gray-200 mt-auto">
          <button
            onClick={handleInstallBundle}
            className="w-full flex items-center justify-center space-x-2 px-4 py-3 bg-pink-600 text-white hover:bg-pink-700 rounded text-sm font-medium transition-colors shadow-sm"
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
        environment={syncEnvironmentName || (selectedEnvironment ? environments.find(env => env.id === selectedEnvironment)?.name || 'default' : 'default')}
      />

      {/* Add Server Modal (includes OpenAPI tab) */}
      <AddServerModal
        isOpen={isAddServerModalOpen}
        onClose={() => setIsAddServerModalOpen(false)}
        environmentName={selectedEnvironment ? environments.find(env => env.id === selectedEnvironment)?.name || 'default' : 'default'}
        onSuccess={() => {
          setIsAddServerModalOpen(false);
          setIsSyncModalOpen(true);
        }}
      />

      {/* Bundle Environment Modal */}
      <BundleEnvironmentModal
        isOpen={isBundleModalOpen}
        onClose={() => setIsBundleModalOpen(false)}
        environmentName={selectedEnvironment ? environments.find(env => env.id === selectedEnvironment)?.name || 'default' : 'default'}
        cloudShipConnected={cloudShipConnected}
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
        onSuccess={async (environmentName: string) => {
          // Refresh environments list after successful installation
          const response = await environmentsApi.getAll();
          const envs = response.data.environments || [];
          setEnvironments(envs);

          // Close install modal and trigger sync modal (auto-sync feature)
          setIsInstallBundleModalOpen(false);
          setSyncEnvironmentName(environmentName);
          setIsSyncModalOpen(true);
        }}
        cloudShipConnected={cloudShipConnected}
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

      {/* Copy Environment Modal */}
      {copySourceEnvId && (
        <CopyEnvironmentModal
          isOpen={isCopyModalOpen}
          onClose={() => setIsCopyModalOpen(false)}
          sourceEnvironmentId={copySourceEnvId}
          sourceEnvironmentName={copySourceEnvName}
          environments={environments}
          onCopyComplete={handleCopyComplete}
        />
      )}

      {/* Confirm Dialog */}
      <ConfirmDialog
        isOpen={confirmDeleteEnv.isOpen}
        onClose={() => setConfirmDeleteEnv({ ...confirmDeleteEnv, isOpen: false })}
        onConfirm={confirmDeleteEnvironment}
        title="Delete Environment"
        message={`Are you sure you want to delete the environment "${confirmDeleteEnv.environmentName}"? This will remove all associated agents, MCP servers, and file-based configurations. This action cannot be undone.`}
        confirmText="Delete"
        cancelText="Cancel"
        type="danger"
      />

      {/* Help Modal */}
      <HelpModal
        isOpen={isHelpModalOpen}
        onClose={() => setIsHelpModalOpen(false)}
        title="Environments"
        pageDescription="Isolate agents, MCP servers, and tools across deployment contexts (dev, staging, prod) with environment-specific configurations, template variables, and separate execution spaces."
        tocItems={envHelpTocItems}
      >
        <div className="space-y-6">
          {/* What are Environments */}
          <div id="multi-env-isolation">
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Globe className="h-5 w-5 text-blue-600" />
              Multi-Environment Isolation
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="grid grid-cols-3 gap-3">
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Development</div>
                  <div className="text-xs text-gray-600">Local testing with PROJECT_ROOT=/home/user/dev</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Staging</div>
                  <div className="text-xs text-gray-600">Pre-production with PROJECT_ROOT=/var/staging</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Production</div>
                  <div className="text-xs text-gray-600">Live deployment with PROJECT_ROOT=/app/prod</div>
                </div>
              </div>
              <div className="text-sm text-gray-700 leading-relaxed">
                Each environment has isolated agents, MCP servers, and tools. Agents in <span className="font-mono bg-gray-100 px-1.5 py-0.5 rounded">dev</span> cannot access tools from <span className="font-mono bg-gray-100 px-1.5 py-0.5 rounded">prod</span>, ensuring clean separation.
              </div>
            </div>
          </div>

          {/* Template Variables */}
          <div id="template-variables">
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <FileText className="h-5 w-5 text-cyan-600" />
              Template Variables
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="text-sm text-gray-700 leading-relaxed">
                Template variables use Go template syntax to inject environment-specific values into MCP server configurations:
              </div>
              <div className="bg-white border border-gray-200 rounded p-3 font-mono text-xs">
                <div className="text-gray-500 mb-2">variables.yml</div>
                <div className="text-gray-900">PROJECT_ROOT: /home/user/myapp</div>
                <div className="text-gray-900">API_ENDPOINT: https://api.staging.example.com</div>
                <div className="text-gray-900 mt-3 mb-1 text-gray-500">template.json</div>
                <div className="text-gray-900">"args": ["npx", "-y", "@modelcontextprotocol/server-filesystem", {'"{{ .PROJECT_ROOT }}"'}]</div>
              </div>
              <div className="text-sm text-gray-700 leading-relaxed">
                Variables are resolved at sync time, allowing the same template to work across dev/staging/prod with different paths and URLs.
              </div>
            </div>
          </div>

          {/* Environment Graph */}
          <div id="environment-graph">
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <GitBranch className="h-5 w-5 text-purple-600" />
              Environment Graph
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="text-sm text-gray-700 leading-relaxed">
                The graph visualizes the relationships in your environment:
              </div>
              <div className="grid grid-cols-3 gap-3">
                <div className="bg-blue-50 border border-blue-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Environment Node</div>
                  <div className="text-xs text-gray-600">Central hub connecting agents and MCP servers</div>
                </div>
                <div className="bg-purple-50 border border-purple-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Agent Nodes</div>
                  <div className="text-xs text-gray-600">AI agents with their execution status</div>
                </div>
                <div className="bg-cyan-50 border border-cyan-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">MCP Server Nodes</div>
                  <div className="text-xs text-gray-600">Connected tool servers and their capabilities</div>
                </div>
              </div>
            </div>
          </div>

          {/* Key Actions */}
          <div id="key-actions">
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Zap className="h-5 w-5 text-yellow-600" />
              Key Actions
            </h3>
            <div className="space-y-2">
              <div className="bg-gray-50 border border-gray-200 rounded-lg p-3">
                <div className="flex items-start gap-3">
                  <Play className="h-4 w-4 text-blue-600 flex-shrink-0 mt-0.5" />
                  <div>
                    <div className="font-medium text-gray-900 text-sm">Sync Environment</div>
                    <div className="text-xs text-gray-600 mt-1">Resolves template variables and activates MCP servers. Run after changing variables.yml or template.json.</div>
                  </div>
                </div>
              </div>
              <div className="bg-gray-50 border border-gray-200 rounded-lg p-3">
                <div className="flex items-start gap-3">
                  <FileText className="h-4 w-4 text-cyan-600 flex-shrink-0 mt-0.5" />
                  <div>
                    <div className="font-medium text-gray-900 text-sm">Edit Variables</div>
                    <div className="text-xs text-gray-600 mt-1">Modify variables.yml to change environment-specific values like PROJECT_ROOT, API_ENDPOINT, DATABASE_URL.</div>
                  </div>
                </div>
              </div>
              <div className="bg-gray-50 border border-gray-200 rounded-lg p-3">
                <div className="flex items-start gap-3">
                  <Plus className="h-4 w-4 text-green-600 flex-shrink-0 mt-0.5" />
                  <div>
                    <div className="font-medium text-gray-900 text-sm">Add MCP Server</div>
                    <div className="text-xs text-gray-600 mt-1">Connect new tool servers (filesystem, cloud APIs, databases) to your environment.</div>
                  </div>
                </div>
              </div>
              <div className="bg-gray-50 border border-gray-200 rounded-lg p-3">
                <div className="flex items-start gap-3">
                  <Archive className="h-4 w-4 text-yellow-600 flex-shrink-0 mt-0.5" />
                  <div>
                    <div className="font-medium text-gray-900 text-sm">Create Bundle</div>
                    <div className="text-xs text-gray-600 mt-1">Package environment into shareable tar.gz with agents, MCP configs, and variables template.</div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Common Use Cases */}
          <div id="use-cases">
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Target className="h-5 w-5 text-green-600" />
              Common Use Cases
            </h3>
            <div className="space-y-2 text-sm text-gray-700">
              <div className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-green-600 flex-shrink-0 mt-1.5"></div>
                <div><span className="font-medium">Local Development:</span> Create <span className="font-mono bg-gray-100 px-1.5 py-0.5 rounded text-xs">dev</span> environment with PROJECT_ROOT pointing to local workspace</div>
              </div>
              <div className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-green-600 flex-shrink-0 mt-1.5"></div>
                <div><span className="font-medium">Team Collaboration:</span> Share bundles with teammates - they install and customize variables for their setup</div>
              </div>
              <div className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-green-600 flex-shrink-0 mt-1.5"></div>
                <div><span className="font-medium">CI/CD Pipelines:</span> Deploy production environment with Docker image containing agents and MCP configs</div>
              </div>
              <div className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-green-600 flex-shrink-0 mt-1.5"></div>
                <div><span className="font-medium">Tool Isolation:</span> Keep sensitive production tools (AWS, databases) separate from dev/staging environments</div>
              </div>
            </div>
          </div>

          {/* Best Practices */}
          <div id="best-practices-env" className="bg-blue-50 border border-blue-200 rounded-lg p-4">
            <div className="font-semibold text-gray-900 mb-2 flex items-center gap-2">
              <Shield className="h-4 w-4 text-blue-600" />
              Best Practices
            </div>
            <ul className="space-y-1.5 text-sm text-gray-700">
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-blue-600 flex-shrink-0 mt-1.5"></div>
                <div>Always run <span className="font-mono bg-white px-1.5 py-0.5 rounded text-xs">stn sync</span> after editing variables.yml or template.json</div>
              </li>
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-blue-600 flex-shrink-0 mt-1.5"></div>
                <div>Use descriptive variable names: PROJECT_ROOT, API_ENDPOINT, DATABASE_URL (not x, path, url)</div>
              </li>
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-blue-600 flex-shrink-0 mt-1.5"></div>
                <div>Never commit secrets to variables.yml - use environment variables for API keys and credentials</div>
              </li>
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-blue-600 flex-shrink-0 mt-1.5"></div>
                <div>Test bundles locally before sharing - verify template variables work with different values</div>
              </li>
            </ul>
          </div>
        </div>
      </HelpModal>

      {/* Toast Notification */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={hideToast}
        />
      )}

    </div>
  );
};

// Simple Bundles Page
const BundlesPage = () => {
  const [loading, setLoading] = useState(true);
  const [bundles, setBundles] = useState<any[]>([]);
  const [isHelpModalOpen, setIsHelpModalOpen] = useState(false);

  // Define TOC items for help modal
  const bundleHelpTocItems: TocItem[] = [
    { id: 'bundle-contents', label: 'Bundle Contents' },
    { id: 'creating-bundles', label: 'Creating Bundles' },
    { id: 'installing-bundles', label: 'Installing Bundles' },
    { id: 'gitops-workflow', label: 'GitOps Workflow' },
    { id: 'use-cases-bundles', label: 'Use Cases' },
    { id: 'best-practices-bundles', label: 'Best Practices' }
  ];

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
    <div className="h-full p-6 bg-gray-50 overflow-y-auto">
      <div className="max-w-6xl mx-auto">
        <div className="mb-8 flex items-start justify-between">
          <div className="flex-1">
            <h1 className="text-3xl font-bold text-gray-900 mb-2">Agent Bundles</h1>
            <p className="text-gray-600">Pre-configured agent templates for common workflows and integrations</p>
          </div>
          <button
            onClick={() => setIsHelpModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 border border-gray-300 rounded-md transition-all shadow-sm"
          >
            <HelpCircle className="h-4 w-4" />
            Help
          </button>
        </div>

        {bundles.length === 0 ? (
          <div className="space-y-6">
            {/* Empty State Message */}
            <div className="bg-white rounded-lg border border-gray-200 p-8 shadow-sm text-center">
              <Package className="h-12 w-12 text-gray-300 mx-auto mb-3" />
              <h2 className="text-xl font-semibold text-gray-900 mb-2">No bundles found</h2>
              <p className="text-sm text-gray-400 max-w-md mx-auto">
                Bundles are pre-configured collections of agents, MCP servers, and tools. Click "What are Bundles?" above to learn how to install or create them.
              </p>
            </div>

            {/* Skeleton Placeholders */}
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3 opacity-25">
              {[1, 2, 3].map((i) => (
                <div key={i} className="bg-white border border-gray-200 rounded-lg p-4">
                  <div className="flex items-center gap-2 mb-3">
                    <div className="h-5 w-5 bg-gray-200 rounded"></div>
                    <div className="h-5 w-40 bg-gray-200 rounded"></div>
                  </div>

                  <div className="space-y-2">
                    <div className="flex justify-between">
                      <div className="h-4 w-12 bg-gray-200 rounded"></div>
                      <div className="h-4 w-32 bg-gray-200 rounded"></div>
                    </div>
                    <div className="flex justify-between">
                      <div className="h-4 w-12 bg-gray-200 rounded"></div>
                      <div className="h-4 w-20 bg-gray-200 rounded"></div>
                    </div>
                    <div className="flex justify-between">
                      <div className="h-4 w-16 bg-gray-200 rounded"></div>
                      <div className="h-4 w-24 bg-gray-200 rounded"></div>
                    </div>

                    <div className="pt-2 border-t border-gray-200">
                      <div className="h-3 w-10 bg-gray-200 rounded mb-1"></div>
                      <div className="h-12 bg-gray-100 rounded"></div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
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

      {/* Help Modal */}
      <HelpModal
        isOpen={isHelpModalOpen}
        onClose={() => setIsHelpModalOpen(false)}
        title="Agent Bundles"
        pageDescription="Shareable packages with pre-configured agents, MCP servers, and templates. Enable GitOps workflows and distribute agent setups across teams and environments."
        tocItems={bundleHelpTocItems}
      >
        <div className="space-y-6">
          {/* What's in a Bundle */}
          <div id="bundle-contents">
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Package className="h-5 w-5 text-[#0084FF]" />
              Bundle Contents
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1 flex items-center gap-2">
                    <Sparkles className="h-4 w-4 text-gray-600" />
                    Agent Prompts
                  </div>
                  <div className="text-xs text-gray-600">Complete agent definitions with system prompts, tools, and configurations</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1 flex items-center gap-2">
                    <Server className="h-4 w-4 text-gray-600" />
                    MCP Servers
                  </div>
                  <div className="text-xs text-gray-600">Server definitions with connection settings and tool registrations</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1 flex items-center gap-2">
                    <FileText className="h-4 w-4 text-gray-600" />
                    Template Variables
                  </div>
                  <div className="text-xs text-gray-600">Environment-specific variables (PROJECT_ROOT, API_ENDPOINT, etc.)</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1 flex items-center gap-2">
                    <Wrench className="h-4 w-4 text-gray-600" />
                    Tool Assignments
                  </div>
                  <div className="text-xs text-gray-600">Pre-configured tool permissions and access controls</div>
                </div>
              </div>
            </div>
          </div>

          {/* Creating Bundles */}
          <div id="creating-bundles">
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Archive className="h-5 w-5 text-[#0084FF]" />
              Creating Bundles
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="text-sm text-gray-700 leading-relaxed">
                Bundles are created from existing environments and packaged as tar.gz files:
              </div>
              <div className="bg-gray-900 text-gray-100 p-3 rounded-md font-mono text-xs space-y-2">
                <div className="text-gray-400"># Create bundle from environment</div>
                <div>$ stn bundle create production</div>
                <div className="text-gray-400 mt-3"># Custom output path</div>
                <div>$ stn bundle create dev --output my-agents.tar.gz</div>
              </div>
              <div className="text-sm text-gray-700 leading-relaxed">
                The resulting tar.gz contains your environment's <span className="font-mono bg-gray-100 px-1.5 py-0.5 rounded text-xs">agents/</span>, <span className="font-mono bg-gray-100 px-1.5 py-0.5 rounded text-xs">template.json</span>, and <span className="font-mono bg-gray-100 px-1.5 py-0.5 rounded text-xs">variables.yml</span> ready for distribution.
              </div>
            </div>
          </div>

          {/* Installing Bundles */}
          <div id="installing-bundles">
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Download className="h-5 w-5 text-[#0084FF]" />
              Installing Bundles
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="text-sm text-gray-700 leading-relaxed">
                Install bundles from local files, URLs, or the Station registry:
              </div>
              <div className="bg-gray-900 text-gray-100 p-3 rounded-md font-mono text-xs space-y-2">
                <div className="text-gray-400"># From local file</div>
                <div>$ stn bundle install ./my-bundle.tar.gz new-env</div>
                <div className="text-gray-400 mt-3"># From URL</div>
                <div>$ stn bundle install https://example.com/bundle.tar.gz prod</div>
                <div className="text-gray-400 mt-3"># From Station registry</div>
                <div>$ stn template install security-scanner-bundle</div>
              </div>
              <div className="text-sm text-gray-700 leading-relaxed">
                After installation, you'll be prompted to configure template variables (PROJECT_ROOT, etc.) for your environment.
              </div>
            </div>
          </div>

          {/* Bundle Workflow */}
          <div id="gitops-workflow">
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <GitBranch className="h-5 w-5 text-[#0084FF]" />
              GitOps Workflow
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="space-y-2">
                <div className="flex items-start gap-3">
                  <div className="flex-shrink-0 w-6 h-6 rounded-full bg-blue-600 text-white text-xs flex items-center justify-center font-bold">1</div>
                  <div className="flex-1">
                    <div className="font-medium text-gray-900 text-sm">Develop Locally</div>
                    <div className="text-xs text-gray-600 mt-0.5">Create and test agents in your <span className="font-mono bg-gray-100 px-1 py-0.5 rounded">dev</span> environment</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="flex-shrink-0 w-6 h-6 rounded-full bg-green-600 text-white text-xs flex items-center justify-center font-bold">2</div>
                  <div className="flex-1">
                    <div className="font-medium text-gray-900 text-sm">Create Bundle</div>
                    <div className="text-xs text-gray-600 mt-0.5">Package environment into tar.gz with <span className="font-mono bg-gray-100 px-1 py-0.5 rounded text-xs">stn bundle create</span></div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="flex-shrink-0 w-6 h-6 rounded-full bg-purple-600 text-white text-xs flex items-center justify-center font-bold">3</div>
                  <div className="flex-1">
                    <div className="font-medium text-gray-900 text-sm">Version Control</div>
                    <div className="text-xs text-gray-600 mt-0.5">Commit bundle to Git or upload to registry for team distribution</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="flex-shrink-0 w-6 h-6 rounded-full bg-orange-600 text-white text-xs flex items-center justify-center font-bold">4</div>
                  <div className="flex-1">
                    <div className="font-medium text-gray-900 text-sm">Deploy Anywhere</div>
                    <div className="text-xs text-gray-600 mt-0.5">Team installs bundle and customizes variables for their environments</div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Common Use Cases */}
          <div id="use-cases-bundles">
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Target className="h-5 w-5 text-[#0084FF]" />
              Common Use Cases
            </h3>
            <div className="space-y-2 text-sm text-gray-700">
              <div className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-cyan-600 flex-shrink-0 mt-1.5"></div>
                <div><span className="font-medium">Team Onboarding:</span> New developers install team's agent bundle and customize for their workspace</div>
              </div>
              <div className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-cyan-600 flex-shrink-0 mt-1.5"></div>
                <div><span className="font-medium">CI/CD Pipelines:</span> Deploy security/testing agents to pipeline environments with production configs</div>
              </div>
              <div className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-cyan-600 flex-shrink-0 mt-1.5"></div>
                <div><span className="font-medium">Registry Publishing:</span> Share proven agent configurations with the community via Station registry</div>
              </div>
              <div className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-cyan-600 flex-shrink-0 mt-1.5"></div>
                <div><span className="font-medium">Multi-Environment Sync:</span> Keep dev/staging/prod environments in sync with identical agent setups</div>
              </div>
            </div>
          </div>

          {/* Best Practices */}
          <div id="best-practices-bundles" className="bg-blue-50 border border-blue-200 rounded-lg p-4">
            <div className="font-semibold text-gray-900 mb-2 flex items-center gap-2">
              <Shield className="h-4 w-4 text-blue-600" />
              Best Practices
            </div>
            <ul className="space-y-1.5 text-sm text-gray-700">
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-blue-600 flex-shrink-0 mt-1.5"></div>
                <div>Document required template variables in bundle README with example values</div>
              </li>
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-blue-600 flex-shrink-0 mt-1.5"></div>
                <div>Test bundles on clean environments before sharing with team or publishing</div>
              </li>
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-blue-600 flex-shrink-0 mt-1.5"></div>
                <div>Version your bundles (v1.0.0) and maintain changelog for breaking changes</div>
              </li>
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-blue-600 flex-shrink-0 mt-1.5"></div>
                <div>Never include secrets in bundles - use template variables for sensitive values</div>
              </li>
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-blue-600 flex-shrink-0 mt-1.5"></div>
                <div>Include example agents and clear documentation for bundle recipients</div>
              </li>
            </ul>
          </div>
        </div>
      </HelpModal>
      </div>
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
      <div className="min-h-screen bg-white flex items-center justify-center">
        <div className="text-gray-600">Loading agent configuration...</div>
      </div>
    );
  }

  if (error && !agentData) {
    return (
      <div className="min-h-screen bg-white flex items-center justify-center">
        <div className="text-red-600">{error}</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-white">
      {/* Header with breadcrumbs */}
      <div className="bg-white border-b border-gray-200 px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <button
              onClick={handleBack}
              className="flex items-center gap-2 text-gray-600 hover:text-gray-900 transition-colors"
            >
              <ArrowLeft className="h-4 w-4" />
              <span>Back to Agents</span>
            </button>
            <div className="text-gray-400">/</div>
            <h1 className="text-xl font-semibold text-gray-900">
              Edit Agent: {agentData?.name || 'Unknown'}
            </h1>
          </div>

          <div className="flex items-center gap-3">
            {saveSuccess && (
              <div className="text-sm text-green-600">Saved successfully!</div>
            )}
            {error && (
              <div className="text-sm text-red-600">{error}</div>
            )}
            <button
              onClick={handleSave}
              disabled={saving || !schemaValid}
              className="flex items-center gap-2 px-4 py-2 bg-white text-gray-700 hover:bg-gray-50 border border-gray-300 rounded-lg font-medium transition-colors disabled:opacity-50 shadow-sm"
            >
              <Save className="h-4 w-4" />
              {saving ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </div>

        {/* Agent description */}
        {agentData?.description && (
          <div className="mt-2 text-sm text-gray-600">
            {agentData.description}
          </div>
        )}
      </div>

      {/* Two-column editor layout */}
      <div className="flex-1 p-6">
        <div className="grid grid-cols-2 gap-4 h-[calc(100vh-200px)]">
          {/* Left column: YAML Editor */}
          <div className="bg-white rounded-lg border border-gray-200 flex flex-col shadow-sm">
            <div className="p-4 border-b border-gray-200">
              <h2 className="text-lg font-semibold text-gray-900">Agent Configuration</h2>
              <p className="text-sm text-gray-600 mt-1">
                Edit the agent's prompt file. After saving, run <code className="bg-gray-50 px-1 rounded text-gray-800 border border-gray-200">stn sync {agentData?.environment_name || 'environment'}</code> to apply.
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
          <div className="bg-white rounded-lg border border-gray-200 flex flex-col overflow-hidden shadow-sm">
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
  environmentName,
  isOpenAPIServer,
  openAPISpec,
  onOpenAPISpecChange,
  activeTab,
  onTabChange
}: {
  isOpen: boolean;
  onClose: () => void;
  config: string;
  onConfigChange: (config: string) => void;
  onSave: () => void;
  environmentName: string;
  isOpenAPIServer?: boolean;
  openAPISpec?: string;
  onOpenAPISpecChange?: (spec: string) => void;
  activeTab?: 'mcp' | 'openapi';
  onTabChange?: (tab: 'mcp' | 'openapi') => void;
}) => {
  if (!isOpen) return null;

  const currentTab = activeTab || 'mcp';
  const editorValue = currentTab === 'openapi' ? (openAPISpec || '') : config;
  const handleEditorChange = (value: string) => {
    if (currentTab === 'openapi' && onOpenAPISpecChange) {
      onOpenAPISpecChange(value);
    } else {
      onConfigChange(value);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/30 flex items-center justify-center z-50 backdrop-blur-sm animate-in fade-in duration-200">
      <div className="bg-white border border-gray-200 rounded-xl shadow-lg w-full max-w-6xl mx-4 max-h-[90vh] overflow-hidden flex flex-col animate-in zoom-in-95 fade-in slide-in-from-bottom-4 duration-300">
        <div className="flex items-center justify-between p-6 border-b border-gray-200">
          <h2 className="text-xl font-semibold text-gray-900">
            Server Config Editor - {environmentName}
          </h2>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-100 rounded transition-colors"
          >
            <X className="h-5 w-5 text-gray-500 hover:text-gray-900" />
          </button>
        </div>

        <div className="flex-1 flex flex-col overflow-hidden p-6 bg-gray-50">
          <div className="mb-4 p-4 bg-blue-50 border border-blue-200 rounded-lg">
            <p className="text-sm text-gray-700">
              Edit the configuration for this specific MCP server.
              Changes will be merged back into the environment's template.json file.
            </p>
          </div>

          {/* Tab Switcher - Only show if this is an OpenAPI server */}
          {isOpenAPIServer && onTabChange && (
            <div className="flex gap-2 mb-4 border-b border-gray-200 bg-white rounded-t-lg px-2">
              <button
                onClick={() => onTabChange('mcp')}
                className={`px-4 py-3 text-sm font-medium transition-colors border-b-2 ${
                  currentTab === 'mcp'
                    ? 'text-primary border-primary'
                    : 'border-transparent text-gray-600 hover:text-gray-900'
                }`}
              >
                MCP Config
              </button>
              <button
                onClick={() => onTabChange('openapi')}
                className={`px-4 py-3 text-sm font-medium transition-colors border-b-2 ${
                  currentTab === 'openapi'
                    ? 'text-primary border-primary'
                    : 'border-transparent text-gray-600 hover:text-gray-900'
                }`}
              >
                OpenAPI Spec
              </button>
            </div>
          )}

          {/* Monaco Editor */}
          <div className="flex-1 border border-gray-300 rounded-lg overflow-hidden min-h-[500px] shadow-sm bg-white">
            <Editor
              height="500px"
              defaultLanguage="json"
              value={editorValue}
              onChange={(value) => handleEditorChange(value || '')}
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
          <div className="flex justify-between items-center mt-6 pt-4 border-t border-gray-200 bg-white px-4 py-3 rounded-lg">
            <div className="text-gray-600 text-sm flex items-center gap-2">
              <span className="text-lg">💡</span>
              <span>Tip: Use Ctrl+Shift+F to format JSON</span>
            </div>
            <div className="flex gap-3">
              <button
                onClick={onClose}
                className="px-4 py-2 bg-gray-200 hover:bg-gray-300 text-gray-800 rounded font-medium text-sm transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={onSave}
                className="px-4 py-2 bg-green-600 hover:bg-green-700 text-white rounded font-medium text-sm transition-colors"
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
  const { toast, showToast, hideToast } = useToast();

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
      showToast(response.data.message || 'Variables saved successfully', 'success');
      onClose();
    } catch (error: any) {
      console.error('Failed to save variables:', error);
      showToast(error.response?.data?.error || 'Failed to save variables', 'error');
    } finally {
      setSaving(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div 
      className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-[9999]"
      onClick={onClose}
    >
      <div 
        className="bg-white border border-gray-200 rounded-lg shadow-xl max-w-4xl w-full mx-4 z-[10000] relative max-h-[90vh] overflow-hidden flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-gray-200 bg-white rounded-t-lg">
          <h2 className="text-lg font-semibold text-gray-900">
            Environment Variables - {environmentName}
          </h2>
          <button 
            onClick={onClose} 
            className="text-gray-500 hover:text-gray-900 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-4 overflow-y-auto flex-1 bg-white">
          {/* Warning Banner */}
          <div className="bg-blue-50 border border-blue-200 rounded p-4">
            <p className="text-sm text-gray-700">
              <strong>⚠️ Important:</strong> These variables are local to your machine and will NOT be included in bundles or Docker images. They are for local development only.
            </p>
          </div>

          <p className="text-gray-600 text-sm">
            Edit the variables.yml file for this environment. After saving, run 'stn sync' to apply changes to your MCP servers.
          </p>

          {loading ? (
            <div className="flex-1 flex items-center justify-center py-12">
              <p className="text-gray-500">Loading variables...</p>
            </div>
          ) : (
            <>
              {/* Monaco Editor */}
              <div className="border border-gray-200 rounded-lg overflow-hidden" style={{ height: '400px' }}>
                <Editor
                  height="400px"
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
            </>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-gray-200 bg-white">
          <div className="flex justify-between items-center mb-4">
            <div className="text-gray-500 text-sm">
              💡 After saving, run 'stn sync' to apply variable changes
            </div>
          </div>
          <div className="flex gap-3 justify-end">
            <button
              onClick={onClose}
              className="px-4 py-2 text-gray-700 hover:text-gray-900 transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-2 bg-cyan-600 text-white rounded font-medium hover:bg-cyan-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            >
              {saving ? 'Saving...' : 'Save Variables'}
            </button>
          </div>
        </div>
      </div>

      {/* Toast Notification */}
      {toast && (
        <Toast
          message={toast.message}
          type={toast.type}
          onClose={hideToast}
        />
      )}
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
  const [isHelpModalOpen, setIsHelpModalOpen] = useState(false);
  const [expandedSections, setExpandedSections] = useState({
    ai: true,
    cloudship: false,
    telemetry: false,
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

  const updateTelemetryConfig = (updates: any) => {
    const newTelemetry = { ...configObj.telemetry, ...updates };
    updateConfig({ telemetry: newTelemetry });
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
    <div className="h-full flex flex-col bg-gray-50">
      <div className="flex items-center justify-between p-4 border-b border-gray-200 bg-white">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">Settings</h1>
          {configPath && (
            <p className="text-xs text-gray-600 mt-1 font-mono">{configPath}</p>
          )}
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => setIsHelpModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2 text-sm font-medium text-gray-700 bg-white hover:bg-gray-50 border border-gray-300 rounded-md transition-all shadow-sm"
          >
            <HelpCircle className="h-4 w-4" />
            Help
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="px-4 py-2.5 bg-white text-gray-700 hover:bg-gray-50 border border-gray-300 rounded-md font-medium text-sm transition-all shadow-sm hover:shadow disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {saving ? 'Saving...' : 'Save Config'}
          </button>
        </div>
      </div>

      {/* Warning Banner */}
      <div className="bg-amber-50 border-b border-amber-200 p-3">
        <div className="flex items-center gap-2 text-amber-700 text-sm">
          <AlertTriangle className="h-4 w-4" />
          <span>Station needs to be restarted to apply configuration changes</span>
        </div>
      </div>

      {/* Success/Error Messages */}
      {success && (
        <div className="bg-green-50 border-b border-green-200 p-3">
          <div className="flex items-center gap-2 text-green-700 text-sm">
            <CircleCheck className="h-4 w-4" />
            <span>Config file saved successfully. Restart Station to apply changes.</span>
          </div>
        </div>
      )}

      {error && (
        <div className="bg-red-50 border-b border-red-200 p-3">
          <div className="flex items-center gap-2 text-red-700 text-sm">
            <AlertTriangle className="h-4 w-4" />
            <span>{error}</span>
          </div>
        </div>
      )}

      <div className="flex-1 flex overflow-hidden">
        {loading ? (
          <div className="flex-1 flex items-center justify-center">
            <div className="text-gray-600">Loading config...</div>
          </div>
        ) : (
          <>
            {/* Left: YAML Editor */}
            <div className="flex-1 border-r border-gray-200 p-4 bg-white">
              <h2 className="text-sm font-semibold text-gray-900 mb-2">Raw Configuration</h2>
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
            <div className="w-96 overflow-y-auto p-4 bg-white border-l border-gray-200">
              <h2 className="text-sm font-semibold text-gray-900 mb-4">Quick Settings</h2>

              {/* AI Provider Section */}
              <div className="mb-4">
                <button
                  onClick={() => toggleSection('ai')}
                  className="w-full flex items-center justify-between p-3 bg-gray-50 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-100 transition-colors"
                >
                  <span>AI Provider</span>
                  {expandedSections.ai ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                </button>
                {expandedSections.ai && (
                  <div className="mt-2 space-y-3 p-3 bg-gray-50 border border-gray-300 rounded-md">
                    <div>
                      <label className="block text-xs text-gray-600 mb-1">Provider</label>
                      <select
                        value={configObj.ai_provider || 'openai'}
                        onChange={(e) => updateConfig({ ai_provider: e.target.value })}
                        className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                      >
                        <option value="openai">OpenAI</option>
                        <option value="gemini">Google Gemini</option>
                        <option value="cloudflare">Cloudflare</option>
                        <option value="ollama">Ollama</option>
                      </select>
                    </div>
                    <div>
                      <label className="block text-xs text-gray-600 mb-1">Model</label>
                      <input
                        type="text"
                        value={configObj.ai_model || ''}
                        onChange={(e) => updateConfig({ ai_model: e.target.value })}
                        placeholder="gpt-4o-mini"
                        className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-gray-600 mb-1">Base URL (optional)</label>
                      <input
                        type="text"
                        value={configObj.ai_base_url || ''}
                        onChange={(e) => updateConfig({ ai_base_url: e.target.value })}
                        placeholder="https://api.openai.com/v1"
                        className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                      />
                    </div>
                  </div>
                )}
              </div>

              {/* CloudShip Integration Section */}
              <div className="mb-4">
                <button
                  onClick={() => toggleSection('cloudship')}
                  className="w-full flex items-center justify-between p-3 bg-gray-50 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-100 transition-colors"
                >
                  <span>CloudShip Integration</span>
                  {expandedSections.cloudship ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                </button>
                {expandedSections.cloudship && (
                  <div className="mt-2 space-y-3 p-3 bg-gray-50 border border-gray-300 rounded-md">
                    <div className="flex items-center justify-between">
                      <label className="text-xs text-gray-600">Enabled</label>
                      <input
                        type="checkbox"
                        checked={configObj.cloudship?.enabled || false}
                        onChange={(e) => updateCloudShipConfig({ enabled: e.target.checked })}
                        className="bg-white border border-gray-300"
                      />
                    </div>
                    
                    {/* Station Identity */}
                    <div className="pt-2 border-t border-gray-200">
                      <div className="text-xs font-medium text-gray-500 mb-2">Station Identity</div>
                      <div className="space-y-2">
                        <div>
                          <label className="block text-xs text-gray-600 mb-1">Station Name</label>
                          <input
                            type="text"
                            value={configObj.cloudship?.name || ''}
                            onChange={(e) => updateCloudShipConfig({ name: e.target.value })}
                            placeholder="my-station"
                            className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                          />
                        </div>
                      </div>
                    </div>

                    {/* Bundle Access (API Key) */}
                    <div className="pt-2 border-t border-gray-200">
                      <div className="text-xs font-medium text-gray-500 mb-2">Bundle Access</div>
                      <div className="space-y-2">
                        <div>
                          <label className="block text-xs text-gray-600 mb-1">API Key (Personal Access Token)</label>
                          <input
                            type="password"
                            value={configObj.cloudship?.api_key || ''}
                            onChange={(e) => updateCloudShipConfig({ api_key: e.target.value })}
                            placeholder="cst_..."
                            className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                          />
                          <p className="text-[10px] text-gray-400 mt-1">Run `stn auth login` to get an API key</p>
                        </div>
                        <div>
                          <label className="block text-xs text-gray-600 mb-1">API URL</label>
                          <input
                            type="text"
                            value={configObj.cloudship?.api_url || 'https://app.cloudshipai.com'}
                            onChange={(e) => updateCloudShipConfig({ api_url: e.target.value })}
                            placeholder="https://app.cloudshipai.com"
                            className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                          />
                        </div>
                      </div>
                    </div>

                    {/* Lighthouse Connection (for deployed stations) */}
                    <div className="pt-2 border-t border-gray-200">
                      <div className="text-xs font-medium text-gray-500 mb-2">Lighthouse Connection</div>
                      <div className="space-y-2">
                        <div>
                          <label className="block text-xs text-gray-600 mb-1">Registration Key</label>
                          <input
                            type="password"
                            value={configObj.cloudship?.registration_key || ''}
                            onChange={(e) => updateCloudShipConfig({ registration_key: e.target.value })}
                            placeholder="Enter registration key"
                            className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                          />
                          <p className="text-[10px] text-gray-400 mt-1">Used for remote station management</p>
                        </div>
                        <div>
                          <label className="block text-xs text-gray-600 mb-1">Endpoint</label>
                          <input
                            type="text"
                            value={configObj.cloudship?.endpoint || 'lighthouse.cloudshipai.com:443'}
                            onChange={(e) => updateCloudShipConfig({ endpoint: e.target.value })}
                            placeholder="lighthouse.cloudshipai.com:443"
                            className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                          />
                        </div>
                        <div className="flex items-center justify-between">
                          <label className="text-xs text-gray-600">Use TLS</label>
                          <input
                            type="checkbox"
                            checked={configObj.cloudship?.use_tls !== false}
                            onChange={(e) => updateCloudShipConfig({ use_tls: e.target.checked })}
                            className="bg-white border border-gray-300"
                          />
                        </div>
                      </div>
                    </div>

                    {/* OAuth Settings */}
                    <div className="pt-2 border-t border-gray-200">
                      <div className="text-xs font-medium text-gray-500 mb-2">OAuth Settings</div>
                      <div className="space-y-2">
                        <div className="flex items-center justify-between">
                          <label className="text-xs text-gray-600">OAuth Enabled</label>
                          <input
                            type="checkbox"
                            checked={configObj.cloudship?.oauth?.enabled || false}
                            onChange={(e) => updateCloudShipConfig({ 
                              oauth: { ...configObj.cloudship?.oauth, enabled: e.target.checked }
                            })}
                            className="bg-white border border-gray-300"
                          />
                        </div>
                        <div>
                          <label className="block text-xs text-gray-600 mb-1">Client ID</label>
                          <input
                            type="text"
                            value={configObj.cloudship?.oauth?.client_id || ''}
                            onChange={(e) => updateCloudShipConfig({ 
                              oauth: { ...configObj.cloudship?.oauth, client_id: e.target.value }
                            })}
                            placeholder="OAuth client ID"
                            className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                          />
                        </div>
                        <div>
                          <label className="block text-xs text-gray-600 mb-1">Introspect URL</label>
                          <input
                            type="text"
                            value={configObj.cloudship?.oauth?.introspect_url || 'https://app.cloudshipai.com/oauth/introspect/'}
                            onChange={(e) => updateCloudShipConfig({ 
                              oauth: { ...configObj.cloudship?.oauth, introspect_url: e.target.value }
                            })}
                            placeholder="https://app.cloudshipai.com/oauth/introspect/"
                            className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                          />
                        </div>
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {/* Telemetry Section */}
              <div className="mb-4">
                <button
                  onClick={() => toggleSection('telemetry')}
                  className="w-full flex items-center justify-between p-3 bg-gray-50 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-100 transition-colors"
                >
                  <span>Telemetry (OTEL)</span>
                  {expandedSections.telemetry ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                </button>
                {expandedSections.telemetry && (
                  <div className="mt-2 space-y-3 p-3 bg-gray-50 border border-gray-300 rounded-md">
                    <div className="flex items-center justify-between">
                      <label className="text-xs text-gray-600">Enabled</label>
                      <input
                        type="checkbox"
                        checked={configObj.telemetry?.enabled !== false}
                        onChange={(e) => updateTelemetryConfig({ enabled: e.target.checked })}
                        className="bg-white border border-gray-300"
                      />
                    </div>

                    {/* Endpoint - always show when enabled */}
                    {configObj.telemetry?.enabled !== false && (
                      <>
                        {/* Quick preset buttons */}
                        <div>
                          <label className="block text-xs text-gray-600 mb-1">Quick Setup</label>
                          <div className="flex gap-2">
                            <button
                              type="button"
                              onClick={() => updateTelemetryConfig({ endpoint: 'http://localhost:4318' })}
                              className={`flex-1 px-2 py-1.5 text-xs font-medium rounded border transition-colors ${
                                configObj.telemetry?.endpoint === 'http://localhost:4318' || !configObj.telemetry?.endpoint
                                  ? 'bg-blue-50 border-blue-300 text-blue-700'
                                  : 'bg-white border-gray-300 text-gray-600 hover:bg-gray-50'
                              }`}
                            >
                              Local Jaeger
                            </button>
                            <button
                              type="button"
                              onClick={() => updateTelemetryConfig({ endpoint: 'https://telemetry.cloudshipai.com/v1/traces' })}
                              className={`flex-1 px-2 py-1.5 text-xs font-medium rounded border transition-colors ${
                                configObj.telemetry?.endpoint === 'https://telemetry.cloudshipai.com/v1/traces'
                                  ? 'bg-blue-50 border-blue-300 text-blue-700'
                                  : 'bg-white border-gray-300 text-gray-600 hover:bg-gray-50'
                              }`}
                            >
                              CloudShip
                            </button>
                            <button
                              type="button"
                              onClick={() => updateTelemetryConfig({ endpoint: '' })}
                              className={`flex-1 px-2 py-1.5 text-xs font-medium rounded border transition-colors ${
                                configObj.telemetry?.endpoint && 
                                configObj.telemetry?.endpoint !== 'http://localhost:4318' &&
                                configObj.telemetry?.endpoint !== 'https://telemetry.cloudshipai.com/v1/traces'
                                  ? 'bg-blue-50 border-blue-300 text-blue-700'
                                  : 'bg-white border-gray-300 text-gray-600 hover:bg-gray-50'
                              }`}
                            >
                              Custom
                            </button>
                          </div>
                        </div>

                        <div>
                          <label className="block text-xs text-gray-600 mb-1">Endpoint</label>
                          <input
                            type="text"
                            value={configObj.telemetry?.endpoint || 'http://localhost:4318'}
                            onChange={(e) => updateTelemetryConfig({ endpoint: e.target.value })}
                            placeholder="http://localhost:4318"
                            className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                          />
                          <p className="text-[10px] text-gray-400 mt-1">
                            {configObj.telemetry?.endpoint === 'https://telemetry.cloudshipai.com/v1/traces' 
                              ? 'CloudShip managed telemetry (uses registration key for auth)'
                              : 'OTLP HTTP endpoint (Jaeger, Grafana Cloud, Datadog, etc.)'}
                          </p>
                        </div>

                        {/* Auth Header - show if endpoint is not localhost and not cloudship */}
                        {configObj.telemetry?.endpoint && 
                         !configObj.telemetry?.endpoint.includes('localhost') &&
                         !configObj.telemetry?.endpoint.includes('cloudshipai.com') && (
                          <div>
                            <label className="block text-xs text-gray-600 mb-1">Authorization Header</label>
                            <input
                              type="password"
                              value={configObj.telemetry?.headers?.Authorization || ''}
                              onChange={(e) => updateTelemetryConfig({ 
                                headers: { ...configObj.telemetry?.headers, Authorization: e.target.value }
                              })}
                              placeholder="Basic <base64> or Bearer <token>"
                              className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                            />
                            <p className="text-[10px] text-gray-400 mt-1">For Grafana Cloud: Basic base64(instanceId:token)</p>
                          </div>
                        )}
                      </>
                    )}
                  </div>
                )}
              </div>

              {/* Server Ports Section */}
              <div className="mb-4">
                <button
                  onClick={() => toggleSection('ports')}
                  className="w-full flex items-center justify-between p-3 bg-gray-50 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-100 transition-colors"
                >
                  <span>Server Ports</span>
                  {expandedSections.ports ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                </button>
                {expandedSections.ports && (
                  <div className="mt-2 space-y-3 p-3 bg-gray-50 border border-gray-300 rounded-md">
                    <div>
                      <label className="block text-xs text-gray-600 mb-1">API Port (Dev Mode Only)</label>
                      <input
                        type="number"
                        value={configObj.api_port || 8585}
                        onChange={(e) => updateConfig({ api_port: parseInt(e.target.value) })}
                        className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                      />
                      <p className="text-[10px] text-gray-400 mt-1">Management UI and REST API</p>
                    </div>
                    <div>
                      <label className="block text-xs text-gray-600 mb-1">Management MCP Port</label>
                      <input
                        type="number"
                        value={configObj.mcp_port || 8586}
                        onChange={(e) => updateConfig({ mcp_port: parseInt(e.target.value) })}
                        className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                      />
                      <p className="text-[10px] text-gray-400 mt-1">MCP server for local tools</p>
                    </div>
                    <div>
                      <label className="block text-xs text-gray-600 mb-1">Dynamic Agent MCP Port</label>
                      <input
                        type="number"
                        value={(configObj.mcp_port || 8586) + 1}
                        disabled
                        className="w-full bg-gray-100 border border-gray-300 text-gray-500 font-mono text-sm p-2 rounded"
                      />
                      <p className="text-[10px] text-gray-400 mt-1">Auto-calculated as MCP Port + 1 (for CloudShip agent access)</p>
                    </div>
                  </div>
                )}
              </div>

              {/* Other Settings Section */}
              <div className="mb-4">
                <button
                  onClick={() => toggleSection('other')}
                  className="w-full flex items-center justify-between p-3 bg-gray-50 border border-gray-300 rounded-md text-sm font-medium text-gray-700 hover:bg-gray-100 transition-colors"
                >
                  <span>Other Settings</span>
                  {expandedSections.other ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                </button>
                {expandedSections.other && (
                  <div className="mt-2 space-y-3 p-3 bg-gray-50 border border-gray-300 rounded-md">
                    <div>
                      <label className="block text-xs text-gray-600 mb-1">Admin Username</label>
                      <input
                        type="text"
                        value={configObj.admin_username || 'admin'}
                        onChange={(e) => updateConfig({ admin_username: e.target.value })}
                        className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <label className="text-xs text-gray-600">Debug Mode</label>
                      <input
                        type="checkbox"
                        checked={configObj.debug || false}
                        onChange={(e) => updateConfig({ debug: e.target.checked })}
                        className="bg-white border border-gray-300"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-gray-600 mb-1">Database URL</label>
                      <input
                        type="text"
                        value={configObj.database_url || ''}
                        onChange={(e) => updateConfig({ database_url: e.target.value })}
                        className="w-full bg-white border border-gray-300 text-gray-900 font-mono text-sm p-2 rounded focus:outline-none focus:border-gray-500 focus:ring-1 focus:ring-gray-500"
                      />
                    </div>
                  </div>
                )}
              </div>
            </div>
          </>
        )}
      </div>

      {/* Help Modal */}
      <HelpModal
        isOpen={isHelpModalOpen}
        onClose={() => setIsHelpModalOpen(false)}
        title="Settings"
        pageDescription="Configure Station's core behavior including AI provider settings, CloudShip integration, server ports, and system-wide preferences. Changes require Station restart to take effect."
      >
        <div className="space-y-6">
          {/* Configuration File */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <FileText className="h-5 w-5 text-gray-600" />
              Configuration File
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="text-sm text-gray-700 leading-relaxed">
                Station uses a YAML configuration file for system settings. Default location: <span className="font-mono bg-gray-100 px-1.5 py-0.5 rounded text-xs">~/.config/station/station.yml</span>
              </div>
              <div className="bg-amber-50 border border-amber-200 rounded p-3">
                <div className="flex items-start gap-2">
                  <AlertTriangle className="h-4 w-4 text-amber-600 flex-shrink-0 mt-0.5" />
                  <div className="text-xs text-amber-700">
                    <span className="font-semibold">Important:</span> Station must be restarted after saving configuration changes for them to take effect.
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* AI Configuration */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Sparkles className="h-5 w-5 text-gray-600" />
              AI Configuration
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="text-sm text-gray-700 leading-relaxed">
                Configure AI model provider settings for agent execution:
              </div>
              <div className="space-y-2">
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">API Keys</div>
                  <div className="text-xs text-gray-600">Set <span className="font-mono bg-gray-100 px-1 py-0.5 rounded">OPENAI_API_KEY</span>, <span className="font-mono bg-gray-100 px-1 py-0.5 rounded">GEMINI_API_KEY</span>, or <span className="font-mono bg-gray-100 px-1 py-0.5 rounded">ANTHROPIC_API_KEY</span> environment variables</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Default Model</div>
                  <div className="text-xs text-gray-600">Model used when agents don't specify: <span className="font-mono bg-gray-100 px-1 py-0.5 rounded">gpt-4o-mini</span>, <span className="font-mono bg-gray-100 px-1 py-0.5 rounded">gemini-2.0-flash-exp</span>, <span className="font-mono bg-gray-100 px-1 py-0.5 rounded">claude-3-5-sonnet-latest</span></div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Temperature</div>
                  <div className="text-xs text-gray-600">Controls randomness (0.0-2.0). Lower values = more deterministic responses</div>
                </div>
              </div>
            </div>
          </div>

          {/* CloudShip Integration */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Cloud className="h-5 w-5 text-gray-600" />
              CloudShip Integration
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="text-sm text-gray-700 leading-relaxed">
                CloudShip provides OAuth authentication for secure MCP tool access:
              </div>
              <div className="space-y-2">
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">OAuth URL</div>
                  <div className="text-xs text-gray-600">CloudShip OAuth endpoint for token validation</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Required Scopes</div>
                  <div className="text-xs text-gray-600">OAuth scopes needed for MCP authentication</div>
                </div>
              </div>
              <div className="text-sm text-gray-700 leading-relaxed">
                Used by Dynamic Agent MCP server to authenticate CloudShip clients securely.
              </div>
            </div>
          </div>

          {/* Server Ports */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Server className="h-5 w-5 text-gray-600" />
              Server Ports
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4">
              <div className="grid grid-cols-2 gap-3">
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">HTTP Server</div>
                  <div className="text-xs text-gray-600 mb-2">Default: <span className="font-mono bg-gray-100 px-1 py-0.5 rounded">8580</span></div>
                  <div className="text-xs text-gray-600">Web UI and REST API</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">MCP Server</div>
                  <div className="text-xs text-gray-600 mb-2">Default: <span className="font-mono bg-gray-100 px-1 py-0.5 rounded">8586</span></div>
                  <div className="text-xs text-gray-600">MCP protocol stdio transport</div>
                </div>
              </div>
            </div>
          </div>

          {/* Other Settings */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Settings className="h-5 w-5 text-gray-600" />
              System Settings
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="space-y-2">
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Admin Username</div>
                  <div className="text-xs text-gray-600">Username for admin operations (currently not enforced)</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Debug Mode</div>
                  <div className="text-xs text-gray-600">Enable verbose logging for troubleshooting</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Telemetry</div>
                  <div className="text-xs text-gray-600">Send anonymous usage statistics to improve Station (if enabled)</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Database URL</div>
                  <div className="text-xs text-gray-600">SQLite database location: <span className="font-mono bg-gray-100 px-1 py-0.5 rounded">~/.config/station/station.db</span></div>
                </div>
              </div>
            </div>
          </div>

          {/* Editing Modes */}
          <div>
            <h3 className="text-base font-semibold text-gray-900 mb-3 flex items-center gap-2">
              <Wrench className="h-5 w-5 text-gray-600" />
              Configuration Editing
            </h3>
            <div className="bg-gray-50 border border-gray-200 rounded-lg p-4 space-y-3">
              <div className="text-sm text-gray-700 leading-relaxed">
                The Settings page provides two editing modes:
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Visual Editor</div>
                  <div className="text-xs text-gray-600">Form-based editing with collapsible sections for common settings</div>
                </div>
                <div className="bg-white border border-gray-200 rounded p-3">
                  <div className="font-medium text-gray-900 text-sm mb-1">Raw YAML Editor</div>
                  <div className="text-xs text-gray-600">Direct YAML editing with syntax highlighting for advanced configuration</div>
                </div>
              </div>
            </div>
          </div>

          {/* Best Practices */}
          <div className="bg-gray-50 border border-gray-200 rounded-lg p-4">
            <div className="font-semibold text-gray-900 mb-2 flex items-center gap-2">
              <Shield className="h-4 w-4 text-gray-600" />
              Best Practices
            </div>
            <ul className="space-y-1.5 text-sm text-gray-700">
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-gray-600 flex-shrink-0 mt-1.5"></div>
                <div>Always restart Station after saving configuration changes</div>
              </li>
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-gray-600 flex-shrink-0 mt-1.5"></div>
                <div>Never commit API keys to station.yml - use environment variables instead</div>
              </li>
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-gray-600 flex-shrink-0 mt-1.5"></div>
                <div>Back up station.yml before making significant changes</div>
              </li>
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-gray-600 flex-shrink-0 mt-1.5"></div>
                <div>Validate YAML syntax before saving to avoid startup errors</div>
              </li>
              <li className="flex items-start gap-2">
                <div className="w-1.5 h-1.5 rounded-full bg-gray-600 flex-shrink-0 mt-1.5"></div>
                <div>Use Debug Mode when troubleshooting agent execution or MCP connection issues</div>
              </li>
            </ul>
          </div>
        </div>
      </HelpModal>
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
                  <Route path="/" element={<AgentsPage />} />
                  <Route path="/getting-started" element={<GettingStartedPage />} />
                  <Route path="/agents" element={<AgentsPage />} />
                  <Route path="/agents/:env" element={<AgentsPage />} />
                  <Route path="/agents/:env/:agentId" element={<AgentsPage />} />
                  <Route path="/agent/:agentId" element={<AgentsPage />} />
                  <Route path="/mcps" element={<MCPServersPage />} />
                  <Route path="/mcps/:env" element={<MCPServersPage />} />
                  <Route path="/mcp-directory" element={<MCPDirectoryPage />} />
                  <Route path="/runs" element={<RunsPage />} />
                  <Route path="/reports" element={<ReportsPageWrapper />} />
                  <Route path="/reports/:reportId" element={<ReportDetailPageWrapper />} />
                  <Route path="/environments" element={<EnvironmentsPage />} />
                  <Route path="/bundles" element={<BundlesPage />} />
                  <Route path="/live-demo" element={<LiveDemoPage />} />
                  <Route path="/settings" element={<SettingsPage />} />
                  <Route path="/agent-editor/:agentId" element={<AgentEditor />} />
                  <Route path="*" element={<AgentsPage />} />
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
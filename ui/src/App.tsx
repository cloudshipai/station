import { useState, useCallback, useEffect } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { 
  ReactFlowProvider,
  ReactFlow,
  addEdge,
  useNodesState,
  useEdgesState,
  Background,
  Controls,
  MiniMap,
  Handle,
  Position,
  type Node,
  type Edge,
  type Connection,
  type NodeTypes,
} from '@xyflow/react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Bot, Database, Settings, Plus, Home } from 'lucide-react';
import '@xyflow/react/dist/style.css';
import { agentsApi } from './api/station';
import { getLayoutedNodes } from './utils/layoutUtils';

// Station ASCII Banner Component
const StationBanner = () => (
  <div className="font-mono text-xs leading-tight">
    <div className="text-tokyo-blue">  ███████╗████████╗ █████╗ ████████╗██╗ ██████╗ ███╗   ██╗ </div>
    <div className="text-tokyo-cyan">  ██╔════╝╚══██╔══╝██╔══██╗╚══██╔══╝██║██╔═══██╗████╗  ██║ </div>
    <div className="text-tokyo-blue5">  ███████╗   ██║   ███████║   ██║   ██║██║   ██║██╔██╗ ██║ </div>
    <div className="text-tokyo-blue1">  ╚════██║   ██║   ██╔══██║   ██║   ██║██║   ██║██║╚██╗██║ </div>
    <div className="text-tokyo-blue2">  ███████║   ██║   ██║  ██║   ██║   ██║╚██████╔╝██║ ╚████║ </div>
    <div className="text-tokyo-cyan">  ╚══════╝   ╚═╝   ╚═╝  ╚═╝   ╚═╝   ╚═╝ ╚═════╝ ╚═╝  ╚═══╝ </div>
    <div className="text-tokyo-comment mt-2 text-center italic">🚂 Easiest way to build secure, intelligent, background, tool agents</div>
    <div className="text-tokyo-dark5 text-center mt-1">by the CloudshipAI team</div>
  </div>
);

// Custom Node Components (sized for better visibility and layout)
const AgentNode = ({ data }: { data: any }) => (
  <div className="w-[280px] h-[130px] px-4 py-3 shadow-tokyo-blue border border-tokyo-blue7 bg-tokyo-bg-dark rounded-lg relative">
    {/* Output handle on the right side */}
    <Handle type="source" position={Position.Right} className="w-3 h-3 bg-tokyo-blue" />
    
    <div className="flex items-center gap-2 mb-2">
      <Bot className="h-5 w-5 text-tokyo-blue" />
      <div className="font-mono text-base text-tokyo-blue font-medium">{data.label}</div>
    </div>
    <div className="text-sm text-tokyo-comment mb-2 line-clamp-2">{data.description}</div>
    <div className="text-sm text-tokyo-green font-medium">{data.status}</div>
  </div>
);

const MCPNode = ({ data }: { data: any }) => (
  <div className="w-[280px] h-[130px] px-4 py-3 shadow-tokyo-blue border border-tokyo-blue7 bg-tokyo-bg-dark rounded-lg relative">
    {/* Input handle on the left side */}
    <Handle type="target" position={Position.Left} className="w-3 h-3 bg-tokyo-cyan" />
    {/* Output handle on the right side */}
    <Handle type="source" position={Position.Right} className="w-3 h-3 bg-tokyo-cyan" />
    
    <div className="flex items-center gap-2 mb-2">
      <Database className="h-5 w-5 text-tokyo-cyan" />
      <div className="font-mono text-base text-tokyo-cyan font-medium">{data.label}</div>
    </div>
    <div className="text-sm text-tokyo-comment mb-2">{data.description}</div>
    <div className="text-sm text-tokyo-purple font-medium">{data.tools?.length || 0} tools available</div>
  </div>
);

const ToolNode = ({ data }: { data: any }) => (
  <div className="w-[280px] h-[130px] px-4 py-3 shadow-tokyo-blue border border-tokyo-blue7 bg-tokyo-bg-dark rounded-lg relative">
    {/* Input handle on the left side */}
    <Handle type="target" position={Position.Left} className="w-3 h-3 bg-tokyo-green" />
    
    <div className="flex items-center gap-2 mb-2">
      <Settings className="h-5 w-5 text-tokyo-green" />
      <div className="font-mono text-base text-tokyo-green font-medium">{data.label}</div>
    </div>
    <div className="text-sm text-tokyo-comment mb-2">{data.description || 'Tool function'}</div>
    <div className="text-sm text-tokyo-blue1 font-medium">from {data.category}</div>
  </div>
);

const nodeTypes: NodeTypes = {
  agent: AgentNode,
  mcp: MCPNode,
  tool: ToolNode,
};

// Layout component using Tailwind classes
const Layout = ({ children, currentPage, onPageChange }: any) => (
  <div className="flex h-screen bg-tokyo-bg">
    <div className="w-64 bg-tokyo-bg-dark border-r border-tokyo-blue7 p-4">
      <div className="flex items-center gap-2 mb-6">
        <Home className="h-6 w-6 text-tokyo-blue" />
        <h2 className="text-lg font-mono font-semibold text-tokyo-fg">Station</h2>
      </div>
      <nav className="space-y-2">
        <button 
          onClick={() => onPageChange('agents')}
          className={`w-full text-left p-3 rounded border font-mono transition-colors ${
            currentPage === 'agents' 
              ? 'bg-tokyo-blue text-tokyo-bg border-tokyo-blue shadow-tokyo-glow' 
              : 'bg-transparent text-tokyo-fg-dark hover:bg-tokyo-bg-highlight hover:text-tokyo-blue border-transparent hover:border-tokyo-blue7'
          }`}
        >
          Agents
        </button>
        <button 
          onClick={() => onPageChange('mcps')}
          className={`w-full text-left p-3 rounded border font-mono transition-colors ${
            currentPage === 'mcps' 
              ? 'bg-tokyo-cyan text-tokyo-bg border-tokyo-cyan shadow-tokyo-glow' 
              : 'bg-transparent text-tokyo-fg-dark hover:bg-tokyo-bg-highlight hover:text-tokyo-cyan border-transparent hover:border-tokyo-blue7'
          }`}
        >
          MCP Servers
        </button>
        <button 
          onClick={() => onPageChange('runs')}
          className={`w-full text-left p-3 rounded border font-mono transition-colors ${
            currentPage === 'runs' 
              ? 'bg-tokyo-green text-tokyo-bg border-tokyo-green shadow-tokyo-glow' 
              : 'bg-transparent text-tokyo-fg-dark hover:bg-tokyo-bg-highlight hover:text-tokyo-green border-transparent hover:border-tokyo-blue7'
          }`}
        >
          Runs
        </button>
      </nav>
    </div>
    <div className="flex-1">
      {children}
    </div>
  </div>
);

const AgentsCanvas = () => {
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [selectedAgent, setSelectedAgent] = useState<number | null>(null);
  const [agents, setAgents] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  // Fetch agents list
  useEffect(() => {
    const fetchAgents = async () => {
      try {
        const response = await agentsApi.getAll();
        setAgents(response.data.agents);
        if (response.data.agents.length > 0) {
          setSelectedAgent(response.data.agents[0].id);
        }
      } catch (error) {
        console.error('Failed to fetch agents:', error);
      } finally {
        setLoading(false);
      }
    };
    fetchAgents();
  }, []);

  // Fetch agent details and generate graph when agent selected
  useEffect(() => {
    if (!selectedAgent) return;

    const fetchAgentDetails = async () => {
      try {
        const response = await agentsApi.getWithTools(selectedAgent);
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
          },
        });

        // MCP servers and tools
        mcp_servers.forEach((server) => {
          // MCP server node
          newNodes.push({
            id: `mcp-${server.id}`,
            type: 'mcp',
            position: { x: 0, y: 0 }, // ELK will position this
            data: {
              label: server.name,
              description: 'MCP Server',
              tools: server.tools,
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

          // Tool nodes for this server
          server.tools.forEach((tool) => {
            newNodes.push({
              id: `tool-${tool.id}`,
              type: 'tool',
              position: { x: 0, y: 0 }, // ELK will position this
              data: {
                label: tool.name.replace('__', ''),
                description: tool.description || 'Tool function',
                category: server.name,
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
        });


        // Apply automatic layout using ELK.js
        console.log('Before layout - nodes:', newNodes.length, 'edges:', newEdges.length);
        console.log('Edges being created:', newEdges.map(e => `${e.source} -> ${e.target}`));
        console.log('Node IDs:', newNodes.map(n => n.id));
        
        const layoutedNodes = await getLayoutedNodes(newNodes, newEdges);
        
        console.log('After layout - nodes:', layoutedNodes.length, 'edges:', newEdges.length);
        console.log('Final edges:', newEdges);
        setNodes(layoutedNodes);
        setEdges(newEdges);
      } catch (error) {
        console.error('Failed to fetch agent details:', error);
      }
    };

    fetchAgentDetails();
  }, [selectedAgent, setNodes, setEdges]);

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge({
      ...params,
      animated: true,
      style: { 
        stroke: '#ff00ff', 
        strokeWidth: 3,
        filter: 'drop-shadow(0 0 8px #ff00ff80)'
      },
      className: 'neon-edge',
    }, eds)),
    [setEdges]
  );

  const onNodeClick = useCallback((event: any, node: Node) => {
    if (node.type === 'agent') {
      // Could open agent details modal
      console.log('Agent clicked:', node.data);
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
            <select 
              value={selectedAgent || ''} 
              onChange={(e) => setSelectedAgent(Number(e.target.value))}
              className="ml-4 px-3 py-1 bg-tokyo-bg-dark border border-tokyo-blue7 text-tokyo-fg font-mono rounded"
            >
              {agents.map((agent) => (
                <option key={agent.id} value={agent.id}>
                  {agent.name}
                </option>
              ))}
            </select>
          )}
        </div>
        <button className="px-4 py-2 bg-tokyo-blue text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-blue5 transition-colors shadow-tokyo-glow flex items-center gap-2">
          <Plus className="h-4 w-4" />
          Create Agent
        </button>
      </div>
      <div className="flex-1 relative">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          onNodeClick={onNodeClick}
          nodeTypes={nodeTypes}
          fitView
          className="bg-tokyo-bg"
          defaultEdgeOptions={{
            animated: true,
            style: { 
              stroke: '#ff00ff', 
              strokeWidth: 3,
              zIndex: 1000
            },
          }}
        >
          <Background 
            color="#394b70" 
            gap={20} 
            size={1}
            className="opacity-20"
          />
          <Controls className="bg-tokyo-bg-dark border border-tokyo-blue7" />
          <MiniMap 
            className="bg-tokyo-bg-dark border border-tokyo-blue7"
            nodeStrokeColor={(n) => {
              if (n.type === 'agent') return '#7aa2f7';
              if (n.type === 'mcp') return '#7dcfff';
              return '#9ece6a';
            }}
            nodeColor={(n) => {
              if (n.type === 'agent') return '#16161e';
              if (n.type === 'mcp') return '#16161e';
              return '#16161e';
            }}
          />
        </ReactFlow>
      </div>
    </div>
  );
};

const MCPServers = () => (
  <div className="h-full flex flex-col bg-tokyo-bg">
    <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
      <h1 className="text-xl font-mono font-semibold text-tokyo-cyan">MCP Servers</h1>
      <button className="px-4 py-2 bg-tokyo-cyan text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-blue1 transition-colors shadow-tokyo-glow flex items-center gap-2">
        <Plus className="h-4 w-4" />
        Add Server
      </button>
    </div>
    <div className="flex-1 p-4">
      <div className="grid gap-4">
        <div className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo">
          <h3 className="font-mono font-medium text-tokyo-cyan">Filesystem Server</h3>
          <p className="text-sm text-tokyo-comment mt-1 font-mono">File operations and directory management</p>
          <div className="mt-2 flex gap-2">
            <span className="px-2 py-1 bg-tokyo-green text-tokyo-bg text-xs rounded font-mono">Active</span>
            <span className="px-2 py-1 bg-tokyo-blue text-tokyo-bg text-xs rounded font-mono">14 Tools</span>
          </div>
        </div>
      </div>
    </div>
  </div>
);

const Runs = () => (
  <div className="h-full flex flex-col bg-tokyo-bg">
    <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
      <h1 className="text-xl font-mono font-semibold text-tokyo-green">Agent Runs</h1>
    </div>
    <div className="flex-1 p-4">
      <p className="text-tokyo-comment font-mono">Recent agent executions will appear here</p>
    </div>
  </div>
);

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      refetchOnWindowFocus: false,
    },
  },
});

type Page = 'agents' | 'mcps' | 'runs';

function App() {
  const [currentPage, setCurrentPage] = useState<Page>('agents');

  const renderPage = () => {
    switch (currentPage) {
      case 'agents':
        return <AgentsCanvas />;
      case 'mcps':
        return <MCPServers />;
      case 'runs':
        return <Runs />;
      default:
        return <AgentsCanvas />;
    }
  };

  return (
    <QueryClientProvider client={queryClient}>
      <ReactFlowProvider>
        <BrowserRouter>
          <div className="min-h-screen bg-background">
            <Layout currentPage={currentPage} onPageChange={setCurrentPage}>
              <Routes>
                <Route path="*" element={renderPage()} />
              </Routes>
            </Layout>
          </div>
        </BrowserRouter>
      </ReactFlowProvider>
    </QueryClientProvider>
  );
}

export default App

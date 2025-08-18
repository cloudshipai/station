import React, { useState, useCallback, useEffect, useRef } from 'react';
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
  type NodeProps,
} from '@xyflow/react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Bot, Database, Settings, Plus, Eye, X, Play, ChevronDown, ChevronRight, Users, RotateCw, Train, Globe, Package } from 'lucide-react';
import '@xyflow/react/dist/style.css';
import { agentsApi, environmentsApi, syncApi, agentRunsApi, mcpServersApi } from './api/station';
import { apiClient } from './api/client';
import { getLayoutedNodes } from './utils/layoutUtils';

// Station Banner Component  
const StationBanner = () => (
  <div className="font-mono leading-tight">
    <div className="text-2xl font-bold bg-gradient-to-r from-tokyo-blue via-tokyo-cyan to-tokyo-blue5 bg-clip-text text-transparent">
      STATION
    </div>
    <div className="text-tokyo-comment text-xs mt-1 italic">🚂 agents for engineers. Be in control</div>
    <div className="text-tokyo-dark5 text-xs">by CloudshipAI</div>
  </div>
);

// Run Details Modal Component
const RunDetailsModal = ({ runId, isOpen, onClose }: { runId: number | null, isOpen: boolean, onClose: () => void }) => {
  const [runDetails, setRunDetails] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!isOpen || !runId) return;

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
  }, [runId, isOpen]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo-lg max-w-4xl w-full max-h-[90vh] overflow-auto">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-tokyo-blue7">
          <div className="flex items-center gap-3">
            <Play className="h-6 w-6 text-tokyo-green" />
            <h2 className="text-xl font-mono font-semibold text-tokyo-green">Run Details</h2>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-tokyo-bg-highlight text-tokyo-comment hover:text-tokyo-fg transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="text-tokyo-comment font-mono">Loading run details...</div>
            </div>
          ) : runDetails ? (
            <div className="space-y-6">
              {/* Basic Info */}
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Basic Information</h3>
                <div className="grid grid-cols-3 gap-4">
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Agent</div>
                    <div className="font-mono text-tokyo-green">{runDetails?.agent_name || `Agent ${runDetails?.agent_id}`}</div>
                  </div>
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Status</div>
                    <div className={`font-mono ${
                      runDetails.status === 'completed' ? 'text-tokyo-green' :
                      runDetails.status === 'running' ? 'text-tokyo-blue' :
                      'text-tokyo-red'
                    }`}>
                      {runDetails.status}
                    </div>
                  </div>
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Duration</div>
                    <div className="font-mono text-tokyo-cyan">
                      {runDetails.duration_seconds ? `${runDetails.duration_seconds.toFixed(2)}s` : 'N/A'}
                    </div>
                  </div>
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Started</div>
                    <div className="font-mono text-tokyo-fg">
                      {runDetails.started_at ? new Date(runDetails.started_at).toLocaleString() : 'N/A'}
                    </div>
                  </div>
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Completed</div>
                    <div className="font-mono text-tokyo-fg">
                      {runDetails.completed_at ? new Date(runDetails.completed_at).toLocaleString() : 'N/A'}
                    </div>
                  </div>
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Steps Taken</div>
                    <div className="font-mono text-tokyo-purple">{runDetails.steps_taken || 0}</div>
                  </div>
                </div>
              </div>

              {/* Token Usage */}
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Token Usage</h3>
                <div className="grid grid-cols-4 gap-4">
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Input Tokens</div>
                    <div className="font-mono text-tokyo-cyan">{runDetails.input_tokens || 'N/A'}</div>
                  </div>
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Output Tokens</div>
                    <div className="font-mono text-tokyo-purple">{runDetails.output_tokens || 'N/A'}</div>
                  </div>
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Total Tokens</div>
                    <div className="font-mono text-tokyo-green">{runDetails.total_tokens || 'N/A'}</div>
                  </div>
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Tools Used</div>
                    <div className="font-mono text-tokyo-blue">{runDetails.tools_used || 0}</div>
                  </div>
                </div>
              </div>

              {/* Model Info */}
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Model Information</h3>
                <div className="space-y-2">
                  <div className="text-sm text-tokyo-comment">Model</div>
                  <div className="font-mono text-tokyo-blue bg-tokyo-bg border border-tokyo-blue7 rounded p-2">
                    {runDetails.model_name || 'N/A'}
                  </div>
                </div>
              </div>

              {/* Input */}
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Input Task</h3>
                <div className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4">
                  <pre className="text-sm text-tokyo-fg font-mono whitespace-pre-wrap">{runDetails.task || 'No task specified'}</pre>
                </div>
              </div>

              {/* Output */}
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Final Response</h3>
                <div className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4 max-h-64 overflow-auto">
                  <pre className="text-sm text-tokyo-fg font-mono whitespace-pre-wrap">{runDetails.final_response || 'No response available'}</pre>
                </div>
              </div>

              {/* Tool Calls */}
              {runDetails.tool_calls && runDetails.tool_calls.length > 0 && (
                <div>
                  <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Tool Calls</h3>
                  <div className="space-y-2">
                    {(runDetails.tool_calls || []).map((call: any, index: number) => (
                      <div key={index} className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4">
                        <div className="flex items-center justify-between mb-2">
                          <span className="font-mono text-tokyo-green">{call.tool_name}</span>
                          {call.server_name && (
                            <span className="text-sm text-tokyo-comment">via {call.server_name}</span>
                          )}
                        </div>
                        {call.arguments && (
                          <div className="mb-2">
                            <div className="text-sm text-tokyo-comment mb-1">Arguments:</div>
                            <pre className="text-xs text-tokyo-fg font-mono bg-tokyo-bg-dark p-2 rounded overflow-x-auto">
                              {JSON.stringify(call.arguments, null, 2)}
                            </pre>
                          </div>
                        )}
                        {call.result && (
                          <div>
                            <div className="text-sm text-tokyo-comment mb-1">Result:</div>
                            <div className="text-sm text-tokyo-cyan font-mono">{call.result}</div>
                          </div>
                        )}
                        {call.error && (
                          <div>
                            <div className="text-sm text-tokyo-comment mb-1">Error:</div>
                            <div className="text-sm text-tokyo-red font-mono">{call.error}</div>
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Execution Steps */}
              {runDetails.execution_steps && runDetails.execution_steps.length > 0 && (
                <div>
                  <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Execution Steps</h3>
                  <div className="space-y-2">
                    {(runDetails.execution_steps || []).map((step: any, index: number) => (
                      <div key={index} className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4">
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-tokyo-comment font-mono text-sm">Step {step.step_number || index + 1}</span>
                          {step.timestamp && (
                            <span className="text-xs text-tokyo-comment">
                              {new Date(step.timestamp).toLocaleTimeString()}
                            </span>
                          )}
                        </div>
                        <div className="text-tokyo-fg font-mono">{step.action || step.response}</div>
                        {step.tool_calls && step.tool_calls.length > 0 && (
                          <div className="mt-2 text-sm text-tokyo-cyan">
                            Tools used: {(step.tool_calls || []).map((tc: any) => tc.tool_name).join(', ')}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            <div className="text-center py-12">
              <div className="text-tokyo-comment font-mono">Run details not found</div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

// Agent Details Modal Component
const AgentDetailsModal = ({ agentId, isOpen, onClose }: { agentId: number | null, isOpen: boolean, onClose: () => void }) => {
  const [agentDetails, setAgentDetails] = useState<any>(null);
  const [agentRuns, setAgentRuns] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadingRuns, setLoadingRuns] = useState(false);
  const [selectedRunId, setSelectedRunId] = useState<number | null>(null);
  const [isRunModalOpen, setIsRunModalOpen] = useState(false);

  useEffect(() => {
    if (!isOpen || !agentId) return;

    const fetchAgentDetails = async () => {
      setLoading(true);
      try {
        const response = await agentsApi.getWithTools(agentId);
        setAgentDetails(response.data);
        
        // After agent details are loaded, fetch runs
        const fetchAgentRuns = async () => {
          setLoadingRuns(true);
          try {
            const response = await agentRunsApi.getByAgent(agentId);
            setAgentRuns(response.data.runs || []);
          } catch (error) {
            console.error('Failed to fetch agent runs:', error);
            setAgentRuns([]);
          } finally {
            setLoadingRuns(false);
          }
        };
        
        // Fetch runs after agent details are loaded
        await fetchAgentRuns();
        
      } catch (error) {
        console.error('Failed to fetch agent details:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchAgentDetails();
  }, [agentId, isOpen]);

  const openRunDetails = (runId: number) => {
    setSelectedRunId(runId);
    setIsRunModalOpen(true);
  };

  const closeRunDetails = () => {
    setIsRunModalOpen(false);
    setSelectedRunId(null);
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo-lg max-w-4xl w-full max-h-[90vh] overflow-auto">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-tokyo-blue7">
          <div className="flex items-center gap-3">
            <Bot className="h-6 w-6 text-tokyo-blue" />
            <h2 className="text-xl font-mono font-semibold text-tokyo-blue">Agent Details</h2>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-tokyo-bg-highlight text-tokyo-comment hover:text-tokyo-fg transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="text-tokyo-comment font-mono">Loading agent details...</div>
            </div>
          ) : agentDetails ? (
            <div className="space-y-6">
              {/* Basic Info */}
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Basic Information</h3>
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Name</div>
                    <div className="font-mono text-tokyo-blue">{agentDetails.agent.name}</div>
                  </div>
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Status</div>
                    <div className="font-mono text-tokyo-green">
                      {agentDetails.agent.is_scheduled ? 'Scheduled' : 'Manual'}
                    </div>
                  </div>
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Max Steps</div>
                    <div className="font-mono text-tokyo-purple">{agentDetails.agent.max_steps}</div>
                  </div>
                  <div className="space-y-2">
                    <div className="text-sm text-tokyo-comment">Environment</div>
                    <div className="font-mono text-tokyo-cyan">{agentDetails.agent.environment_id}</div>
                  </div>
                  {agentDetails.agent.cron_schedule && (
                    <div className="space-y-2">
                      <div className="text-sm text-tokyo-comment">Cron Schedule</div>
                      <div className="font-mono text-tokyo-orange">{agentDetails.agent.cron_schedule}</div>
                    </div>
                  )}
                  {agentDetails.agent.next_scheduled_run && (
                    <div className="space-y-2">
                      <div className="text-sm text-tokyo-comment">Next Run</div>
                      <div className="font-mono text-tokyo-blue">{new Date(agentDetails.agent.next_scheduled_run).toLocaleString()}</div>
                    </div>
                  )}
                </div>
              </div>

              {/* Input Schema */}
              {agentDetails.agent.input_schema && (
                <div>
                  <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Input Schema</h3>
                  <div className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4 max-h-64 overflow-auto">
                    <pre className="text-xs text-tokyo-comment font-mono whitespace-pre-wrap">
                      {typeof agentDetails.agent.input_schema === 'string' 
                        ? agentDetails.agent.input_schema 
                        : JSON.stringify(agentDetails.agent.input_schema, null, 2)}
                    </pre>
                  </div>
                </div>
              )}

              {/* Description */}
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Description</h3>
                <div className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4">
                  <div className="text-sm text-tokyo-comment font-mono whitespace-pre-wrap">
                    {agentDetails.agent.description}
                  </div>
                </div>
              </div>

              {/* Tools & MCP Servers */}
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Available Tools</h3>
                <div className="space-y-4">
                  {(agentDetails.mcp_servers || []).map((server: any) => (
                    <div key={server.id} className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4">
                      <div className="flex items-center gap-2 mb-3">
                        <Database className="h-4 w-4 text-tokyo-cyan" />
                        <span className="font-mono font-medium text-tokyo-cyan">{server.name}</span>
                        <span className="text-xs text-tokyo-comment">({(server.tools || []).length} tools)</span>
                      </div>
                      <div className="grid grid-cols-3 gap-2">
                        {(server.tools || []).map((tool: any) => (
                          <div key={tool.id} className="flex items-center gap-2 p-2 bg-tokyo-bg-highlight rounded">
                            <Settings className="h-3 w-3 text-tokyo-green" />
                            <span className="text-xs font-mono text-tokyo-green">
                              {tool.name.replace('__', '')}
                            </span>
                          </div>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              {/* Recent Runs */}
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Recent Runs</h3>
                {loadingRuns ? (
                  <div className="text-center py-4">
                    <div className="text-tokyo-comment font-mono">Loading runs...</div>
                  </div>
                ) : agentRuns.length > 0 ? (
                  <div className="space-y-2 max-h-64 overflow-auto">
                    {agentRuns.slice(0, 10).map((run: any) => (
                      <div 
                        key={run.id} 
                        className="flex items-center justify-between p-3 bg-tokyo-bg border border-tokyo-blue7 rounded cursor-pointer hover:bg-tokyo-bg-highlight transition-colors"
                        onClick={() => openRunDetails(run.id)}
                      >
                        <div className="flex items-center gap-3">
                          <Play className="h-4 w-4 text-tokyo-green" />
                          <div>
                            <div className="font-mono text-sm text-tokyo-fg">Run #{run.id}</div>
                            <div className="text-xs text-tokyo-comment">
                              {run.task && run.task.length > 50 ? `${run.task.substring(0, 50)}...` : run.task || 'No task description'}
                            </div>
                          </div>
                        </div>
                        <div className="flex items-center gap-3">
                          <div className="text-xs text-tokyo-comment">
                            {run.started_at ? new Date(run.started_at).toLocaleDateString() : 'No date'}
                          </div>
                          <div className={`px-2 py-1 text-xs rounded font-mono ${
                            run.status === 'completed' ? 'bg-tokyo-green text-tokyo-bg' :
                            run.status === 'running' ? 'bg-tokyo-blue text-tokyo-bg' :
                            run.status === 'failed' ? 'bg-tokyo-red text-tokyo-bg' :
                            'bg-tokyo-comment text-tokyo-bg'
                          }`}>
                            {run.status}
                          </div>
                          <Eye className="h-4 w-4 text-tokyo-cyan" />
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="text-center py-8 bg-tokyo-bg border border-tokyo-blue7 rounded-lg">
                    <Play className="h-8 w-8 text-tokyo-comment mx-auto mb-2" />
                    <div className="text-tokyo-comment font-mono">No runs found for this agent</div>
                    <div className="text-xs text-tokyo-dark3 mt-1">Runs will appear here after agent execution</div>
                  </div>
                )}
              </div>

              {/* System Prompt */}
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">System Prompt</h3>
                <div className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4 max-h-64 overflow-auto">
                  <pre className="text-xs text-tokyo-comment font-mono whitespace-pre-wrap">
                    {agentDetails.agent.prompt}
                  </pre>
                </div>
              </div>
            </div>
          ) : (
            <div className="text-tokyo-comment font-mono text-center py-12">
              Failed to load agent details
            </div>
          )}
        </div>
      </div>
      
      {/* Run Details Modal */}
      <RunDetailsModal 
        runId={selectedRunId} 
        isOpen={isRunModalOpen} 
        onClose={closeRunDetails} 
      />
    </div>
  );
};

// Custom Node Components (sized for better visibility and layout)
const AgentNode = ({ data }: NodeProps) => {
  const handleInfoClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onOpenModal && data.agentId) {
      data.onOpenModal(data.agentId);
    }
  };

  return (
    <div className="w-[280px] h-[130px] px-4 py-3 shadow-tokyo-blue border border-tokyo-blue7 bg-tokyo-bg-dark rounded-lg relative group">
      {/* Output handle on the right side */}
      <Handle type="source" position={Position.Right} className="w-3 h-3 bg-tokyo-blue" />
      
      {/* Info button - appears on hover */}
      <button
        onClick={handleInfoClick}
        className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-tokyo-blue hover:bg-tokyo-blue5 text-tokyo-bg"
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

const MCPNode = ({ data }: NodeProps) => {
  const handleInfoClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onOpenMCPModal && data.serverId) {
      data.onOpenMCPModal(data.serverId);
    } else {
      console.log('Opening MCP server details for:', data.serverId);
    }
  };

  const handleExpandClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    console.log('MCP Node expand button clicked for server:', data.serverId);
    if (data.onToggleExpand && data.serverId) {
      console.log('Calling onToggleExpand for server:', data.serverId);
      data.onToggleExpand(data.serverId);
    } else {
      console.log('Missing onToggleExpand function or serverId:', { onToggleExpand: !!data.onToggleExpand, serverId: data.serverId });
    }
  };

  const isExpanded = data.expanded || false;
  const toolCount = data.tools?.length || 0;

  return (
    <div className="w-[280px] h-[130px] px-4 py-3 shadow-tokyo-blue border border-tokyo-blue7 bg-tokyo-bg-dark rounded-lg relative group">
      {/* Input handle on the left side */}
      <Handle type="target" position={Position.Left} className="w-3 h-3 bg-tokyo-cyan" />
      {/* Output handle on the right side - only show when expanded */}
      {isExpanded && <Handle type="source" position={Position.Right} className="w-3 h-3 bg-tokyo-cyan" />}
      
      {/* Expand/Collapse button - always visible */}
      <button
        onClick={handleExpandClick}
        className="absolute top-2 left-2 p-1 rounded bg-tokyo-cyan hover:bg-tokyo-blue1 text-tokyo-bg transition-colors duration-200"
        title={isExpanded ? 'Collapse tools' : 'Expand tools'}
      >
        {isExpanded ? (
          <ChevronDown className="h-3 w-3" />
        ) : (
          <ChevronRight className="h-3 w-3" />
        )}
      </button>
      
      {/* Info button - appears on hover */}
      <button
        onClick={handleInfoClick}
        className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-tokyo-cyan hover:bg-tokyo-blue1 text-tokyo-bg"
        title="View MCP server details"
      >
        <Eye className="h-3 w-3" />
      </button>
      
      <div className="flex items-center gap-2 mb-2 ml-6">
        <Database className="h-5 w-5 text-tokyo-cyan" />
        <div className="font-mono text-base text-tokyo-cyan font-medium">{data.label}</div>
      </div>
      <div className="text-sm text-tokyo-comment mb-2 ml-6">{data.description}</div>
      <div className="text-sm text-tokyo-purple font-medium ml-6">
        {toolCount} tools {isExpanded ? 'expanded' : 'available'}
      </div>
    </div>
  );
};

const ToolNode = ({ data }: NodeProps) => {
  const handleInfoClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    console.log('Opening tool details for:', data.toolId);
  };

  return (
    <div className="w-[280px] h-[130px] px-4 py-3 shadow-tokyo-blue border border-tokyo-blue7 bg-tokyo-bg-dark rounded-lg relative group">
      {/* Input handle on the left side */}
      <Handle type="target" position={Position.Left} className="w-3 h-3 bg-tokyo-green" />
      
      {/* Info button - appears on hover */}
      <button
        onClick={handleInfoClick}
        className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-tokyo-green hover:bg-tokyo-green1 text-tokyo-bg"
        title="View tool details"
      >
        <Eye className="h-3 w-3" />
      </button>
      
      <div className="flex items-center gap-2 mb-2">
        <Settings className="h-5 w-5 text-tokyo-green" />
        <div className="font-mono text-base text-tokyo-green font-medium">{data.label}</div>
      </div>
      <div className="text-sm text-tokyo-comment mb-2">{data.description || 'Tool function'}</div>
      <div className="text-sm text-tokyo-blue1 font-medium">from {data.category}</div>
    </div>
  );
};

// Environment Node Component
const EnvironmentNode = ({ data }: NodeProps) => {
  return (
    <div className="w-[320px] h-[160px] px-4 py-3 shadow-tokyo-orange border border-tokyo-orange rounded-lg relative bg-tokyo-bg-dark">
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

const nodeTypes: NodeTypes = {
  agent: AgentNode,
  mcp: MCPNode,
  tool: ToolNode,
  environment: EnvironmentNode,
};

// Layout component using Tailwind classes
// Create a global environment context component
const EnvironmentProvider = ({ children }: { children: React.ReactNode }) => {
  const [environments, setEnvironments] = useState<any[]>([]);
  const [selectedEnvironment, setSelectedEnvironment] = useState<number | null>(null);
  const [refreshTrigger, setRefreshTrigger] = useState(0);

  // Fetch environments
  useEffect(() => {
    const fetchEnvironments = async () => {
      try {
        const response = await environmentsApi.getAll();
        const environmentsData = response.data.environments || [];
        setEnvironments(Array.isArray(environmentsData) ? environmentsData : []);
        if (Array.isArray(environmentsData) && environmentsData.length > 0) {
          setSelectedEnvironment(environmentsData[0].id);
        }
      } catch (error) {
        console.error('Failed to fetch environments:', error);
        setEnvironments([]); // Ensure environments is always an array
      }
    };
    fetchEnvironments();
  }, []);

  // Function to change environment and trigger data refresh
  const changeEnvironment = (envId: number) => {
    setSelectedEnvironment(envId);
    setRefreshTrigger(prev => prev + 1); // Increment to trigger refetch
  };

  const environmentContext = {
    environments: Array.isArray(environments) ? environments : [],
    selectedEnvironment,
    setSelectedEnvironment: changeEnvironment,
    refreshTrigger
  };

  return (
    <EnvironmentContext.Provider value={environmentContext}>
      {children}
    </EnvironmentContext.Provider>
  );
};

// Environment Context
const EnvironmentContext = React.createContext<any>({
  environments: [],
  selectedEnvironment: null,
  setSelectedEnvironment: () => {},
  refreshTrigger: 0
});

const Layout = ({ children, currentPage, onPageChange }: any) => {
  const environmentContext = React.useContext(EnvironmentContext);

  return (
    <div className="flex h-screen bg-tokyo-bg">
      <div className="w-64 bg-tokyo-bg-dark border-r border-tokyo-blue7 p-4">
        {/* Header with Station logo */}
        <div className="flex items-center gap-2 mb-4">
          <Train className="h-6 w-6 text-tokyo-blue" />
          <h2 className="text-lg font-mono font-semibold text-tokyo-fg">Station</h2>
        </div>

        {/* Global Environment Selector */}
        <div className="mb-6 p-3 bg-tokyo-bg border border-tokyo-blue7 rounded-lg">
          <label className="text-xs text-tokyo-comment font-mono block mb-2">Environment</label>
          <select 
            value={environmentContext?.selectedEnvironment || ''} 
            onChange={(e) => environmentContext?.setSelectedEnvironment(Number(e.target.value))}
            className="w-full px-2 py-1 bg-tokyo-bg-dark border border-tokyo-blue7 text-tokyo-fg font-mono text-sm rounded hover:border-tokyo-blue5 transition-colors"
          >
            <option value="">All Environments</option>
            {Array.isArray(environmentContext?.environments) ? environmentContext.environments.map((env: any) => (
              <option key={env.id} value={env.id}>
                {env.name}
              </option>
            )) : []}
          </select>
        </div>


        {/* Navigation */}
        <nav className="space-y-2">
          <button 
            onClick={() => onPageChange('agents')}
            className={`w-full text-left p-3 rounded border font-mono transition-colors ${
              currentPage === 'agents' 
                ? 'bg-tokyo-blue text-tokyo-bg border-tokyo-blue shadow-tokyo-glow' 
                : 'bg-transparent text-tokyo-fg-dark hover:bg-tokyo-bg-highlight hover:text-tokyo-blue border-transparent hover:border-tokyo-blue7'
            }`}
          >
            <Bot className="inline h-4 w-4 mr-2" />
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
            <Database className="inline h-4 w-4 mr-2" />
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
            <Play className="inline h-4 w-4 mr-2" />
            Runs
          </button>
          <button 
            onClick={() => onPageChange('users')}
            className={`w-full text-left p-3 rounded border font-mono transition-colors ${
              currentPage === 'users' 
                ? 'bg-tokyo-purple text-tokyo-bg border-tokyo-purple shadow-tokyo-glow' 
                : 'bg-transparent text-tokyo-fg-dark hover:bg-tokyo-bg-highlight hover:text-tokyo-purple border-transparent hover:border-tokyo-blue7'
            }`}
          >
            <Users className="inline h-4 w-4 mr-2" />
            Users
          </button>
          <button 
            onClick={() => onPageChange('environments')}
            className={`w-full text-left p-3 rounded border font-mono transition-colors ${
              currentPage === 'environments' 
                ? 'bg-tokyo-orange text-tokyo-bg border-tokyo-orange shadow-tokyo-glow' 
                : 'bg-transparent text-tokyo-fg-dark hover:bg-tokyo-bg-highlight hover:text-tokyo-orange border-transparent hover:border-tokyo-blue7'
            }`}
          >
            <Globe className="inline h-4 w-4 mr-2" />
            Environments
          </button>
          <button 
            onClick={() => onPageChange('bundles')}
            className={`w-full text-left p-3 rounded border font-mono transition-colors ${
              currentPage === 'bundles' 
                ? 'bg-tokyo-magenta text-tokyo-bg border-tokyo-magenta shadow-tokyo-glow' 
                : 'bg-transparent text-tokyo-fg-dark hover:bg-tokyo-bg-highlight hover:text-tokyo-magenta border-transparent hover:border-tokyo-blue7'
            }`}
          >
            <Package className="inline h-4 w-4 mr-2" />
            Bundles
          </button>
        </nav>
      </div>
      <div className="flex-1">
        {children}
      </div>
    </div>
  );
};

const AgentsCanvas = () => {
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
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
    console.log('Toggling server expansion for:', serverId);
    
    setExpandedServers(prevExpanded => {
      const newExpandedServers = new Set(prevExpanded);
      if (newExpandedServers.has(serverId)) {
        console.log('Collapsing server:', serverId);
        newExpandedServers.delete(serverId);
      } else {
        console.log('Expanding server:', serverId);
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

  const onConnect = useCallback(
    (params: Connection) => setEdges((eds) => addEdge({
      ...params,
      animated: true,
    }, eds)),
    [setEdges]
  );

  const onNodeClick = useCallback((_event: React.MouseEvent, node: Node) => {
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
          <ReactFlowProvider>
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
          </ReactFlowProvider>
        )}
      </div>
      
      {/* Agent Details Modal */}
      <AgentDetailsModal 
        agentId={modalAgentId} 
        isOpen={isModalOpen} 
        onClose={closeAgentModal} 
      />
      
      {/* MCP Server Details Modal */}
      <MCPServerDetailsModal 
        serverId={modalMCPServerId} 
        isOpen={isMCPModalOpen} 
        onClose={closeMCPServerModal} 
      />
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

const MCPServers = () => {
  const [isSyncing, setIsSyncing] = useState(false);
  const [mcpServers, setMcpServers] = useState<any[]>([]);
  const [modalMCPServerId, setModalMCPServerId] = useState<number | null>(null);
  const [isMCPModalOpen, setIsMCPModalOpen] = useState(false);
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

  // Fetch MCP servers data when environment changes
  useEffect(() => {
    const fetchMCPServers = async () => {
      try {
        if (environmentContext?.selectedEnvironment) {
          // Fetch servers for specific environment
          const response = await mcpServersApi.getByEnvironment(environmentContext.selectedEnvironment);
          setMcpServers(Array.isArray(response.data) ? response.data : []);
        } else {
          // No specific environment selected, show empty for now
          setMcpServers([]);
        }
      } catch (error) {
        console.error('Failed to fetch MCP servers:', error);
        setMcpServers([]);
      }
    };
    fetchMCPServers();
  }, [environmentContext?.selectedEnvironment, environmentContext?.refreshTrigger]);

  const handleSync = async () => {
    setIsSyncing(true);
    try {
      await syncApi.trigger();
      console.log('Sync completed successfully');
    } catch (error) {
      console.error('Sync failed:', error);
    } finally {
      setIsSyncing(false);
    }
  };

  // MCP servers are already filtered by environment on the backend
  const filteredServers = mcpServers || [];

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
        <h1 className="text-xl font-mono font-semibold text-tokyo-cyan">MCP Servers</h1>
        <div className="flex items-center gap-3">
          <button 
            onClick={handleSync}
            disabled={isSyncing}
            className="px-4 py-2 bg-gradient-to-r from-tokyo-purple to-tokyo-magenta text-tokyo-bg rounded font-mono font-bold hover:from-tokyo-magenta hover:to-tokyo-purple transition-all shadow-tokyo-glow flex items-center gap-2 disabled:opacity-50"
          >
            <RotateCw className={`h-4 w-4 ${isSyncing ? 'animate-spin' : ''}`} />
            {isSyncing ? 'Syncing...' : 'SYNC'}
          </button>
          <button className="px-4 py-2 bg-tokyo-cyan text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-blue1 transition-colors shadow-tokyo-glow flex items-center gap-2">
            <Plus className="h-4 w-4" />
            Add Server
          </button>
        </div>
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
                {/* Info button - appears on hover */}
                <button
                  onClick={() => openMCPServerModal(server.id)}
                  className="absolute top-3 right-3 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-tokyo-cyan hover:bg-tokyo-blue text-tokyo-bg"
                  title="View server details"
                >
                  <Eye className="h-4 w-4" />
                </button>
                
                <h3 className="font-mono font-medium text-tokyo-cyan">{server.name}</h3>
                <p className="text-sm text-tokyo-comment mt-1 font-mono">
                  {server.command} {server.args ? server.args.join(' ') : ''}
                </p>
                <div className="mt-2 flex gap-2">
                  <span className="px-2 py-1 bg-tokyo-green text-tokyo-bg text-xs rounded font-mono">Active</span>
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
    </div>
  );
};

const Runs = () => {
  const [runs, setRuns] = useState<any[]>([]);
  const [modalRunId, setModalRunId] = useState<number | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const environmentContext = React.useContext(EnvironmentContext);

  // Function to open run details modal
  const openRunModal = (runId: number) => {
    setModalRunId(runId);
    setIsModalOpen(true);
  };

  const closeRunModal = () => {
    setIsModalOpen(false);
    setModalRunId(null);
  };

  // Fetch runs data (not affected by environment changes - shows all runs)
  useEffect(() => {
    const fetchRuns = async () => {
      try {
        const response = await agentRunsApi.getAll();
        setRuns(response.data.runs || []);
      } catch (error) {
        console.error('Failed to fetch runs:', error);
        setRuns([]);
      }
    };
    fetchRuns();
  }, [environmentContext?.refreshTrigger]); // Only refetch on manual refresh, not environment change

  // Show all runs regardless of environment selection
  const filteredRuns = runs || [];

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
        <h1 className="text-xl font-mono font-semibold text-tokyo-green">Agent Runs</h1>
      </div>
      <div className="flex-1 p-4 overflow-y-auto">
        {filteredRuns.length === 0 ? (
          <div className="h-full flex items-center justify-center">
            <div className="text-center">
              <Play className="h-16 w-16 text-tokyo-comment mx-auto mb-4" />
              <div className="text-tokyo-fg font-mono text-lg mb-2">No agent runs found</div>
              <div className="text-tokyo-comment font-mono text-sm">
                Agent executions will appear here when agents are run
              </div>
            </div>
          </div>
        ) : (
          <div className="grid gap-4 max-h-full overflow-y-auto">
            {filteredRuns.map((run) => (
              <div key={run.id} className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo">
                <div className="flex items-center justify-between">
                  <h3 className="font-mono font-medium text-tokyo-green">{run.agent_name}</h3>
                  <div className="flex items-center gap-3">
                    <button
                      onClick={() => openRunModal(run.id)}
                      className="p-2 rounded-lg hover:bg-tokyo-bg-highlight text-tokyo-comment hover:text-tokyo-blue transition-colors"
                      title="View run details"
                    >
                      <Eye className="h-4 w-4" />
                    </button>
                    <span className={`px-2 py-1 text-xs rounded font-mono ${
                      run.status === 'completed' ? 'bg-tokyo-green text-tokyo-bg' :
                      run.status === 'running' ? 'bg-tokyo-blue text-tokyo-bg' :
                      'bg-tokyo-red text-tokyo-bg'
                    }`}>
                      {run.status}
                    </span>
                  </div>
                </div>
                <div className="mt-2 flex gap-4 text-sm text-tokyo-comment font-mono">
                  <span>Duration: {run.execution_time}</span>
                  <span>Time: {run.timestamp}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
      
      {/* Run Details Modal */}
      <RunDetailsModal 
        runId={modalRunId} 
        isOpen={isModalOpen} 
        onClose={closeRunModal} 
      />
    </div>
  );
};

const UsersPage = () => {
  const [users, setUsers] = useState<any[]>([]);
  const environmentContext = React.useContext(EnvironmentContext);

  // Fetch users data
  useEffect(() => {
    const fetchUsers = async () => {
      try {
        const response = await usersApi.getAll();
        setUsers(Array.isArray(response.data) ? response.data : []);
      } catch (error) {
        console.error('Failed to fetch users:', error);
        setUsers([]);
      }
    };
    fetchUsers();
  }, [environmentContext?.refreshTrigger]);

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
        <h1 className="text-xl font-mono font-semibold text-tokyo-purple">Users</h1>
        <button className="px-4 py-2 bg-tokyo-purple text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-purple transition-colors shadow-tokyo-glow flex items-center gap-2">
          <Plus className="h-4 w-4" />
          Add User
        </button>
      </div>
      <div className="flex-1 p-4 overflow-y-auto">
        {users.length === 0 ? (
          <div className="h-full flex items-center justify-center">
            <div className="text-center">
              <Users className="h-16 w-16 text-tokyo-comment mx-auto mb-4" />
              <div className="text-tokyo-fg font-mono text-lg mb-2">No users found</div>
              <div className="text-tokyo-comment font-mono text-sm">
                User accounts will appear here when created
              </div>
            </div>
          </div>
        ) : (
          <div className="grid gap-4 max-h-full overflow-y-auto">
            {users.map((user) => (
              <div key={user.id} className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo">
                <div className="flex items-center gap-3">
                  <Users className="h-8 w-8 text-tokyo-purple" />
                  <div>
                    <h3 className="font-mono font-medium text-tokyo-purple">{user.username || user.name}</h3>
                    <p className="text-sm text-tokyo-comment mt-1 font-mono">{user.email || 'No email specified'}</p>
                  </div>
                </div>
                <div className="mt-2 flex gap-2">
                  <span className="px-2 py-1 bg-tokyo-green text-tokyo-bg text-xs rounded font-mono">
                    {user.is_active ? 'Active' : 'Inactive'}
                  </span>
                  <span className="px-2 py-1 bg-tokyo-purple text-tokyo-bg text-xs rounded font-mono">
                    {user.is_admin ? 'Admin' : 'User'}
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

// Environments Page Component
const EnvironmentsPage = () => {
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [environments, setEnvironments] = useState<any[]>([]);
  const [selectedEnvironment, setSelectedEnvironment] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const environmentContext = React.useContext(EnvironmentContext);

  // Fetch environments data
  useEffect(() => {
    const fetchEnvironments = async () => {
      try {
        const response = await environmentsApi.getAll();
        const environmentsData = response.data.environments || [];
        setEnvironments(Array.isArray(environmentsData) ? environmentsData : []);
        if (Array.isArray(environmentsData) && environmentsData.length > 0) {
          setSelectedEnvironment(environmentsData[0].id);
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

  // Generate nodes and edges for the selected environment
  useEffect(() => {
    if (!selectedEnvironment) return;

    const generateEnvironmentGraph = async () => {
      try {
        const [agentsResponse, mcpServersResponse] = await Promise.all([
          agentsApi.getByEnvironment(selectedEnvironment),
          mcpServersApi.getByEnvironment(selectedEnvironment)
        ]);

        const agents = Array.isArray(agentsResponse.data) ? agentsResponse.data : [];
        const mcpServers = Array.isArray(mcpServersResponse.data) ? mcpServersResponse.data : [];

        const newNodes = [];
        const newEdges = [];

        // Create environment node (center)
        const selectedEnv = environments.find(env => env.id === selectedEnvironment);
        newNodes.push({
          id: `env-${selectedEnvironment}`,
          type: 'environment',
          position: { x: 0, y: 0 },
          data: {
            label: selectedEnv?.name || 'Environment',
            description: selectedEnv?.description || 'Environment Hub',
            agentCount: agents.length,
            serverCount: mcpServers.length,
          },
        });

        // Create agent nodes
        agents.forEach((agent, index) => {
          newNodes.push({
            id: `agent-${agent.id}`,
            type: 'agent',
            position: { x: 0, y: 0 },
            data: {
              label: agent.name,
              description: agent.description,
              status: agent.is_scheduled ? 'Scheduled' : 'Manual',
              agentId: agent.id,
            },
          });

          // Connect agent to environment
          newEdges.push({
            id: `edge-env-${selectedEnvironment}-to-agent-${agent.id}`,
            source: `env-${selectedEnvironment}`,
            target: `agent-${agent.id}`,
            animated: true,
            style: { stroke: '#7aa2f7', strokeWidth: 2 },
          });
        });

        // Create MCP server nodes
        mcpServers.forEach((server, index) => {
          newNodes.push({
            id: `mcp-${server.id}`,
            type: 'mcp',
            position: { x: 0, y: 0 },
            data: {
              label: server.name,
              description: 'MCP Server',
              serverId: server.id,
            },
          });

          // Connect MCP server to environment
          newEdges.push({
            id: `edge-env-${selectedEnvironment}-to-mcp-${server.id}`,
            source: `env-${selectedEnvironment}`,
            target: `mcp-${server.id}`,
            animated: true,
            style: { stroke: '#0db9d7', strokeWidth: 2 },
          });
        });

        // Layout the nodes
        const layoutedElements = await getLayoutedNodes(newNodes, newEdges);
        setNodes(layoutedElements.nodes);
        setEdges(layoutedElements.edges);
      } catch (error) {
        console.error('Failed to generate environment graph:', error);
      }
    };

    generateEnvironmentGraph();
  }, [selectedEnvironment, environments]);

  const onConnect = useCallback(
    (params: any) => setEdges((eds) => addEdge(params, eds)),
    [setEdges]
  );

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
        <h1 className="text-xl font-mono font-semibold text-tokyo-orange">Environments</h1>
        <div className="flex items-center gap-3">
          <select
            value={selectedEnvironment || ''}
            onChange={(e) => setSelectedEnvironment(Number(e.target.value))}
            className="px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 rounded font-mono text-tokyo-fg focus:outline-none focus:border-tokyo-orange"
          >
            {environments.map((env) => (
              <option key={env.id} value={env.id}>
                {env.name}
              </option>
            ))}
          </select>
          <button className="px-4 py-2 bg-tokyo-orange text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-orange5 transition-colors shadow-tokyo-glow flex items-center gap-2">
            <Plus className="h-4 w-4" />
            Add Environment
          </button>
        </div>
      </div>
      
      <div className="flex-1">
        {loading ? (
          <div className="flex items-center justify-center h-full">
            <div className="text-tokyo-comment font-mono">Loading environments...</div>
          </div>
        ) : (
          <ReactFlowProvider>
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onConnect={onConnect}
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
            />
          </ReactFlowProvider>
        )}
      </div>
    </div>
  );
};

// Bundles Page Component
const BundlesPage = () => {
  const [bundles, setBundles] = useState<any[]>([]);
  const environmentContext = React.useContext(EnvironmentContext);

  // Fetch bundles data (placeholder for now)
  useEffect(() => {
    // TODO: Implement bundles API
    setBundles([]);
  }, [environmentContext?.refreshTrigger]);

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
        <h1 className="text-xl font-mono font-semibold text-tokyo-magenta">Bundles</h1>
        <div className="flex items-center gap-3">
          <button className="px-4 py-2 bg-tokyo-magenta text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-purple transition-colors shadow-tokyo-glow flex items-center gap-2">
            <Plus className="h-4 w-4" />
            Add Bundle
          </button>
        </div>
      </div>
      
      <div className="flex-1 p-4">
        <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-8 text-center">
          <Package className="h-16 w-16 text-tokyo-comment mx-auto mb-4" />
          <h2 className="text-lg font-mono font-medium text-tokyo-fg mb-2">No bundles found</h2>
          <p className="text-tokyo-comment font-mono">Create your first bundle to get started with agent templates and configurations.</p>
        </div>
      </div>
    </div>
  );
};

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      refetchOnWindowFocus: false,
    },
  },
});

type Page = 'agents' | 'mcps' | 'runs' | 'users' | 'environments' | 'bundles';

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
      case 'users':
        return <UsersPage />;
      case 'environments':
        return <EnvironmentsPage />;
      case 'bundles':
        return <BundlesPage />;
      default:
        return <AgentsCanvas />;
    }
  };

  return (
    <QueryClientProvider client={queryClient}>
      <EnvironmentProvider>
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
      </EnvironmentProvider>
    </QueryClientProvider>
  );
}

export default App

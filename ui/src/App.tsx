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
import { BrowserRouter, Routes, Route, useNavigate, useParams } from 'react-router-dom';
import { Bot, Database, Settings, Plus, Eye, X, Play, ChevronDown, ChevronRight, Train, Globe, Package, Download, Copy, Edit, ArrowLeft, Save } from 'lucide-react';
import '@xyflow/react/dist/style.css';
import Editor from '@monaco-editor/react';
import { agentsApi, environmentsApi, syncApi, agentRunsApi, mcpServersApi, bundlesApi, type BundleInfo } from './api/station';
import { RunsPage } from './components/runs/RunsPage';
import CloudShipStatus from './components/CloudShipStatus';
import { SyncModal } from './components/sync/SyncModal';
import { apiClient } from './api/client';
import { getLayoutedNodes } from './utils/layoutUtils';
// Import node components when needed
// import { AgentNode } from './components/nodes/AgentNode';
// import { MCPNode } from './components/nodes/MCPNode';
// import { ToolNode } from './components/nodes/ToolNode';

// Station Banner Component
const StationBanner = () => (
  <div className="flex items-start justify-between">
    <div className="font-mono leading-tight">
      <div className="text-2xl font-bold bg-gradient-to-r from-tokyo-blue via-tokyo-cyan to-tokyo-blue5 bg-clip-text text-transparent">
        STATION
      </div>
      <div className="text-tokyo-comment text-xs mt-1 italic">🚂 agents for engineers. Be in control</div>
      <div className="text-tokyo-dark5 text-xs">by CloudshipAI</div>
    </div>
    <div className="mt-1">
      <CloudShipStatus />
    </div>
  </div>
);

// Run Details Modal Component
const RunDetailsModal = ({ runId, isOpen, onClose }: { runId: number | null, isOpen: boolean, onClose: () => void }) => {
  const [runDetails, setRunDetails] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [logs, setLogs] = useState<any[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const [autoRefreshing, setAutoRefreshing] = useState(false);

  // Fetch run details with auto-refresh for running agents
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

    // Initial fetch
    fetchRunDetails();

    // Set up auto-refresh for running agents
    const pollRunDetails = async () => {
      setAutoRefreshing(true);
      try {
        const response = await agentRunsApi.getById(runId);
        setRunDetails(response.data.run);
      } catch (error) {
        console.error('Failed to refresh run details:', error);
      } finally {
        setAutoRefreshing(false);
      }
    };

    // Check if we need to poll (run is still running)
    if (runDetails?.status === 'running') {
      setAutoRefreshing(true);
      const interval = setInterval(pollRunDetails, 3000); // Poll every 3 seconds
      return () => {
        clearInterval(interval);
        setAutoRefreshing(false);
      };
    } else {
      setAutoRefreshing(false);
    }
  }, [runId, isOpen, runDetails?.status]);

  // Fetch logs when logs panel is opened
  useEffect(() => {
    if (!isOpen || !runId || !showLogs) return;

    const fetchLogs = async () => {
      setLogsLoading(true);
      try {
        const response = await apiClient.get(`/runs/${runId}/logs?limit=100`);
        console.log('Logs API response:', response.data); // Debug logging
        setLogs(response.data.logs || []);
      } catch (error) {
        console.error('Failed to fetch logs:', error);
        setLogs([]);
      } finally {
        setLogsLoading(false);
      }
    };

    // Always fetch logs when the panel is opened
    fetchLogs();
  }, [runId, isOpen, showLogs]);

  // Separate polling effect for running agents
  useEffect(() => {
    if (!isOpen || !runId || !showLogs || !runDetails) return;

    const isRunning = runDetails.status === 'running';
    if (!isRunning) return;

    const fetchLogs = async () => {
      try {
        const response = await apiClient.get(`/runs/${runId}/logs?limit=100`);
        setLogs(response.data.logs || []);
      } catch (error) {
        console.error('Failed to fetch logs during polling:', error);
      }
    };

    const interval = setInterval(fetchLogs, 2000); // Poll every 2 seconds
    return () => clearInterval(interval);
  }, [runId, isOpen, showLogs, runDetails]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 backdrop-blur-sm z-50 flex items-center justify-center p-4">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo-lg max-w-4xl w-full max-h-[90vh] overflow-auto">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-tokyo-blue7">
          <div className="flex items-center gap-3">
            <Play className="h-6 w-6 text-tokyo-green" />
            <h2 className="text-xl font-mono font-semibold text-tokyo-green">Run Details</h2>
            {autoRefreshing && (
              <div className="flex items-center gap-2 text-tokyo-comment">
                <div className="w-4 h-4 border-2 border-tokyo-comment border-t-transparent rounded-full animate-spin"></div>
                <span className="text-sm font-mono">Auto-refreshing...</span>
              </div>
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setShowLogs(!showLogs)}
              className={`px-3 py-2 rounded-lg font-mono text-sm transition-colors ${
                showLogs 
                  ? 'bg-tokyo-green text-tokyo-bg hover:bg-tokyo-green/80' 
                  : 'bg-tokyo-bg-highlight text-tokyo-comment hover:text-tokyo-green hover:bg-tokyo-bg-highlight/80'
              }`}
            >
              {showLogs ? '🟢 Live Logs' : '⚫ Show Logs'}
            </button>
            <button
              onClick={onClose}
              className="p-2 rounded-lg hover:bg-tokyo-bg-highlight text-tokyo-comment hover:text-tokyo-fg transition-colors"
            >
              <X className="h-5 w-5" />
            </button>
          </div>
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
                </div>
              </div>

              {/* Token Usage */}
              <div>
                <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Token Usage</h3>
                <div className="grid grid-cols-3 gap-4">
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


              {/* Execution Steps */}
              {runDetails.execution_steps && runDetails.execution_steps.length > 0 && (
                <div>
                  <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-3">Execution Steps</h3>
                  <div className="space-y-2">
                    {(runDetails.execution_steps || []).map((step: any, index: number) => (
                      <div key={index} className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4">
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-tokyo-comment font-mono text-sm">
                            Step {step.step || index + 1} - {step.type || 'Unknown'}
                          </span>
                          {step.timestamp && (
                            <span className="text-xs text-tokyo-comment">
                              {new Date(step.timestamp).toLocaleTimeString()}
                            </span>
                          )}
                        </div>
                        <div className="text-tokyo-fg font-mono whitespace-pre-wrap">
                          {step.type === 'model_reasoning' && (
                            <div>
                              <div className="text-tokyo-purple mb-2">🤔 Model Reasoning</div>
                              <div className="text-tokyo-fg">
                                {step.content}
                              </div>
                            </div>
                          )}
                          {!step.type && (
                            <div className="text-tokyo-comment">
                              {step.action || step.response || step.content || 'No content available'}
                            </div>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Live Logs Section */}
              {showLogs && (
                <div>
                  <div className="flex items-center justify-between mb-3">
                    <h3 className="text-lg font-mono font-medium text-tokyo-fg">Live Execution Logs</h3>
                    {logsLoading && (
                      <div className="flex items-center gap-2 text-tokyo-comment">
                        <div className="w-4 h-4 border-2 border-tokyo-comment border-t-transparent rounded-full animate-spin"></div>
                        <span className="text-sm font-mono">Loading logs...</span>
                      </div>
                    )}
                  </div>
                  <div className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4 max-h-96 overflow-auto">
                    {logs.length === 0 ? (
                      <div className="text-center py-8">
                        <div className="text-tokyo-comment font-mono">
                          {logsLoading ? 'Loading logs...' : 'No logs available'}
                        </div>
                      </div>
                    ) : (
                      <div className="space-y-2">
                        {logs.map((log, index) => (
                          <div key={index} className="border-b border-tokyo-blue7/30 pb-2 last:border-b-0">
                            <div className="flex items-center justify-between mb-1">
                              <span className={`text-xs font-mono px-2 py-1 rounded ${
                                log.level === 'error' ? 'bg-tokyo-red/20 text-tokyo-red' :
                                log.level === 'warning' ? 'bg-tokyo-orange/20 text-tokyo-orange' :
                                log.level === 'info' ? 'bg-tokyo-blue/20 text-tokyo-blue' :
                                'bg-tokyo-comment/20 text-tokyo-comment'
                              }`}>
                                {log.level?.toUpperCase() || 'LOG'}
                              </span>
                              <span className="text-xs text-tokyo-comment font-mono">
                                {log.timestamp ? new Date(log.timestamp).toLocaleTimeString() : ''}
                              </span>
                            </div>
                            <div className="text-sm text-tokyo-fg font-mono leading-relaxed">
                              {log.message}
                            </div>
                            {log.details && (
                              <div className="mt-2 text-xs text-tokyo-comment bg-tokyo-bg-dark rounded p-2">
                                <pre className="whitespace-pre-wrap">
                                  {typeof log.details === 'string' ? log.details : JSON.stringify(log.details, null, 2)}
                                </pre>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                  {runDetails?.status === 'running' && (
                    <div className="mt-2 text-xs text-tokyo-comment font-mono flex items-center gap-2">
                      <div className="w-2 h-2 bg-tokyo-green rounded-full animate-pulse"></div>
                      Auto-refreshing every 2 seconds...
                    </div>
                  )}
                  {runDetails?.status === 'completed' && (
                    <div className="mt-2 text-xs text-tokyo-comment font-mono">
                      Execution completed - logs are final
                    </div>
                  )}
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


// Environment-specific Node Components (simplified for hub visualization)
const EnvironmentAgentNode = ({ data }: { data: any }) => {
  return (
    <div className="w-[240px] h-[100px] px-3 py-2 shadow-tokyo-blue border border-tokyo-blue7 bg-tokyo-bg-dark rounded-lg relative">
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

const EnvironmentMCPNode = ({ data }: { data: any }) => {
  return (
    <div className="w-[240px] h-[100px] px-3 py-2 shadow-tokyo-cyan border border-tokyo-blue7 bg-tokyo-bg-dark rounded-lg relative">
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

// Bundle Environment Modal Component
const BundleEnvironmentModal = ({ 
  isOpen, 
  onClose, 
  environmentName 
}: { 
  isOpen: boolean; 
  onClose: () => void; 
  environmentName: string;
}) => {
  const [endpoint, setEndpoint] = useState('https://share.cloudshipai.com/upload');
  const [isLocal, setIsLocal] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [response, setResponse] = useState<any>(null);

  const handleBundle = async () => {
    setIsLoading(true);
    setResponse(null);
    
    try {
      const bundleData = {
        environment: environmentName,
        local: isLocal,
        endpoint: isLocal ? undefined : endpoint
      };

      // Call the bundles API (to be implemented)
      const result = await apiClient.post('/bundles', bundleData);
      setResponse(result.data);
    } catch (error) {
      console.error('Failed to create bundle:', error);
      setResponse({ error: 'Failed to create bundle' });
    } finally {
      setIsLoading(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-[9999]">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo-glow max-w-md w-full mx-4 z-[10000] relative max-h-[90vh] overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark rounded-t-lg">
          <h2 className="text-lg font-mono font-semibold text-tokyo-fg z-10 relative">
            Bundle Environment: {environmentName}
          </h2>
          <button onClick={onClose} className="text-tokyo-comment hover:text-tokyo-fg transition-colors z-10 relative">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-4 space-y-4 overflow-y-auto flex-1">
          {/* Warning */}
          <div className="bg-yellow-900 bg-opacity-30 border border-yellow-500 border-opacity-50 rounded p-3">
            <p className="text-sm text-yellow-300 font-mono">
              Note: Make sure your MCP servers are templates. Your variables.yml will not be part of this bundle.
            </p>
          </div>

          {/* Local Toggle */}
          <div className="flex items-center gap-3">
            <input
              type="checkbox"
              id="local-toggle"
              checked={isLocal}
              onChange={(e) => setIsLocal(e.target.checked)}
              className="w-4 h-4 text-tokyo-orange bg-tokyo-bg border-tokyo-blue7 rounded focus:ring-tokyo-orange focus:ring-2"
            />
            <label htmlFor="local-toggle" className="text-sm font-mono text-tokyo-fg">
              Save locally (skip upload)
            </label>
          </div>

          {/* Endpoint Input - Hidden when local is selected */}
          {!isLocal && (
            <div className="space-y-2">
              <label className="text-sm font-mono text-tokyo-comment">Upload Endpoint:</label>
              <input
                type="text"
                value={endpoint}
                onChange={(e) => setEndpoint(e.target.value)}
                className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 rounded font-mono text-tokyo-fg focus:outline-none focus:border-tokyo-orange"
                placeholder="https://share.cloudshipai.com/upload"
              />
            </div>
          )}

          {/* Response Display */}
          {response && (
            <div className="space-y-3">
              {/* Success Response for share.cloudshipai.com */}
              {response.success && endpoint.includes('share.cloudshipai.com') && response.share_url && (
                <div className="bg-green-900 bg-opacity-30 border border-green-500 border-opacity-50 rounded p-4">
                  <div className="flex items-center justify-between mb-3">
                    <h4 className="text-sm font-mono text-white font-medium">Bundle Shared Successfully</h4>
                    <button
                      onClick={() => navigator.clipboard.writeText(response.share_url)}
                      className="p-1 text-green-400 hover:text-green-300 transition-colors"
                      title="Copy share URL"
                    >
                      <Copy className="h-4 w-4" />
                    </button>
                  </div>
                  
                  <div className="space-y-3">
                    <div>
                      <div className="text-xs text-green-400 font-mono mb-1 font-medium">Share URL:</div>
                      <div className="p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200 break-all">
                        {response.share_url}
                      </div>
                    </div>
                    
                    {response.share_id && (
                      <div>
                        <div className="text-xs text-green-400 font-mono mb-1 font-medium">Share ID:</div>
                        <div className="p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200">
                          {response.share_id}
                        </div>
                      </div>
                    )}
                    
                    {response.expires && (
                      <div>
                        <div className="text-xs text-green-400 font-mono mb-1 font-medium">Expires:</div>
                        <div className="p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200">
                          {response.expires}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}
              
              {/* Local bundle success */}
              {response.success && response.local_path && (
                <div className="bg-blue-900 bg-opacity-30 border border-blue-500 border-opacity-50 rounded p-4">
                  <h4 className="text-sm font-mono text-white font-medium mb-3">Bundle Saved Locally</h4>
                  <div>
                    <div className="text-xs text-blue-400 font-mono mb-1 font-medium">Local Path:</div>
                    <div className="p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200 break-all">
                      {response.local_path}
                    </div>
                  </div>
                </div>
              )}
              
              {/* Error response */}
              {response.error && (
                <div className="bg-red-900 bg-opacity-30 border border-red-500 border-opacity-50 rounded p-4">
                  <h4 className="text-sm font-mono text-red-400 font-medium mb-2">Error</h4>
                  <div className="text-xs text-red-400 font-mono">
                    {response.error}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="p-4 border-t border-tokyo-blue7">
          <button
            onClick={handleBundle}
            disabled={isLoading}
            className="w-full px-4 py-2 bg-tokyo-orange text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-orange5 transition-colors shadow-tokyo-glow disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            {isLoading ? (
              <>
                <div className="animate-spin rounded-full h-4 w-4 border-2 border-tokyo-bg border-t-transparent"></div>
                Creating Bundle...
              </>
            ) : (
              <>
                <Package className="h-4 w-4" />
                Bundle
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
};

// Environment Node Component
const EnvironmentNode = ({ data }: { data: any }) => {
  const handleCopySync = (e: React.MouseEvent) => {
    e.stopPropagation();
    const syncCommand = `stn sync ${data.label}`;
    navigator.clipboard.writeText(syncCommand);
  };

  return (
    <div className="w-[320px] h-[160px] px-4 py-3 shadow-tokyo-orange border border-tokyo-orange rounded-lg relative bg-tokyo-bg-dark group">
      {/* Output handles for connecting to agents and MCP servers */}
      <Handle
        type="source"
        position={Position.Right}
        style={{ background: '#ff9e64', width: 12, height: 12 }}
      />
      <Handle
        type="source"
        position={Position.Bottom}
        style={{ background: '#7dcfff', width: 12, height: 12 }}
      />
      
      {/* Copy sync command button - appears on hover */}
      <button
        onClick={handleCopySync}
        className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-tokyo-orange hover:bg-tokyo-orange1 text-tokyo-bg"
        title={`Copy sync command: stn sync ${data.label}`}
      >
        <Copy className="h-3 w-3" />
      </button>
      
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

// Custom Agent Node for agents page (with full functionality)
const AgentsPageAgentNode = ({ data }: { data: any }) => {
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

const AgentsPageMCPNode = ({ data }: { data: any }) => {
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

const AgentsPageToolNode = ({ data }: { data: any }) => {
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

// Node types for the main agents page (with full functionality)
const agentPageNodeTypes: NodeTypes = {
  agent: AgentsPageAgentNode,
  mcp: AgentsPageMCPNode,
  tool: AgentsPageToolNode,
  environment: EnvironmentNode,
};

// Node types for the environments page (simplified hub visualization)
const environmentPageNodeTypes: NodeTypes = {
  agent: EnvironmentAgentNode,
  mcp: EnvironmentMCPNode,
  tool: AgentsPageToolNode,
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
          <button 
            onClick={() => onPageChange('ship')}
            className={`w-full text-left p-3 rounded border font-mono transition-colors ${
              currentPage === 'ship' 
                ? 'bg-tokyo-yellow text-tokyo-bg border-tokyo-yellow shadow-tokyo-glow' 
                : 'bg-transparent text-tokyo-fg-dark hover:bg-tokyo-bg-highlight hover:text-tokyo-yellow border-transparent hover:border-tokyo-blue7'
            }`}
          >
            🚢 Ship CLI
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
              nodeTypes={agentPageNodeTypes}
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

// Add Server Modal Component
const AddServerModal = ({ 
  isOpen, 
  onClose, 
  environmentName 
}: { 
  isOpen: boolean; 
  onClose: () => void; 
  environmentName: string;
}) => {
  const [serverName, setServerName] = useState('');
  const [serverConfig, setServerConfig] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [response, setResponse] = useState<any>(null);
  const [showSuccess, setShowSuccess] = useState(false);

  const defaultConfig = `{
  "mcpServers": {
    "filesystem": {
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest",
        "{{ .ROOT_PATH }}"
      ],
      "autoApprove": [],
      "command": "npx",
      "disabled": false
    }
  }
}`;

  const handleSubmit = async () => {
    if (!serverName.trim() || !serverConfig.trim()) {
      setResponse({ error: 'Server name and config are required' });
      return;
    }

    setIsLoading(true);
    setResponse(null);
    
    try {
      const result = await apiClient.post('/mcp-servers', {
        name: serverName,
        config: serverConfig,
        environment: environmentName
      });
      setResponse(result.data);
      setShowSuccess(true);
    } catch (error) {
      console.error('Failed to create MCP server:', error);
      setResponse({ error: 'Failed to create MCP server' });
    } finally {
      setIsLoading(false);
    }
  };

  const resetModal = () => {
    setServerName('');
    setServerConfig('');
    setResponse(null);
    setShowSuccess(false);
    setIsLoading(false);
  };

  const handleClose = () => {
    resetModal();
    onClose();
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-[9999]">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo-glow max-w-4xl w-full mx-4 z-[10000] relative max-h-[90vh] overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark rounded-t-lg">
          <h2 className="text-lg font-mono font-semibold text-white z-10 relative">
            Add MCP Server: {environmentName}
          </h2>
          <button onClick={handleClose} className="text-tokyo-comment hover:text-tokyo-fg transition-colors z-10 relative">
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-6 overflow-y-auto flex-1">
          {!showSuccess ? (
            <>
              {/* Server Name Input */}
              <div className="space-y-2">
                <label className="text-sm font-mono text-tokyo-cyan font-medium">Server Name:</label>
                <input
                  type="text"
                  value={serverName}
                  onChange={(e) => setServerName(e.target.value)}
                  className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 rounded font-mono text-tokyo-fg focus:outline-none focus:border-tokyo-cyan"
                  placeholder="e.g., filesystem, database, etc."
                />
              </div>

              {/* Server Config Input */}
              <div className="space-y-2">
                <label className="text-sm font-mono text-tokyo-cyan font-medium">Server Configuration:</label>
                <div className="border border-tokyo-blue7 rounded overflow-hidden">
                  <Editor
                    height="320px"
                    defaultLanguage="json"
                    theme="vs-dark"
                    value={serverConfig}
                    onChange={(value) => setServerConfig(value || '')}
                    options={{
                      minimap: { enabled: false },
                      fontSize: 12,
                      fontFamily: '"Fira Code", "Consolas", "Monaco", monospace',
                      lineNumbers: 'on',
                      folding: true,
                      wordWrap: 'on',
                      automaticLayout: true,
                      tabSize: 2,
                      insertSpaces: true,
                      bracketPairColorization: { enabled: true },
                      suggest: { showSnippets: false },
                      quickSuggestions: false,
                      parameterHints: { enabled: false },
                      hover: { enabled: false },
                      contextmenu: false,
                      scrollBeyondLastLine: false,
                      renderLineHighlight: 'all',
                      selectOnLineNumbers: true,
                    }}
                  />
                </div>
              </div>

              {/* Documentation Note */}
              <div className="bg-blue-900 bg-opacity-30 border border-blue-500 border-opacity-50 rounded p-4">
                <p className="text-sm text-blue-300 font-mono">
                  <strong>Note:</strong> Replace any arguments you want as variables with <code className="bg-gray-800 px-1 rounded">{'{{ .VAR }}'}</code> Go variable notation.{' '}
                  <a 
                    href="https://cloudshipai.github.io/station/en/mcp/overview/" 
                    target="_blank" 
                    rel="noopener noreferrer"
                    className="text-blue-400 underline hover:text-blue-300"
                  >
                    More info here
                  </a>
                </p>
              </div>

              {/* Error Display */}
              {response?.error && (
                <div className="bg-red-900 bg-opacity-30 border border-red-500 border-opacity-50 rounded p-4">
                  <h4 className="text-sm font-mono text-red-400 font-medium mb-2">Error</h4>
                  <div className="text-xs text-red-400 font-mono">
                    {response.error}
                  </div>
                </div>
              )}
            </>
          ) : (
            /* Success Card */
            <div className="space-y-4">
              <div className="bg-green-900 bg-opacity-30 border border-green-500 border-opacity-50 rounded p-6 text-center">
                <h3 className="text-lg font-mono text-white font-medium mb-4">MCP Server Created Successfully!</h3>
                
                <div className="space-y-3 text-left">
                  <div>
                    <span className="text-xs text-green-400 font-mono font-medium">Server Name:</span>
                    <div className="mt-1 p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200">
                      {serverName}
                    </div>
                  </div>
                  
                  <div>
                    <span className="text-xs text-green-400 font-mono font-medium">Environment:</span>
                    <div className="mt-1 p-2 bg-gray-900 border border-gray-600 rounded font-mono text-xs text-gray-200">
                      {environmentName}
                    </div>
                  </div>
                </div>
              </div>

              {/* Next Steps */}
              <div className="bg-blue-900 bg-opacity-30 border border-blue-500 border-opacity-50 rounded p-4">
                <h4 className="text-sm font-mono text-blue-400 font-medium mb-3">Next Steps</h4>
                <p className="text-xs text-blue-300 font-mono mb-3">
                  Sync this config and input your variables:
                </p>
                
                <div className="bg-gray-900 border border-gray-600 rounded p-3 flex items-center justify-between">
                  <code className="text-xs text-gray-200 font-mono">stn sync</code>
                  <button
                    onClick={() => navigator.clipboard.writeText('stn sync')}
                    className="p-1 text-blue-400 hover:text-blue-300 transition-colors"
                    title="Copy command"
                  >
                    <Copy className="h-4 w-4" />
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        {!showSuccess && (
          <div className="p-4 border-t border-tokyo-blue7">
            <button
              onClick={handleSubmit}
              disabled={isLoading || !serverName.trim() || !serverConfig.trim()}
              className="w-full px-4 py-2 bg-tokyo-cyan text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-blue1 transition-colors shadow-tokyo-glow disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            >
              {isLoading ? (
                <>
                  <div className="animate-spin rounded-full h-4 w-4 border-2 border-tokyo-bg border-t-transparent"></div>
                  Creating Server...
                </>
              ) : (
                <>
                  <Plus className="h-4 w-4" />
                  Create Server
                </>
              )}
            </button>
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

const MCPServers = () => {
  const [isSyncing, setIsSyncing] = useState(false);
  const [mcpServers, setMcpServers] = useState<any[]>([]);
  const [modalMCPServerId, setModalMCPServerId] = useState<number | null>(null);
  const [isMCPModalOpen, setIsMCPModalOpen] = useState(false);
  const [environments, setEnvironments] = useState<any[]>([]);
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
                
                {/* Error message */}
                {server.error && (
                  <div className="mt-2 p-2 bg-tokyo-red bg-opacity-20 border border-tokyo-red border-opacity-50 rounded">
                    <p className="text-sm text-tokyo-red font-mono">{server.error}</p>
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
    </div>
  );
};

const Runs = () => {
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

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <RunsPage onRunClick={openRunModal} refreshTrigger={environmentContext?.refreshTrigger} />
      
      {/* Run Details Modal */}
      <RunDetailsModal 
        runId={modalRunId} 
        isOpen={isModalOpen} 
        onClose={closeRunModal} 
      />
    </div>
  );
};


// Environments Page Component
const EnvironmentsPage = () => {
  const [nodes, setNodes, onNodesChange] = useNodesState<any>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<any>([]);
  const [environments, setEnvironments] = useState<any[]>([]);
  const [selectedEnvironment, setSelectedEnvironment] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [graphLoading, setGraphLoading] = useState(false);
  const [isBundleModalOpen, setIsBundleModalOpen] = useState(false);
  const [isAddServerModalOpen, setIsAddServerModalOpen] = useState(false);
  const [isSyncModalOpen, setIsSyncModalOpen] = useState(false);
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
    generateEnvironmentGraph();
  }, [selectedEnvironment, environments]);

  const onConnect = useCallback(
    (params: any) => setEdges((eds) => addEdge(params, eds)),
    [setEdges]
  );

  const handleSync = () => {
    setIsSyncModalOpen(true);
  };

  const handleAddServer = () => {
    setIsAddServerModalOpen(true);
  };

  const handleSyncComplete = () => {
    // Refresh the environment graph after sync
    generateEnvironmentGraph();
  };

  const generateEnvironmentGraph = async () => {
    if (!selectedEnvironment) return;
    
    try {
      setGraphLoading(true);
      const [agentsResponse, mcpServersResponse] = await Promise.all([
        agentsApi.getByEnvironment(selectedEnvironment),
        mcpServersApi.getByEnvironment(selectedEnvironment)
      ]);

      console.log('API Responses:', { agentsResponse: agentsResponse.data, mcpServersResponse: mcpServersResponse.data });

      const agents = agentsResponse.data?.agents ? 
        (Array.isArray(agentsResponse.data.agents) ? agentsResponse.data.agents : []) : [];
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
      const layoutedNodes = await getLayoutedNodes(newNodes, newEdges);
      console.log('Environment graph generated:', {
        rawNodes: newNodes?.length || 0,
        rawEdges: newEdges?.length || 0,
        agents: agents?.length || 0,
        mcpServers: mcpServers?.length || 0,
        layoutedNodes: layoutedNodes?.length || 0,
        environment: selectedEnv?.name
      });
      setNodes(layoutedNodes);
      setEdges(newEdges);
    } catch (error) {
      console.error('Failed to generate environment graph:', error);
    } finally {
      setGraphLoading(false);
    }
  };

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
          <button 
            onClick={handleSync}
            disabled={!selectedEnvironment}
            className="px-4 py-2 bg-tokyo-blue text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-blue1 transition-colors flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Play className="h-4 w-4" />
            Sync
          </button>
          <button 
            onClick={handleAddServer}
            disabled={!selectedEnvironment}
            className="px-4 py-2 bg-tokyo-cyan text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-blue1 transition-colors shadow-tokyo-glow flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Plus className="h-4 w-4" />
            Add Server
          </button>
          <button 
            onClick={() => setIsBundleModalOpen(true)}
            disabled={!selectedEnvironment}
            className="px-4 py-2 bg-tokyo-magenta text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-purple transition-colors shadow-tokyo-glow flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Package className="h-4 w-4" />
            Bundle
          </button>
        </div>
      </div>
      
      <div className="flex-1 h-full relative">
        {loading ? (
          <div className="flex items-center justify-center h-full">
            <div className="text-tokyo-comment font-mono">Loading environments...</div>
          </div>
        ) : (
          <>
            <ReactFlow
              key={`environment-${selectedEnvironment}`}
              nodes={nodes}
              edges={edges}
              onNodesChange={onNodesChange}
              onEdgesChange={onEdgesChange}
              onConnect={onConnect}
              nodeTypes={environmentPageNodeTypes}
              fitView
              fitViewOptions={{ padding: 0.1, duration: 500 }}
              className="bg-tokyo-bg w-full h-full"
              defaultEdgeOptions={{
                animated: true,
                style: { 
                  stroke: '#ff00ff', 
                  strokeWidth: 3,
                  zIndex: 1000
                },
              }}
            />
            
            {/* Graph loading overlay */}
            {graphLoading && (
              <div className="absolute inset-0 bg-tokyo-bg bg-opacity-80 flex items-center justify-center z-50">
                <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 flex items-center gap-3">
                  <div className="animate-spin h-6 w-6 border-2 border-tokyo-orange border-t-transparent rounded-full"></div>
                  <span className="text-tokyo-fg font-mono">Rebuilding environment graph...</span>
                </div>
              </div>
            )}
          </>
        )}
      </div>
      
      {/* Bundle Environment Modal */}
      <BundleEnvironmentModal
        isOpen={isBundleModalOpen}
        onClose={() => setIsBundleModalOpen(false)}
        environmentName={environments.find(env => env.id === selectedEnvironment)?.name || 'default'}
      />
      
      {/* Add Server Modal */}
      <AddServerModal 
        isOpen={isAddServerModalOpen} 
        onClose={() => setIsAddServerModalOpen(false)} 
        environmentName={environments.find(env => env.id === selectedEnvironment)?.name || 'default'}
      />
      
      {/* Sync Modal */}
      <SyncModal 
        isOpen={isSyncModalOpen} 
        onClose={() => setIsSyncModalOpen(false)} 
        environment={environments.find(env => env.id === selectedEnvironment)?.name || 'default'}
        onSyncComplete={handleSyncComplete}
      />
    </div>
  );
};

// Install Bundle Modal Component
const InstallBundleModal = ({ isOpen, onClose, onSuccess }: { isOpen: boolean; onClose: () => void; onSuccess?: () => void }) => {
  const [bundleSource, setBundleSource] = useState<'url' | 'file'>('url');
  const [bundleLocation, setBundleLocation] = useState('');
  const [environmentName, setEnvironmentName] = useState('');
  const [installing, setInstalling] = useState(false);
  const [installSuccess, setInstallSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleInstall = async () => {
    if (!bundleLocation.trim() || !environmentName.trim()) {
      setError('Please provide both bundle location and environment name');
      return;
    }

    try {
      setInstalling(true);
      setError(null);

      // Install bundle via API
      const response = await bundlesApi.install(bundleLocation, environmentName, bundleSource);
      
      // The API handles:
      // 1. Download/extract the bundle from URL or file path
      // 2. Create new environment via database mechanism
      // 3. Unpack contents into the environment directory
      // 4. Move tar.gz to ~/.config/station/bundles directory
      
      setInstallSuccess(true);
      onSuccess?.(); // Refresh bundles list
      setTimeout(() => {
        setInstallSuccess(false);
        onClose();
        setBundleLocation('');
        setEnvironmentName('');
        setBundleSource('url');
      }, 2000);
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to install bundle');
    } finally {
      setInstalling(false);
    }
  };

  const handleClose = () => {
    if (!installing) {
      onClose();
      setError(null);
      setInstallSuccess(false);
      setBundleLocation('');
      setEnvironmentName('');
      setBundleSource('url');
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 w-full max-w-md mx-4">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-mono font-semibold text-tokyo-magenta">Install Bundle</h2>
          <button
            onClick={handleClose}
            disabled={installing}
            className="text-tokyo-comment hover:text-tokyo-fg transition-colors disabled:opacity-50"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {installSuccess ? (
          <div className="text-center py-8">
            <div className="bg-transparent border border-tokyo-green border-opacity-50 rounded-lg p-4 mb-4">
              <Package className="h-8 w-8 text-tokyo-green mx-auto mb-2" />
              <p className="text-tokyo-green font-mono">Bundle installed successfully!</p>
            </div>
            <div className="bg-transparent border border-tokyo-blue border-opacity-50 rounded-lg p-3 relative group">
              <button
                onClick={() => navigator.clipboard.writeText(`stn sync ${environmentName}`)}
                className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 p-1 rounded bg-tokyo-blue hover:bg-tokyo-blue1 text-tokyo-bg"
                title={`Copy sync command: stn sync ${environmentName}`}
              >
                <Copy className="h-3 w-3" />
              </button>
              <p className="text-sm text-tokyo-blue font-mono pr-8">
                Next step: Run <code className="bg-tokyo-bg px-1 rounded text-tokyo-orange">stn sync {environmentName}</code> to apply changes
              </p>
            </div>
          </div>
        ) : (
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-mono text-tokyo-fg mb-2">Bundle Source</label>
              <div className="flex gap-2">
                <button
                  onClick={() => setBundleSource('url')}
                  className={`px-3 py-2 rounded font-mono text-sm transition-colors ${
                    bundleSource === 'url'
                      ? 'bg-tokyo-blue text-tokyo-bg'
                      : 'bg-tokyo-bg border border-tokyo-blue7 text-tokyo-comment hover:text-tokyo-fg'
                  }`}
                >
                  URL/Endpoint
                </button>
                <button
                  onClick={() => setBundleSource('file')}
                  className={`px-3 py-2 rounded font-mono text-sm transition-colors ${
                    bundleSource === 'file'
                      ? 'bg-tokyo-blue text-tokyo-bg'
                      : 'bg-tokyo-bg border border-tokyo-blue7 text-tokyo-comment hover:text-tokyo-fg'
                  }`}
                >
                  File Path
                </button>
              </div>
            </div>

            <div>
              <label className="block text-sm font-mono text-tokyo-fg mb-2">
                Bundle {bundleSource === 'url' ? 'URL' : 'File Path'}
              </label>
              <input
                type="text"
                value={bundleLocation}
                onChange={(e) => setBundleLocation(e.target.value)}
                placeholder={bundleSource === 'url' ? 'https://example.com/bundle.tar.gz' : '/path/to/bundle.tar.gz'}
                className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 rounded text-tokyo-fg font-mono text-sm focus:border-tokyo-blue focus:outline-none"
                disabled={installing}
              />
            </div>

            <div>
              <label className="block text-sm font-mono text-tokyo-fg mb-2">New Environment Name</label>
              <input
                type="text"
                value={environmentName}
                onChange={(e) => setEnvironmentName(e.target.value)}
                placeholder="production"
                className="w-full px-3 py-2 bg-tokyo-bg border border-tokyo-blue7 rounded text-tokyo-fg font-mono text-sm focus:border-tokyo-blue focus:outline-none"
                disabled={installing}
              />
            </div>

            {error && (
              <div className="bg-tokyo-red bg-opacity-20 border border-tokyo-red rounded p-3">
                <p className="text-tokyo-red text-sm font-mono">{error}</p>
              </div>
            )}

            <div className="flex gap-3 pt-2">
              <button
                onClick={handleClose}
                disabled={installing}
                className="flex-1 px-4 py-2 bg-tokyo-bg border border-tokyo-blue7 text-tokyo-comment rounded font-mono transition-colors hover:text-tokyo-fg disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                onClick={handleInstall}
                disabled={installing || !bundleLocation.trim() || !environmentName.trim()}
                className="flex-1 px-4 py-2 bg-tokyo-magenta text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-purple transition-colors disabled:opacity-50 flex items-center justify-center gap-2"
              >
                {installing ? (
                  <>
                    <div className="animate-spin rounded-full h-4 w-4 border-2 border-tokyo-bg border-t-transparent"></div>
                    Installing...
                  </>
                ) : (
                  <>
                    <Download className="h-4 w-4" />
                    Install Bundle
                  </>
                )}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

// Ship CLI Page Component
const ShipCLIPage = () => {
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>({});
  
  const toggleSection = (sectionId: string) => {
    setExpandedSections(prev => ({
      ...prev,
      [sectionId]: !prev[sectionId]
    }));
  };

  const CollapsibleCard = ({ id, title, children, defaultExpanded = false }: { id: string; title: string; children: React.ReactNode; defaultExpanded?: boolean }) => {
    const isExpanded = expandedSections[id] ?? defaultExpanded;
    
    return (
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg overflow-hidden">
        <button
          onClick={() => toggleSection(id)}
          className="w-full p-4 text-left flex items-center justify-between hover:bg-tokyo-bg-highlight transition-colors"
        >
          <h3 className="text-lg font-mono font-medium text-tokyo-yellow">{title}</h3>
          <ChevronRight className={`h-4 w-4 text-tokyo-yellow transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
        </button>
        {isExpanded && (
          <div className="p-4 border-t border-tokyo-blue7">
            {children}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
        <div className="flex items-center gap-3">
          <span className="text-2xl">🚢</span>
          <h1 className="text-xl font-mono font-semibold text-tokyo-yellow">Ship CLI</h1>
          <span className="px-2 py-1 text-tokyo-yellow text-xs font-mono rounded border border-tokyo-yellow">
            Station Companion
          </span>
        </div>
      </div>
      
      <div className="flex-1 p-6 overflow-y-auto">
        <div className="max-w-4xl mx-auto space-y-6">
          {/* Introduction */}
          <div className="bg-tokyo-bg-dark border border-tokyo-yellow border-opacity-50 rounded-lg p-6">
            <h2 className="text-2xl font-mono font-bold text-tokyo-yellow mb-4">Ship MCP Framework</h2>
            <p className="text-tokyo-fg font-mono leading-relaxed mb-4">
              <strong className="text-tokyo-yellow">Ship is the perfect companion to Station</strong> - a comprehensive collection of CloudShip AI team curated MCP servers that run on top of Dagger engine with the ability to use it as a framework to build MCP servers that run securely in containers.
            </p>
            <p className="text-tokyo-fg font-mono leading-relaxed mb-4">
              Ship provides <strong className="text-tokyo-cyan">92 ready-to-use infrastructure and security tools</strong> that integrate seamlessly with Station's MCP server management, giving you off-the-shelf MCP servers powered by secure Dagger containers.
            </p>
            
            {/* Beta Note */}
            <div className="border border-tokyo-yellow rounded p-3">
              <div className="flex items-center gap-2 mb-2">
                <span className="text-tokyo-yellow font-mono font-bold">📝 Note:</span>
              </div>
              <p className="text-tokyo-fg font-mono text-sm">
                Ship is currently in <strong className="text-tokyo-yellow">early beta</strong>. While functional and actively maintained by the CloudShip AI team, expect regular updates and potential breaking changes as we refine the framework.
              </p>
            </div>
          </div>

          {/* Quick Install */}
          <CollapsibleCard id="install" title="🚀 Quick Installation" defaultExpanded={true}>
            <div className="space-y-4">
              <div>
                <h4 className="text-md font-mono font-medium text-tokyo-cyan mb-2">Install Ship CLI</h4>
                <div className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                  <code className="text-tokyo-green font-mono text-sm">
                    curl -fsSL https://raw.githubusercontent.com/cloudshipai/ship/main/install.sh | bash
                  </code>
                </div>
              </div>
              <div>
                <h4 className="text-md font-mono font-medium text-tokyo-cyan mb-2">Or install with Go</h4>
                <div className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                  <code className="text-tokyo-green font-mono text-sm">
                    go install github.com/cloudshipai/ship/cmd/ship@latest
                  </code>
                </div>
              </div>
              <div>
                <h4 className="text-md font-mono font-medium text-tokyo-cyan mb-2">Use with Station</h4>
                <p className="text-tokyo-fg font-mono text-sm mb-2">
                  Once installed, add Ship MCP servers to your Station environment using the "Add Server" button above.
                </p>
                <div className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                  <code className="text-tokyo-green font-mono text-sm">
                    ship mcp all  # Start MCP server with all 92 tools
                  </code>
                </div>
              </div>
            </div>
          </CollapsibleCard>

          {/* Module Categories */}
          <CollapsibleCard id="terraform" title="🔧 Terraform Tools (11 modules)">
            <div className="space-y-3">
              {[
                { name: 'lint', desc: 'TFLint for syntax checking', usage: 'ship tf lint' },
                { name: 'checkov', desc: 'Security & compliance scanning', usage: 'ship tf checkov' },
                { name: 'trivy', desc: 'Security scanning', usage: 'ship tf trivy' },
                { name: 'terraform-docs', desc: 'Documentation generation', usage: 'ship tf docs' },
                { name: 'diagram', desc: 'InfraMap visualization', usage: 'ship tf diagram' },
                { name: 'cost', desc: 'OpenInfraQuote analysis', usage: 'ship tf cost' },
                { name: 'infracost', desc: 'Advanced cost analysis', usage: 'Detailed pricing' },
              ].map(tool => (
                <div key={tool.name} className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                  <div className="flex justify-between items-start mb-1">
                    <span className="font-mono font-medium text-tokyo-blue">{tool.name}</span>
                    <code className="text-tokyo-green font-mono text-xs">{tool.usage}</code>
                  </div>
                  <p className="text-tokyo-fg font-mono text-sm">{tool.desc}</p>
                </div>
              ))}
            </div>
          </CollapsibleCard>

          <CollapsibleCard id="security" title="🛡️ Security Tools (41 modules)">
            <div className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {[
                  { category: 'Secret Detection', tools: ['gitleaks', 'trufflehog', 'git-secrets', 'history-scrub'] },
                  { category: 'Vulnerability Scanning', tools: ['grype', 'trivy', 'osv-scanner', 'semgrep'] },
                  { category: 'SBOM & Supply Chain', tools: ['syft', 'guac', 'dependency-track', 'in-toto'] },
                  { category: 'Container Security', tools: ['dockle', 'cosign', 'cosign-golden', 'trivy-golden'] },
                  { category: 'Kubernetes Security', tools: ['kubescape', 'kube-bench', 'kube-hunter', 'falco'] },
                  { category: 'Cloud Security', tools: ['prowler', 'scout-suite', 'cloudsplaining', 'cfn-nag'] }
                ].map(cat => (
                  <div key={cat.category} className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                    <h5 className="font-mono font-medium text-tokyo-red mb-2">{cat.category}</h5>
                    <div className="flex flex-wrap gap-1">
                      {cat.tools.map(tool => (
                        <span key={tool} className="px-2 py-1 text-tokyo-fg text-xs font-mono rounded border border-tokyo-blue7">
                          {tool}
                        </span>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </CollapsibleCard>

          <CollapsibleCard id="cloud" title="☁️ Cloud Infrastructure (20 modules)">
            <div className="space-y-3">
              {[
                { name: 'cloudquery', desc: 'Asset inventory', support: 'Multi-cloud' },
                { name: 'custodian', desc: 'Governance engine', support: 'AWS, Azure, GCP' },
                { name: 'terraformer', desc: 'Infrastructure import', support: 'Multi-cloud' },
                { name: 'packer', desc: 'Image building', support: 'Multi-platform' },
                { name: 'velero', desc: 'Backup & recovery', support: 'Kubernetes' },
                { name: 'cert-manager', desc: 'Certificate mgmt', support: 'Kubernetes' }
              ].map(tool => (
                <div key={tool.name} className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3 flex justify-between items-center">
                  <div>
                    <span className="font-mono font-medium text-tokyo-cyan">{tool.name}</span>
                    <p className="text-tokyo-fg font-mono text-sm">{tool.desc}</p>
                  </div>
                  <span className="px-2 py-1 text-tokyo-cyan text-xs font-mono rounded border border-tokyo-cyan">
                    {tool.support}
                  </span>
                </div>
              ))}
            </div>
          </CollapsibleCard>

          <CollapsibleCard id="external" title="🌐 External MCP Servers (4 modules)">
            <div className="space-y-3">
              {[
                { 
                  name: 'filesystem', 
                  desc: 'File and directory operations', 
                  vars: 'FILESYSTEM_ROOT (optional)',
                  usage: 'ship mcp filesystem --var FILESYSTEM_ROOT=/custom/path'
                },
                { 
                  name: 'memory', 
                  desc: 'Persistent knowledge storage', 
                  vars: 'MEMORY_STORAGE_PATH, MEMORY_MAX_SIZE',
                  usage: 'ship mcp memory --var MEMORY_STORAGE_PATH=/data'
                },
                { 
                  name: 'brave-search', 
                  desc: 'Web search capabilities', 
                  vars: 'BRAVE_API_KEY (required), BRAVE_SEARCH_COUNT',
                  usage: 'ship mcp brave-search --var BRAVE_API_KEY=your_key'
                },
                { 
                  name: 'steampipe', 
                  desc: 'SQL-based cloud resource queries', 
                  vars: 'STEAMPIPE_DATABASE_CONNECTIONS',
                  usage: 'ship mcp steampipe'
                }
              ].map(server => (
                <div key={server.name} className="bg-tokyo-bg border border-tokyo-blue7 rounded p-4">
                  <div className="flex justify-between items-start mb-2">
                    <h5 className="font-mono font-medium text-tokyo-purple">{server.name}</h5>
                    <span className="px-2 py-1 text-tokyo-purple text-xs font-mono rounded border border-tokyo-purple">
                      External MCP
                    </span>
                  </div>
                  <p className="text-tokyo-fg font-mono text-sm mb-2">{server.desc}</p>
                  <p className="text-tokyo-fg font-mono text-xs mb-2">Variables: {server.vars}</p>
                  <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded p-2">
                    <code className="text-tokyo-green font-mono text-xs">{server.usage}</code>
                  </div>
                </div>
              ))}
            </div>
          </CollapsibleCard>

          <CollapsibleCard id="aws-labs" title="🏢 AWS Labs MCP Servers (6 modules)">
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {[
                { name: 'aws-core', focus: 'General AWS', desc: 'Core AWS operations' },
                { name: 'aws-iam', focus: 'Identity & Access', desc: 'IAM management' },
                { name: 'aws-pricing', focus: 'Cost Analysis', desc: 'AWS pricing data' },
                { name: 'aws-eks', focus: 'Kubernetes', desc: 'EKS operations' },
                { name: 'aws-ec2', focus: 'Compute', desc: 'EC2 management' },
                { name: 'aws-s3', focus: 'Storage', desc: 'S3 operations' }
              ].map(server => (
                <div key={server.name} className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                  <h5 className="font-mono font-medium text-tokyo-orange">{server.name}</h5>
                  <p className="text-tokyo-fg font-mono text-xs">{server.focus}</p>
                  <p className="text-tokyo-fg font-mono text-sm">{server.desc}</p>
                </div>
              ))}
            </div>
          </CollapsibleCard>

          <CollapsibleCard id="usage" title="💻 Usage Examples">
            <div className="space-y-4">
              <div>
                <h4 className="text-md font-mono font-medium text-tokyo-cyan mb-2">Start All Tools as MCP Server</h4>
                <div className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                  <code className="text-tokyo-green font-mono text-sm">ship mcp all</code>
                </div>
                <p className="text-tokyo-fg font-mono text-sm mt-1">Starts MCP server with all 92 tools</p>
              </div>
              <div>
                <h4 className="text-md font-mono font-medium text-tokyo-cyan mb-2">Category-Specific MCP Servers</h4>
                <div className="space-y-2">
                  <div className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                    <code className="text-tokyo-green font-mono text-sm">ship mcp security  # All 41 security tools</code>
                  </div>
                  <div className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                    <code className="text-tokyo-green font-mono text-sm">ship mcp terraform  # All 11 Terraform tools</code>
                  </div>
                  <div className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                    <code className="text-tokyo-green font-mono text-sm">ship mcp cloud  # All 20 cloud tools</code>
                  </div>
                </div>
              </div>
              <div>
                <h4 className="text-md font-mono font-medium text-tokyo-cyan mb-2">Direct CLI Usage</h4>
                <div className="space-y-2">
                  <div className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                    <code className="text-tokyo-green font-mono text-sm">ship tf lint  # Run TFLint on current directory</code>
                  </div>
                  <div className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3">
                    <code className="text-tokyo-green font-mono text-sm">ship tf cost  # Estimate infrastructure costs</code>
                  </div>
                </div>
              </div>
            </div>
          </CollapsibleCard>

          <CollapsibleCard id="integration" title="🔗 Station Integration">
            <div className="space-y-4">
              <p className="text-tokyo-fg font-mono">
                Ship pairs perfectly with Station! Use Station's <strong className="text-tokyo-cyan">"Add Server"</strong> feature to add Ship MCP servers to your environments.
              </p>
              <div>
                <h4 className="text-md font-mono font-medium text-tokyo-cyan mb-2">Example Station MCP Configuration</h4>
                <div className="bg-tokyo-bg border border-tokyo-blue7 rounded p-3 overflow-x-auto">
                  <pre className="text-tokyo-green font-mono text-sm">
{`{
  "name": "Ship All Tools",
  "description": "All 92 Ship tools via MCP",
  "mcpServers": {
    "ship-all": {
      "command": "ship",
      "args": ["mcp", "all"],
      "env": {
        "AWS_PROFILE": "{{ .AWS_PROFILE }}",
        "FILESYSTEM_ROOT": "{{ .PROJECT_ROOT }}"
      }
    }
  }
}`}
                  </pre>
                </div>
              </div>
            </div>
          </CollapsibleCard>

          {/* Key Features Summary */}
          <div className="bg-tokyo-bg-dark border border-tokyo-green border-opacity-50 rounded-lg p-6">
            <h3 className="text-lg font-mono font-medium text-tokyo-green mb-4">🎯 Why Ship + Station?</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <span className="text-tokyo-green">✅</span>
                  <span className="font-mono text-sm text-tokyo-fg">92 Total Tools Available</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-tokyo-green">✅</span>
                  <span className="font-mono text-sm text-tokyo-fg">Containerized Execution</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-tokyo-green">✅</span>
                  <span className="font-mono text-sm text-tokyo-fg">MCP Integration</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-tokyo-green">✅</span>
                  <span className="font-mono text-sm text-tokyo-fg">Security-First Design</span>
                </div>
              </div>
              <div className="space-y-2">
                <div className="flex items-center gap-2">
                  <span className="text-tokyo-green">✅</span>
                  <span className="font-mono text-sm text-tokyo-fg">Multi-Cloud Support</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-tokyo-green">✅</span>
                  <span className="font-mono text-sm text-tokyo-fg">Variable Support</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-tokyo-green">✅</span>
                  <span className="font-mono text-sm text-tokyo-fg">External Server Proxying</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-tokyo-green">✅</span>
                  <span className="font-mono text-sm text-tokyo-fg">Framework Extensibility</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

// Bundle Details Modal Component
const BundleDetailsModal = ({ bundle, isOpen, onClose }: { bundle: BundleInfo | null; isOpen: boolean; onClose: () => void }) => {
  if (!isOpen || !bundle) return null;

  const handleCopySync = () => {
    navigator.clipboard.writeText(`stn sync default`);
  };

  const handleCopyPath = () => {
    navigator.clipboard.writeText(bundle.file_path);
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 w-full max-w-2xl mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <Package className="h-6 w-6 text-tokyo-magenta" />
            <h2 className="text-xl font-mono font-semibold text-tokyo-magenta">Bundle Details</h2>
          </div>
          <button
            onClick={onClose}
            className="text-tokyo-comment hover:text-tokyo-fg transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="space-y-6">
          {/* Bundle Info */}
          <div className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4">
            <h3 className="text-lg font-mono font-medium text-tokyo-fg mb-4">{bundle.name}</h3>
            
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm font-mono">
              <div>
                <span className="text-tokyo-comment block mb-1">File Name:</span>
                <span className="text-tokyo-fg">{bundle.file_name}</span>
              </div>
              
              <div>
                <span className="text-tokyo-comment block mb-1">Size:</span>
                <span className="text-tokyo-fg">{(bundle.size / 1024).toFixed(1)} KB</span>
              </div>
              
              <div>
                <span className="text-tokyo-comment block mb-1">Modified:</span>
                <span className="text-tokyo-fg">{bundle.modified_time}</span>
              </div>
              
              <div>
                <span className="text-tokyo-comment block mb-1">File Path:</span>
                <div className="flex items-center gap-2">
                  <span className="text-tokyo-fg text-xs break-all">{bundle.file_path}</span>
                  <button
                    onClick={handleCopyPath}
                    className="p-1 rounded bg-tokyo-blue hover:bg-tokyo-blue1 text-tokyo-bg flex-shrink-0"
                    title="Copy file path"
                  >
                    <Copy className="h-3 w-3" />
                  </button>
                </div>
              </div>
            </div>
          </div>

          {/* Actions */}
          <div className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4">
            <h4 className="text-md font-mono font-medium text-tokyo-fg mb-3">Actions</h4>
            <div className="flex flex-wrap gap-3">
              <button
                onClick={handleCopySync}
                className="flex items-center gap-2 px-3 py-2 bg-tokyo-orange hover:bg-tokyo-orange1 text-tokyo-bg rounded font-mono text-sm transition-colors"
              >
                <Copy className="h-4 w-4" />
                Copy Sync Command
              </button>
              
              <div className="flex items-center gap-2 px-3 py-2 bg-tokyo-bg-dark border border-tokyo-blue7 rounded">
                <code className="text-tokyo-orange text-sm">stn sync default</code>
              </div>
            </div>
          </div>

          {/* Bundle Usage */}
          <div className="bg-tokyo-bg border border-tokyo-blue7 rounded-lg p-4">
            <h4 className="text-md font-mono font-medium text-tokyo-fg mb-3">Usage</h4>
            <div className="space-y-3 text-sm">
              <div>
                <p className="text-tokyo-comment font-mono mb-2">
                  This bundle contains agent configurations and templates. To use it:
                </p>
                <ol className="list-decimal list-inside space-y-1 text-tokyo-fg font-mono text-xs ml-4">
                  <li>Run the sync command to apply the bundle to your environment</li>
                  <li>Check the agents page to see any new agents created from the bundle</li>
                  <li>Configure any required environment variables or MCP servers</li>
                </ol>
              </div>
            </div>
          </div>
        </div>

        <div className="flex justify-end mt-6">
          <button
            onClick={onClose}
            className="px-4 py-2 bg-tokyo-bg border border-tokyo-blue7 text-tokyo-comment rounded font-mono transition-colors hover:text-tokyo-fg"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};

// Bundles Page Component
const BundlesPage = () => {
  const [bundles, setBundles] = useState<BundleInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isInstallModalOpen, setIsInstallModalOpen] = useState(false);
  const [selectedBundle, setSelectedBundle] = useState<BundleInfo | null>(null);
  const [isBundleDetailsOpen, setIsBundleDetailsOpen] = useState(false);
  const environmentContext = React.useContext(EnvironmentContext);

  // Function to open bundle details modal
  const openBundleDetails = (bundle: BundleInfo) => {
    setSelectedBundle(bundle);
    setIsBundleDetailsOpen(true);
  };

  const closeBundleDetails = () => {
    setIsBundleDetailsOpen(false);
    setSelectedBundle(null);
  };

  // Fetch bundles data
  const fetchBundles = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await bundlesApi.getAll();
      setBundles(response.data.bundles || []);
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to load bundles');
      setBundles([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchBundles();
  }, [environmentContext?.refreshTrigger]);

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
        <h1 className="text-xl font-mono font-semibold text-tokyo-magenta">Bundles</h1>
        <div className="flex items-center gap-3">
          <button 
            onClick={() => setIsInstallModalOpen(true)}
            className="px-4 py-2 bg-tokyo-magenta text-tokyo-bg rounded font-mono font-medium hover:bg-tokyo-purple transition-colors shadow-tokyo-glow flex items-center gap-2"
          >
            <Download className="h-4 w-4" />
            Install Bundle
          </button>
        </div>
      </div>
      
      <div className="flex-1 p-4">
        {loading ? (
          <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-8 text-center">
            <div className="animate-spin h-8 w-8 border-2 border-tokyo-blue border-t-transparent rounded-full mx-auto mb-4"></div>
            <p className="text-tokyo-comment font-mono">Loading bundles...</p>
          </div>
        ) : error ? (
          <div className="bg-tokyo-bg-dark border border-tokyo-red border-opacity-50 rounded-lg p-8 text-center">
            <div className="h-16 w-16 text-tokyo-red mx-auto mb-4">⚠️</div>
            <h2 className="text-lg font-mono font-medium text-tokyo-red mb-2">Error loading bundles</h2>
            <p className="text-tokyo-comment font-mono">{error}</p>
          </div>
        ) : bundles.length === 0 ? (
          <div className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-8 text-center">
            <Package className="h-16 w-16 text-tokyo-comment mx-auto mb-4" />
            <h2 className="text-lg font-mono font-medium text-tokyo-fg mb-2">No bundles found</h2>
            <p className="text-tokyo-comment font-mono">Install your first bundle to get started with agent templates and configurations.</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {bundles.map((bundle, index) => (
              <div key={index} className="bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg p-6 hover:border-tokyo-blue5 transition-colors relative group">
                <div className="flex items-start justify-between mb-4">
                  <div className="flex items-center gap-3">
                    <Package className="h-8 w-8 text-tokyo-magenta" />
                    <div>
                      <h3 className="text-lg font-mono font-medium text-tokyo-fg">{bundle.name}</h3>
                      <p className="text-sm text-tokyo-comment font-mono">{bundle.file_name}</p>
                    </div>
                  </div>
                  
                  {/* Action buttons - appear on hover */}
                  <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
                    <button
                      onClick={() => navigator.clipboard.writeText(`stn sync default`)}
                      className="p-1 rounded bg-tokyo-orange hover:bg-tokyo-orange1 text-tokyo-bg"
                      title="Copy sync command: stn sync default"
                    >
                      <Copy className="h-3 w-3" />
                    </button>
                    <button
                      onClick={() => openBundleDetails(bundle)}
                      className="p-1 rounded bg-tokyo-magenta hover:bg-tokyo-purple text-tokyo-bg"
                      title="View bundle details"
                    >
                      <Eye className="h-3 w-3" />
                    </button>
                  </div>
                </div>
                
                <div className="space-y-2 text-sm font-mono">
                  <div className="flex justify-between">
                    <span className="text-tokyo-comment">Size:</span>
                    <span className="text-tokyo-fg">{(bundle.size / 1024).toFixed(1)} KB</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-tokyo-comment">Modified:</span>
                    <span className="text-tokyo-fg">{bundle.modified_time}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-tokyo-comment">Path:</span>
                    <span className="text-tokyo-fg text-xs truncate ml-2" title={bundle.file_path}>
                      {bundle.file_path}
                    </span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
      
      {/* Install Bundle Modal */}
      <InstallBundleModal 
        isOpen={isInstallModalOpen} 
        onClose={() => setIsInstallModalOpen(false)}
        onSuccess={fetchBundles}
      />
      
      {/* Bundle Details Modal */}
      <BundleDetailsModal 
        bundle={selectedBundle}
        isOpen={isBundleDetailsOpen} 
        onClose={closeBundleDetails}
      />
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

// Agent Editor Component
const AgentEditor = () => {
  const { agentId } = useParams<{ agentId: string }>();
  const navigate = useNavigate();
  const [agentData, setAgentData] = useState<any>(null);
  const [promptContent, setPromptContent] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [saveSuccess, setSaveSuccess] = useState(false);
  const [error, setError] = useState<string | null>(null);

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
        setPromptContent(promptResponse.data.content || '');
      } catch (err: any) {
        setError(err.response?.data?.error || 'Failed to load agent data');
      } finally {
        setLoading(false);
      }
    };

    fetchAgentData();
  }, [agentId]);

  const handleSave = async () => {
    if (!agentId) return;
    
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
      <div className="bg-tokyo-bg-dark border-b border-tokyo-blue7 px-6 py-4">
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
              disabled={saving}
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

      {/* Editor content */}
      <div className="flex-1 p-6">
        <div className="bg-tokyo-bg-dark rounded-lg border border-tokyo-blue7 h-[calc(100vh-200px)]">
          <div className="p-4 border-b border-tokyo-blue7">
            <h2 className="text-lg font-mono text-tokyo-blue">Agent Configuration</h2>
            <p className="text-sm text-tokyo-comment mt-1">
              Edit the agent's prompt file configuration. After saving, run <code className="bg-tokyo-bg px-1 rounded text-tokyo-orange">stn sync {agentData?.environment_name || 'environment'}</code> to apply changes.
            </p>
          </div>
          
          <div className="h-[calc(100%-80px)]">
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
      </div>
    </div>
  );
};

type Page = 'agents' | 'mcps' | 'runs' | 'environments' | 'bundles' | 'ship';

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
      case 'environments':
        return <EnvironmentsPage />;
      case 'bundles':
        return <BundlesPage />;
      case 'ship':
        return <ShipCLIPage />;
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
                  <Route path="/agent-editor/:agentId" element={<AgentEditor />} />
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

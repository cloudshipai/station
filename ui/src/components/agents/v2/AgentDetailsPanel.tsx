import React, { useEffect, useState } from 'react';
import { Copy, Trash2, ExternalLink, Clock, CheckCircle, XCircle, PlayCircle, Loader2, HelpCircle, Calendar } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { agentRunsApi, agentsApi } from '../../../api/station';
import type { Agent, AgentRun } from '../../../types/station';

interface AgentDetailsPanelProps {
  agent: Agent;
  onRunAgent: (agent: Agent) => void;
  onViewRun: (runId: number) => void;
  onScheduleAgent: (agent: Agent) => void;
}

export const AgentDetailsPanel: React.FC<AgentDetailsPanelProps> = ({ agent, onRunAgent, onViewRun, onScheduleAgent }) => {
  const navigate = useNavigate();
  const [runs, setRuns] = useState<AgentRun[]>([]);
  const [tools, setTools] = useState<any[]>([]);
  const [loadingRuns, setLoadingRuns] = useState(false);
  const [loadingTools, setLoadingTools] = useState(false);

  useEffect(() => {
    if (agent?.id) {
      fetchRuns();
      fetchTools();
    }
  }, [agent?.id]);

  const fetchRuns = async () => {
    setLoadingRuns(true);
    try {
      // Fetch all runs and filter client-side since /agents/:id/runs endpoint might not exist
      const response = await agentRunsApi.getAll();
      const allRuns = response.data.runs || [];
      const agentRuns = allRuns
        .filter((r: any) => r.agent_id === agent.id)
        .sort((a: any, b: any) => new Date(b.started_at).getTime() - new Date(a.started_at).getTime())
        .slice(0, 5);
      setRuns(agentRuns);
    } catch (error) {
      console.error('Failed to fetch runs:', error);
    } finally {
      setLoadingRuns(false);
    }
  };

  const fetchTools = async () => {
    setLoadingTools(true);
    try {
        // Try to get detailed agent info which usually includes tools
        // Using getWithTools which maps to /agents/:id/details
        const response = await agentsApi.getWithTools(agent.id);
        if (response.data && response.data.mcp_servers) {
            const servers = response.data.mcp_servers || [];
            const allTools = servers.flatMap(server => server.tools.map(t => ({ ...t, server_name: server.name })));
            setTools(allTools);
        } else {
            // If structure matches AgentTool[] directly (fallback)
            setTools((response.data as any) || []);
        }
    } catch (error) {
        console.error('Failed to fetch tool details:', error);
        // Fallback to tool_names if available on the agent list object
        if ((agent as any).tool_names) {
             setTools((agent as any).tool_names.map((name: string) => ({ name })));
        }
    } finally {
        setLoadingTools(false);
    }
  };

  if (!agent) return null;

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'completed': return <CheckCircle className="h-4 w-4 text-green-500" />;
      case 'failed': return <XCircle className="h-4 w-4 text-red-500" />;
      case 'running': return <Loader2 className="h-4 w-4 text-blue-500 animate-spin" />;
      default: return <HelpCircle className="h-4 w-4 text-gray-400" />;
    }
  };

  const formatTimeAgo = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const seconds = Math.floor((now.getTime() - date.getTime()) / 1000);
    
    if (seconds < 60) return `${seconds}s ago`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    return `${Math.floor(hours / 24)}d ago`;
  };

  const formatDuration = (seconds?: number) => {
    if (!seconds) return 'N/A';
    if (seconds < 60) return `${seconds.toFixed(1)}s`;
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    return `${m}m ${s}s`;
  };

  return (
    <div className="h-full flex flex-col bg-[#fafaf8]">
      {/* Header - Paper matte style */}
      <div className="px-6 py-5 border-b border-gray-200/60 bg-white/60 backdrop-blur-sm animate-in fade-in slide-in-from-top-2 duration-300">
        <div className="flex items-start justify-between mb-5">
          <div className="flex-1 min-w-0 mr-4">
            <h2 className="text-xl font-medium text-gray-900 truncate mb-1.5" title={agent.name}>{agent.name}</h2>
            <p className="text-sm text-gray-600 leading-relaxed line-clamp-2" title={agent.description || ''}>
              {agent.description || 'No description provided'}
            </p>
          </div>
          <button 
            className="p-2 text-gray-400 hover:text-gray-700 hover:bg-gray-100/80 rounded-lg transition-all"
            onClick={() => navigator.clipboard.writeText(agent.name)}
            title="Copy agent name"
          >
            <Copy className="h-4 w-4" />
          </button>
        </div>
      </div>

      {/* Content - Paper sections */}
      <div className="flex-1 p-6 overflow-y-auto space-y-6">
        {/* Configuration Section - Card style */}
        <section className="bg-white rounded-xl border border-gray-200/60 p-5 shadow-sm animate-in fade-in slide-in-from-bottom-4 duration-500">
          <h3 className="text-sm font-semibold text-gray-900 mb-4">Configuration</h3>
          <div className="space-y-5">
            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="text-xs font-medium text-gray-600">Model</label>
                <span className="text-xs text-gray-900 font-mono bg-gray-50 px-2.5 py-1 rounded-md border border-gray-200">
                  {agent.model || 'gpt-4o-mini'}
                </span>
              </div>
            </div>
            
            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="text-xs font-medium text-gray-600">Max Steps</label>
                <span className="text-xs text-gray-900 font-mono bg-gray-50 px-2.5 py-1 rounded-md border border-gray-200">
                  {agent.max_steps}
                </span>
              </div>
              <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
                <div 
                  className="h-full bg-gradient-to-r from-gray-900 to-gray-700 rounded-full transition-all" 
                  style={{ width: `${Math.min((agent.max_steps / 50) * 100, 100)}%` }}
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-medium text-gray-600 mb-2">Instructions</label>
              <div className="relative">
                <div className="text-xs text-gray-700 bg-gray-50/50 p-4 rounded-lg border border-gray-200/60 min-h-[120px] font-mono leading-relaxed max-h-[200px] overflow-y-auto whitespace-pre-wrap">
                  {agent.prompt}
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Tools Section - Clean card */}
        <section className="bg-white rounded-xl border border-gray-200/60 p-5 shadow-sm animate-in fade-in slide-in-from-bottom-4 duration-500 delay-100">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-sm font-semibold text-gray-900">Tools</h3>
            <span className="px-2.5 py-1 bg-gray-100 text-gray-700 text-xs rounded-md font-medium border border-gray-200">
              {tools.length}
            </span>
          </div>
          
          {loadingTools ? (
             <div className="flex items-center justify-center py-6 text-gray-400">
               <Loader2 className="h-4 w-4 animate-spin mr-2" />
               <span className="text-xs">Loading tools...</span>
             </div>
          ) : (
            <div className="space-y-1.5">
              {tools.slice(0, 8).map((tool: any, idx) => (
                <div key={idx} className="flex items-center gap-2.5 px-3 py-2.5 bg-gray-50/50 border border-gray-100 rounded-lg hover:bg-white hover:border-gray-200 hover:shadow-sm transition-all cursor-default group">
                  <div className="w-1.5 h-1.5 rounded-full bg-emerald-500 flex-shrink-0"></div>
                  <span className="text-xs text-gray-800 font-mono truncate flex-1">{tool.name}</span>
                </div>
              ))}
              {tools.length > 8 && (
                <button className="text-xs text-gray-600 hover:text-gray-900 hover:underline px-3 py-1 w-full text-left">
                  + {tools.length - 8} more tools
                </button>
              )}
              {tools.length === 0 && (
                <div className="text-xs text-gray-500 italic px-3 py-6 text-center bg-gray-50/50 rounded-lg border border-gray-100">
                  No tools assigned
                </div>
              )}
            </div>
          )}
        </section>

        {/* Recent Activity Section - Timeline style */}
        <section className="bg-white rounded-xl border border-gray-200/60 p-5 shadow-sm animate-in fade-in slide-in-from-bottom-4 duration-500 delay-200">
          <h3 className="text-sm font-semibold text-gray-900 mb-4">Recent Runs</h3>
          
          {loadingRuns ? (
             <div className="flex items-center justify-center py-6 text-gray-400">
               <Loader2 className="h-4 w-4 animate-spin mr-2" />
               <span className="text-xs">Loading runs...</span>
             </div>
          ) : (
            <div className="space-y-2">
              {runs.length === 0 ? (
                <div className="text-center py-8 bg-gray-50/50 rounded-lg border border-gray-100">
                  <Clock className="h-10 w-10 text-gray-300 mx-auto mb-3" />
                  <p className="text-xs text-gray-500 font-medium">No runs yet</p>
                  <p className="text-xs text-gray-400 mt-1">Click &quot;Run Agent&quot; to get started</p>
                </div>
              ) : (
                runs.map((run) => (
                  <div 
                    key={run.id}
                    onClick={() => onViewRun(run.id)}
                    className="flex items-center justify-between p-3.5 bg-gray-50/50 border border-gray-100 rounded-lg hover:bg-white hover:border-gray-300 hover:shadow-md transition-all cursor-pointer group"
                  >
                    <div className="flex items-center gap-3 overflow-hidden flex-1">
                      <div className="flex-shrink-0">
                        {getStatusIcon(run.status)}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="text-sm font-medium text-gray-900 flex items-center gap-2">
                          <span>Run #{run.id}</span>
                          {run.status === 'running' && <span className="w-1.5 h-1.5 bg-blue-500 rounded-full animate-pulse"></span>}
                        </div>
                        <div className="text-xs text-gray-500 flex items-center gap-1.5 mt-0.5">
                          <Clock className="h-3 w-3 shrink-0" />
                          <span>{formatTimeAgo(run.started_at)}</span>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-mono text-gray-600 bg-white px-2.5 py-1 rounded-md border border-gray-200 shrink-0 group-hover:bg-gray-50 transition-colors">
                        {formatDuration(run.duration_seconds)}
                      </span>
                    </div>
                  </div>
                ))
              )}
            </div>
          )}
          
          {runs.length > 0 && (
            <button 
              className="w-full mt-4 py-2.5 text-xs font-medium text-gray-600 hover:text-gray-900 bg-gray-50/50 hover:bg-gray-100 rounded-lg border border-gray-200 hover:border-gray-300 transition-all"
              onClick={() => navigate('/runs')}
            >
              View All Runs →
            </button>
          )}
        </section>

        {/* Schedule Section */}
        <section className="bg-white rounded-xl border border-gray-200/60 p-5 shadow-sm animate-in fade-in slide-in-from-bottom-4 duration-500 delay-300">
          <h3 className="text-sm font-semibold text-gray-900 mb-4">Schedule</h3>
          <button 
            className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-gray-50 border border-gray-200 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-100 hover:border-gray-300 transition-all"
            onClick={() => onScheduleAgent(agent)}
          >
            <Calendar className="h-4 w-4" />
            Set Schedule
          </button>
        </section>
      </div>
    </div>
  );
};

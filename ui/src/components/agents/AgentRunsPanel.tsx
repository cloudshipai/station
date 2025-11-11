import React, { useState, useEffect } from 'react';
import { Play, Clock, CheckCircle, XCircle, Loader, TrendingUp, TrendingDown, DollarSign, Eye, Activity } from 'lucide-react';
import { agentRunsApi } from '../../api/station';

interface Run {
  id: number;
  agent_id: number;
  agent_name: string;
  status: 'completed' | 'running' | 'failed';
  duration_seconds?: number;
  started_at: string;
  total_tokens?: number;
  input_tokens?: number;
  output_tokens?: number;
}

interface AgentRunsPanelProps {
  agentId: number | null;
  agentName: string;
  onRunClick: (runId: number, agentId: number) => void;
  onExecutionViewClick?: (runId: number, agentId: number) => void;
  selectedRunId?: number | null;
}

export const AgentRunsPanel: React.FC<AgentRunsPanelProps> = ({ agentId, agentName, onRunClick, onExecutionViewClick, selectedRunId }) => {
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!agentId) {
      setRuns([]);
      return;
    }

    const fetchRuns = async () => {
      setLoading(true);
      try {
        const response = await agentRunsApi.getAll();
        const allRuns = response.data.runs || [];
        // Filter by agent and take last 10
        const agentRuns = allRuns
          .filter((r: Run) => r.agent_id === agentId)
          .slice(0, 10);
        setRuns(agentRuns);
      } catch (error) {
        console.error('Failed to fetch runs:', error);
        setRuns([]);
      } finally {
        setLoading(false);
      }
    };

    fetchRuns();
    
    // Refresh every 5 seconds
    const interval = setInterval(fetchRuns, 5000);
    return () => clearInterval(interval);
  }, [agentId]);

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'completed':
        return <CheckCircle className="h-4 w-4 text-green-400" />;
      case 'failed':
        return <XCircle className="h-4 w-4 text-red-400" />;
      case 'running':
        return <Loader className="h-4 w-4 text-blue-400 animate-spin" />;
      default:
        return <Clock className="h-4 w-4 text-gray-400" />;
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed':
        return 'border-green-500/30 bg-green-900/10';
      case 'failed':
        return 'border-red-500/30 bg-red-900/10';
      case 'running':
        return 'border-blue-500/30 bg-blue-900/10';
      default:
        return 'border-gray-700';
    }
  };

  const formatDuration = (seconds?: number) => {
    if (!seconds) return 'N/A';
    if (seconds < 1) return `${Math.round(seconds * 1000)}ms`;
    if (seconds < 60) return `${seconds.toFixed(1)}s`;
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}m ${secs}s`;
  };

  const formatTokens = (tokens?: number) => {
    if (!tokens) return null;
    if (tokens < 1000) return `${tokens}`;
    if (tokens < 1000000) return `${(tokens / 1000).toFixed(1)}k`;
    return `${(tokens / 1000000).toFixed(2)}M`;
  };

  const estimateCost = (inputTokens?: number, outputTokens?: number): string | null => {
    if (!inputTokens || !outputTokens) return null;
    // Assuming gpt-4o-mini pricing: $0.150/1M input, $0.600/1M output
    const inputCost = (inputTokens / 1_000_000) * 0.150;
    const outputCost = (outputTokens / 1_000_000) * 0.600;
    const total = inputCost + outputCost;
    if (total < 0.0001) return '<$0.0001';
    return `$${total.toFixed(4)}`;
  };

  // Calculate trend compared to previous run
  const getTrend = (index: number): 'up' | 'down' | 'same' | null => {
    if (index >= runs.length - 1) return null;
    const current = runs[index];
    const previous = runs[index + 1];
    if (!current.duration_seconds || !previous.duration_seconds) return null;
    
    if (current.duration_seconds > previous.duration_seconds * 1.1) return 'up';
    if (current.duration_seconds < previous.duration_seconds * 0.9) return 'down';
    return 'same';
  };

  if (!agentId) {
    return (
      <div className="w-96 h-full border-l border-gray-700 bg-gray-900 flex items-center justify-center">
        <div className="text-center text-gray-500 font-mono text-sm">
          Select an agent to view runs
        </div>
      </div>
    );
  }

  return (
    <div className="w-96 h-full border-l border-gray-700 bg-gray-900 overflow-hidden flex flex-col">
      {/* Header */}
      <div className="p-4 border-b border-gray-700 bg-gray-800">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <h2 className="text-lg font-mono font-semibold text-cyan-400">Recent Runs</h2>
            {runs.some(r => r.status === 'running') && (
              <div className="flex items-center gap-1.5 px-2 py-0.5 bg-red-900/30 border border-red-500/50 rounded animate-pulse">
                <div className="w-2 h-2 bg-red-500 rounded-full animate-ping absolute"></div>
                <div className="w-2 h-2 bg-red-500 rounded-full"></div>
                <span className="text-xs font-mono font-bold text-red-400">LIVE</span>
              </div>
            )}
          </div>
          {loading && <Loader className="h-4 w-4 text-gray-400 animate-spin" />}
        </div>
        <p className="text-xs text-gray-400 font-mono">
          {agentName} · Last {runs.length} executions
        </p>
      </div>

      {/* Runs List */}
      <div className="flex-1 overflow-y-auto p-3 space-y-2">
        {loading && runs.length === 0 ? (
          <div className="text-center py-8">
            <Loader className="h-8 w-8 text-gray-500 animate-spin mx-auto mb-2" />
            <div className="text-gray-500 font-mono text-sm">Loading runs...</div>
          </div>
        ) : runs.length === 0 ? (
          <div className="text-center py-8">
            <Play className="h-12 w-12 text-gray-600 mx-auto mb-3" />
            <div className="text-gray-400 font-mono text-sm mb-1">No runs yet</div>
            <div className="text-gray-500 font-mono text-xs">
              Execute this agent to see runs here
            </div>
          </div>
        ) : (
          runs.map((run, index) => {
            const trend = getTrend(index);
            const cost = estimateCost(run.input_tokens, run.output_tokens);
            const isSelected = selectedRunId === run.id;
            
            return (
              <div
                key={run.id}
                className={`w-full p-3 rounded-lg border transition-all duration-200 ${getStatusColor(run.status)} ${
                  isSelected ? 'ring-2 ring-green-500/60 border-green-500/70 bg-green-900/20' : ''
                }`}
              >
                {/* Header Row */}
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    {getStatusIcon(run.status)}
                    <span className="text-xs text-gray-400 font-mono">
                      Run #{run.id}
                    </span>
                    {run.status === 'running' && (
                      <span className="px-1.5 py-0.5 bg-red-900/40 border border-red-500/50 rounded text-xs font-mono font-bold text-red-400 animate-pulse">
                        LIVE
                      </span>
                    )}
                  </div>
                  <div className="text-xs text-gray-500 font-mono">
                    {new Date(run.started_at).toLocaleTimeString()}
                  </div>
                </div>

                {/* Metrics Row */}
                <div className="flex items-center gap-3 text-xs flex-wrap">
                  {/* Duration */}
                  <div className="flex items-center gap-1">
                    <Clock className="h-3 w-3 text-gray-500" />
                    <span className="text-gray-300 font-mono">
                      {formatDuration(run.duration_seconds)}
                    </span>
                    {trend && trend !== 'same' && (
                      trend === 'down' ? (
                        <TrendingDown className="h-3 w-3 text-green-400" />
                      ) : (
                        <TrendingUp className="h-3 w-3 text-orange-400" />
                      )
                    )}
                  </div>

                  {/* Tokens */}
                  {run.total_tokens && (
                    <span className="text-purple-400 font-mono">
                      {formatTokens(run.total_tokens)} tok
                    </span>
                  )}

                  {/* Cost */}
                  {cost && (
                    <div className="flex items-center gap-1">
                      <DollarSign className="h-3 w-3 text-yellow-400" />
                      <span className="text-yellow-400 font-mono">
                        {cost}
                      </span>
                    </div>
                  )}
                </div>

                {/* Status Badge and Action Buttons */}
                <div className="mt-2 pt-2 border-t border-gray-700/50 flex items-center justify-between">
                  <span className={`text-xs font-mono uppercase ${
                    run.status === 'completed' ? 'text-green-400' :
                    run.status === 'failed' ? 'text-red-400' :
                    'text-blue-400'
                  }`}>
                    {run.status}
                  </span>
                  
                  {/* Action Buttons */}
                  <div className="flex items-center gap-1">
                    {/* Execution View Button */}
                    {onExecutionViewClick && run.status === 'completed' && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          onExecutionViewClick(run.id, run.agent_id);
                        }}
                        className="px-2 py-1 bg-cyan-900/40 border border-cyan-500/50 rounded text-xs font-mono text-cyan-400 hover:bg-cyan-900/60 hover:border-cyan-400 transition-all flex items-center gap-1"
                        title="View execution flow"
                      >
                        <Activity className="h-3 w-3" />
                        <span>Flow</span>
                      </button>
                    )}
                    
                    {/* Details Modal Button */}
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        onRunClick(run.id, run.agent_id);
                      }}
                      className="px-2 py-1 bg-gray-700 border border-gray-600 rounded text-xs font-mono text-gray-300 hover:bg-gray-600 hover:border-gray-500 transition-all flex items-center gap-1"
                      title="View run details"
                    >
                      <Eye className="h-3 w-3" />
                      <span>Details</span>
                    </button>
                  </div>
                </div>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
};

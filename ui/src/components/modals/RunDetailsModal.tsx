import React, { useState, useEffect } from 'react';
import { X, FileText, Clock, Wrench, BarChart, Terminal, ChevronDown, ChevronUp, Copy, CheckCircle, Loader } from 'lucide-react';
import { agentRunsApi, tracesApi } from '../../api/station';
import type { AgentRunWithDetails, JaegerSpan } from '../../types/station';
import { ToolCallsView } from './ToolCallsView';

interface TimelineViewProps {
  runId: number;
}

const TimelineView: React.FC<TimelineViewProps> = ({ runId }) => {
  const [spans, setSpans] = useState<JaegerSpan[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchTrace = async () => {
      setLoading(true);
      setError(null);
      
      try {
        const response = await tracesApi.getByRunId(runId);
        const trace = response.data.trace;
        
        if (trace && trace.spans) {
          // Filter to meaningful spans only (exclude MCP setup noise)
          const meaningfulSpans = trace.spans.filter((span: JaegerSpan) => {
            const op = span.operationName;
            return (
              op === 'agent_execution_engine.execute' ||
              op === 'generate' ||
              op.startsWith('faker.') ||
              op.startsWith('__') ||
              op === 'openai/gpt-4o-mini' ||
              op === 'dotprompt.execute'
            );
          });
          
          // Sort by start time
          meaningfulSpans.sort((a: JaegerSpan, b: JaegerSpan) => a.startTime - b.startTime);
          
          setSpans(meaningfulSpans);
        }
      } catch (err: any) {
        console.error('Failed to fetch trace:', err);
        setError(err.response?.data?.error || 'Failed to load trace data. Jaeger may not be available.');
      } finally {
        setLoading(false);
      }
    };

    fetchTrace();
  }, [runId]);

  const formatDuration = (micros: number): string => {
    const ms = micros / 1000;
    if (ms < 1000) return `${Math.round(ms)}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };

  const getSpanColor = (op: string): string => {
    if (op === 'agent_execution_engine.execute') return 'bg-purple-500';
    if (op === 'generate' || op === 'openai/gpt-4o-mini') return 'bg-blue-500';
    if (op.startsWith('faker.') || op.startsWith('__')) return 'bg-green-500';
    return 'bg-gray-500';
  };

  const getSpanLabel = (op: string): string => {
    if (op === 'agent_execution_engine.execute') return 'Agent Execution';
    if (op === 'generate') return 'LLM Generate';
    if (op === 'openai/gpt-4o-mini') return 'OpenAI API';
    if (op.startsWith('faker.')) return op.replace('faker.', 'Tool: ');
    if (op.startsWith('__')) return `Tool: ${op.substring(2)}`;
    return op;
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader className="h-8 w-8 text-gray-400 animate-spin" />
        <span className="ml-3 text-gray-400 font-mono">Loading trace data...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-12">
        <div className="text-red-400 font-mono mb-2">{error}</div>
        <div className="text-gray-500 font-mono text-sm">
          Make sure Jaeger is running at http://localhost:16686
        </div>
      </div>
    );
  }

  if (spans.length === 0) {
    return (
      <div className="text-center py-12">
        <Clock className="h-12 w-12 text-gray-600 mx-auto mb-3" />
        <div className="text-gray-400 font-mono">No trace data found for this run</div>
      </div>
    );
  }

  const minTime = Math.min(...spans.map(s => s.startTime));
  const maxTime = Math.max(...spans.map(s => s.startTime + s.duration));
  const totalDuration = maxTime - minTime;

  return (
    <div className="space-y-4">
      {/* Timeline Header */}
      <div className="flex items-center justify-between p-4 bg-gray-800 rounded-lg border border-gray-700">
        <div>
          <div className="text-sm text-gray-400 mb-1">Total Execution Time</div>
          <div className="text-2xl font-mono text-cyan-400">{formatDuration(totalDuration)}</div>
        </div>
        <div>
          <div className="text-sm text-gray-400 mb-1">Span Count</div>
          <div className="text-2xl font-mono text-purple-400">{spans.length}</div>
        </div>
      </div>

      {/* Timeline Visualization */}
      <div className="space-y-2">
        {spans.map((span, index) => {
          const relativeStart = ((span.startTime - minTime) / totalDuration) * 100;
          const width = (span.duration / totalDuration) * 100;
          
          return (
            <div key={span.spanID} className="relative">
              {/* Span Label */}
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs font-mono text-gray-300">
                  {getSpanLabel(span.operationName)}
                </span>
                <span className="text-xs font-mono text-gray-500">
                  {formatDuration(span.duration)}
                </span>
              </div>
              
              {/* Timeline Bar Container */}
              <div className="relative h-8 bg-gray-800 rounded border border-gray-700 overflow-hidden">
                {/* Timeline Bar */}
                <div
                  className={`absolute h-full ${getSpanColor(span.operationName)} opacity-80 transition-all hover:opacity-100 cursor-pointer`}
                  style={{
                    left: `${relativeStart}%`,
                    width: `${Math.max(width, 0.5)}%`, // Minimum 0.5% width for visibility
                  }}
                  title={`${span.operationName}\nDuration: ${formatDuration(span.duration)}\nStart: ${formatDuration(span.startTime - minTime)}`}
                >
                  {/* Duration label inside bar if wide enough */}
                  {width > 10 && (
                    <div className="absolute inset-0 flex items-center justify-center text-xs font-mono text-white">
                      {formatDuration(span.duration)}
                    </div>
                  )}
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {/* Legend */}
      <div className="flex items-center gap-4 pt-4 border-t border-gray-700">
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-purple-500 rounded"></div>
          <span className="text-xs text-gray-400 font-mono">Agent</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-blue-500 rounded"></div>
          <span className="text-xs text-gray-400 font-mono">LLM</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-green-500 rounded"></div>
          <span className="text-xs text-gray-400 font-mono">Tools</span>
        </div>
      </div>
    </div>
  );
};

interface RunDetailsModalProps {
  runId: number | null;
  isOpen: boolean;
  onClose: () => void;
}

type TabType = 'overview' | 'timeline' | 'tools' | 'metrics' | 'debug';

interface ToolCall {
  toolName: string;
  startTime: number;
  endTime: number;
  duration: number;
  input: any;
  output: any;
  status: string;
  errorMessage?: string;
}

export const RunDetailsModal: React.FC<RunDetailsModalProps> = ({ runId, isOpen, onClose }) => {
  const [runDetails, setRunDetails] = useState<AgentRunWithDetails | null>(null);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<TabType>('overview');
  const [expandedTools, setExpandedTools] = useState<Set<number>>(new Set());
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);

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

  const toggleToolExpand = (index: number) => {
    const newExpanded = new Set(expandedTools);
    if (newExpanded.has(index)) {
      newExpanded.delete(index);
    } else {
      newExpanded.add(index);
    }
    setExpandedTools(newExpanded);
  };

  const copyToClipboard = (text: string, index: number) => {
    navigator.clipboard.writeText(text);
    setCopiedIndex(index);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  const formatDuration = (seconds: number) => {
    if (seconds < 1) return `${(seconds * 1000).toFixed(0)}ms`;
    if (seconds < 60) return `${seconds.toFixed(1)}s`;
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}m ${secs}s`;
  };

  const formatTokens = (tokens: number) => {
    return tokens.toLocaleString();
  };

  const estimateCost = (inputTokens: number, outputTokens: number, model: string): string => {
    // Pricing per 1M tokens (as of Nov 2024)
    const pricing: Record<string, { input: number; output: number }> = {
      'gpt-4o-mini': { input: 0.150, output: 0.600 },
      'gpt-4o': { input: 2.50, output: 10.00 },
      'claude-3-5-sonnet-20241022': { input: 3.00, output: 15.00 },
    };

    const modelPricing = pricing[model] || pricing['gpt-4o-mini'];
    const inputCost = (inputTokens / 1_000_000) * modelPricing.input;
    const outputCost = (outputTokens / 1_000_000) * modelPricing.output;
    const total = inputCost + outputCost;

    return `$${total.toFixed(4)}`;
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'completed': return 'text-green-400';
      case 'failed': return 'text-red-400';
      case 'running': return 'text-blue-400';
      default: return 'text-gray-400';
    }
  };

  const getStatusBg = (status: string) => {
    switch (status) {
      case 'completed': return 'bg-green-900 bg-opacity-20 border-green-500';
      case 'failed': return 'bg-red-900 bg-opacity-20 border-red-500';
      case 'running': return 'bg-blue-900 bg-opacity-20 border-blue-500';
      default: return 'bg-gray-900 bg-opacity-20 border-gray-500';
    }
  };

  if (!isOpen || !runId) return null;

  const tabs = [
    { id: 'overview', label: 'Overview', icon: FileText },
    { id: 'timeline', label: 'Timeline', icon: Clock },
    { id: 'tools', label: 'Tool Calls', icon: Wrench },
    { id: 'metrics', label: 'Performance', icon: BarChart },
    { id: 'debug', label: 'Debug', icon: Terminal },
  ];

  const toolCalls = runDetails?.execution_steps || [];

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-gray-900 border-2 border-gray-700 rounded-lg w-full max-w-6xl mx-4 max-h-[90vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-700">
          <div className="flex items-center gap-3">
            <FileText className="h-6 w-6 text-cyan-400" />
            <h2 className="text-xl font-mono font-semibold text-cyan-400">
              Run Details #{runId}
            </h2>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-800 rounded transition-colors"
          >
            <X className="h-5 w-5 text-gray-400 hover:text-gray-100" />
          </button>
        </div>

        {/* Tabs */}
        <div className="flex border-b border-gray-700 px-6 bg-gray-800">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as TabType)}
                className={`flex items-center gap-2 px-4 py-3 font-mono text-sm transition-colors border-b-2 ${
                  activeTab === tab.id
                    ? 'border-purple-500 text-purple-400 bg-gray-900'
                    : 'border-transparent text-gray-400 hover:text-gray-200'
                }`}
              >
                <Icon className="h-4 w-4" />
                {tab.label}
              </button>
            );
          })}
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6">
          {loading ? (
            <div className="text-center py-12">
              <div className="text-gray-400 font-mono">Loading run details...</div>
            </div>
          ) : runDetails ? (
            <>
              {/* Overview Tab */}
              {activeTab === 'overview' && (
                <div className="space-y-6">
                  {/* Status Card */}
                  <div className={`p-4 rounded-lg border ${getStatusBg(runDetails.status)}`}>
                    <div className="flex items-center justify-between">
                      <div>
                        <div className="text-sm text-gray-400 mb-1">Status</div>
                        <div className={`text-2xl font-mono font-bold ${getStatusColor(runDetails.status)}`}>
                          {runDetails.status.toUpperCase()}
                        </div>
                      </div>
                      <div className="text-right">
                        <div className="text-sm text-gray-400 mb-1">Duration</div>
                        <div className="text-xl font-mono text-gray-100">
                          {runDetails.duration_seconds ? formatDuration(runDetails.duration_seconds) : 'N/A'}
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Agent Info */}
                  <div className="grid grid-cols-2 gap-4">
                    <div className="bg-gray-800 p-4 rounded-lg border border-gray-700">
                      <div className="text-sm text-gray-400 mb-2">Agent</div>
                      <div className="text-lg font-mono text-cyan-400">{runDetails.agent_name}</div>
                    </div>
                    <div className="bg-gray-800 p-4 rounded-lg border border-gray-700">
                      <div className="text-sm text-gray-400 mb-2">User</div>
                      <div className="text-lg font-mono text-gray-100">{runDetails.username}</div>
                    </div>
                  </div>

                  {/* Task */}
                  <div className="bg-gray-800 p-4 rounded-lg border border-gray-700">
                    <div className="text-sm text-gray-400 mb-2">Task</div>
                    <div className="text-sm font-mono text-gray-100 whitespace-pre-wrap">{runDetails.task}</div>
                  </div>

                  {/* Response */}
                  <div className="bg-gray-800 p-4 rounded-lg border border-gray-700">
                    <div className="text-sm text-gray-400 mb-2">Final Response</div>
                    <div className="text-sm font-mono text-gray-100 whitespace-pre-wrap max-h-96 overflow-y-auto">
                      {runDetails.final_response}
                    </div>
                  </div>

                  {/* Timestamps */}
                  <div className="grid grid-cols-2 gap-4">
                    <div className="bg-gray-800 p-4 rounded-lg border border-gray-700">
                      <div className="text-sm text-gray-400 mb-2">Started At</div>
                      <div className="text-sm font-mono text-gray-100">
                        {new Date(runDetails.started_at).toLocaleString()}
                      </div>
                    </div>
                    {runDetails.completed_at && (
                      <div className="bg-gray-800 p-4 rounded-lg border border-gray-700">
                        <div className="text-sm text-gray-400 mb-2">Completed At</div>
                        <div className="text-sm font-mono text-gray-100">
                          {new Date(runDetails.completed_at).toLocaleString()}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Timeline Tab */}
              {activeTab === 'timeline' && (
                <TimelineView runId={runId} />
              )}

              {/* Tool Calls Tab */}
              {activeTab === 'tools' && runId && (
                <ToolCallsView runId={runId} />
              )}

              {/* Performance Metrics Tab */}
              {activeTab === 'metrics' && (
                <div className="space-y-6">
                  {/* Token Usage */}
                  <div className="bg-gray-800 p-6 rounded-lg border border-gray-700">
                    <h3 className="text-lg font-mono font-semibold text-cyan-400 mb-4">Token Usage</h3>
                    <div className="grid grid-cols-3 gap-4">
                      <div>
                        <div className="text-sm text-gray-400 mb-1">Input Tokens</div>
                        <div className="text-2xl font-mono text-blue-400">
                          {runDetails.input_tokens ? formatTokens(runDetails.input_tokens) : 'N/A'}
                        </div>
                      </div>
                      <div>
                        <div className="text-sm text-gray-400 mb-1">Output Tokens</div>
                        <div className="text-2xl font-mono text-green-400">
                          {runDetails.output_tokens ? formatTokens(runDetails.output_tokens) : 'N/A'}
                        </div>
                      </div>
                      <div>
                        <div className="text-sm text-gray-400 mb-1">Total Tokens</div>
                        <div className="text-2xl font-mono text-purple-400">
                          {runDetails.total_tokens ? formatTokens(runDetails.total_tokens) : 'N/A'}
                        </div>
                      </div>
                    </div>

                    {/* Cost Estimate */}
                    {runDetails.input_tokens && runDetails.output_tokens && runDetails.model_name && (
                      <div className="mt-6 pt-6 border-t border-gray-700">
                        <div className="flex items-center justify-between">
                          <div>
                            <div className="text-sm text-gray-400 mb-1">Estimated Cost</div>
                            <div className="text-sm text-gray-500 font-mono">
                              Model: {runDetails.model_name}
                            </div>
                          </div>
                          <div className="text-3xl font-mono text-yellow-400">
                            {estimateCost(runDetails.input_tokens, runDetails.output_tokens, runDetails.model_name)}
                          </div>
                        </div>
                      </div>
                    )}
                  </div>

                  {/* Duration Breakdown */}
                  <div className="bg-gray-800 p-6 rounded-lg border border-gray-700">
                    <h3 className="text-lg font-mono font-semibold text-cyan-400 mb-4">Execution Metrics</h3>
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <div className="text-sm text-gray-400 mb-1">Total Duration</div>
                        <div className="text-2xl font-mono text-gray-100">
                          {runDetails.duration_seconds ? formatDuration(runDetails.duration_seconds) : 'N/A'}
                        </div>
                      </div>
                      <div>
                        <div className="text-sm text-gray-400 mb-1">Tool Calls</div>
                        <div className="text-2xl font-mono text-gray-100">
                          {toolCalls.length}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {/* Debug Tab */}
              {activeTab === 'debug' && (
                <div className="space-y-4">
                  <div className="bg-gray-800 p-4 rounded-lg border border-gray-700">
                    <h3 className="text-sm font-mono font-semibold text-cyan-400 mb-3">Raw Run Data</h3>
                    <pre className="text-xs text-gray-300 overflow-x-auto font-mono">
                      {JSON.stringify(runDetails, null, 2)}
                    </pre>
                  </div>
                </div>
              )}
            </>
          ) : (
            <div className="text-center text-gray-400 font-mono py-12">
              Failed to load run details
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

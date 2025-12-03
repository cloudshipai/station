import React, { useState, useEffect } from 'react';
import { X, FileText, Clock, Wrench, BarChart, Terminal, ChevronDown, ChevronUp, Copy, CheckCircle, Loader, Sparkles } from 'lucide-react';
import { agentRunsApi, tracesApi } from '../../api/station';
import type { AgentRunWithDetails, JaegerSpan } from '../../types/station';
import { ToolCallsView } from './ToolCallsView';
import { BenchmarkTab } from './BenchmarkTab';

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
        <span className="ml-3 text-gray-600">Loading trace data...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-4xl mx-auto py-6">
        {/* Header */}
        <div className="mb-6">
          <h2 className="text-2xl font-semibold text-gray-900 mb-2">Execution Trace</h2>
          <p className="text-gray-600">Detailed performance insights and debugging information</p>
        </div>

        {/* Warning Banner */}
        <div className="bg-amber-50 border-l-4 border-amber-400 p-4 mb-6">
          <div className="flex items-start">
            <div className="flex-shrink-0">
              <svg className="h-5 w-5 text-amber-400" viewBox="0 0 20 20" fill="currentColor">
                <path fillRule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
              </svg>
            </div>
            <div className="ml-3">
              <p className="text-sm text-amber-800 font-medium">Jaeger tracing is not available</p>
              <p className="text-sm text-amber-700 mt-1">{error}</p>
            </div>
          </div>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
          {/* Benefits */}
          <div className="bg-white border border-gray-200 rounded-lg p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">Why Enable Tracing?</h3>
            <div className="space-y-3">
              <div className="flex items-start">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 rounded-lg flex items-center justify-center mr-3">
                  <svg className="w-5 h-5 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                  </svg>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-900">Visual Timeline</p>
                  <p className="text-sm text-gray-600">Interactive graph showing execution flow and timing</p>
                </div>
              </div>

              <div className="flex items-start">
                <div className="flex-shrink-0 w-8 h-8 bg-purple-100 rounded-lg flex items-center justify-center mr-3">
                  <svg className="w-5 h-5 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                  </svg>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-900">Performance Analysis</p>
                  <p className="text-sm text-gray-600">Identify bottlenecks and optimize execution speed</p>
                </div>
              </div>

              <div className="flex items-start">
                <div className="flex-shrink-0 w-8 h-8 bg-green-100 rounded-lg flex items-center justify-center mr-3">
                  <svg className="w-5 h-5 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                  </svg>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-900">Tool Call Inspection</p>
                  <p className="text-sm text-gray-600">View parameters and responses for every tool call</p>
                </div>
              </div>

              <div className="flex items-start">
                <div className="flex-shrink-0 w-8 h-8 bg-red-100 rounded-lg flex items-center justify-center mr-3">
                  <svg className="w-5 h-5 text-red-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-900">Debug Information</p>
                  <p className="text-sm text-gray-600">Trace errors and failures to exact operations</p>
                </div>
              </div>
            </div>
          </div>

          {/* Installation */}
          <div className="bg-white border border-gray-200 rounded-lg p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">Installation</h3>

            <div className="space-y-4">
              <div>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium text-gray-700">Step 1: Setup volume</span>
                  <span className="text-xs text-gray-500">~10 sec</span>
                </div>
                <div className="bg-gray-900 rounded-lg p-3 overflow-x-auto">
                  <code className="text-xs text-green-400 font-mono block">docker volume create jaeger-badger-data</code>
                  <code className="text-xs text-green-400 font-mono block mt-1">docker run --rm -v jaeger-badger-data:/badger busybox chown -R 10001:10001 /badger</code>
                </div>
              </div>

              <div>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium text-gray-700">Step 2: Start Jaeger</span>
                  <span className="text-xs text-gray-500">~30 sec</span>
                </div>
                <div className="bg-gray-900 rounded-lg p-3 overflow-x-auto">
                  <code className="text-xs text-green-400 font-mono block">docker run -d --name station-jaeger \</code>
                  <code className="text-xs text-gray-400 font-mono block ml-2">-e COLLECTOR_OTLP_ENABLED=true \</code>
                  <code className="text-xs text-gray-400 font-mono block ml-2">-e SPAN_STORAGE_TYPE=badger \</code>
                  <code className="text-xs text-gray-400 font-mono block ml-2">-e BADGER_EPHEMERAL=false \</code>
                  <code className="text-xs text-gray-400 font-mono block ml-2">-v jaeger-badger-data:/badger \</code>
                  <code className="text-xs text-gray-400 font-mono block ml-2">-p 16686:16686 -p 4318:4318 \</code>
                  <code className="text-xs text-gray-400 font-mono block ml-2">jaegertracing/all-in-one:latest</code>
                </div>
              </div>

              <div>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium text-gray-700">Step 3: Verify</span>
                </div>
                <a
                  href="http://localhost:16686"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="block w-full text-center px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors"
                >
                  Open Jaeger UI
                </a>
              </div>
            </div>

            <div className="mt-4 pt-4 border-t border-gray-200">
              <p className="text-xs text-gray-600">
                After Jaeger is running, <button onClick={() => window.location.reload()} className="text-blue-600 hover:text-blue-700 font-medium underline">refresh this page</button> to see the execution trace.
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (spans.length === 0) {
    return (
      <div className="text-center py-12">
        <Clock className="h-12 w-12 text-gray-400 mx-auto mb-3" />
        <div className="text-gray-600">No trace data found for this run</div>
      </div>
    );
  }

  const minTime = Math.min(...spans.map(s => s.startTime));
  const maxTime = Math.max(...spans.map(s => s.startTime + s.duration));
  const totalDuration = maxTime - minTime;

  return (
    <div className="space-y-4">
      {/* Timeline Header */}
      <div className="flex items-center justify-between p-4 bg-white rounded-lg border border-gray-200 shadow-sm">
        <div>
          <div className="text-sm text-gray-600 mb-1">Total Execution Time</div>
          <div className="text-2xl font-semibold text-primary">{formatDuration(totalDuration)}</div>
        </div>
        <div>
          <div className="text-sm text-gray-600 mb-1">Span Count</div>
          <div className="text-2xl font-semibold text-purple-600">{spans.length}</div>
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
                <span className="text-xs text-gray-700 font-medium">
                  {getSpanLabel(span.operationName)}
                </span>
                <span className="text-xs text-gray-500">
                  {formatDuration(span.duration)}
                </span>
              </div>
              
              {/* Timeline Bar Container */}
              <div className="relative h-8 bg-gray-100 rounded border border-gray-200 overflow-hidden">
                {/* Timeline Bar */}
                <div
                  className={`absolute h-full ${getSpanColor(span.operationName)} opacity-90 transition-all hover:opacity-100 cursor-pointer shadow-sm`}
                  style={{
                    left: `${relativeStart}%`,
                    width: `${Math.max(width, 0.5)}%`, // Minimum 0.5% width for visibility
                  }}
                  title={`${span.operationName}\nDuration: ${formatDuration(span.duration)}\nStart: ${formatDuration(span.startTime - minTime)}`}
                >
                  {/* Duration label inside bar if wide enough */}
                  {width > 10 && (
                    <div className="absolute inset-0 flex items-center justify-center text-xs font-medium text-white">
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
      <div className="flex items-center gap-4 pt-4 border-t border-gray-200">
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-purple-500 rounded shadow-sm"></div>
          <span className="text-xs text-gray-600 font-medium">Agent</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-blue-500 rounded shadow-sm"></div>
          <span className="text-xs text-gray-600 font-medium">LLM</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-green-500 rounded shadow-sm"></div>
          <span className="text-xs text-gray-600 font-medium">Tools</span>
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

type TabType = 'overview' | 'timeline' | 'tools' | 'metrics' | 'benchmark' | 'debug';

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
      case 'completed': return 'text-green-700';
      case 'failed': return 'text-red-700';
      case 'running': return 'text-blue-700';
      default: return 'text-gray-700';
    }
  };

  const getStatusBg = (status: string) => {
    switch (status) {
      case 'completed': return 'bg-green-50 border-green-200';
      case 'failed': return 'bg-red-50 border-red-200';
      case 'running': return 'bg-blue-50 border-blue-200';
      default: return 'bg-gray-50 border-gray-200';
    }
  };

  if (!isOpen || !runId) return null;

  const tabs = [
    { id: 'overview', label: 'Overview', icon: FileText },
    { id: 'timeline', label: 'Timeline', icon: Clock },
    { id: 'tools', label: 'Tool Calls', icon: Wrench },
    { id: 'metrics', label: 'Performance', icon: BarChart },
    { id: 'benchmark', label: 'Quality', icon: Sparkles },
    { id: 'debug', label: 'Debug', icon: Terminal },
  ];

  const toolCalls = runDetails?.tool_calls || [];

  return (
    <div 
      className="fixed inset-0 bg-black/30 flex items-center justify-center z-50 backdrop-blur-sm animate-in fade-in duration-200"
      onClick={onClose}
    >
      <div 
        className="bg-white border border-gray-200 rounded-xl shadow-lg w-full max-w-6xl mx-4 max-h-[90vh] flex flex-col animate-in zoom-in-95 fade-in slide-in-from-bottom-4 duration-300"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200">
          <div className="flex items-center gap-3">
            <FileText className="h-6 w-6 text-primary" />
            <h2 className="text-xl font-semibold text-gray-900">
              Run Details #{runId}
            </h2>
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-100 rounded transition-colors"
          >
            <X className="h-5 w-5 text-gray-500 hover:text-gray-900" />
          </button>
        </div>

        {/* Tabs */}
        <div className="flex border-b border-gray-200 px-6 bg-gray-50">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as TabType)}
                className={`flex items-center gap-2 px-4 py-3 text-sm font-medium transition-colors border-b-2 ${
                  activeTab === tab.id
                    ? 'border-primary text-primary bg-white'
                    : 'border-transparent text-gray-600 hover:text-gray-900'
                }`}
              >
                <Icon className="h-4 w-4" />
                {tab.label}
              </button>
            );
          })}
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto p-6 bg-gray-50">
          {loading ? (
            <div className="text-center py-12">
              <div className="text-gray-600">Loading run details...</div>
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
                        <div className="text-sm text-gray-600 mb-1">Status</div>
                        <div className={`text-2xl font-semibold ${getStatusColor(runDetails.status)}`}>
                          {runDetails.status.toUpperCase()}
                        </div>
                      </div>
                      <div className="text-right">
                        <div className="text-sm text-gray-600 mb-1">Duration</div>
                        <div className="text-xl font-medium text-gray-900">
                          {runDetails.duration_seconds ? formatDuration(runDetails.duration_seconds) : 'N/A'}
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Agent Info */}
                  <div className="grid grid-cols-2 gap-4">
                    <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-sm">
                      <div className="text-sm text-gray-600 mb-2">Agent</div>
                      <div className="text-lg font-medium text-primary">{runDetails.agent_name}</div>
                    </div>
                    <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-sm">
                      <div className="text-sm text-gray-600 mb-2">User</div>
                      <div className="text-lg font-medium text-gray-900">{runDetails.username}</div>
                    </div>
                  </div>

                  {/* Task */}
                  <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-sm">
                    <div className="text-sm text-gray-600 mb-2 font-medium">Task</div>
                    <div className="text-sm text-gray-900 whitespace-pre-wrap">{runDetails.task}</div>
                  </div>

                  {/* Response */}
                  <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-sm">
                    <div className="text-sm text-gray-600 mb-2 font-medium">Final Response</div>
                    <div className="text-sm text-gray-900 whitespace-pre-wrap max-h-96 overflow-y-auto">
                      {runDetails.final_response}
                    </div>
                  </div>

                  {/* Timestamps */}
                  <div className="grid grid-cols-2 gap-4">
                    <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-sm">
                      <div className="text-sm text-gray-600 mb-2">Started At</div>
                      <div className="text-sm text-gray-900">
                        {new Date(runDetails.started_at).toLocaleString()}
                      </div>
                    </div>
                    {runDetails.completed_at && (
                      <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-sm">
                        <div className="text-sm text-gray-600 mb-2">Completed At</div>
                        <div className="text-sm text-gray-900">
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
                  <div className="bg-white p-6 rounded-lg border border-gray-200 shadow-sm">
                    <h3 className="text-lg font-semibold text-gray-900 mb-4">Token Usage</h3>
                    <div className="grid grid-cols-3 gap-4">
                      <div>
                        <div className="text-sm text-gray-600 mb-1">Input Tokens</div>
                        <div className="text-2xl font-semibold text-blue-600">
                          {runDetails.input_tokens ? formatTokens(runDetails.input_tokens) : 'N/A'}
                        </div>
                      </div>
                      <div>
                        <div className="text-sm text-gray-600 mb-1">Output Tokens</div>
                        <div className="text-2xl font-semibold text-green-600">
                          {runDetails.output_tokens ? formatTokens(runDetails.output_tokens) : 'N/A'}
                        </div>
                      </div>
                      <div>
                        <div className="text-sm text-gray-600 mb-1">Total Tokens</div>
                        <div className="text-2xl font-semibold text-purple-600">
                          {runDetails.total_tokens ? formatTokens(runDetails.total_tokens) : 'N/A'}
                        </div>
                      </div>
                    </div>

                    {/* Cost Estimate */}
                    {runDetails.input_tokens && runDetails.output_tokens && runDetails.model_name && (
                      <div className="mt-6 pt-6 border-t border-gray-200">
                        <div className="flex items-center justify-between">
                          <div>
                            <div className="text-sm text-gray-600 mb-1">Estimated Cost</div>
                            <div className="text-sm text-gray-500">
                              Model: {runDetails.model_name}
                            </div>
                          </div>
                          <div className="text-3xl font-semibold text-yellow-600">
                            {estimateCost(runDetails.input_tokens, runDetails.output_tokens, runDetails.model_name)}
                          </div>
                        </div>
                      </div>
                    )}
                  </div>

                  {/* Duration Breakdown */}
                  <div className="bg-white p-6 rounded-lg border border-gray-200 shadow-sm">
                    <h3 className="text-lg font-semibold text-gray-900 mb-4">Execution Metrics</h3>
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <div className="text-sm text-gray-600 mb-1">Total Duration</div>
                        <div className="text-2xl font-semibold text-gray-900">
                          {runDetails.duration_seconds ? formatDuration(runDetails.duration_seconds) : 'N/A'}
                        </div>
                      </div>
                      <div>
                        <div className="text-sm text-gray-600 mb-1">Tool Calls</div>
                        <div className="text-2xl font-semibold text-gray-900">
                          {toolCalls.length}
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              )}

              {/* Benchmark Tab */}
              {activeTab === 'benchmark' && (
                <BenchmarkTab runId={runId} />
              )}

              {/* Debug Tab */}
              {activeTab === 'debug' && (
                <div className="space-y-4">
                  <div className="bg-white p-4 rounded-lg border border-gray-200 shadow-sm">
                    <h3 className="text-sm font-semibold text-gray-900 mb-3">Raw Run Data</h3>
                    <pre className="text-xs text-gray-700 overflow-x-auto font-mono bg-gray-50 p-4 rounded border border-gray-200">
                      {JSON.stringify(runDetails, null, 2)}
                    </pre>
                  </div>
                </div>
              )}
            </>
          ) : (
            <div className="text-center text-gray-600 py-12">
              Failed to load run details
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

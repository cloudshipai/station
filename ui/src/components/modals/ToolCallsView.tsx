import React, { useState, useEffect, useCallback } from 'react';
import { Wrench, ChevronDown, ChevronUp, Copy, CheckCircle, Clock, Zap, Database, DollarSign, Rocket, AlertTriangle, Bot, BarChart2, Check } from 'lucide-react';
import { tracesApi } from '../../api/station';
import type { JaegerSpan, JaegerTag } from '../../types/station';

const JAEGER_DOCKER_COMMAND = `docker run -d --name jaeger -e COLLECTOR_OTLP_ENABLED=true -e SPAN_STORAGE_TYPE=badger -e BADGER_EPHEMERAL=false -e BADGER_DIRECTORY_VALUE=/badger/data -e BADGER_DIRECTORY_KEY=/badger/key -v jaeger_data:/badger -p 16686:16686 -p 4317:4317 -p 4318:4318 jaegertracing/all-in-one:latest`;

interface ToolCallsViewProps {
  runId: number;
}

interface ToolCallSpan {
  spanID: string;
  toolName: string;
  operationName: string;
  startTime: number;
  duration: number; // microseconds
  input?: any;
  output?: any;
  error?: string;
  tags: Record<string, any>;
}

export const ToolCallsView: React.FC<ToolCallsViewProps> = ({ runId }) => {
  const [toolCalls, setToolCalls] = useState<ToolCallSpan[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expandedCalls, setExpandedCalls] = useState<Set<string>>(new Set());
  const [copiedIndex, setCopiedIndex] = useState<string | null>(null);
  const [copiedCommand, setCopiedCommand] = useState(false);

  const copyDockerCommand = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(JAEGER_DOCKER_COMMAND);
      setCopiedCommand(true);
      setTimeout(() => setCopiedCommand(false), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  }, []);

  useEffect(() => {
    const fetchToolCalls = async () => {
      setLoading(true);
      setError(null);
      
      try {
        const response = await tracesApi.getByRunId(runId);
        const trace = response.data.trace;
        
        if (trace && trace.spans) {
          // Filter spans that represent tool calls
          const toolSpans = trace.spans.filter((span: JaegerSpan) => {
            const op = span.operationName;
            return (
              op.startsWith('faker.') || 
              op.startsWith('__') ||
              op.includes('tool.')
            );
          });

          // Parse span tags into a more usable format
          const parsedToolCalls: ToolCallSpan[] = toolSpans.map((span: JaegerSpan) => {
            const tags: Record<string, any> = {};
            span.tags.forEach((tag: JaegerTag) => {
              tags[tag.key] = tag.value;
            });

            // Extract tool name from operation name
            let toolName = span.operationName;
            if (toolName.startsWith('faker.')) {
              toolName = toolName.replace('faker.', '');
            } else if (toolName.startsWith('__')) {
              toolName = toolName.substring(2);
            }

            // Try to extract input/output from genkit tags (primary) or tool tags (fallback)
            let input: any = tags['genkit:input'] || tags['tool.input'] || null;
            let output: any = tags['genkit:output'] || tags['tool.output'] || tags['tool.result'] || null;
            let errorMsg: string | undefined = tags['error.message'] || tags['error'] || undefined;

            // Parse JSON strings and handle genkit's nested structure
            if (typeof input === 'string') {
              try { 
                input = JSON.parse(input);
              } catch {}
            }
            
            if (typeof output === 'string') {
              try { 
                const parsed = JSON.parse(output);
                // Genkit wraps output in { content: [{ type: "text", text: "..." }] }
                if (parsed.content && Array.isArray(parsed.content) && parsed.content[0]?.text) {
                  // Try to parse the inner text as JSON
                  try {
                    output = JSON.parse(parsed.content[0].text);
                  } catch {
                    // If it's not JSON, use the text as-is
                    output = parsed.content[0].text;
                  }
                } else {
                  output = parsed;
                }
              } catch {}
            }

            return {
              spanID: span.spanID,
              toolName,
              operationName: span.operationName,
              startTime: span.startTime,
              duration: span.duration,
              input,
              output,
              error: errorMsg,
              tags,
            };
          });

          // Sort by start time
          parsedToolCalls.sort((a, b) => a.startTime - b.startTime);
          setToolCalls(parsedToolCalls);
        }
      } catch (err: any) {
        console.error('Failed to fetch tool calls:', err);
        setError(err.response?.data?.error || 'Failed to load tool call data');
      } finally {
        setLoading(false);
      }
    };

    fetchToolCalls();
  }, [runId]);

  const toggleExpand = (spanID: string) => {
    const newExpanded = new Set(expandedCalls);
    if (newExpanded.has(spanID)) {
      newExpanded.delete(spanID);
    } else {
      newExpanded.add(spanID);
    }
    setExpandedCalls(newExpanded);
  };

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedIndex(id);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  const formatDuration = (micros: number): string => {
    const ms = micros / 1000;
    if (ms < 1000) return `${Math.round(ms)}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };

  const getToolIconComponent = (toolName: string) => {
    const iconClass = "h-5 w-5";
    if (toolName.includes('agent_')) return <Bot className={`${iconClass} text-purple-600`} />;
    if (toolName.includes('get_') || toolName.includes('list_')) return <BarChart2 className={`${iconClass} text-blue-600`} />;
    if (toolName.includes('cost') || toolName.includes('billing')) return <DollarSign className={`${iconClass} text-green-600`} />;
    if (toolName.includes('deployment')) return <Rocket className={`${iconClass} text-purple-600`} />;
    if (toolName.includes('incident')) return <AlertTriangle className={`${iconClass} text-red-600`} />;
    return <Wrench className={`${iconClass} text-gray-600`} />;
  };

  if (loading) {
    return (
      <div className="text-center py-12">
        <div className="text-gray-600">Loading tool calls...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-4xl mx-auto py-6">
        {/* Header */}
        <div className="mb-6">
          <h2 className="text-2xl font-semibold text-gray-900 mb-2">Tool Calls</h2>
          <p className="text-gray-600">MCP tools invoked during agent execution</p>
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
          {/* Benefits Section */}
          <div className="bg-white border border-gray-200 rounded-lg p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">Why Enable Tool Call Tracking?</h3>
            <div className="space-y-3">
              <div className="flex items-start">
                <div className="flex-shrink-0 w-8 h-8 bg-blue-100 rounded-lg flex items-center justify-center mr-3">
                  <svg className="w-5 h-5 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                  </svg>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-900">Detailed Tool Inspection</p>
                  <p className="text-sm text-gray-600">View input parameters and output results for every MCP tool call</p>
                </div>
              </div>

              <div className="flex items-start">
                <div className="flex-shrink-0 w-8 h-8 bg-purple-100 rounded-lg flex items-center justify-center mr-3">
                  <svg className="w-5 h-5 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-900">Performance Metrics</p>
                  <p className="text-sm text-gray-600">Track execution time for each tool call to optimize agent performance</p>
                </div>
              </div>

              <div className="flex items-start">
                <div className="flex-shrink-0 w-8 h-8 bg-green-100 rounded-lg flex items-center justify-center mr-3">
                  <svg className="w-5 h-5 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                  </svg>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-900">Debugging Support</p>
                  <p className="text-sm text-gray-600">Identify which tools were called and troubleshoot agent behavior</p>
                </div>
              </div>

              <div className="flex items-start">
                <div className="flex-shrink-0 w-8 h-8 bg-orange-100 rounded-lg flex items-center justify-center mr-3">
                  <svg className="w-5 h-5 text-orange-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                  </svg>
                </div>
                <div>
                  <p className="text-sm font-medium text-gray-900">Usage Analytics</p>
                  <p className="text-sm text-gray-600">See which MCP tools are used most frequently in your agents</p>
                </div>
              </div>
            </div>
          </div>

          {/* Installation Section */}
          <div className="bg-white border border-gray-200 rounded-lg p-6">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">Quick Setup</h3>
            <div className="space-y-4">
              <div>
                <p className="text-sm text-gray-600 mb-3">
                  Run this command to start Jaeger with persistent storage:
                </p>
                <div className="relative">
                  <div className="bg-gray-900 rounded-lg p-3 pr-12 overflow-x-auto">
                    <code className="text-xs text-green-400 font-mono whitespace-pre-wrap break-all">
                      {JAEGER_DOCKER_COMMAND}
                    </code>
                  </div>
                  <button
                    onClick={copyDockerCommand}
                    className={`absolute top-2 right-2 p-2 rounded-md transition-colors ${
                      copiedCommand 
                        ? 'bg-green-600 text-white' 
                        : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
                    }`}
                    title="Copy command"
                  >
                    {copiedCommand ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </button>
                </div>
              </div>

              <div className="flex items-center gap-3">
                <a
                  href="http://localhost:16686"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex-1 text-center px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors"
                >
                  Open Jaeger UI
                </a>
                <button
                  onClick={() => window.location.reload()}
                  className="flex-1 px-4 py-2 border border-gray-300 hover:bg-gray-50 text-gray-700 rounded-lg text-sm font-medium transition-colors"
                >
                  Refresh Page
                </button>
              </div>

              <p className="text-xs text-gray-500">
                Data persists in the <code className="bg-gray-100 px-1 rounded">jaeger_data</code> Docker volume across restarts.
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  if (toolCalls.length === 0) {
    return (
      <div className="max-w-2xl mx-auto py-12 text-center">
        <div className="inline-flex items-center justify-center w-16 h-16 bg-gray-100 rounded-full mb-4">
          <Wrench className="h-8 w-8 text-gray-400" />
        </div>
        <p className="text-gray-600 font-medium">No Tool Calls</p>
        <p className="text-gray-500 text-sm mt-1">This agent run didn't invoke any tools</p>
      </div>
    );
  }

  return (
    <div className="max-w-5xl mx-auto">
      {/* Header */}
      <div className="mb-6">
        <h2 className="text-2xl font-semibold text-gray-900 mb-2">Tool Calls</h2>
        <p className="text-gray-600">MCP tools invoked during agent execution</p>
      </div>

      {/* Summary Stats */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-600">Total Calls</span>
            <Wrench className="h-4 w-4 text-gray-400" />
          </div>
          <div className="text-2xl font-bold text-gray-900">{toolCalls.length}</div>
        </div>

        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-600">Total Time</span>
            <Clock className="h-4 w-4 text-gray-400" />
          </div>
          <div className="text-2xl font-bold text-gray-900">
            {formatDuration(toolCalls.reduce((sum, call) => sum + call.duration, 0))}
          </div>
        </div>

        <div className="bg-white border border-gray-200 rounded-lg p-4">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-600">Avg Duration</span>
            <Zap className="h-4 w-4 text-gray-400" />
          </div>
          <div className="text-2xl font-bold text-gray-900">
            {formatDuration(toolCalls.reduce((sum, call) => sum + call.duration, 0) / toolCalls.length)}
          </div>
        </div>
      </div>

      {/* Tool Call List */}
      <div className="space-y-3">
        {toolCalls.map((call, index) => (
          <div
            key={call.spanID}
            className="bg-white border border-gray-200 rounded-lg overflow-hidden transition-all shadow-sm hover:shadow-md"
          >
            <div
              className="flex items-center justify-between p-4 cursor-pointer hover:bg-gray-50 transition-colors"
              onClick={() => toggleExpand(call.spanID)}
            >
              <div className="flex items-center gap-3 flex-1">
                {getToolIconComponent(call.toolName)}
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-gray-500 text-xs font-medium">#{index + 1}</span>
                    <span className="text-gray-900 font-medium">{call.toolName}</span>
                  </div>
                  {call.error && (
                    <div className="flex items-center gap-1 text-red-600 text-xs mt-1">
                      <AlertTriangle className="h-3 w-3" />
                      {call.error}
                    </div>
                  )}
                </div>
              </div>

              <div className="flex items-center gap-4">
                <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-50 rounded-lg border border-gray-200">
                  <Clock className="h-3.5 w-3.5 text-gray-500" />
                  <span className="text-gray-900 text-sm font-semibold">
                    {formatDuration(call.duration)}
                  </span>
                </div>
                {expandedCalls.has(call.spanID) ? (
                  <ChevronUp className="h-5 w-5 text-gray-400" />
                ) : (
                  <ChevronDown className="h-5 w-5 text-gray-400" />
                )}
              </div>
            </div>

            {expandedCalls.has(call.spanID) && (
              <div className="border-t border-gray-200 bg-gray-50 p-5 space-y-4">
              {/* Input Section */}
              {call.input && (
                <div>
                  <div className="flex items-center justify-between mb-3">
                    <h4 className="text-sm font-semibold text-gray-900 flex items-center gap-2">
                      <Database className="h-4 w-4 text-gray-600" />
                      Input
                    </h4>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        copyToClipboard(JSON.stringify(call.input, null, 2), `${call.spanID}-input`);
                      }}
                      className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-white hover:bg-gray-100 text-gray-700 rounded-lg border border-gray-200 transition-colors font-medium"
                    >
                      {copiedIndex === `${call.spanID}-input` ? (
                        <><CheckCircle className="h-3.5 w-3.5 text-green-600" /> Copied</>
                      ) : (
                        <><Copy className="h-3.5 w-3.5" /> Copy</>
                      )}
                    </button>
                  </div>
                  <div className="bg-gray-900 border border-gray-700 rounded-lg p-4 max-h-80 overflow-auto">
                    <pre className="text-xs text-green-400 font-mono whitespace-pre-wrap break-words">
                      {JSON.stringify(call.input, null, 2)}
                    </pre>
                  </div>
                </div>
              )}

              {/* Output Section */}
              {call.output && (
                <div>
                  <div className="flex items-center justify-between mb-3">
                    <h4 className="text-sm font-semibold text-gray-900 flex items-center gap-2">
                      <CheckCircle className="h-4 w-4 text-gray-600" />
                      Output
                    </h4>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        copyToClipboard(JSON.stringify(call.output, null, 2), `${call.spanID}-output`);
                      }}
                      className="flex items-center gap-1.5 px-3 py-1.5 text-xs bg-white hover:bg-gray-100 text-gray-700 rounded-lg border border-gray-200 transition-colors font-medium"
                    >
                      {copiedIndex === `${call.spanID}-output` ? (
                        <><CheckCircle className="h-3.5 w-3.5 text-green-600" /> Copied</>
                      ) : (
                        <><Copy className="h-3.5 w-3.5" /> Copy</>
                      )}
                    </button>
                  </div>
                  <div className="bg-gray-900 border border-gray-700 rounded-lg p-4 max-h-[32rem] overflow-auto">
                    <pre className="text-xs text-green-400 font-mono whitespace-pre-wrap break-words">
                      {JSON.stringify(call.output, null, 2)}
                    </pre>
                  </div>
                </div>
              )}

              {/* Span Metadata - only show if no input/output */}
              {!call.input && !call.output && (
                <div className="bg-white border border-gray-200 rounded-lg p-4">
                  <h4 className="text-sm font-semibold text-gray-900 mb-3">Span Metadata</h4>
                  <div className="grid grid-cols-2 gap-3 text-xs">
                    <div className="text-gray-600 font-medium">Span ID:</div>
                    <div className="text-gray-900 font-mono">{call.spanID.substring(0, 16)}...</div>

                    <div className="text-gray-600 font-medium">Operation:</div>
                    <div className="text-gray-900">{call.operationName}</div>

                    {Object.keys(call.tags).length > 0 && (
                      <>
                        <div className="text-gray-700 font-semibold col-span-2 mt-2 mb-1">Available Tags:</div>
                        {Object.entries(call.tags).slice(0, 8).map(([key, value]) => {
                          // Skip genkit internal tags for cleaner display
                          if (key.startsWith('otel.') || key.startsWith('span.')) return null;
                          const valStr = String(value);
                          return (
                            <React.Fragment key={key}>
                              <div className="text-gray-600 font-medium pl-2">{key}:</div>
                              <div className="text-gray-900 font-mono break-all">
                                {valStr.length > 100 ? `${valStr.substring(0, 100)}...` : valStr}
                              </div>
                            </React.Fragment>
                          );
                        })}
                      </>
                    )}
                  </div>
                </div>
              )}
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

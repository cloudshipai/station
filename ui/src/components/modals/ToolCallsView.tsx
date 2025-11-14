import React, { useState, useEffect } from 'react';
import { Wrench, ChevronDown, ChevronUp, Copy, CheckCircle, Clock, Zap } from 'lucide-react';
import { tracesApi } from '../../api/station';
import type { JaegerSpan, JaegerTag } from '../../types/station';

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

  const getToolColor = (toolName: string): string => {
    if (toolName.includes('get_') || toolName.includes('list_') || toolName.includes('read_')) {
      return 'border-blue-500 bg-blue-900/20';
    }
    if (toolName.includes('cost') || toolName.includes('billing')) {
      return 'border-green-500 bg-green-900/20';
    }
    if (toolName.includes('agent_')) {
      return 'border-purple-500 bg-purple-900/20';
    }
    return 'border-gray-500 bg-gray-900/20';
  };

  const getToolIcon = (toolName: string): string => {
    if (toolName.includes('agent_')) return '🤖';
    if (toolName.includes('get_') || toolName.includes('list_')) return '📊';
    if (toolName.includes('cost') || toolName.includes('billing')) return '💰';
    if (toolName.includes('deployment')) return '🚀';
    if (toolName.includes('incident')) return '🚨';
    return '🔧';
  };

  if (loading) {
    return (
      <div className="text-center py-12">
        <div className="text-gray-400 font-mono">Loading tool calls...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="text-center py-12">
        <div className="text-red-400 font-mono mb-2">{error}</div>
        <div className="text-gray-500 font-mono text-sm">
          Tool call data may not be available for this run
        </div>
      </div>
    );
  }

  if (toolCalls.length === 0) {
    return (
      <div className="text-center py-12">
        <div className="text-gray-400 font-mono">No tool calls recorded for this run</div>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {/* Summary Stats */}
      <div className="grid grid-cols-3 gap-4 mb-6">
        <div className="bg-gray-800 p-4 rounded-lg border border-gray-700">
          <div className="flex items-center gap-2 mb-2">
            <Wrench className="h-4 w-4 text-cyan-400" />
            <div className="text-sm text-gray-400">Total Tool Calls</div>
          </div>
          <div className="text-2xl font-mono text-cyan-400">{toolCalls.length}</div>
        </div>
        
        <div className="bg-gray-800 p-4 rounded-lg border border-gray-700">
          <div className="flex items-center gap-2 mb-2">
            <Clock className="h-4 w-4 text-blue-400" />
            <div className="text-sm text-gray-400">Total Tool Time</div>
          </div>
          <div className="text-2xl font-mono text-blue-400">
            {formatDuration(toolCalls.reduce((sum, call) => sum + call.duration, 0))}
          </div>
        </div>

        <div className="bg-gray-800 p-4 rounded-lg border border-gray-700">
          <div className="flex items-center gap-2 mb-2">
            <Zap className="h-4 w-4 text-green-400" />
            <div className="text-sm text-gray-400">Avg Duration</div>
          </div>
          <div className="text-2xl font-mono text-green-400">
            {formatDuration(toolCalls.reduce((sum, call) => sum + call.duration, 0) / toolCalls.length)}
          </div>
        </div>
      </div>

      {/* Tool Call List */}
      {toolCalls.map((call, index) => (
        <div
          key={call.spanID}
          className={`border rounded-lg overflow-hidden transition-all ${getToolColor(call.toolName)}`}
        >
          <div
            className="flex items-center justify-between p-4 cursor-pointer hover:bg-gray-800/50 transition-colors"
            onClick={() => toggleExpand(call.spanID)}
          >
            <div className="flex items-center gap-3 flex-1">
              <span className="text-2xl">{getToolIcon(call.toolName)}</span>
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-gray-500 font-mono text-xs">#{index + 1}</span>
                  <span className="font-mono text-gray-100 font-semibold">{call.toolName}</span>
                </div>
                {call.error && (
                  <div className="text-red-400 text-xs font-mono mt-1">
                    ⚠️ {call.error}
                  </div>
                )}
              </div>
            </div>
            
            <div className="flex items-center gap-4">
              <div className="flex items-center gap-2 px-3 py-1 bg-gray-900/50 rounded border border-gray-700">
                <Clock className="h-3 w-3 text-gray-400" />
                <span className="text-gray-300 font-mono text-sm font-semibold">
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
            <div className="border-t border-gray-700 bg-gray-900/30 p-4 space-y-4">
              {/* Input Section */}
              {call.input && (
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="text-sm font-mono font-semibold text-cyan-400">Input</h4>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        copyToClipboard(JSON.stringify(call.input, null, 2), `${call.spanID}-input`);
                      }}
                      className="flex items-center gap-1 px-2 py-1 text-xs bg-gray-700 hover:bg-gray-600 text-gray-300 rounded transition-colors"
                    >
                      {copiedIndex === `${call.spanID}-input` ? (
                        <><CheckCircle className="h-3 w-3" /> Copied</>
                      ) : (
                        <><Copy className="h-3 w-3" /> Copy</>
                      )}
                    </button>
                  </div>
                  <div className="bg-gray-950 border border-gray-800 rounded p-4 max-h-80 overflow-auto">
                    <pre className="text-xs text-gray-300 font-mono whitespace-pre-wrap break-words">
                      {JSON.stringify(call.input, null, 2)}
                    </pre>
                  </div>
                </div>
              )}

              {/* Output Section */}
              {call.output && (
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="text-sm font-mono font-semibold text-green-400">Output</h4>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        copyToClipboard(JSON.stringify(call.output, null, 2), `${call.spanID}-output`);
                      }}
                      className="flex items-center gap-1 px-2 py-1 text-xs bg-gray-700 hover:bg-gray-600 text-gray-300 rounded transition-colors"
                    >
                      {copiedIndex === `${call.spanID}-output` ? (
                        <><CheckCircle className="h-3 w-3" /> Copied</>
                      ) : (
                        <><Copy className="h-3 w-3" /> Copy</>
                      )}
                    </button>
                  </div>
                  <div className="bg-gray-950 border border-gray-800 rounded p-4 max-h-[32rem] overflow-auto">
                    <pre className="text-xs text-gray-300 font-mono whitespace-pre-wrap break-words">
                      {JSON.stringify(call.output, null, 2)}
                    </pre>
                  </div>
                </div>
              )}

              {/* Span Metadata - only show if no input/output */}
              {!call.input && !call.output && (
                <div className="pt-4 border-t border-gray-800">
                  <h4 className="text-xs font-mono font-semibold text-gray-400 mb-2">Span Metadata</h4>
                  <div className="grid grid-cols-2 gap-2 text-xs font-mono">
                    <div className="text-gray-500">Span ID:</div>
                    <div className="text-gray-300 font-mono">{call.spanID.substring(0, 16)}...</div>
                    
                    <div className="text-gray-500">Operation:</div>
                    <div className="text-gray-300">{call.operationName}</div>
                    
                    {Object.keys(call.tags).length > 0 && (
                      <>
                        <div className="text-gray-500 col-span-2 mt-2 mb-1">Available Tags:</div>
                        {Object.entries(call.tags).slice(0, 8).map(([key, value]) => {
                          // Skip genkit internal tags for cleaner display
                          if (key.startsWith('otel.') || key.startsWith('span.')) return null;
                          const valStr = String(value);
                          return (
                            <React.Fragment key={key}>
                              <div className="text-gray-500 pl-2 text-xs">{key}:</div>
                              <div className="text-gray-300 font-mono text-xs break-all">
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
  );
};

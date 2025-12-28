import React, { useState, useEffect, useCallback } from 'react';
import { Clock, Loader, Copy, Check, ExternalLink } from 'lucide-react';
import { tracesApi } from '../../api/station';
import type { JaegerSpan } from '../../types/station';

interface WorkflowTimelineViewProps {
  runId: string;
}

const JAEGER_DOCKER_COMMAND = `docker run -d --name jaeger -e COLLECTOR_OTLP_ENABLED=true -e SPAN_STORAGE_TYPE=badger -e BADGER_EPHEMERAL=false -e BADGER_DIRECTORY_VALUE=/badger/data -e BADGER_DIRECTORY_KEY=/badger/key -v jaeger_data:/badger -p 16686:16686 -p 4317:4317 -p 4318:4318 jaegertracing/all-in-one:latest`;

export const WorkflowTimelineView: React.FC<WorkflowTimelineViewProps> = ({ runId }) => {
  const [spans, setSpans] = useState<JaegerSpan[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const copyCommand = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(JAEGER_DOCKER_COMMAND);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  }, []);

  useEffect(() => {
    const fetchTrace = async () => {
      setLoading(true);
      setError(null);
      
      try {
        const response = await tracesApi.getByWorkflowRunId(runId);
        const trace = response.data.trace;
        
        if (trace && trace.spans) {
          const meaningfulSpans = trace.spans.filter((span: JaegerSpan) => {
            const op = span.operationName;
            return (
              op.startsWith('workflow.') ||
              op.startsWith('agent.') ||
              op === 'generate' ||
              op.startsWith('faker.') ||
              op.startsWith('__') ||
              op === 'openai/gpt-4o-mini' ||
              op === 'dotprompt.execute' ||
              op === 'mcp.load_tools'
            );
          });
          
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
    if (op.startsWith('workflow.run')) return 'bg-indigo-500';
    if (op.startsWith('workflow.step')) return 'bg-purple-500';
    if (op.startsWith('agent.')) return 'bg-blue-500';
    if (op === 'generate' || op === 'openai/gpt-4o-mini') return 'bg-cyan-500';
    if (op.startsWith('faker.') || op.startsWith('__')) return 'bg-green-500';
    if (op === 'mcp.load_tools') return 'bg-gray-400';
    return 'bg-gray-500';
  };

  const getSpanLabel = (op: string): string => {
    if (op.startsWith('workflow.run.')) return `Workflow: ${op.replace('workflow.run.', '')}`;
    if (op.startsWith('workflow.step.')) return `Step: ${op.replace('workflow.step.', '')}`;
    if (op === 'agent.execute_with_run_id') return 'Agent Execution';
    if (op.startsWith('agent.')) return op.replace('agent.', 'Agent: ');
    if (op === 'generate') return 'LLM Generate';
    if (op === 'openai/gpt-4o-mini') return 'OpenAI API';
    if (op === 'dotprompt.execute') return 'Prompt Template';
    if (op === 'mcp.load_tools') return 'Load MCP Tools';
    if (op.startsWith('faker.')) return op.replace('faker.', 'Tool: ');
    if (op.startsWith('__')) return `Tool: ${op.substring(2)}`;
    return op;
  };

  const getSpanDepth = (span: JaegerSpan, allSpans: JaegerSpan[]): number => {
    let depth = 0;
    let currentSpan = span;
    
    while (currentSpan.references && currentSpan.references.length > 0) {
      const parentRef = currentSpan.references.find(r => r.refType === 'CHILD_OF');
      if (!parentRef) break;
      
      const parent = allSpans.find(s => s.spanID === parentRef.spanID);
      if (!parent) break;
      
      depth++;
      currentSpan = parent;
    }
    
    return depth;
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
      <div className="py-6">
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

        <div className="bg-white border border-gray-200 rounded-lg p-6">
          <h3 className="text-lg font-semibold text-gray-900 mb-4">Quick Setup</h3>
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
              onClick={copyCommand}
              className={`absolute top-2 right-2 p-2 rounded-md transition-colors ${
                copied 
                  ? 'bg-green-600 text-white' 
                  : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
              }`}
              title="Copy command"
            >
              {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
            </button>
          </div>
          <div className="flex items-center gap-3 mt-4">
            <a
              href="http://localhost:16686"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-sm font-medium transition-colors"
            >
              Open Jaeger UI
              <ExternalLink className="h-4 w-4" />
            </a>
          </div>
        </div>
      </div>
    );
  }

  if (spans.length === 0) {
    return (
      <div className="text-center py-12">
        <Clock className="h-12 w-12 text-gray-400 mx-auto mb-3" />
        <div className="text-gray-600">No trace data found for this workflow run</div>
      </div>
    );
  }

  const minTime = Math.min(...spans.map(s => s.startTime));
  const maxTime = Math.max(...spans.map(s => s.startTime + s.duration));
  const totalDuration = maxTime - minTime;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between p-4 bg-white rounded-lg border border-gray-200 shadow-sm">
        <div>
          <div className="text-sm text-gray-600 mb-1">Total Execution Time</div>
          <div className="text-2xl font-semibold text-indigo-600">{formatDuration(totalDuration)}</div>
        </div>
        <div>
          <div className="text-sm text-gray-600 mb-1">Span Count</div>
          <div className="text-2xl font-semibold text-purple-600">{spans.length}</div>
        </div>
        <a
          href={`http://localhost:16686/search?service=station&tags=%7B%22workflow.run_id%22%3A%22${runId}%22%7D`}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-2 px-3 py-2 text-sm text-gray-600 hover:text-gray-900 border border-gray-200 rounded-lg hover:bg-gray-50"
        >
          Open in Jaeger
          <ExternalLink className="h-4 w-4" />
        </a>
      </div>

      <div className="space-y-2">
        {spans.map((span) => {
          const relativeStart = ((span.startTime - minTime) / totalDuration) * 100;
          const width = (span.duration / totalDuration) * 100;
          const depth = getSpanDepth(span, spans);
          const indentPx = depth * 16;
          
          return (
            <div key={span.spanID} className="relative" style={{ marginLeft: `${indentPx}px` }}>
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs text-gray-700 font-medium truncate">
                  {getSpanLabel(span.operationName)}
                </span>
                <span className="text-xs text-gray-500 ml-2 shrink-0">
                  {formatDuration(span.duration)}
                </span>
              </div>
              
              <div className="relative h-6 bg-gray-100 rounded border border-gray-200 overflow-hidden">
                <div
                  className={`absolute h-full ${getSpanColor(span.operationName)} opacity-90 transition-all hover:opacity-100 cursor-pointer shadow-sm rounded-sm`}
                  style={{
                    left: `${relativeStart}%`,
                    width: `${Math.max(width, 0.5)}%`,
                  }}
                  title={`${span.operationName}\nDuration: ${formatDuration(span.duration)}\nStart: ${formatDuration(span.startTime - minTime)}`}
                >
                  {width > 12 && (
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

      <div className="flex items-center gap-4 pt-4 border-t border-gray-200">
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-indigo-500 rounded shadow-sm"></div>
          <span className="text-xs text-gray-600 font-medium">Workflow</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-purple-500 rounded shadow-sm"></div>
          <span className="text-xs text-gray-600 font-medium">Steps</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-blue-500 rounded shadow-sm"></div>
          <span className="text-xs text-gray-600 font-medium">Agent</span>
        </div>
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 bg-cyan-500 rounded shadow-sm"></div>
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

export default WorkflowTimelineView;

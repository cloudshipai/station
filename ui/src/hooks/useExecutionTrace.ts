import { useState, useCallback, useEffect } from 'react';
import { tracesApi } from '../api/station';
import type { JaegerTrace, JaegerSpan } from '../types/station';

interface ExecutionSpanData {
  spanID: string;
  startTime: number; // microseconds from run start
  duration: number; // microseconds
  status: 'success' | 'error' | 'running';
  toolName: string;
}

interface ProcessedTraceData {
  spans: ExecutionSpanData[];
  runStartTime: number;
  runDuration: number;
}

export const useExecutionTrace = () => {
  const [traceData, setTraceData] = useState<JaegerTrace | null>(null);
  const [traceAgentId, setTraceAgentId] = useState<number | null>(null); // Track which agent this trace belongs to
  const [isExecutionView, setIsExecutionView] = useState(false);
  const [loading, setLoading] = useState(false);

  const fetchTrace = useCallback(async (runId: number, agentId: number) => {
    setLoading(true);
    try {
      const response = await tracesApi.getByRunId(runId);
      setTraceData(response.data.trace);
      setTraceAgentId(agentId); // Store which agent this trace belongs to
      setIsExecutionView(true); // Auto-enable execution view when trace is loaded
    } catch (error) {
      console.error('Failed to fetch trace:', error);
      setTraceData(null);
      setTraceAgentId(null);
    } finally {
      setLoading(false);
    }
  }, []);

  const clearTrace = useCallback(() => {
    setTraceData(null);
    setTraceAgentId(null);
    setIsExecutionView(false);
  }, []);

  const toggleExecutionView = useCallback(() => {
    setIsExecutionView(prev => !prev);
  }, []);

  // Process trace data for node injection
  const processTraceForNode = useCallback((agentId: number): ProcessedTraceData | null => {
    if (!traceData || !traceData.spans) return null;
    
    // CRITICAL: Only show trace data if it belongs to this agent
    if (traceAgentId !== agentId) return null;

    // Filter spans relevant to this agent (tool calls, LLM calls)
    const relevantSpans = traceData.spans.filter((span: JaegerSpan) => {
      const op = span.operationName;
      return op.startsWith('__') || op.startsWith('faker.') || op === 'generate';
    });

    if (relevantSpans.length === 0) return null;

    const runStartTime = traceData.spans[0]?.startTime || 0;
    const runEndTime = Math.max(...traceData.spans.map((s: JaegerSpan) => s.startTime + s.duration));
    const runDuration = runEndTime - runStartTime;

    const executionSpans: ExecutionSpanData[] = relevantSpans.map((span: JaegerSpan) => {
      const hasError = span.tags?.some(t => t.key === 'error' || t.key === 'error.message');
      
      return {
        spanID: span.spanID,
        startTime: span.startTime - runStartTime,
        duration: span.duration,
        status: hasError ? 'error' : 'success',
        toolName: span.operationName,
      };
    });

    return {
      spans: executionSpans,
      runStartTime,
      runDuration,
    };
  }, [traceData, traceAgentId]);

  // Get total run duration for scrubber
  const getTotalDuration = useCallback((): number => {
    if (!traceData || !traceData.spans || traceData.spans.length === 0) return 0;
    
    const runStartTime = traceData.spans[0].startTime;
    const runEndTime = Math.max(...traceData.spans.map((s: JaegerSpan) => s.startTime + s.duration));
    return runEndTime - runStartTime;
  }, [traceData]);

  // Get spans active at a specific time (for playback highlighting)
  const getActiveSpansAt = useCallback((time: number): string[] => {
    if (!traceData || !traceData.spans) return [];

    const runStartTime = traceData.spans[0]?.startTime || 0;
    const absoluteTime = runStartTime + time;

    return traceData.spans
      .filter((span: JaegerSpan) => {
        const spanStart = span.startTime;
        const spanEnd = span.startTime + span.duration;
        return absoluteTime >= spanStart && absoluteTime <= spanEnd;
      })
      .map((span: JaegerSpan) => span.spanID);
  }, [traceData]);

  // Get raw trace data for execution flow graph
  const getTraceData = useCallback(() => {
    return traceData;
  }, [traceData]);

  return {
    traceData,
    traceAgentId,
    isExecutionView,
    loading,
    fetchTrace,
    clearTrace,
    toggleExecutionView,
    processTraceForNode,
    hasTraceData: !!traceData && !!traceAgentId,
    getTotalDuration,
    getActiveSpansAt,
    getTraceData,
  };
};

import React, { memo } from 'react';
import { Clock, Zap, DollarSign, Activity } from 'lucide-react';
import type { JaegerTrace } from '../../types/station';

interface ExecutionStatsHUDProps {
  traceData: JaegerTrace | null;
}

export const ExecutionStatsHUD = memo(({ traceData }: ExecutionStatsHUDProps) => {
  if (!traceData || !traceData.spans || traceData.spans.length === 0) {
    return null;
  }

  // Calculate stats
  const runStartTime = traceData.spans[0]?.startTime || 0;
  const runEndTime = Math.max(...traceData.spans.map(s => s.startTime + s.duration));
  const totalDuration = runEndTime - runStartTime;
  
  const spanCount = traceData.spans.filter(s => 
    s.operationName.startsWith('__') || s.operationName.startsWith('faker.') || s.operationName === 'generate'
  ).length;

  // Extract token counts from tags
  let totalTokens = 0;
  let inputTokens = 0;
  let outputTokens = 0;
  
  traceData.spans.forEach(span => {
    span.tags?.forEach(tag => {
      if (tag.key === 'llm.tokens.total' || tag.key === 'tokens.total') {
        totalTokens += Number(tag.value) || 0;
      }
      if (tag.key === 'llm.tokens.input' || tag.key === 'tokens.input') {
        inputTokens += Number(tag.value) || 0;
      }
      if (tag.key === 'llm.tokens.output' || tag.key === 'tokens.output') {
        outputTokens += Number(tag.value) || 0;
      }
    });
  });

  // Estimate cost (gpt-4o-mini pricing)
  const estimatedCost = totalTokens > 0
    ? ((inputTokens / 1_000_000) * 0.150) + ((outputTokens / 1_000_000) * 0.600)
    : 0;

  // Format duration
  const formatDuration = (micros: number): string => {
    const seconds = micros / 1_000_000;
    if (seconds < 1) return `${Math.round(seconds * 1000)}ms`;
    if (seconds < 60) return `${seconds.toFixed(1)}s`;
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}m ${secs}s`;
  };

  const formatTokens = (tokens: number): string => {
    if (tokens === 0) return '0';
    if (tokens < 1000) return `${tokens}`;
    return `${(tokens / 1000).toFixed(1)}k`;
  };

  return (
    <div className="absolute top-4 right-4 z-50 bg-gray-900/95 backdrop-blur-sm border border-cyan-500/30 rounded-lg px-4 py-2 flex items-center gap-4 font-mono text-xs shadow-lg">
      {/* Total Duration */}
      <div className="flex items-center gap-1.5">
        <Clock className="h-4 w-4 text-cyan-400" />
        <span className="text-gray-300">{formatDuration(totalDuration)}</span>
      </div>

      {/* Span Count */}
      <div className="flex items-center gap-1.5 border-l border-gray-700 pl-4">
        <Activity className="h-4 w-4 text-purple-400" />
        <span className="text-gray-300">{spanCount} spans</span>
      </div>

      {/* Token Count */}
      {totalTokens > 0 && (
        <div className="flex items-center gap-1.5 border-l border-gray-700 pl-4">
          <Zap className="h-4 w-4 text-yellow-400" />
          <span className="text-gray-300">{formatTokens(totalTokens)} tok</span>
        </div>
      )}

      {/* Estimated Cost */}
      {estimatedCost > 0 && (
        <div className="flex items-center gap-1.5 border-l border-gray-700 pl-4">
          <DollarSign className="h-4 w-4 text-green-400" />
          <span className="text-gray-300">
            {estimatedCost < 0.0001 ? '<$0.0001' : `$${estimatedCost.toFixed(4)}`}
          </span>
        </div>
      )}

      {/* Trace Source Indicator */}
      <div className="flex items-center gap-1.5 border-l border-gray-700 pl-4">
        <div className="w-2 h-2 bg-cyan-500 rounded-full"></div>
        <span className="text-gray-400 text-[10px]">Local</span>
      </div>
    </div>
  );
});

ExecutionStatsHUD.displayName = 'ExecutionStatsHUD';

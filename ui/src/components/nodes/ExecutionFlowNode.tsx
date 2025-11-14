import React, { memo, useState } from 'react';
import { Handle, Position } from '@xyflow/react';
import { Zap, Clock, CheckCircle, XCircle, Cpu, Wrench, Play, Square } from 'lucide-react';
import { ExecutionNodeHoverCard } from './ExecutionNodeHoverCard';

export interface ExecutionFlowNodeData {
  label: string;
  type: 'tool' | 'llm' | 'start' | 'end';
  duration: number; // microseconds
  status: 'success' | 'error' | 'running';
  toolName?: string;
  toolParams?: Record<string, any>;
  toolResult?: any;
  llmPrompt?: string;
  llmTokens?: { input: number; output: number };
  llmModel?: string;
  startTime: number;
  spanID: string;
  isActive?: boolean;
}

interface ExecutionFlowNodeProps {
  data: ExecutionFlowNodeData;
}

export const ExecutionFlowNode = memo(({ data }: ExecutionFlowNodeProps) => {
  const { label, type, duration, status, isActive, toolParams, llmTokens, llmModel } = data;
  const [showHover, setShowHover] = useState(false);
  const [anchorEl, setAnchorEl] = useState<HTMLDivElement | null>(null);
  const hoverTimeoutRef = React.useRef<NodeJS.Timeout | null>(null);

  // Format duration
  const formatDuration = (micros: number): string => {
    const ms = micros / 1000;
    if (ms < 1) return `${micros}µs`;
    if (ms < 1000) return `${ms.toFixed(0)}ms`;
    const s = ms / 1000;
    if (s < 60) return `${s.toFixed(1)}s`;
    const mins = Math.floor(s / 60);
    const secs = Math.floor(s % 60);
    return `${mins}m${secs}s`;
  };

  // Get colors based on type and status
  const getBarColor = () => {
    if (type === 'start') return 'bg-blue-500';
    if (type === 'end') return 'bg-purple-500';
    if (status === 'error') return 'bg-red-500';
    if (type === 'llm') return 'bg-cyan-500';
    return 'bg-emerald-500'; // tool
  };

  const getBorderColor = () => {
    if (type === 'start') return 'border-blue-500/50';
    if (type === 'end') return 'border-purple-500/50';
    if (status === 'error') return 'border-red-500/50';
    if (type === 'llm') return 'border-cyan-500/50';
    return 'border-emerald-500/50';
  };

  const getIcon = () => {
    if (type === 'start') return <Play className="h-3 w-3" />;
    if (type === 'end') return <Square className="h-3 w-3" />;
    if (type === 'llm') return <Cpu className="h-3 w-3" />;
    if (status === 'error') return <XCircle className="h-3 w-3" />;
    return <Wrench className="h-3 w-3" />;
  };

  // Fixed width for now (we can make it dynamic later)
  const barWidthPx = 200;

  return (
    <>
      <div
        ref={setAnchorEl}
        onMouseEnter={() => {
          if (hoverTimeoutRef.current) clearTimeout(hoverTimeoutRef.current);
          setShowHover(true);
        }}
        onMouseLeave={() => {
          // Delay closing to allow moving to hover card
          hoverTimeoutRef.current = setTimeout(() => setShowHover(false), 150);
        }}
        className="select-none"
        style={{ width: `${barWidthPx}px` }}
      >
        {/* Input Handle */}
        {type !== 'start' && (
          <Handle
            type="target"
            position={Position.Left}
            className="w-2 h-2 !bg-cyan-400 !border-2 !border-cyan-300"
          />
        )}

        {/* Timeline Bar */}
        <div className={`h-6 rounded border-2 ${getBorderColor()} bg-slate-800/50 overflow-hidden relative ${isActive ? 'ring-2 ring-cyan-400' : ''}`}>
          <div className={`h-full ${getBarColor()} transition-all`} style={{ width: '100%' }} />
        </div>

        {/* Caption Strip */}
        <div className="mt-2 flex flex-col gap-1">
          <div className="flex items-center gap-1.5 text-xs text-white/90">
            {getIcon()}
            <span className="truncate font-medium" title={label}>
              {label}
            </span>
          </div>
          <div className="flex items-center gap-2 text-[11px] text-white/60">
            <span className="tabular-nums">{formatDuration(duration)}</span>
            {llmTokens && (
              <>
                <span>·</span>
                <span className="tabular-nums">{llmTokens.input + llmTokens.output} tok</span>
              </>
            )}
          </div>
        </div>

        {/* Output Handle */}
        {type !== 'end' && (
          <Handle
            type="source"
            position={Position.Right}
            className="w-2 h-2 !bg-cyan-400 !border-2 !border-cyan-300"
          />
        )}
      </div>

      {/* Hover Card */}
      <ExecutionNodeHoverCard
        anchorEl={anchorEl}
        open={showHover}
        onOpenChange={(open) => {
          if (hoverTimeoutRef.current) clearTimeout(hoverTimeoutRef.current);
          setShowHover(open);
        }}
        data={data}
      />
    </>
  );
});

ExecutionFlowNode.displayName = 'ExecutionFlowNode';

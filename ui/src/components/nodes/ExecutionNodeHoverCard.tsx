import React, { memo } from 'react';
import { createPortal } from 'react-dom';
import { Clock, Zap, DollarSign, AlertCircle, CheckCircle } from 'lucide-react';
import type { ExecutionFlowNodeData } from './ExecutionFlowNode';

interface ExecutionNodeHoverCardProps {
  anchorEl: HTMLElement | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  data: ExecutionFlowNodeData;
}

export const ExecutionNodeHoverCard = memo(({ anchorEl, open, onOpenChange, data }: ExecutionNodeHoverCardProps) => {
  if (!open || !anchorEl || data.type === 'start' || data.type === 'end') {
    return null;
  }

  // Calculate position - show ABOVE the node to avoid cutoff
  const rect = anchorEl.getBoundingClientRect();
  const cardHeight = 320; // Approximate hover card height
  const top = rect.top + window.scrollY - cardHeight - 10; // Position above node
  const left = Math.max(10, Math.min(rect.left + window.scrollX, window.innerWidth - 360)); // Keep within viewport

  // Format duration
  const formatDuration = (micros: number): string => {
    const s = micros / 1_000_000;
    return s < 1 ? `${Math.round(s * 1000)}ms` : `${s.toFixed(2)}s`;
  };

  // Estimate cost
  const estimateCost = (): number | null => {
    if (!data.llmTokens) return null;
    const inputCost = (data.llmTokens.input / 1_000_000) * 0.150;
    const outputCost = (data.llmTokens.output / 1_000_000) * 0.600;
    return inputCost + outputCost;
  };

  const cost = estimateCost();

  // Status badge
  const statusBadge = data.status === 'error'
    ? 'bg-red-500/20 text-red-300 border-red-500/30'
    : 'bg-emerald-500/20 text-emerald-200 border-emerald-500/30';

  return createPortal(
    <div
      onMouseEnter={() => onOpenChange(true)}
      onMouseLeave={() => onOpenChange(false)}
      style={{ 
        position: 'absolute', 
        top: `${top}px`, 
        left: `${left}px`, 
        zIndex: 10000,
        pointerEvents: 'auto'
      }}
      className="w-[340px] rounded-xl border-2 border-cyan-500/30 bg-slate-900 backdrop-blur-sm px-4 py-3 shadow-2xl"
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-2 mb-3">
        <div className="min-w-0 flex-1">
          <div className="text-sm font-semibold text-white truncate" title={data.label}>
            {data.label}
          </div>
          <div className="text-xs text-white/60 mt-0.5">
            {data.type === 'llm' ? '🧠 LLM Generation' : '🛠️ Tool Call'}
            {data.llmModel && ` · ${data.llmModel}`}
          </div>
        </div>
        <span className={`px-2 py-1 text-xs rounded border ${statusBadge} flex items-center gap-1`}>
          {data.status === 'error' ? <AlertCircle className="h-3 w-3" /> : <CheckCircle className="h-3 w-3" />}
          {data.status}
        </span>
      </div>

      {/* Progress Bar (visual) */}
      <div className="h-2 w-full rounded bg-white/10 overflow-hidden mb-3">
        <div
          className={`h-full ${data.status === 'error' ? 'bg-red-500' : data.type === 'llm' ? 'bg-cyan-500' : 'bg-emerald-500'}`}
          style={{ width: '100%' }}
        />
      </div>

      {/* Metrics Grid */}
      <div className="grid grid-cols-2 gap-x-4 gap-y-2 text-[11px] mb-3">
        <Metric icon={<Clock className="h-3 w-3" />} label="Duration" value={formatDuration(data.duration)} />
        
        {data.llmTokens && (
          <>
            <Metric 
              icon={<Zap className="h-3 w-3 text-yellow-400" />} 
              label="Tokens" 
              value={`${data.llmTokens.input} → ${data.llmTokens.output}`} 
            />
            {cost !== null && (
              <Metric 
                icon={<DollarSign className="h-3 w-3 text-green-400" />} 
                label="Est. Cost" 
                value={cost < 0.0001 ? '<$0.0001' : `$${cost.toFixed(4)}`} 
              />
            )}
          </>
        )}
      </div>

      {/* LLM Prompt Preview */}
      {data.type === 'llm' && data.llmPrompt && (
        <div className="mt-3 pt-3 border-t border-white/10">
          <div className="text-[10px] text-white/50 uppercase font-semibold mb-1">Prompt Preview</div>
          <div className="text-xs text-white/70 line-clamp-3">
            {data.llmPrompt}
          </div>
        </div>
      )}

      {/* Tool Parameters */}
      {data.type === 'tool' && data.toolParams && Object.keys(data.toolParams).length > 0 && (
        <div className="mt-3 pt-3 border-t border-white/10">
          <div className="text-[10px] text-white/50 uppercase font-semibold mb-1">Parameters</div>
          <div className="space-y-1">
            {Object.entries(data.toolParams).slice(0, 3).map(([key, value]) => (
              <div key={key} className="text-xs text-white/70">
                <span className="text-white/50">{key}:</span>{' '}
                <span className="font-mono">
                  {typeof value === 'string' && value.length > 40 
                    ? value.substring(0, 37) + '...' 
                    : String(value)}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Actions */}
      <div className="mt-3 pt-3 border-t border-white/10 flex items-center gap-2">
        <button className="text-xs px-2 py-1 rounded border border-white/10 bg-white/5 text-white/80 hover:bg-white/10 transition-colors">
          View Details
        </button>
        <button className="text-xs px-2 py-1 rounded border border-white/10 bg-white/5 text-white/80 hover:bg-white/10 transition-colors">
          Open in Jaeger
        </button>
      </div>
    </div>,
    document.body
  );
});

function Metric({ icon, label, value }: { icon: React.ReactNode; label: string; value: string }) {
  return (
    <div className="flex items-start gap-2">
      <div className="text-white/50 mt-0.5">{icon}</div>
      <div className="flex flex-col min-w-0">
        <span className="text-white/50">{label}</span>
        <span className="text-white/90 tabular-nums">{value}</span>
      </div>
    </div>
  );
}

ExecutionNodeHoverCard.displayName = 'ExecutionNodeHoverCard';

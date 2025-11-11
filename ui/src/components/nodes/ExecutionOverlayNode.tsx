import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { Bot, Settings, Network, Users, Clock, Play, Zap } from 'lucide-react';
import { cn } from '../../utils/cn';
import type { Agent } from '../../types/station';
import type { AgentHierarchyInfo} from '../../utils/agentHierarchy';
import type { JaegerSpan } from '../../types/station';

interface ExecutionSpanData {
  spanID: string;
  startTime: number; // microseconds from run start
  duration: number; // microseconds
  status: 'success' | 'error' | 'running';
  toolName?: string;
}

interface HierarchicalAgentNodeData {
  agent: Agent;
  hierarchyInfo?: AgentHierarchyInfo;
  isSelected?: boolean;
  onEditAgent?: (agentId: number) => void;
  onOpenModal?: (agentId: number) => void;
  onScheduleAgent?: (agentId: number) => void;
  onRunAgent?: (agentId: number) => void;
  // NEW: Execution overlay data
  executionSpans?: ExecutionSpanData[];
  runStartTime?: number; // microseconds
  runDuration?: number; // microseconds
  isExecutionView?: boolean; // Ghost mode if no execution
  currentPlaybackTime?: number; // microseconds - for highlighting active spans
  activeSpanIds?: string[]; // IDs of currently active spans
}

interface HierarchicalAgentNodeProps {
  data: HierarchicalAgentNodeData;
}

export const ExecutionOverlayNode: React.FC<HierarchicalAgentNodeProps> = ({ data }) => {
  const { agent, hierarchyInfo, isSelected, executionSpans, runStartTime, runDuration, isExecutionView, currentPlaybackTime, activeSpanIds } = data;
  
  const isOrchestrator = hierarchyInfo && hierarchyInfo.childAgents.length > 0;
  const isCallable = hierarchyInfo?.isCallable || false;
  const isInHierarchy = hierarchyInfo && (
    hierarchyInfo.childAgents.length > 0 || 
    hierarchyInfo.isCallable || 
    hierarchyInfo.parentAgents.length > 0
  );

  const hasExecutionData = executionSpans && executionSpans.length > 0;
  const isGhostMode = isExecutionView && !hasExecutionData;

  const handleEditClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onEditAgent) {
      data.onEditAgent(agent.id);
    }
  };

  const handleInfoClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onOpenModal) {
      data.onOpenModal(agent.id);
    }
  };

  const handleScheduleClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onScheduleAgent) {
      data.onScheduleAgent(agent.id);
    }
  };

  const handleRunClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (data.onRunAgent) {
      data.onRunAgent(agent.id);
    }
  };

  const formatDuration = (micros: number): string => {
    const ms = micros / 1000;
    if (ms < 1000) return `${Math.round(ms)}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'success': return 'bg-green-500';
      case 'error': return 'bg-red-500';
      case 'running': return 'bg-blue-500 animate-pulse';
      default: return 'bg-gray-500';
    }
  };

  return (
    <div className={cn(
      "relative w-[280px] transition-all duration-200 group",
      isSelected && "ring-2 ring-tokyo-blue",
      isGhostMode && "opacity-50" // Ghost mode: 50% opacity when no execution data
    )}>
      {/* Hierarchy Badge - Top Left */}
      {isInHierarchy && (
        <div className="absolute -top-3 -left-3 z-10">
          {isOrchestrator ? (
            <div className="flex items-center gap-1 px-2 py-1 bg-tokyo-purple rounded-full border border-tokyo-purple text-xs font-mono">
              <Network className="w-3 h-3" />
              <span>Orchestrator</span>
            </div>
          ) : isCallable ? (
            <div className="flex items-center gap-1 px-2 py-1 bg-tokyo-cyan rounded-full border border-tokyo-cyan text-xs font-mono">
              <Users className="w-3 h-3" />
              <span>Callable</span>
            </div>
          ) : null}
        </div>
      )}

      {/* Execution Badge - Top Right */}
      {hasExecutionData && (
        <div className="absolute -top-3 -right-3 z-10">
          <div className="flex items-center gap-1 px-2 py-1 bg-green-900/80 border border-green-500 rounded-full text-xs font-mono">
            <Zap className="w-3 h-3 text-green-400" />
            <span className="text-green-300">{executionSpans!.length} calls</span>
          </div>
        </div>
      )}

      {/* Main Node Container */}
      <div className={cn(
        "bg-tokyo-bg-dark border-2 rounded-lg overflow-hidden shadow-lg backdrop-blur-sm",
        isSelected ? "border-tokyo-blue" : "border-tokyo-blue7 hover:border-tokyo-blue8",
        isGhostMode && "border-dashed border-red-500" // Dashed red border for uninstrumented nodes
      )}>
        {/* Header */}
        <div className="bg-gradient-to-r from-tokyo-blue7 to-tokyo-purple7 p-3 flex items-center justify-between">
          <div className="flex items-center gap-2 flex-1 min-w-0">
            <div className="flex-shrink-0 p-1.5 bg-tokyo-bg rounded-lg">
              <Bot className="w-4 h-4 text-tokyo-blue" />
            </div>
            <div className="flex-1 min-w-0">
              <h3 className="font-mono font-semibold text-tokyo-fg truncate text-sm">
                {agent.name}
              </h3>
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex items-center gap-1 flex-shrink-0">
            <button
              onClick={handleRunClick}
              className="p-1 hover:bg-tokyo-green/20 rounded transition-colors"
              title="Run Agent"
            >
              <Play className="w-3.5 h-3.5 text-tokyo-green" />
            </button>
            <button
              onClick={handleScheduleClick}
              className="p-1 hover:bg-tokyo-cyan/20 rounded transition-colors"
              title="Schedule"
            >
              <Clock className="w-3.5 h-3.5 text-tokyo-cyan" />
            </button>
            <button
              onClick={handleEditClick}
              className="p-1 hover:bg-tokyo-orange/20 rounded transition-colors"
              title="Edit"
            >
              <Settings className="w-3.5 h-3.5 text-tokyo-orange" />
            </button>
          </div>
        </div>

        {/* Description */}
        <div className="p-3 border-b border-tokyo-blue7">
          <p className="text-xs text-tokyo-comment font-mono line-clamp-2">
            {agent.description || 'No description provided'}
          </p>
        </div>

        {/* Stats */}
        <div className="px-3 py-2 bg-tokyo-bg flex items-center justify-between text-xs">
          <span className="text-tokyo-comment font-mono">
            {agent.max_steps} max steps
          </span>
          {agent.is_scheduled && (
            <div className="flex items-center gap-1 text-tokyo-green">
              <Clock className="w-3 h-3" />
              <span className="font-mono">Scheduled</span>
            </div>
          )}
        </div>

        {/* MICRO-TIMELINE: Inline execution visualization */}
        {hasExecutionData && runDuration && (
          <div className="px-3 py-2 bg-tokyo-bg-dark border-t border-tokyo-blue7">
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs text-tokyo-comment font-mono">Execution</span>
              <span className="text-xs text-tokyo-cyan font-mono">
                {formatDuration(runDuration)}
              </span>
            </div>
            
            {/* Micro-timeline bars */}
            <div className="space-y-0.5">
              {executionSpans!.map((span, idx) => {
                const relativeStart = runDuration ? (span.startTime / runDuration) * 100 : 0;
                const width = runDuration ? (span.duration / runDuration) * 100 : 0;
                const isActive = activeSpanIds?.includes(span.spanID);
                
                return (
                  <div key={span.spanID} className="relative h-1.5 bg-gray-900 rounded overflow-hidden">
                    <div
                      className={cn(
                        "absolute h-full rounded transition-all",
                        getStatusColor(span.status),
                        isActive && "ring-2 ring-white ring-offset-1 ring-offset-gray-900 scale-110 z-10"
                      )}
                      style={{
                        left: `${relativeStart}%`,
                        width: `${Math.max(width, 1)}%`,
                      }}
                      title={`${span.toolName || 'Unknown'}: ${formatDuration(span.duration)}`}
                    />
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* Ghost Mode Indicator */}
        {isGhostMode && (
          <div className="px-3 py-2 bg-red-900/20 border-t border-red-500/30">
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 bg-red-500 rounded-full animate-ping absolute"></div>
              <div className="w-2 h-2 bg-red-500 rounded-full"></div>
              <span className="text-xs text-red-400 font-mono">No execution data</span>
            </div>
          </div>
        )}
      </div>

      {/* Connection Handles */}
      <Handle type="target" position={Position.Left} className="w-2 h-2 bg-tokyo-blue border-2 border-tokyo-fg" />
      <Handle type="source" position={Position.Right} className="w-2 h-2 bg-tokyo-blue border-2 border-tokyo-fg" />
    </div>
  );
};

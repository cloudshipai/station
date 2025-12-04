import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { Bot, Settings, Network, Users, Clock, Play, Zap, Database } from 'lucide-react';
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
      isSelected && "ring-2 ring-primary ring-offset-2",
      isGhostMode && "opacity-50" // Ghost mode: 50% opacity when no execution data
    )}>
      {/* Hierarchy Badge - Top Left */}
      {isInHierarchy && (
        <div className="absolute -top-3 -left-3 z-10">
          {isOrchestrator ? (
            <div className="flex items-center gap-1 px-2 py-1 bg-station-lavender-400 text-white rounded-full shadow-sm text-xs font-medium">
              <Network className="w-3 h-3" />
              <span>Orchestrator</span>
            </div>
          ) : isCallable ? (
            <div className="flex items-center gap-1 px-2 py-1 bg-station-mint-400 text-white rounded-full shadow-sm text-xs font-medium">
              <Users className="w-3 h-3" />
              <span>Callable</span>
            </div>
          ) : null}
        </div>
      )}

      {/* Execution Badge - Top Right */}
      {hasExecutionData && (
        <div className="absolute -top-3 -right-3 z-10">
          <div className="flex items-center gap-1 px-2 py-1 bg-green-50 border border-green-200 rounded-full text-xs font-medium shadow-sm">
            <Zap className="w-3 h-3 text-green-600" />
            <span className="text-green-700">{executionSpans!.length} calls</span>
          </div>
        </div>
      )}

      {/* Main Node Container - Soft colored background */}
      <div className={cn(
        "border rounded-xl overflow-hidden shadow-sm hover:shadow-lg hover:-translate-y-1 transition-all duration-300 cursor-pointer",
        isOrchestrator 
          ? "bg-purple-50/40 border-purple-200/60 hover:bg-purple-50/60" 
          : isCallable
            ? "bg-emerald-50/40 border-emerald-200/60 hover:bg-emerald-50/60"
            : "bg-blue-50/40 border-blue-200/60 hover:bg-blue-50/60",
        isSelected && "ring-2 ring-primary scale-105",
        isGhostMode && "border-dashed border-red-300" // Dashed red border for uninstrumented nodes
      )}>
        {/* Header - Clean without icons */}
        <div className={cn(
          "p-3 flex items-center justify-between border-b",
          isOrchestrator 
            ? "bg-purple-100/50 border-purple-200/60" 
            : isCallable
              ? "bg-emerald-100/50 border-emerald-200/60"
              : "bg-blue-100/50 border-blue-200/60"
        )}>
          <div className="flex items-center gap-2 flex-1 min-w-0">
            <div className="flex-1 min-w-0">
              <h3 className="font-semibold text-gray-900 truncate text-sm">
                {agent.name}
              </h3>
            </div>
          </div>

          {/* Type Badge */}
          <span className={cn(
            "text-[10px] font-medium px-2 py-0.5 rounded-full whitespace-nowrap shrink-0",
            isOrchestrator ? "bg-purple-200/60 text-purple-800" : 
            isCallable ? "bg-emerald-200/60 text-emerald-800" : 
            "bg-blue-200/60 text-blue-800"
          )}>
            {isOrchestrator ? "Orchestrator" : isCallable ? "Callable" : "Agent"}
          </span>
        </div>

        {/* Description */}
        <div className="p-3 border-b border-gray-100">
          <p className="text-xs text-gray-600 line-clamp-2">
            {agent.description || 'No description provided'}
          </p>
        </div>

        {/* Stats */}
        <div className="px-3 py-2 bg-gray-50 flex items-center justify-between text-xs">
          <span className="text-gray-500">
            {agent.max_steps} max steps
          </span>
          <div className="flex items-center gap-2">
            {agent.memory_topic_key && (
              <div className="flex items-center gap-1 text-purple-600" title={`Memory: ${agent.memory_topic_key}`}>
                <Database className="w-3 h-3" />
                <span>Memory</span>
              </div>
            )}
            {agent.is_scheduled && (
              <div className="flex items-center gap-1 text-green-600">
                <Clock className="w-3 h-3" />
                <span>Scheduled</span>
              </div>
            )}
          </div>
        </div>

        {/* MICRO-TIMELINE: Inline execution visualization */}
        {hasExecutionData && runDuration && (
          <div className="px-3 py-2 bg-gray-50 border-t border-gray-100">
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs text-gray-500 font-medium">Execution</span>
              <span className="text-xs text-blue-600 font-medium">
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
                  <div key={span.spanID} className="relative h-1.5 bg-gray-200 rounded overflow-hidden">
                    <div
                      className={cn(
                        "absolute h-full rounded transition-all",
                        getStatusColor(span.status),
                        isActive && "ring-2 ring-primary ring-offset-1 ring-offset-gray-50 scale-110 z-10"
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
      <Handle type="target" position={Position.Left} className="w-2 h-2 bg-primary border-2 border-white shadow-sm" />
      <Handle type="source" position={Position.Right} className="w-2 h-2 bg-primary border-2 border-white shadow-sm" />
    </div>
  );
};

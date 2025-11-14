import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { Bot, Settings, Network, Users, Clock, Play } from 'lucide-react';
import { cn } from '../../utils/cn';
import type { Agent } from '../../types/station';
import type { AgentHierarchyInfo} from '../../utils/agentHierarchy';

interface HierarchicalAgentNodeData {
  agent: Agent;
  hierarchyInfo?: AgentHierarchyInfo;
  isSelected?: boolean;
  onEditAgent?: (agentId: number) => void;
  onOpenModal?: (agentId: number) => void;
  onScheduleAgent?: (agentId: number) => void;
  onRunAgent?: (agentId: number) => void;
}

interface HierarchicalAgentNodeProps {
  data: HierarchicalAgentNodeData;
}

export const HierarchicalAgentNode: React.FC<HierarchicalAgentNodeProps> = ({ data }) => {
  const { agent, hierarchyInfo, isSelected } = data;
  
  const isOrchestrator = hierarchyInfo && hierarchyInfo.childAgents.length > 0;
  const isCallable = hierarchyInfo?.isCallable || false;
  const isInHierarchy = hierarchyInfo && (
    hierarchyInfo.childAgents.length > 0 || 
    hierarchyInfo.isCallable || 
    hierarchyInfo.parentAgents.length > 0
  );

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

  return (
    <div className={cn(
      "relative w-[280px] transition-all duration-200 group",
      isSelected && "ring-2 ring-tokyo-blue"
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

      {/* Main Node Card */}
      <div className={cn(
        "px-4 py-3 shadow-tokyo-blue border rounded-lg relative",
        isOrchestrator 
          ? "border-tokyo-purple bg-tokyo-purple/10" 
          : isCallable
            ? "border-tokyo-cyan bg-tokyo-cyan/10"
            : "border-tokyo-blue7 bg-tokyo-bg-dark"
      )}>
        {/* Connection Handles */}
        {(isCallable || hierarchyInfo?.parentAgents.length) && (
          <Handle 
            type="target" 
            position={Position.Left} 
            className={cn(
              "w-3 h-3 border-2 border-tokyo-bg",
              isCallable ? "bg-tokyo-cyan" : "bg-tokyo-blue"
            )}
          />
        )}
        {/* Always show source handle for MCP server connections */}
        <Handle 
          type="source" 
          position={Position.Right} 
          className={cn(
            "w-3 h-3 border-2 border-tokyo-bg",
            isOrchestrator ? "bg-tokyo-purple" : "bg-tokyo-blue"
          )}
        />

        {/* Action Buttons - Top Right */}
        <div className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 flex gap-1">
          <button
            onClick={handleRunClick}
            className="p-1.5 rounded bg-tokyo-blue hover:bg-tokyo-blue5 text-tokyo-bg shadow-lg"
            title="Run agent"
          >
            <Play className="h-3.5 w-3.5" />
          </button>
          <button
            onClick={handleScheduleClick}
            className="p-1.5 rounded bg-tokyo-green hover:bg-tokyo-green/80 text-tokyo-bg shadow-lg"
            title="Schedule agent"
          >
            <Clock className="h-3.5 w-3.5" />
          </button>
          <button
            onClick={handleEditClick}
            className="p-1.5 rounded bg-tokyo-orange hover:bg-tokyo-orange text-tokyo-bg shadow-lg"
            title="Edit agent configuration"
          >
            <Settings className="h-3.5 w-3.5" />
          </button>
        </div>

        {/* Header */}
        <div className="flex items-center gap-2 mb-2">
          <Bot className={cn(
            "h-5 w-5",
            isOrchestrator ? "text-tokyo-purple" : isCallable ? "text-tokyo-cyan" : "text-tokyo-blue"
          )} />
          <div className="font-mono text-base font-medium text-tokyo-fg">
            {agent.name}
          </div>
        </div>

        {/* Description */}
        <div className="text-sm text-tokyo-comment mb-2 line-clamp-2">
          {agent.description || 'No description'}
        </div>

        {/* Hierarchy Info Footer */}
        {hierarchyInfo && isInHierarchy && (
          <div className="mt-2 pt-2 border-t border-tokyo-blue7/30 space-y-1">
            {hierarchyInfo.childAgents.length > 0 && (
              <div className="text-xs text-tokyo-purple font-mono">
                → Calls: {hierarchyInfo.childAgents.length <= 2 
                  ? hierarchyInfo.childAgents.join(', ')
                  : `${hierarchyInfo.childAgents.slice(0, 2).join(', ')} +${hierarchyInfo.childAgents.length - 2} more`
                }
              </div>
            )}
            {hierarchyInfo.parentAgents.length > 0 && (
              <div className="text-xs text-tokyo-cyan font-mono">
                ← Called by: {hierarchyInfo.parentAgents.length <= 2
                  ? hierarchyInfo.parentAgents.join(', ')
                  : `${hierarchyInfo.parentAgents.slice(0, 2).join(', ')} +${hierarchyInfo.parentAgents.length - 2} more`
                }
              </div>
            )}
          </div>
        )}

        {/* Status Info */}
        <div className="mt-2 flex items-center justify-between text-xs">
          <span className="text-tokyo-green font-mono">
            {agent.max_steps} steps
          </span>
          {agent.is_scheduled && agent.cron_schedule && (
            <span className="px-2 py-0.5 bg-tokyo-green/20 text-tokyo-green rounded font-mono" title={`Schedule: ${agent.cron_schedule}`}>
              ⏰ {agent.cron_schedule}
            </span>
          )}
        </div>
      </div>
    </div>
  );
};

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
      isSelected && "ring-2 ring-primary ring-offset-2"
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

      {/* Main Node Card */}
      <div className={cn(
        "px-4 py-3 shadow-sm border rounded-xl relative hover:shadow-md transition-all",
        isOrchestrator 
          ? "bg-station-lavender-50 border-station-lavender-200" 
          : isCallable
            ? "bg-station-mint-50 border-station-mint-200"
            : "bg-white border-border"
      )}>
        {/* Connection Handles */}
        {(isCallable || hierarchyInfo?.parentAgents.length) && (
          <Handle 
            type="target" 
            position={Position.Left} 
            className={cn(
              "w-3 h-3 border-2 border-white shadow-sm",
              isCallable ? "bg-station-mint-400" : "bg-primary"
            )}
          />
        )}
        {/* Always show source handle for MCP server connections */}
        <Handle 
          type="source" 
          position={Position.Right} 
          className={cn(
            "w-3 h-3 border-2 border-white shadow-sm",
            isOrchestrator ? "bg-station-lavender-400" : "bg-primary"
          )}
        />

        {/* Action Buttons - Top Right */}
        <div className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity duration-200 flex gap-1">
          <button
            onClick={handleRunClick}
            className="p-1.5 rounded-md bg-station-blue hover:bg-station-blue-dark text-white shadow-sm hover:shadow-md transition-all"
            title="Run agent"
          >
            <Play className="h-3.5 w-3.5" />
          </button>
          <button
            onClick={handleScheduleClick}
            className="p-1.5 rounded-md bg-green-600 hover:bg-green-700 text-white shadow-sm hover:shadow-md transition-all"
            title="Schedule agent"
          >
            <Clock className="h-3.5 w-3.5" />
          </button>
          <button
            onClick={handleEditClick}
            className="p-1.5 rounded-md bg-gray-600 hover:bg-gray-700 text-white shadow-sm hover:shadow-md transition-all"
            title="Edit agent configuration"
          >
            <Settings className="h-3.5 w-3.5" />
          </button>
        </div>

        {/* Header */}
        <div className="flex items-center gap-2 mb-2">
          <Bot className={cn(
            "h-5 w-5",
            isOrchestrator ? "text-station-lavender-500" : isCallable ? "text-station-mint-500" : "text-primary"
          )} />
          <div className="text-base font-semibold">
            {agent.name}
          </div>
        </div>

        {/* Description */}
        <div className="text-sm text-muted-foreground mb-2 line-clamp-2">
          {agent.description || 'No description'}
        </div>

        {/* Hierarchy Info Footer */}
        {hierarchyInfo && isInHierarchy && (
          <div className="mt-2 pt-2 border-t space-y-1">
            {hierarchyInfo.childAgents.length > 0 && (
              <div className="text-xs text-station-lavender-500 font-medium">
                → Calls: {hierarchyInfo.childAgents.length <= 2 
                  ? hierarchyInfo.childAgents.join(', ')
                  : `${hierarchyInfo.childAgents.slice(0, 2).join(', ')} +${hierarchyInfo.childAgents.length - 2} more`
                }
              </div>
            )}
            {hierarchyInfo.parentAgents.length > 0 && (
              <div className="text-xs text-station-mint-500 font-medium">
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
          <span className="text-muted-foreground">
            {agent.max_steps} steps
          </span>
          {agent.is_scheduled && agent.cron_schedule && (
            <span className="px-2 py-0.5 bg-green-50 text-green-700 rounded border border-green-200" title={`Schedule: ${agent.cron_schedule}`}>
              ⏰ {agent.cron_schedule}
            </span>
          )}
        </div>
      </div>
    </div>
  );
};

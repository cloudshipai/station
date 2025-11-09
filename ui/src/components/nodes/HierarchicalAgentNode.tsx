import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { Bot, Settings, Network, Users } from 'lucide-react';
import { cn } from '../../utils/cn';
import type { Agent } from '../../types/station';
import type { AgentHierarchyInfo } from '../../utils/agentHierarchy';

interface HierarchicalAgentNodeData {
  agent: Agent;
  hierarchyInfo?: AgentHierarchyInfo;
  isSelected?: boolean;
  onEditAgent?: (agentId: number) => void;
  onOpenModal?: (agentId: number) => void;
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

  return (
    <div className={cn(
      "relative w-[280px] transition-all duration-200 group",
      isSelected && "ring-2 ring-tokyo-blue"
    )}>
      {/* Hierarchy Badge - Top Left */}
      {isInHierarchy && (
        <div className="absolute -top-2 -left-2 z-10">
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
        {isOrchestrator && (
          <Handle 
            type="source" 
            position={Position.Right} 
            className="w-3 h-3 bg-tokyo-purple border-2 border-tokyo-bg"
          />
        )}

        {/* Action Buttons - Top Right */}
        <div className="absolute top-2 right-2 flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
          <button
            onClick={handleInfoClick}
            className="p-1 rounded bg-tokyo-blue hover:bg-tokyo-blue5 text-tokyo-bg"
            title="View agent details"
          >
            <Settings className="h-3 w-3" />
          </button>
          <button
            onClick={handleEditClick}
            className="p-1 rounded bg-tokyo-orange hover:bg-tokyo-orange text-tokyo-bg"
            title="Edit agent configuration"
          >
            <Settings className="h-3 w-3" />
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
                → Calls: {hierarchyInfo.childAgents.slice(0, 2).join(', ')}
                {hierarchyInfo.childAgents.length > 2 && ` +${hierarchyInfo.childAgents.length - 2} more`}
              </div>
            )}
            {hierarchyInfo.parentAgents.length > 0 && (
              <div className="text-xs text-tokyo-cyan font-mono">
                ← Called by: {hierarchyInfo.parentAgents.slice(0, 2).join(', ')}
                {hierarchyInfo.parentAgents.length > 2 && ` +${hierarchyInfo.parentAgents.length - 2} more`}
              </div>
            )}
          </div>
        )}

        {/* Status Badge */}
        <div className="mt-2 flex items-center justify-between">
          <span className="text-xs text-tokyo-green font-mono">
            {agent.max_steps} steps
          </span>
          {agent.is_scheduled && (
            <span className="text-xs px-2 py-1 bg-tokyo-green/20 text-tokyo-green rounded font-mono">
              Scheduled
            </span>
          )}
        </div>
      </div>
    </div>
  );
};

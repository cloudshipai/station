import React from 'react';
import { Bot, Network, Users } from 'lucide-react';
import { cn } from '../../utils/cn';
import type { Agent } from '../../types/station';

interface AgentListSidebarProps {
  agents: Agent[];
  selectedAgentId: number | null;
  onSelectAgent: (agentId: number) => void;
  hierarchyMap?: Map<number, any>;
}

export const AgentListSidebar: React.FC<AgentListSidebarProps> = ({
  agents,
  selectedAgentId,
  onSelectAgent,
  hierarchyMap,
}) => {
  return (
    <div className="w-80 h-full border-l border-tokyo-blue7 bg-tokyo-bg-dark overflow-hidden flex flex-col">
      {/* Header */}
      <div className="p-4 border-b border-tokyo-blue7">
        <h2 className="text-lg font-mono font-semibold text-tokyo-blue">Agents</h2>
        <p className="text-xs text-tokyo-comment font-mono mt-1">
          {agents.length} agent{agents.length !== 1 ? 's' : ''} available
        </p>
      </div>

      {/* Scrollable Agent List */}
      <div className="flex-1 overflow-y-auto p-3 space-y-2">
        {agents.map((agent) => {
          const isSelected = agent.id === selectedAgentId;
          const hierarchyInfo = hierarchyMap?.get(agent.id);
          const isOrchestrator = hierarchyInfo && hierarchyInfo.childAgents.length > 0;
          const isCallable = hierarchyInfo?.isCallable || false;

          return (
            <button
              key={agent.id}
              onClick={() => onSelectAgent(agent.id)}
              className={cn(
                'w-full text-left p-3 rounded-lg border transition-all duration-200',
                'hover:border-tokyo-blue5 hover:bg-tokyo-dark2',
                isSelected
                  ? 'border-tokyo-blue bg-tokyo-dark2 shadow-lg ring-2 ring-tokyo-blue/50'
                  : 'border-tokyo-blue7 bg-tokyo-bg'
              )}
            >
              {/* Agent Badge */}
              <div className="flex items-center gap-2 mb-2">
                {isOrchestrator ? (
                  <div className="flex items-center gap-1 px-2 py-0.5 bg-tokyo-purple rounded-full text-xs font-mono">
                    <Network className="w-3 h-3" />
                    <span>Orchestrator</span>
                  </div>
                ) : isCallable ? (
                  <div className="flex items-center gap-1 px-2 py-0.5 bg-tokyo-cyan rounded-full text-xs font-mono">
                    <Users className="w-3 h-3" />
                    <span>Callable</span>
                  </div>
                ) : (
                  <Bot className="w-4 h-4 text-tokyo-blue" />
                )}
              </div>

              {/* Agent Name */}
              <div className="font-mono text-sm font-medium text-tokyo-fg mb-1">
                {agent.name}
              </div>

              {/* Agent Description */}
              <div className="text-xs text-tokyo-comment line-clamp-2 mb-2">
                {agent.description || 'No description'}
              </div>

              {/* Agent Details */}
              <div className="flex items-center justify-between text-xs flex-wrap gap-1">
                <span className="text-tokyo-green font-mono">
                  {agent.max_steps} steps
                </span>
                {agent.is_scheduled && agent.cron_schedule && (
                  <span className="px-2 py-0.5 bg-tokyo-green/20 text-tokyo-green rounded font-mono" title={`Schedule: ${agent.cron_schedule}`}>
                    ⏰ {agent.cron_schedule}
                  </span>
                )}
              </div>

              {/* Hierarchy Info */}
              {hierarchyInfo && (
                <div className="mt-2 pt-2 border-t border-tokyo-blue7/30">
                  {hierarchyInfo.childAgents.length > 0 && (
                    <div className="text-xs text-tokyo-purple font-mono">
                      → Calls: {hierarchyInfo.childAgents.length <= 2
                        ? hierarchyInfo.childAgents.join(', ')
                        : `${hierarchyInfo.childAgents.slice(0, 2).join(', ')} +${hierarchyInfo.childAgents.length - 2}`
                      }
                    </div>
                  )}
                  {hierarchyInfo.parentAgents.length > 0 && (
                    <div className="text-xs text-tokyo-cyan font-mono mt-1">
                      ← Called by: {hierarchyInfo.parentAgents.length <= 2
                        ? hierarchyInfo.parentAgents.join(', ')
                        : `${hierarchyInfo.parentAgents.slice(0, 2).join(', ')} +${hierarchyInfo.parentAgents.length - 2}`
                      }
                    </div>
                  )}
                </div>
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
};

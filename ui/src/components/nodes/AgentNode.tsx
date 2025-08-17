import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { Bot, Settings } from 'lucide-react';
import { cn } from '../../utils/cn';
import type { Agent } from '../../types/station';

interface AgentNodeData {
  agent: Agent;
  isSelected?: boolean;
}

interface AgentNodeProps {
  data: AgentNodeData;
}

export const AgentNode: React.FC<AgentNodeProps> = ({ data }) => {
  const { agent, isSelected } = data;

  return (
    <div className={cn(
      "bg-card border border rounded-lg shadow-lg min-w-[200px] transition-all",
      isSelected && "ring-2 ring-primary"
    )}>
      {/* Node Header */}
      <div className="flex items-center space-x-2 p-3 bg-primary/10 rounded-t-lg border-b border">
        <Bot className="w-5 h-5 text-primary" />
        <span className="font-medium text-sm">{agent.name}</span>
        <Settings className="w-4 h-4 text-muted-foreground ml-auto" />
      </div>

      {/* Node Body */}
      <div className="p-3">
        <p className="text-xs text-muted-foreground mb-2 line-clamp-2">
          {agent.description || 'No description'}
        </p>
        
        <div className="flex items-center justify-between text-xs">
          <span className="bg-secondary px-2 py-1 rounded text-secondary-foreground">
            {agent.max_steps} steps
          </span>
          <span className={cn(
            "px-2 py-1 rounded",
            agent.is_scheduled 
              ? "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200"
              : "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200"
          )}>
            {agent.is_scheduled ? 'Scheduled' : 'Manual'}
          </span>
        </div>
      </div>

      {/* Connection Handles */}
      <Handle 
        type="target" 
        position={Position.Left} 
        className="w-3 h-3 bg-primary border-2 border-background"
      />
      <Handle 
        type="source" 
        position={Position.Right} 
        className="w-3 h-3 bg-primary border-2 border-background"
      />
    </div>
  );
};
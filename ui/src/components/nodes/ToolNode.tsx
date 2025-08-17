import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { Wrench } from 'lucide-react';
import { cn } from '../../utils/cn';
import type { Tool } from '../../types/station';

interface ToolNodeData {
  tool: Tool;
  isSelected?: boolean;
}

interface ToolNodeProps {
  data: ToolNodeData;
}

export const ToolNode: React.FC<ToolNodeProps> = ({ data }) => {
  const { tool, isSelected } = data;

  return (
    <div className={cn(
      "bg-card border border rounded-lg shadow-lg min-w-[160px] transition-all",
      isSelected && "ring-2 ring-primary"
    )}>
      {/* Node Header */}
      <div className="flex items-center space-x-2 p-2 bg-orange-50 dark:bg-orange-950 rounded-t-lg border-b border">
        <Wrench className="w-4 h-4 text-orange-600 dark:text-orange-400" />
        <span className="font-medium text-xs">{tool.name}</span>
      </div>

      {/* Node Body */}
      <div className="p-2">
        <p className="text-xs text-muted-foreground line-clamp-2">
          {tool.description || 'No description'}
        </p>
      </div>

      {/* Connection Handles */}
      <Handle 
        type="target" 
        position={Position.Left} 
        className="w-2 h-2 bg-orange-600 border border-background"
      />
      <Handle 
        type="source" 
        position={Position.Right} 
        className="w-2 h-2 bg-orange-600 border border-background"
      />
    </div>
  );
};
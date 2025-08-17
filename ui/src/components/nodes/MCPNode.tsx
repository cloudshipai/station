import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { Server, Activity } from 'lucide-react';
import { cn } from '../../utils/cn';
import type { MCPServer } from '../../types/station';

interface MCPNodeData {
  mcpServer: MCPServer;
  isSelected?: boolean;
}

interface MCPNodeProps {
  data: MCPNodeData;
}

export const MCPNode: React.FC<MCPNodeProps> = ({ data }) => {
  const { mcpServer, isSelected } = data;

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'active':
        return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
      case 'inactive':
        return 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200';
      case 'error':
        return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200';
      default:
        return 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200';
    }
  };

  return (
    <div className={cn(
      "bg-card border border rounded-lg shadow-lg min-w-[200px] transition-all",
      isSelected && "ring-2 ring-primary"
    )}>
      {/* Node Header */}
      <div className="flex items-center space-x-2 p-3 bg-blue-50 dark:bg-blue-950 rounded-t-lg border-b border">
        <Server className="w-5 h-5 text-blue-600 dark:text-blue-400" />
        <span className="font-medium text-sm">{mcpServer.name}</span>
        <Activity className="w-4 h-4 text-muted-foreground ml-auto" />
      </div>

      {/* Node Body */}
      <div className="p-3">
        <p className="text-xs text-muted-foreground mb-2 line-clamp-2">
          {mcpServer.description || 'No description'}
        </p>
        
        <div className="flex items-center justify-between text-xs">
          <span className="bg-secondary px-2 py-1 rounded text-secondary-foreground font-mono">
            {mcpServer.command}
          </span>
          <span className={cn("px-2 py-1 rounded", getStatusColor(mcpServer.status))}>
            {mcpServer.status}
          </span>
        </div>
      </div>

      {/* Connection Handles */}
      <Handle 
        type="target" 
        position={Position.Left} 
        className="w-3 h-3 bg-blue-600 border-2 border-background"
      />
      <Handle 
        type="source" 
        position={Position.Right} 
        className="w-3 h-3 bg-blue-600 border-2 border-background"
      />
    </div>
  );
};
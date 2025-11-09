import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { Database, ChevronDown, ChevronRight } from 'lucide-react';

export const MCPNode = ({ data }: any) => {
  const isExpanded = data.expanded;

  return (
    <div className="relative">
      <Handle type="target" position={Position.Left} className="w-3 h-3 bg-tokyo-purple" />
      <Handle type="source" position={Position.Right} className="w-3 h-3 bg-tokyo-purple" />
      <div className="bg-tokyo-dark2 border-2 border-tokyo-purple rounded-lg p-4 min-w-[260px] shadow-xl hover:shadow-2xl transition-all duration-200 hover:border-tokyo-purple hover:bg-tokyo-dark1 group">
        <button
          onClick={(e) => {
            e.stopPropagation();
            if (data.onToggleExpand) {
              data.onToggleExpand(data.serverId);
            }
          }}
          className="w-full"
        >
          <div className="flex items-center justify-between mb-2">
            <div className="flex items-center gap-2">
              <Database className="h-5 w-5 text-tokyo-purple" />
              <div className="font-mono text-base text-tokyo-purple font-medium">{data.label}</div>
            </div>
            {isExpanded ? (
              <ChevronDown className="h-4 w-4 text-tokyo-purple" />
            ) : (
              <ChevronRight className="h-4 w-4 text-tokyo-purple" />
            )}
          </div>
        </button>

        <div className="text-sm text-tokyo-comment mb-1">{data.description}</div>

        <div className="text-xs text-tokyo-cyan font-medium mt-2">
          {data.tools?.length || 0} tools available
        </div>

        <button
          onClick={(e) => {
            e.stopPropagation();
            if (data.onOpenMCPModal) {
              data.onOpenMCPModal(data.serverId);
            }
          }}
          className="mt-2 w-full text-xs text-tokyo-blue hover:text-tokyo-blue5 transition-colors text-left"
        >
          View Details →
        </button>
      </div>
    </div>
  );
};

export const ToolNode = ({ data }: any) => {
  return (
    <div className="relative bg-tokyo-dark2 border border-tokyo-orange rounded-lg px-3 py-2 min-w-[180px] shadow-lg hover:shadow-xl transition-all duration-200 hover:border-tokyo-orange hover:bg-tokyo-dark1">
      <div className="flex items-center gap-2 mb-1">
        <div className="w-2 h-2 bg-tokyo-orange rounded-full"></div>
        <div className="font-mono text-sm text-tokyo-orange font-medium">{data.label}</div>
      </div>

      <p className="text-tokyo-comment text-xs font-mono line-clamp-2 mb-1">
        {data.description}
      </p>

      <div className="text-xs font-mono text-tokyo-orange opacity-75">
        {data.category}
      </div>

      <div className="absolute top-1/2 -translate-y-1/2 -left-2 w-3 h-3 bg-tokyo-orange rounded-full border-2 border-tokyo-bg"></div>
    </div>
  );
};

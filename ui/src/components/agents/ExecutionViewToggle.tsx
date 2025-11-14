import React from 'react';
import { Zap, Eye } from 'lucide-react';

interface ExecutionViewToggleProps {
  isExecutionView: boolean;
  hasTraceData: boolean;
  onToggle: () => void;
}

export const ExecutionViewToggle: React.FC<ExecutionViewToggleProps> = ({ 
  isExecutionView, 
  hasTraceData,
  onToggle 
}) => {
  if (!hasTraceData) return null;

  return (
    <button
      onClick={onToggle}
      className={`flex items-center gap-2 px-4 py-2 rounded-lg font-mono text-sm transition-all ${
        isExecutionView
          ? 'bg-green-900/40 border-2 border-green-500 text-green-300 hover:bg-green-900/60'
          : 'bg-gray-800 border-2 border-gray-600 text-gray-400 hover:bg-gray-700'
      }`}
    >
      {isExecutionView ? (
        <>
          <Zap className="w-4 h-4" />
          <span>Execution View</span>
          <div className="w-2 h-2 bg-green-500 rounded-full animate-pulse"></div>
        </>
      ) : (
        <>
          <Eye className="w-4 h-4" />
          <span>Definition View</span>
        </>
      )}
    </button>
  );
};

import React from 'react';
import { Eye, CheckSquare, Square, CheckSquare2 } from 'lucide-react';

interface Run {
  id: number;
  agent_name: string;
  status: 'completed' | 'running' | 'failed' | 'pending';
  duration_seconds?: number;
  started_at: string;
  error?: string;
}

interface RunsListProps {
  runs: Run[];
  onRunClick: (runId: number) => void;
  selectedRuns?: Set<number>;
  onToggleSelection?: (runId: number) => void;
  onToggleSelectAll?: () => void;
  allSelected?: boolean;
}

export const RunsList: React.FC<RunsListProps> = ({ 
  runs, 
  onRunClick,
  selectedRuns = new Set(),
  onToggleSelection,
  onToggleSelectAll,
  allSelected = false
}) => {
  return (
    <div className="grid gap-4 max-h-full overflow-y-auto">
      {/* Select All Header */}
      {onToggleSelectAll && runs.length > 0 && (
        <div className="flex items-center gap-3 px-4 py-2 bg-gray-50 border border-gray-200 rounded-lg">
          <button
            onClick={onToggleSelectAll}
            className="flex items-center gap-2 text-sm text-gray-600 hover:text-gray-900"
          >
            {allSelected ? (
              <CheckSquare className="h-5 w-5 text-primary" />
            ) : selectedRuns.size > 0 ? (
              <CheckSquare2 className="h-5 w-5 text-primary" />
            ) : (
              <Square className="h-5 w-5" />
            )}
            <span>
              {allSelected ? 'Deselect All' : selectedRuns.size > 0 ? `${selectedRuns.size} selected` : 'Select All'}
            </span>
          </button>
        </div>
      )}

      {runs.map((run) => (
        <div 
          key={run.id} 
          className={`p-4 bg-white border rounded-lg shadow-sm hover:shadow-md transition-shadow ${
            selectedRuns.has(run.id) ? 'border-primary ring-1 ring-primary' : 'border-gray-200'
          }`}
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              {onToggleSelection && (
                <button
                  onClick={() => onToggleSelection(run.id)}
                  className="text-gray-400 hover:text-primary"
                >
                  {selectedRuns.has(run.id) ? (
                    <CheckSquare className="h-5 w-5 text-primary" />
                  ) : (
                    <Square className="h-5 w-5" />
                  )}
                </button>
              )}
              <h3 className="font-medium text-gray-900">{run.agent_name}</h3>
            </div>
            <div className="flex items-center gap-3">
              <button
                onClick={() => onRunClick(run.id)}
                className="p-2 rounded-lg hover:bg-gray-100 text-gray-600 hover:text-primary transition-colors"
                title="View run details"
              >
                <Eye className="h-4 w-4" />
              </button>
              <span className={`px-2 py-1 text-xs rounded font-medium ${
                run.status === 'completed' ? 'bg-green-50 text-green-700 border border-green-200' :
                run.status === 'running' ? 'bg-blue-50 text-blue-700 border border-blue-200' :
                run.status === 'pending' ? 'bg-yellow-50 text-yellow-700 border border-yellow-200' :
                'bg-red-50 text-red-700 border border-red-200'
              }`}>
                {run.status}
              </span>
            </div>
          </div>
          <div className="mt-2 flex gap-4 text-sm text-gray-600 ml-8">
            <span>Duration: {run.duration_seconds ? `${run.duration_seconds.toFixed(1)}s` : 'N/A'}</span>
            <span>Time: {new Date(run.started_at).toLocaleTimeString()}</span>
          </div>
          {run.error && (
            <div className="mt-2 p-2 bg-red-50 border border-red-200 rounded text-sm text-red-700 ml-8">
              Error: {run.error}
            </div>
          )}
        </div>
      ))}
    </div>
  );
};
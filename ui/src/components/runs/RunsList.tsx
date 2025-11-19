import React from 'react';
import { Eye } from 'lucide-react';

interface Run {
  id: number;
  agent_name: string;
  status: 'completed' | 'running' | 'failed';
  duration_seconds?: number;
  started_at: string;
  error?: string;
}

interface RunsListProps {
  runs: Run[];
  onRunClick: (runId: number) => void;
}

export const RunsList: React.FC<RunsListProps> = ({ runs, onRunClick }) => {
  return (
    <div className="grid gap-4 max-h-full overflow-y-auto">
      {runs.map((run) => (
        <div key={run.id} className="p-4 bg-white border border-gray-200 rounded-lg shadow-sm hover:shadow-md transition-shadow">
          <div className="flex items-center justify-between">
            <h3 className="font-medium text-gray-900">{run.agent_name}</h3>
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
                'bg-red-50 text-red-700 border border-red-200'
              }`}>
                {run.status}
              </span>
            </div>
          </div>
          <div className="mt-2 flex gap-4 text-sm text-gray-600">
            <span>Duration: {run.duration_seconds ? `${run.duration_seconds.toFixed(1)}s` : 'N/A'}</span>
            <span>Time: {new Date(run.started_at).toLocaleTimeString()}</span>
          </div>
          {run.error && (
            <div className="mt-2 p-2 bg-red-50 border border-red-200 rounded text-sm text-red-700">
              Error: {run.error}
            </div>
          )}
        </div>
      ))}
    </div>
  );
};
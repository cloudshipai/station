import React from 'react';
import { Eye } from 'lucide-react';

interface Run {
  id: number;
  agent_name: string;
  status: 'completed' | 'running' | 'failed';
  duration_seconds?: number;
  started_at: string;
}

interface RunsListProps {
  runs: Run[];
  onRunClick: (runId: number) => void;
}

export const RunsList: React.FC<RunsListProps> = ({ runs, onRunClick }) => {
  return (
    <div className="grid gap-4 max-h-full overflow-y-auto">
      {runs.map((run) => (
        <div key={run.id} className="p-4 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo">
          <div className="flex items-center justify-between">
            <h3 className="font-mono font-medium text-tokyo-green">{run.agent_name}</h3>
            <div className="flex items-center gap-3">
              <button
                onClick={() => onRunClick(run.id)}
                className="p-2 rounded-lg hover:bg-tokyo-bg-highlight text-tokyo-comment hover:text-tokyo-blue transition-colors"
                title="View run details"
              >
                <Eye className="h-4 w-4" />
              </button>
              <span className={`px-2 py-1 text-xs rounded font-mono ${
                run.status === 'completed' ? 'bg-tokyo-green text-tokyo-bg' :
                run.status === 'running' ? 'bg-tokyo-blue text-tokyo-bg' :
                'bg-tokyo-red text-tokyo-bg'
              }`}>
                {run.status}
              </span>
            </div>
          </div>
          <div className="mt-2 flex gap-4 text-sm text-tokyo-comment font-mono">
            <span>Duration: {run.duration_seconds ? `${run.duration_seconds.toFixed(1)}s` : 'N/A'}</span>
            <span>Time: {new Date(run.started_at).toLocaleTimeString()}</span>
          </div>
        </div>
      ))}
    </div>
  );
};
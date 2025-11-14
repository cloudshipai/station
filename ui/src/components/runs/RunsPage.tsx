import React, { useState, useEffect } from 'react';
import { Play, BarChart3, List, GitBranch } from 'lucide-react';
import { RunsList } from './RunsList';
import { Pagination } from './Pagination';
import { StatsTab } from './StatsTab';
import { SwimlanePage } from './SwimlanePage';
import { agentRunsApi } from '../../api/station';
import type { TimelineRun } from '../../utils/timelineLayout';

interface Run {
  id: number;
  agent_id: number;
  agent_name: string;
  status: 'completed' | 'running' | 'failed';
  duration_seconds?: number;
  started_at: string;
  total_tokens?: number;
  input_tokens?: number;
  output_tokens?: number;
  cost?: number;
  tools_used?: number;
  steps_taken?: number;
  parent_run_id?: number;
  error?: string;
}

interface RunsPageProps {
  onRunClick: (runId: number) => void;
  refreshTrigger?: any;
}

export const RunsPage: React.FC<RunsPageProps> = ({ onRunClick, refreshTrigger }) => {
  const [runs, setRuns] = useState<Run[]>([]);
  const [activeTab, setActiveTab] = useState<'list' | 'timeline' | 'stats'>('timeline');
  const [currentPage, setCurrentPage] = useState(1);
  const runsPerPage = 20;

  // Fetch runs data
  useEffect(() => {
    const fetchRuns = async () => {
      try {
        const response = await agentRunsApi.getAll();
        setRuns(response.data.runs || []);
      } catch (error) {
        console.error('Failed to fetch runs:', error);
      }
    };

    fetchRuns();
  }, [refreshTrigger]);

  // Pagination logic
  const totalPages = Math.ceil(runs.length / runsPerPage);
  const startIndex = (currentPage - 1) * runsPerPage;
  const endIndex = startIndex + runsPerPage;
  const currentRuns = runs.slice(startIndex, endIndex);

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
  };

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      {/* Content - Timeline has its own header with tabs */}
      <div className="flex-1 overflow-hidden">
        {activeTab === 'list' ? (
          <div className="h-full flex flex-col">
            {/* Header for List view */}
            <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
              <h1 className="text-xl font-mono font-semibold text-tokyo-green">Agent Runs</h1>
              <div className="flex bg-tokyo-bg rounded-lg p-1">
                <button
                  onClick={() => setActiveTab('list')}
                  className={`flex items-center gap-2 px-4 py-2 rounded-md font-mono text-sm transition-colors ${
                    activeTab === 'list'
                      ? 'bg-tokyo-blue text-tokyo-bg'
                      : 'text-tokyo-comment hover:text-tokyo-blue hover:bg-tokyo-bg-highlight'
                  }`}
                >
                  <List className="h-4 w-4" />
                  List
                </button>
                <button
                  onClick={() => setActiveTab('timeline')}
                  className={`flex items-center gap-2 px-4 py-2 rounded-md font-mono text-sm transition-colors ${
                    activeTab === 'timeline'
                      ? 'bg-tokyo-blue text-tokyo-bg'
                      : 'text-tokyo-comment hover:text-tokyo-blue hover:bg-tokyo-bg-highlight'
                  }`}
                >
                  <GitBranch className="h-4 w-4" />
                  Timeline
                </button>
                <button
                  onClick={() => setActiveTab('stats')}
                  className={`flex items-center gap-2 px-4 py-2 rounded-md font-mono text-sm transition-colors ${
                    activeTab === 'stats'
                      ? 'bg-tokyo-blue text-tokyo-bg'
                      : 'text-tokyo-comment hover:text-tokyo-blue hover:bg-tokyo-bg-highlight'
                  }`}
                >
                  <BarChart3 className="h-4 w-4" />
                  Stats
                </button>
              </div>
            </div>
            {runs.length === 0 ? (
              <div className="flex-1 flex items-center justify-center">
                <div className="text-center">
                  <Play className="h-16 w-16 text-tokyo-comment mx-auto mb-4" />
                  <div className="text-tokyo-fg font-mono text-lg mb-2">No agent runs found</div>
                  <div className="text-tokyo-comment font-mono text-sm">
                    Agent executions will appear here when agents are run
                  </div>
                </div>
              </div>
            ) : (
              <div className="flex-1 p-4 overflow-y-auto">
                <RunsList runs={currentRuns} onRunClick={onRunClick} />
                <Pagination
                  currentPage={currentPage}
                  totalPages={totalPages}
                  onPageChange={handlePageChange}
                />
              </div>
            )}
          </div>
        ) : activeTab === 'timeline' ? (
          <SwimlanePage 
            runs={runs as TimelineRun[]} 
            onRunClick={onRunClick}
            activeView={activeTab}
            onViewChange={setActiveTab}
          />
        ) : (
          <div className="h-full flex flex-col">
            {/* Header for Stats view */}
            <div className="flex items-center justify-between p-4 border-b border-tokyo-blue7 bg-tokyo-bg-dark">
              <h1 className="text-xl font-mono font-semibold text-tokyo-green">Agent Runs</h1>
              <div className="flex bg-tokyo-bg rounded-lg p-1">
                <button
                  onClick={() => setActiveTab('list')}
                  className={`flex items-center gap-2 px-4 py-2 rounded-md font-mono text-sm transition-colors ${
                    activeTab === 'list'
                      ? 'bg-tokyo-blue text-tokyo-bg'
                      : 'text-tokyo-comment hover:text-tokyo-blue hover:bg-tokyo-bg-highlight'
                  }`}
                >
                  <List className="h-4 w-4" />
                  List
                </button>
                <button
                  onClick={() => setActiveTab('timeline')}
                  className={`flex items-center gap-2 px-4 py-2 rounded-md font-mono text-sm transition-colors ${
                    activeTab === 'timeline'
                      ? 'bg-tokyo-blue text-tokyo-bg'
                      : 'text-tokyo-comment hover:text-tokyo-blue hover:bg-tokyo-bg-highlight'
                  }`}
                >
                  <GitBranch className="h-4 w-4" />
                  Timeline
                </button>
                <button
                  onClick={() => setActiveTab('stats')}
                  className={`flex items-center gap-2 px-4 py-2 rounded-md font-mono text-sm transition-colors ${
                    activeTab === 'stats'
                      ? 'bg-tokyo-blue text-tokyo-bg'
                      : 'text-tokyo-comment hover:text-tokyo-blue hover:bg-tokyo-bg-highlight'
                  }`}
                >
                  <BarChart3 className="h-4 w-4" />
                  Stats
                </button>
              </div>
            </div>
            <div className="flex-1 p-4 overflow-y-auto">
              <StatsTab runs={runs} />
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
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
  environment_id?: number;
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

  // Fetch all runs (not filtered by environment)
  useEffect(() => {
    const fetchRuns = async () => {
      try {
        // Fetch all runs across all environments
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
    <div className="h-full flex flex-col bg-gray-50">
      {/* Content - Timeline has its own header with tabs */}
      <div className="flex-1 overflow-hidden">
        {activeTab === 'list' ? (
          <div className="h-full flex flex-col">
            {/* Header for List view */}
            <div className="flex items-center gap-4 p-4 border-b border-gray-200 bg-white">
              <h1 className="text-xl font-semibold text-gray-900">Agent Runs</h1>
              
              <div className="flex bg-gray-100 rounded-lg p-1">
                <button
                  onClick={() => setActiveTab('list')}
                  className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                    activeTab === 'list'
                      ? 'bg-white text-primary shadow-sm'
                      : 'text-gray-600 hover:text-gray-900'
                  }`}
                >
                  <List className="h-4 w-4" />
                  List
                </button>
                <button
                  onClick={() => setActiveTab('timeline')}
                  className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                    activeTab === 'timeline'
                      ? 'bg-white text-primary shadow-sm'
                      : 'text-gray-600 hover:text-gray-900'
                  }`}
                >
                  <GitBranch className="h-4 w-4" />
                  Timeline
                </button>
                <button
                  onClick={() => setActiveTab('stats')}
                  className={`flex items-center gap-2 px-4 py-2 rounded-md text-sm font-medium transition-colors ${
                    activeTab === 'stats'
                      ? 'bg-white text-primary shadow-sm'
                      : 'text-gray-600 hover:text-gray-900'
                  }`}
                >
                  <BarChart3 className="h-4 w-4" />
                  Stats
                </button>
              </div>
            </div>
            {runs.length === 0 ? (
              <div className="flex-1 flex items-center justify-center bg-white">
                <div className="text-center">
                  <Play className="h-16 w-16 text-gray-300 mx-auto mb-4" />
                  <div className="text-gray-900 text-lg mb-2">No agent runs found</div>
                  <div className="text-gray-500 text-sm">
                    Agent executions will appear here when agents are run
                  </div>
                </div>
              </div>
            ) : (
              <div className="flex-1 p-4 overflow-y-auto bg-white">
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
          <StatsTab 
            runs={runs}
            activeView={activeTab}
            onViewChange={setActiveTab}
          />
        )}
      </div>
    </div>
  );
};
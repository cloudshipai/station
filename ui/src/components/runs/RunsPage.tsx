import React, { useState, useEffect, useContext } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { Play, BarChart3, List, GitBranch } from 'lucide-react';
import { RunsList } from './RunsList';
import { Pagination } from './Pagination';
import { StatsTab } from './StatsTab';
import { SwimlanePage } from './SwimlanePage';
import { agentRunsApi } from '../../api/station';
import { EnvironmentContext } from '../../contexts/EnvironmentContext';
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
  const { env } = useParams<{ env?: string }>();
  const navigate = useNavigate();
  const { environments } = useContext(EnvironmentContext);
  const [currentEnvironment, setCurrentEnvironment] = useState<any | null>(null);
  const [runs, setRuns] = useState<Run[]>([]);
  const [activeTab, setActiveTab] = useState<'list' | 'timeline' | 'stats'>('timeline');
  const [currentPage, setCurrentPage] = useState(1);
  const runsPerPage = 20;

  // Determine current environment from URL param
  useEffect(() => {
    if (env && environments.length > 0) {
      const environment = environments.find((e: any) => e.name.toLowerCase() === env.toLowerCase());
      setCurrentEnvironment(environment || null);
      
      // If env in URL doesn't exist, redirect to first environment
      if (!environment && environments.length > 0) {
        navigate(`/runs/${environments[0].name.toLowerCase()}`);
      }
    } else if (!env && environments.length > 0) {
      // No env in URL, redirect to first environment
      navigate(`/runs/${environments[0].name.toLowerCase()}`);
    }
  }, [env, environments, navigate]);

  // Fetch runs data
  useEffect(() => {
    const fetchRuns = async () => {
      try {
        const response = await agentRunsApi.getAll();
        let allRuns = response.data.runs || [];
        
        // Filter by environment if one is selected
        if (currentEnvironment) {
          allRuns = allRuns.filter((run: Run) => run.environment_id === currentEnvironment.id);
        }
        
        setRuns(allRuns);
      } catch (error) {
        console.error('Failed to fetch runs:', error);
      }
    };

    fetchRuns();
  }, [refreshTrigger, currentEnvironment]);

  // Pagination logic
  const totalPages = Math.ceil(runs.length / runsPerPage);
  const startIndex = (currentPage - 1) * runsPerPage;
  const endIndex = startIndex + runsPerPage;
  const currentRuns = runs.slice(startIndex, endIndex);

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
  };

  const handleEnvironmentChange = (newEnvName: string) => {
    navigate(`/runs/${newEnvName.toLowerCase()}`);
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
              
              {/* Environment Selector */}
              {env && (
                <>
                  <div className="h-4 w-px bg-gray-300"></div>
                  <select
                    value={env || ''}
                    onChange={(e) => handleEnvironmentChange(e.target.value)}
                    className="px-3 py-1.5 bg-white border border-gray-300 text-gray-900 text-sm rounded-lg hover:border-gray-400 focus:ring-2 focus:ring-gray-900/10 focus:border-gray-400 transition-colors"
                  >
                    {environments.map((environment: any) => (
                      <option key={environment.id} value={environment.name.toLowerCase()}>
                        {environment.name}
                      </option>
                    ))}
                  </select>
                </>
              )}
              
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
            currentEnvironment={currentEnvironment}
            environments={environments}
            onEnvironmentChange={(envName: string) => handleEnvironmentChange(envName)}
          />
        ) : (
          <StatsTab 
            runs={runs}
            currentEnvironment={currentEnvironment}
            environments={environments}
            onEnvironmentChange={(envName: string) => handleEnvironmentChange(envName)}
          />
        )}
      </div>
    </div>
  );
};
import React, { useState, useEffect, useCallback } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Play, BarChart3, List, GitBranch, HelpCircle, Clock, Zap, DollarSign, Activity, Filter, Trash2, X, AlertTriangle } from 'lucide-react';
import { RunsList } from './RunsList';
import { Pagination } from './Pagination';
import { StatsTab } from './StatsTab';
import { SwimlanePage } from './SwimlanePage';
import { agentRunsApi } from '../../api/station';
import type { TimelineRun } from '../../utils/timelineLayout';
import { HelpModal } from '../ui/HelpModal';

interface Run {
  id: number;
  agent_id: number;
  agent_name: string;
  status: 'completed' | 'running' | 'failed' | 'pending';
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

type StatusFilter = '' | 'completed' | 'running' | 'failed' | 'pending';

interface RunsPageProps {
  onRunClick: (runId: number) => void;
  refreshTrigger?: any;
}

export const RunsPage: React.FC<RunsPageProps> = ({ onRunClick, refreshTrigger }) => {
  const [searchParams, setSearchParams] = useSearchParams();
  const [runs, setRuns] = useState<Run[]>([]);
  const [activeTab, setActiveTab] = useState<'list' | 'timeline' | 'stats'>('timeline');
  const [currentPage, setCurrentPage] = useState(1);
  const [isHelpModalOpen, setIsHelpModalOpen] = useState(false);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('');
  const [agentIdFilter, setAgentIdFilter] = useState<number | null>(null);
  const [totalCount, setTotalCount] = useState(0);
  const [selectedRuns, setSelectedRuns] = useState<Set<number>>(new Set());
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [deleteMode, setDeleteMode] = useState<'selected' | 'filtered' | 'all'>('selected');
  const [isDeleting, setIsDeleting] = useState(false);
  const runsPerPage = 20;

  useEffect(() => {
    const runIdParam = searchParams.get('run_id');
    const agentIdParam = searchParams.get('agent_id');
    
    if (runIdParam || agentIdParam) {
      setActiveTab('list');
    }
    
    if (agentIdParam) {
      setAgentIdFilter(parseInt(agentIdParam, 10));
    }
    
    if (runIdParam) {
      const runId = parseInt(runIdParam, 10);
      if (!isNaN(runId)) {
        onRunClick(runId);
        searchParams.delete('run_id');
        setSearchParams(searchParams, { replace: true });
      }
    }
  }, [searchParams, setSearchParams, onRunClick]);

  const fetchRuns = useCallback(async () => {
    try {
      const params: { status?: string; limit?: number; agent_id?: number } = { limit: 500 };
      if (statusFilter) {
        params.status = statusFilter;
      }
      if (agentIdFilter) {
        params.agent_id = agentIdFilter;
      }
      const response = await agentRunsApi.getAll(params);
      setRuns(response.data.runs || []);
      setTotalCount(response.data.total_count || response.data.runs?.length || 0);
    } catch (error) {
      console.error('Failed to fetch runs:', error);
    }
  }, [statusFilter, agentIdFilter]);

  useEffect(() => {
    fetchRuns();
  }, [fetchRuns, refreshTrigger]);

  // Clear selection when filter changes
  useEffect(() => {
    setSelectedRuns(new Set());
    setCurrentPage(1);
  }, [statusFilter]);

  // Pagination logic
  const totalPages = Math.ceil(runs.length / runsPerPage);
  const startIndex = (currentPage - 1) * runsPerPage;
  const endIndex = startIndex + runsPerPage;
  const currentRuns = runs.slice(startIndex, endIndex);

  const handlePageChange = (page: number) => {
    setCurrentPage(page);
  };

  // Selection handlers
  const toggleRunSelection = (runId: number) => {
    setSelectedRuns(prev => {
      const newSet = new Set(prev);
      if (newSet.has(runId)) {
        newSet.delete(runId);
      } else {
        newSet.add(runId);
      }
      return newSet;
    });
  };

  const toggleSelectAll = () => {
    if (selectedRuns.size === runs.length) {
      setSelectedRuns(new Set());
    } else {
      setSelectedRuns(new Set(runs.map(r => r.id)));
    }
  };

  const handleDeleteClick = (mode: 'selected' | 'filtered' | 'all') => {
    setDeleteMode(mode);
    setIsDeleteModalOpen(true);
  };

  const handleConfirmDelete = async () => {
    setIsDeleting(true);
    try {
      if (deleteMode === 'all') {
        await agentRunsApi.deleteMany({ all: true });
      } else if (deleteMode === 'filtered' && statusFilter) {
        await agentRunsApi.deleteMany({ status: statusFilter });
      } else if (deleteMode === 'selected' && selectedRuns.size > 0) {
        await agentRunsApi.deleteMany({ ids: Array.from(selectedRuns) });
      }
      setSelectedRuns(new Set());
      setIsDeleteModalOpen(false);
      fetchRuns();
    } catch (error) {
      console.error('Failed to delete runs:', error);
    } finally {
      setIsDeleting(false);
    }
  };

  const getDeleteMessage = () => {
    if (deleteMode === 'all') {
      return `Are you sure you want to delete ALL ${totalCount} runs? This action cannot be undone.`;
    } else if (deleteMode === 'filtered' && statusFilter) {
      return `Are you sure you want to delete all ${runs.length} runs with status "${statusFilter}"? This action cannot be undone.`;
    } else {
      return `Are you sure you want to delete ${selectedRuns.size} selected run${selectedRuns.size > 1 ? 's' : ''}? This action cannot be undone.`;
    }
  };

  return (
    <div className="h-full flex flex-col bg-gray-50">
      {/* Content - Timeline has its own header with tabs */}
      <div className="flex-1 overflow-hidden">
        {activeTab === 'list' ? (
          <div className="h-full flex flex-col">
            {/* Header for List view */}
            <div className="flex items-center justify-between p-4 border-b border-gray-200 bg-white">
              <div className="flex items-center gap-4">
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

                {/* Status Filter */}
                <div className="flex items-center gap-2">
                  <Filter className="h-4 w-4 text-gray-500" />
                  <select
                    value={statusFilter}
                    onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
                    className="text-sm border border-gray-300 rounded-md px-3 py-1.5 bg-white focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
                  >
                    <option value="">All Statuses</option>
                    <option value="completed">Completed</option>
                    <option value="running">Running</option>
                    <option value="failed">Failed</option>
                    <option value="pending">Pending</option>
                  </select>
                  {statusFilter && (
                    <button
                      onClick={() => setStatusFilter('')}
                      className="text-gray-400 hover:text-gray-600"
                    >
                      <X className="h-4 w-4" />
                    </button>
                  )}
                </div>

                {/* Agent Filter Badge */}
                {agentIdFilter && (
                  <div className="flex items-center gap-1 px-3 py-1.5 bg-blue-50 text-blue-700 rounded-md text-sm border border-blue-200">
                    <span>Agent ID: {agentIdFilter}</span>
                    <button
                      onClick={() => {
                        setAgentIdFilter(null);
                        searchParams.delete('agent_id');
                        setSearchParams(searchParams, { replace: true });
                      }}
                      className="ml-1 text-blue-500 hover:text-blue-700"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </div>
                )}

                <span className="text-sm text-gray-500">
                  {runs.length} of {totalCount} runs
                </span>
              </div>

              {/* Delete Actions */}
              <div className="flex items-center gap-2">
                {selectedRuns.size > 0 && (
                  <button
                    onClick={() => handleDeleteClick('selected')}
                    className="flex items-center gap-2 px-3 py-1.5 text-sm text-red-600 hover:text-red-700 hover:bg-red-50 rounded-md transition-colors"
                  >
                    <Trash2 className="h-4 w-4" />
                    Delete Selected ({selectedRuns.size})
                  </button>
                )}
                {statusFilter && runs.length > 0 && (
                  <button
                    onClick={() => handleDeleteClick('filtered')}
                    className="flex items-center gap-2 px-3 py-1.5 text-sm text-orange-600 hover:text-orange-700 hover:bg-orange-50 rounded-md transition-colors"
                  >
                    <Trash2 className="h-4 w-4" />
                    Delete All {statusFilter}
                  </button>
                )}
                {totalCount > 0 && (
                  <button
                    onClick={() => handleDeleteClick('all')}
                    className="flex items-center gap-2 px-3 py-1.5 text-sm text-red-600 hover:text-red-700 hover:bg-red-50 border border-red-200 rounded-md transition-colors"
                  >
                    <Trash2 className="h-4 w-4" />
                    Delete All Runs
                  </button>
                )}
              </div>
            </div>
            {runs.length === 0 ? (
              <div className="flex-1 bg-white p-4 space-y-4">
                {/* Empty State Message */}
                <div className="text-center py-8">
                  <Play className="h-12 w-12 text-gray-300 mx-auto mb-3" />
                  <div className="text-gray-900 text-lg mb-1">No agent runs found</div>
                  <div className="text-gray-500 text-sm">
                    Agent executions will appear here when agents are run
                  </div>
                </div>

                {/* Skeleton Placeholders */}
                <div className="grid gap-4 opacity-25">
                  {[1, 2, 3].map((i) => (
                    <div key={i} className="p-4 bg-white border border-gray-200 rounded-lg shadow-sm">
                      <div className="flex items-center justify-between">
                        <div className="h-5 w-48 bg-gray-200 rounded"></div>
                        <div className="flex items-center gap-3">
                          <div className="h-8 w-8 bg-gray-200 rounded-lg"></div>
                          <div className="h-6 w-20 bg-gray-200 rounded-full"></div>
                        </div>
                      </div>
                      <div className="mt-2 flex gap-4">
                        <div className="h-4 w-32 bg-gray-200 rounded"></div>
                        <div className="h-4 w-28 bg-gray-200 rounded"></div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <div className="flex-1 p-4 overflow-y-auto bg-white">
                <RunsList 
                  runs={currentRuns} 
                  onRunClick={onRunClick}
                  selectedRuns={selectedRuns}
                  onToggleSelection={toggleRunSelection}
                  onToggleSelectAll={toggleSelectAll}
                  allSelected={selectedRuns.size === runs.length && runs.length > 0}
                />
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

      {/* Delete Confirmation Modal */}
      {isDeleteModalOpen && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-xl max-w-md w-full mx-4">
            <div className="p-6">
              <div className="flex items-center gap-3 mb-4">
                <div className="p-2 bg-red-100 rounded-full">
                  <AlertTriangle className="h-6 w-6 text-red-600" />
                </div>
                <h3 className="text-lg font-semibold text-gray-900">Confirm Delete</h3>
              </div>
              <p className="text-gray-600 mb-6">
                {getDeleteMessage()}
              </p>
              <div className="flex justify-end gap-3">
                <button
                  onClick={() => setIsDeleteModalOpen(false)}
                  disabled={isDeleting}
                  className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50"
                >
                  Cancel
                </button>
                <button
                  onClick={handleConfirmDelete}
                  disabled={isDeleting}
                  className="px-4 py-2 text-sm font-medium text-white bg-red-600 border border-transparent rounded-md hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-red-500 disabled:opacity-50 flex items-center gap-2"
                >
                  {isDeleting ? (
                    <>
                      <div className="animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent"></div>
                      Deleting...
                    </>
                  ) : (
                    <>
                      <Trash2 className="h-4 w-4" />
                      Delete
                    </>
                  )}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
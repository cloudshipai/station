import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { 
  GitBranch, 
  Play, 
  Pause, 
  XCircle, 
  Clock, 
  CheckCircle, 
  AlertTriangle,
  ChevronRight,
  Filter,
  RefreshCw,
  Plus,
  MoreVertical,
  Eye,
  Trash2,
  Settings,
  Square,
  CheckSquare,
  X
} from 'lucide-react';
import { workflowsApi } from '../../api/station';
import type { WorkflowDefinition, WorkflowRun } from '../../types/station';

type TabType = 'definitions' | 'runs';
type StatusFilter = '' | 'pending' | 'running' | 'completed' | 'failed' | 'cancelled' | 'paused' | 'waiting_approval';
type DeleteMode = 'selected' | 'filtered' | 'all' | null;

const statusColors: Record<string, { bg: string; text: string; icon: React.ReactNode }> = {
  pending: { bg: 'bg-gray-100', text: 'text-gray-700', icon: <Clock className="h-3 w-3" /> },
  running: { bg: 'bg-blue-100', text: 'text-blue-700', icon: <Play className="h-3 w-3" /> },
  completed: { bg: 'bg-green-100', text: 'text-green-700', icon: <CheckCircle className="h-3 w-3" /> },
  failed: { bg: 'bg-red-100', text: 'text-red-700', icon: <AlertTriangle className="h-3 w-3" /> },
  cancelled: { bg: 'bg-gray-100', text: 'text-gray-500', icon: <XCircle className="h-3 w-3" /> },
  paused: { bg: 'bg-yellow-100', text: 'text-yellow-700', icon: <Pause className="h-3 w-3" /> },
  waiting_approval: { bg: 'bg-purple-100', text: 'text-purple-700', icon: <Clock className="h-3 w-3" /> },
};

export const WorkflowsPage: React.FC = () => {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<TabType>('definitions');
  const [workflows, setWorkflows] = useState<WorkflowDefinition[]>([]);
  const [runs, setRuns] = useState<WorkflowRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('');
  const [selectedWorkflowFilter, setSelectedWorkflowFilter] = useState<string>('');
  const [selectedRunIds, setSelectedRunIds] = useState<Set<string>>(new Set());
  const [deleteMode, setDeleteMode] = useState<DeleteMode>(null);
  const [deleting, setDeleting] = useState(false);

  const fetchWorkflows = useCallback(async () => {
    try {
      const response = await workflowsApi.getAll();
      setWorkflows(response.data.workflows || []);
    } catch (error) {
      console.error('Failed to fetch workflows:', error);
      setWorkflows([]);
    }
  }, []);

  const fetchRuns = useCallback(async () => {
    try {
      const params: { workflow_id?: string; status?: string; limit?: number } = { limit: 100 };
      if (selectedWorkflowFilter) params.workflow_id = selectedWorkflowFilter;
      if (statusFilter) params.status = statusFilter;
      
      const response = await workflowsApi.listRuns(params);
      setRuns(response.data.runs || []);
    } catch (error) {
      console.error('Failed to fetch workflow runs:', error);
      setRuns([]);
    }
  }, [selectedWorkflowFilter, statusFilter]);

  useEffect(() => {
    const loadData = async () => {
      setLoading(true);
      await Promise.all([fetchWorkflows(), fetchRuns()]);
      setLoading(false);
    };
    loadData();
  }, [fetchWorkflows, fetchRuns]);

  const handleRefresh = async () => {
    setLoading(true);
    await Promise.all([fetchWorkflows(), fetchRuns()]);
    setLoading(false);
  };

  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    return date.toLocaleString();
  };

  const formatDuration = (startedAt: string, completedAt?: string) => {
    const start = new Date(startedAt);
    const end = completedAt ? new Date(completedAt) : new Date();
    const seconds = Math.floor((end.getTime() - start.getTime()) / 1000);
    
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
  };

  const toggleRunSelection = (runId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    setSelectedRunIds(prev => {
      const next = new Set(prev);
      if (next.has(runId)) {
        next.delete(runId);
      } else {
        next.add(runId);
      }
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (selectedRunIds.size === runs.length) {
      setSelectedRunIds(new Set());
    } else {
      setSelectedRunIds(new Set(runs.map(r => r.run_id)));
    }
  };

  const handleDeleteRuns = async () => {
    if (!deleteMode) return;
    
    setDeleting(true);
    try {
      let params: { runIds?: string[]; status?: string; workflowId?: string; all?: boolean } = {};
      
      if (deleteMode === 'selected') {
        params.runIds = Array.from(selectedRunIds);
      } else if (deleteMode === 'filtered') {
        if (statusFilter) params.status = statusFilter;
        if (selectedWorkflowFilter) params.workflowId = selectedWorkflowFilter;
        if (!statusFilter && !selectedWorkflowFilter) params.all = true;
      } else if (deleteMode === 'all') {
        params.all = true;
      }
      
      const response = await workflowsApi.deleteRuns(params);
      console.log(`Deleted ${response.data.deleted} runs`);
      
      setSelectedRunIds(new Set());
      setDeleteMode(null);
      await fetchRuns();
    } catch (error) {
      console.error('Failed to delete runs:', error);
    } finally {
      setDeleting(false);
    }
  };

  const getDeleteConfirmMessage = (): string => {
    if (deleteMode === 'selected') {
      return `Delete ${selectedRunIds.size} selected run${selectedRunIds.size !== 1 ? 's' : ''}?`;
    } else if (deleteMode === 'filtered') {
      const filters: string[] = [];
      if (statusFilter) filters.push(`status: ${statusFilter}`);
      if (selectedWorkflowFilter) filters.push(`workflow: ${selectedWorkflowFilter}`);
      return filters.length > 0
        ? `Delete all ${runs.length} runs matching ${filters.join(', ')}?`
        : `Delete all ${runs.length} runs?`;
    } else if (deleteMode === 'all') {
      return `Delete ALL workflow runs? This cannot be undone.`;
    }
    return '';
  };

  const renderDefinitionsTab = () => (
    <div className="p-4">
      {workflows.length === 0 ? (
        <div className="text-center py-12 max-w-lg mx-auto">
          <GitBranch className="h-12 w-12 text-gray-300 mx-auto mb-3" />
          <h3 className="text-lg font-medium text-gray-900 mb-2">No workflows defined</h3>
          <p className="text-gray-500 text-sm mb-6">
            Workflows are created via MCP tools or YAML files, allowing AI agents to define and manage complex orchestration patterns.
          </p>
          <div className="bg-gray-50 rounded-lg p-4 text-left space-y-3">
            <div>
              <h4 className="text-sm font-medium text-gray-700 mb-1">Via MCP Tools (recommended)</h4>
              <p className="text-xs text-gray-500">
                Use <code className="bg-gray-100 px-1 py-0.5 rounded">station_create_workflow</code> to let AI agents create workflows programmatically.
              </p>
            </div>
            <div>
              <h4 className="text-sm font-medium text-gray-700 mb-1">Via YAML Files</h4>
              <p className="text-xs text-gray-500">
                Create <code className="bg-gray-100 px-1 py-0.5 rounded">.workflow.yaml</code> files in your environment's workflows directory, then run <code className="bg-gray-100 px-1 py-0.5 rounded">stn sync</code>.
              </p>
            </div>
          </div>
        </div>
      ) : (
        <div className="grid gap-4">
          {workflows.map((workflow) => (
            <div
              key={workflow.id}
              onClick={() => navigate(`/workflows/${workflow.workflow_id}`)}
              className="bg-white border border-gray-200 rounded-lg p-4 hover:border-blue-300 hover:shadow-md transition-all cursor-pointer group"
            >
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div className="p-2 bg-blue-50 rounded-lg">
                    <GitBranch className="h-5 w-5 text-blue-600" />
                  </div>
                  <div>
                    <h3 className="font-medium text-gray-900 group-hover:text-blue-600 transition-colors">
                      {workflow.name}
                    </h3>
                    <p className="text-sm text-gray-500">{workflow.workflow_id}</p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-xs text-gray-400">v{workflow.version}</span>
                  <span className={`px-2 py-0.5 text-xs rounded-full ${
                    workflow.status === 'active' 
                      ? 'bg-green-100 text-green-700' 
                      : 'bg-gray-100 text-gray-500'
                  }`}>
                    {workflow.status === 'active' ? 'Active' : 'Disabled'}
                  </span>
                  <ChevronRight className="h-4 w-4 text-gray-400 group-hover:text-blue-500 transition-colors" />
                </div>
              </div>
              {workflow.description && (
                <p className="mt-2 text-sm text-gray-600 line-clamp-2">{workflow.description}</p>
              )}
              <div className="mt-3 flex items-center gap-4 text-xs text-gray-400">
                <span>Updated {formatDate(workflow.updated_at)}</span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );

  const renderRunsTab = () => (
    <div className="p-4">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
            <Filter className="h-4 w-4 text-gray-500" />
            <select
              value={selectedWorkflowFilter}
              onChange={(e) => setSelectedWorkflowFilter(e.target.value)}
              className="text-sm border border-gray-300 rounded-md px-3 py-1.5 bg-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="">All Workflows</option>
              {workflows.map((w) => (
                <option key={w.workflow_id} value={w.workflow_id}>{w.name}</option>
              ))}
            </select>
          </div>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as StatusFilter)}
            className="text-sm border border-gray-300 rounded-md px-3 py-1.5 bg-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="">All Statuses</option>
            <option value="pending">Pending</option>
            <option value="running">Running</option>
            <option value="completed">Completed</option>
            <option value="failed">Failed</option>
            <option value="cancelled">Cancelled</option>
            <option value="paused">Paused</option>
            <option value="waiting_approval">Waiting Approval</option>
          </select>
          <span className="text-sm text-gray-500">{runs.length} runs</span>
          {selectedRunIds.size > 0 && (
            <span className="text-sm text-blue-600 font-medium">
              {selectedRunIds.size} selected
            </span>
          )}
        </div>

        {runs.length > 0 && (
          <div className="flex items-center gap-2">
            {selectedRunIds.size > 0 && (
              <button
                onClick={() => setDeleteMode('selected')}
                className="flex items-center gap-1.5 px-3 py-1.5 text-sm text-red-600 hover:text-red-700 hover:bg-red-50 rounded-md transition-colors"
              >
                <Trash2 className="h-4 w-4" />
                Delete Selected
              </button>
            )}
            <button
              onClick={() => setDeleteMode('filtered')}
              className="flex items-center gap-1.5 px-3 py-1.5 text-sm text-gray-600 hover:text-gray-800 hover:bg-gray-100 rounded-md transition-colors"
              title={statusFilter || selectedWorkflowFilter ? 'Delete runs matching current filters' : 'Delete all runs'}
            >
              <Trash2 className="h-4 w-4" />
              {statusFilter || selectedWorkflowFilter ? 'Delete Filtered' : 'Delete All'}
            </button>
          </div>
        )}
      </div>

      {runs.length === 0 ? (
        <div className="text-center py-12">
          <Play className="h-12 w-12 text-gray-300 mx-auto mb-3" />
          <h3 className="text-lg font-medium text-gray-900 mb-1">No workflow runs</h3>
          <p className="text-gray-500 text-sm">
            Start a workflow to see execution history here
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          <div className="flex items-center gap-2 px-1">
            <button
              onClick={toggleSelectAll}
              className="p-1 text-gray-400 hover:text-gray-600 transition-colors"
              title={selectedRunIds.size === runs.length ? 'Deselect all' : 'Select all'}
            >
              {selectedRunIds.size === runs.length ? (
                <CheckSquare className="h-5 w-5 text-blue-600" />
              ) : selectedRunIds.size > 0 ? (
                <CheckSquare className="h-5 w-5 text-blue-400" />
              ) : (
                <Square className="h-5 w-5" />
              )}
            </button>
            <span className="text-xs text-gray-500">
              {selectedRunIds.size === runs.length ? 'Deselect all' : 'Select all'}
            </span>
          </div>
          {runs.map((run) => {
            const status = statusColors[run.status] || statusColors.pending;
            const workflow = workflows.find(w => w.workflow_id === run.workflow_id);
            const isSelected = selectedRunIds.has(run.run_id);
            
            return (
              <div
                key={run.run_id}
                onClick={() => navigate(`/workflows/${run.workflow_id}?run=${run.run_id}`)}
                className={`bg-white border rounded-lg p-4 hover:border-blue-300 hover:shadow-md transition-all cursor-pointer ${
                  isSelected ? 'border-blue-400 bg-blue-50/30' : 'border-gray-200'
                }`}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <button
                      onClick={(e) => toggleRunSelection(run.run_id, e)}
                      className="p-1 text-gray-400 hover:text-gray-600 transition-colors"
                    >
                      {isSelected ? (
                        <CheckSquare className="h-5 w-5 text-blue-600" />
                      ) : (
                        <Square className="h-5 w-5" />
                      )}
                    </button>
                    <div className={`p-2 rounded-lg ${status.bg}`}>
                      {status.icon}
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-gray-900">
                          {workflow?.name || run.workflow_id}
                        </span>
                        <span className={`px-2 py-0.5 text-xs rounded-full ${status.bg} ${status.text}`}>
                          {run.status.replace('_', ' ')}
                        </span>
                      </div>
                      <p className="text-sm text-gray-500 font-mono">{run.run_id}</p>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-sm text-gray-600">
                      {run.completed_at 
                        ? formatDuration(run.started_at, run.completed_at)
                        : run.status === 'running' 
                          ? formatDuration(run.started_at)
                          : '-'
                      }
                    </div>
                    <div className="text-xs text-gray-400">
                      {formatDate(run.started_at)}
                    </div>
                  </div>
                </div>
                {run.current_state && (
                  <div className="mt-2 text-sm text-gray-500 ml-12">
                    Current state: <span className="font-mono text-gray-700">{run.current_state}</span>
                  </div>
                )}
                {run.error && (
                  <div className="mt-2 text-sm text-red-600 bg-red-50 rounded p-2 ml-12">
                    {run.error}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );

  return (
    <div className="h-full flex flex-col bg-gray-50">
      <div className="bg-white border-b border-gray-200 px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <h1 className="text-xl font-semibold text-gray-900">Workflows</h1>
            
            <div className="flex bg-gray-100 rounded-lg p-1">
              <button
                onClick={() => setActiveTab('definitions')}
                className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
                  activeTab === 'definitions'
                    ? 'bg-white text-blue-600 shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                Definitions ({workflows.length})
              </button>
              <button
                onClick={() => setActiveTab('runs')}
                className={`px-4 py-1.5 rounded-md text-sm font-medium transition-colors ${
                  activeTab === 'runs'
                    ? 'bg-white text-blue-600 shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                Runs ({runs.length})
              </button>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={handleRefresh}
              disabled={loading}
              className="p-2 text-gray-600 hover:text-gray-900 hover:bg-gray-100 rounded-lg transition-colors disabled:opacity-50"
              title="Refresh"
            >
              <RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
            </button>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center h-64">
            <div className="flex items-center gap-2 text-gray-500">
              <RefreshCw className="h-5 w-5 animate-spin" />
              <span>Loading workflows...</span>
            </div>
          </div>
        ) : (
          activeTab === 'definitions' ? renderDefinitionsTab() : renderRunsTab()
        )}
      </div>

      {deleteMode && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-xl max-w-md w-full mx-4 overflow-hidden">
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200">
              <h3 className="text-lg font-semibold text-gray-900">Confirm Delete</h3>
              <button
                onClick={() => setDeleteMode(null)}
                className="p-1 text-gray-400 hover:text-gray-600 transition-colors"
              >
                <X className="h-5 w-5" />
              </button>
            </div>
            <div className="px-6 py-4">
              <div className="flex items-start gap-3">
                <div className="p-2 bg-red-100 rounded-full">
                  <Trash2 className="h-5 w-5 text-red-600" />
                </div>
                <div>
                  <p className="text-gray-900 font-medium">{getDeleteConfirmMessage()}</p>
                  <p className="text-sm text-gray-500 mt-1">
                    This action cannot be undone. All associated step data will also be deleted.
                  </p>
                </div>
              </div>
            </div>
            <div className="flex items-center justify-end gap-3 px-6 py-4 bg-gray-50">
              <button
                onClick={() => setDeleteMode(null)}
                disabled={deleting}
                className="px-4 py-2 text-sm font-medium text-gray-700 hover:text-gray-900 transition-colors disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                onClick={handleDeleteRuns}
                disabled={deleting}
                className="px-4 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-700 rounded-md transition-colors disabled:opacity-50 flex items-center gap-2"
              >
                {deleting ? (
                  <>
                    <RefreshCw className="h-4 w-4 animate-spin" />
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
      )}
    </div>
  );
};

export default WorkflowsPage;

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
  Settings
} from 'lucide-react';
import { workflowsApi } from '../../api/station';
import type { WorkflowDefinition, WorkflowRun } from '../../types/station';

type TabType = 'definitions' | 'runs';
type StatusFilter = '' | 'pending' | 'running' | 'completed' | 'failed' | 'cancelled' | 'paused' | 'waiting_approval';

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

  const renderDefinitionsTab = () => (
    <div className="p-4">
      {workflows.length === 0 ? (
        <div className="text-center py-12">
          <GitBranch className="h-12 w-12 text-gray-300 mx-auto mb-3" />
          <h3 className="text-lg font-medium text-gray-900 mb-1">No workflows defined</h3>
          <p className="text-gray-500 text-sm mb-4">
            Create workflow definitions using YAML/JSON files or the MCP tools
          </p>
          <button className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors">
            <Plus className="h-4 w-4" />
            Create Workflow
          </button>
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
      <div className="flex items-center gap-4 mb-4">
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
          {runs.map((run) => {
            const status = statusColors[run.status] || statusColors.pending;
            const workflow = workflows.find(w => w.workflow_id === run.workflow_id);
            
            return (
              <div
                key={run.run_id}
                onClick={() => navigate(`/workflows/${run.workflow_id}?run=${run.run_id}`)}
                className="bg-white border border-gray-200 rounded-lg p-4 hover:border-blue-300 hover:shadow-md transition-all cursor-pointer"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
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
                  <div className="mt-2 text-sm text-gray-500">
                    Current state: <span className="font-mono text-gray-700">{run.current_state}</span>
                  </div>
                )}
                {run.error && (
                  <div className="mt-2 text-sm text-red-600 bg-red-50 rounded p-2">
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
    </div>
  );
};

export default WorkflowsPage;

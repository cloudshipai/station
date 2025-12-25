import React, { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate, useSearchParams } from 'react-router-dom';
import Editor from '@monaco-editor/react';
import { WorkflowFlowPanel } from './WorkflowFlowPanel';
import { 
  GitBranch, 
  Play, 
  Pause, 
  XCircle, 
  Clock, 
  CheckCircle, 
  AlertTriangle,
  ArrowLeft,
  RefreshCw,
  Code,
  List,
  History,
  Settings,
  ChevronRight,
  CheckCircle2,
  X
} from 'lucide-react';
import { workflowsApi } from '../../api/station';
import type { WorkflowDefinition, WorkflowRun, WorkflowStep, WorkflowApproval } from '../../types/station';

type TabType = 'overview' | 'runs' | 'definition' | 'versions';

const statusColors: Record<string, { bg: string; text: string; border: string }> = {
  pending: { bg: 'bg-gray-100', text: 'text-gray-700', border: 'border-gray-300' },
  running: { bg: 'bg-blue-100', text: 'text-blue-700', border: 'border-blue-300' },
  completed: { bg: 'bg-green-100', text: 'text-green-700', border: 'border-green-300' },
  failed: { bg: 'bg-red-100', text: 'text-red-700', border: 'border-red-300' },
  cancelled: { bg: 'bg-gray-100', text: 'text-gray-500', border: 'border-gray-300' },
  paused: { bg: 'bg-yellow-100', text: 'text-yellow-700', border: 'border-yellow-300' },
  waiting_approval: { bg: 'bg-purple-100', text: 'text-purple-700', border: 'border-purple-300' },
};

const stepStatusIcons: Record<string, React.ReactNode> = {
  pending: <Clock className="h-4 w-4 text-gray-400" />,
  running: <RefreshCw className="h-4 w-4 text-blue-500 animate-spin" />,
  completed: <CheckCircle className="h-4 w-4 text-green-500" />,
  failed: <XCircle className="h-4 w-4 text-red-500" />,
  skipped: <X className="h-4 w-4 text-gray-400" />,
  waiting_approval: <Clock className="h-4 w-4 text-purple-500" />,
};

export const WorkflowDetailPage: React.FC = () => {
  const { workflowId } = useParams<{ workflowId: string }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  
  const [workflow, setWorkflow] = useState<WorkflowDefinition | null>(null);
  const [runs, setRuns] = useState<WorkflowRun[]>([]);
  const [selectedRun, setSelectedRun] = useState<WorkflowRun | null>(null);
  const [runSteps, setRunSteps] = useState<WorkflowStep[]>([]);
  const [approvals, setApprovals] = useState<WorkflowApproval[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<TabType>('overview');
  const [startingRun, setStartingRun] = useState(false);
  const [showStartModal, setShowStartModal] = useState(false);
  const [runInput, setRunInput] = useState('{}');
  const [inputError, setInputError] = useState<string | null>(null);

  const fetchWorkflow = useCallback(async () => {
    if (!workflowId) return;
    try {
      const response = await workflowsApi.getById(workflowId);
      setWorkflow(response.data.workflow);
    } catch (error) {
      console.error('Failed to fetch workflow:', error);
    }
  }, [workflowId]);

  const fetchRuns = useCallback(async () => {
    if (!workflowId) return;
    try {
      const response = await workflowsApi.listRuns({ workflow_id: workflowId, limit: 50 });
      setRuns(response.data.runs || []);
    } catch (error) {
      console.error('Failed to fetch runs:', error);
    }
  }, [workflowId]);

  const fetchRunDetails = useCallback(async (runId: string) => {
    try {
      const [runResponse, stepsResponse, approvalsResponse] = await Promise.all([
        workflowsApi.getRun(runId),
        workflowsApi.getSteps(runId),
        workflowsApi.listApprovals({ run_id: runId }),
      ]);
      setSelectedRun(runResponse.data.run);
      setRunSteps(stepsResponse.data.steps || []);
      setApprovals(approvalsResponse.data.approvals || []);
    } catch (error) {
      console.error('Failed to fetch run details:', error);
    }
  }, []);

  useEffect(() => {
    const loadData = async () => {
      setLoading(true);
      await Promise.all([fetchWorkflow(), fetchRuns()]);
      setLoading(false);
    };
    loadData();
  }, [fetchWorkflow, fetchRuns]);

  useEffect(() => {
    const runId = searchParams.get('run');
    if (runId) {
      fetchRunDetails(runId);
      setActiveTab('runs');
    }
  }, [searchParams, fetchRunDetails]);

  useEffect(() => {
    if (!selectedRun || !['pending', 'running', 'waiting_approval'].includes(selectedRun.status)) {
      return;
    }

    const isDev = import.meta.env.DEV;
    const baseUrl = isDev ? '/api/v1' : 'http://localhost:8585/api/v1';
    const eventSource = new EventSource(`${baseUrl}/workflow-runs/${selectedRun.run_id}/stream`);

    eventSource.onopen = () => {
      console.log('SSE connection opened for run:', selectedRun.run_id);
    };

    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        
        if (data.type === 'run_update' && data.run) {
          setSelectedRun(data.run);
          setRuns(prev => prev.map(r => r.run_id === data.run.run_id ? data.run : r));
        } else if (data.type === 'step_update' && data.step) {
          setRunSteps(prev => {
            const index = prev.findIndex(s => s.id === data.step.id);
            if (index >= 0) {
              const newSteps = [...prev];
              newSteps[index] = data.step;
              return newSteps;
            }
            return [...prev, data.step];
          });
        }
      } catch (e) {
        console.error('Failed to parse SSE message:', e);
      }
    };

    eventSource.onerror = (err) => {
      console.error('SSE error:', err);
      eventSource.close();
    };

    return () => {
      eventSource.close();
    };
  }, [selectedRun?.run_id, selectedRun?.status]);

  const handleStartRunClick = () => {
    setRunInput('{}');
    setInputError(null);
    setShowStartModal(true);
  };

  const handleStartRunSubmit = async () => {
    if (!workflowId) return;
    
    let parsedInput = {};
    try {
      parsedInput = JSON.parse(runInput);
    } catch (e) {
      setInputError('Invalid JSON input');
      return;
    }

    setStartingRun(true);
    setInputError(null);
    
    try {
      const response = await workflowsApi.startRun(workflowId, parsedInput);
      await fetchRuns();
      if (response.data.run) {
        setSelectedRun(response.data.run);
        setActiveTab('runs');
        setShowStartModal(false);
      }
    } catch (error) {
      console.error('Failed to start run:', error);
      setInputError('Failed to start run. Please check the logs.');
    } finally {
      setStartingRun(false);
    }
  };

  const handleApprove = async (approvalId: string) => {
    try {
      await workflowsApi.approve(approvalId, 'Approved via UI');
      if (selectedRun) {
        await fetchRunDetails(selectedRun.run_id);
      }
    } catch (error) {
      console.error('Failed to approve:', error);
    }
  };

  const handleReject = async (approvalId: string) => {
    try {
      await workflowsApi.reject(approvalId, 'Rejected via UI');
      if (selectedRun) {
        await fetchRunDetails(selectedRun.run_id);
      }
    } catch (error) {
      console.error('Failed to reject:', error);
    }
  };

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleString();
  };

  const formatDuration = (startedAt: string, completedAt?: string) => {
    const start = new Date(startedAt);
    const end = completedAt ? new Date(completedAt) : new Date();
    const seconds = Math.floor((end.getTime() - start.getTime()) / 1000);
    
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
    return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
  };

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center bg-gray-50">
        <div className="flex items-center gap-2 text-gray-500">
          <RefreshCw className="h-5 w-5 animate-spin" />
          <span>Loading workflow...</span>
        </div>
      </div>
    );
  }

  if (!workflow) {
    return (
      <div className="h-full flex items-center justify-center bg-gray-50">
        <div className="text-center">
          <AlertTriangle className="h-12 w-12 text-gray-300 mx-auto mb-3" />
          <h3 className="text-lg font-medium text-gray-900">Workflow not found</h3>
          <button
            onClick={() => navigate('/workflows')}
            className="mt-4 text-blue-600 hover:text-blue-700"
          >
            Back to workflows
          </button>
        </div>
      </div>
    );
  }

  const renderOverviewTab = () => (
    <div className="p-6 space-y-6">
      <div className="bg-white rounded-lg border border-gray-200 p-6">
        <h3 className="text-lg font-medium text-gray-900 mb-4">Workflow Flow</h3>
        <WorkflowFlowPanel workflow={workflow} isVisible={true} />
      </div>

      <div className="bg-white rounded-lg border border-gray-200 p-6">
        <h3 className="text-lg font-medium text-gray-900 mb-4">Workflow Info</h3>
        <dl className="grid grid-cols-2 gap-4">
          <div>
            <dt className="text-sm text-gray-500">Workflow ID</dt>
            <dd className="font-mono text-gray-900">{workflow.workflow_id}</dd>
          </div>
          <div>
            <dt className="text-sm text-gray-500">Version</dt>
            <dd className="text-gray-900">v{workflow.version}</dd>
          </div>
          <div>
            <dt className="text-sm text-gray-500">Status</dt>
            <dd>
              <span className={`px-2 py-0.5 text-xs rounded-full ${
                workflow.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'
              }`}>
                {workflow.status === 'active' ? 'Active' : 'Disabled'}
              </span>
            </dd>
          </div>
          <div>
            <dt className="text-sm text-gray-500">Last Updated</dt>
            <dd className="text-gray-900">{formatDate(workflow.updated_at)}</dd>
          </div>
          {workflow.description && (
            <div className="col-span-2">
              <dt className="text-sm text-gray-500">Description</dt>
              <dd className="text-gray-900">{workflow.description}</dd>
            </div>
          )}
        </dl>
      </div>

      <div className="bg-white rounded-lg border border-gray-200 p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-medium text-gray-900">Recent Runs</h3>
          <button
            onClick={() => setActiveTab('runs')}
            className="text-sm text-blue-600 hover:text-blue-700"
          >
            View all
          </button>
        </div>
        {runs.length === 0 ? (
          <p className="text-gray-500 text-sm">No runs yet</p>
        ) : (
          <div className="space-y-2">
            {runs.slice(0, 5).map((run) => {
              const status = statusColors[run.status] || statusColors.pending;
              return (
                <div
                  key={run.run_id}
                  onClick={() => {
                    setSelectedRun(run);
                    fetchRunDetails(run.run_id);
                    setActiveTab('runs');
                  }}
                  className="flex items-center justify-between p-3 bg-gray-50 rounded-lg hover:bg-gray-100 cursor-pointer"
                >
                  <div className="flex items-center gap-3">
                    <span className={`px-2 py-0.5 text-xs rounded-full ${status.bg} ${status.text}`}>
                      {run.status.replace('_', ' ')}
                    </span>
                    <span className="font-mono text-sm text-gray-600">{run.run_id.slice(0, 8)}...</span>
                  </div>
                  <span className="text-sm text-gray-500">{formatDate(run.started_at)}</span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );

  const renderRunsTab = () => (
    <div className="flex h-full">
      <div className="w-80 border-r border-gray-200 bg-white overflow-y-auto">
        <div className="p-4 border-b border-gray-200">
          <h3 className="font-medium text-gray-900">Run History</h3>
          <p className="text-sm text-gray-500">{runs.length} runs</p>
        </div>
        <div className="divide-y divide-gray-100">
          {runs.map((run) => {
            const status = statusColors[run.status] || statusColors.pending;
            const isSelected = selectedRun?.run_id === run.run_id;
            return (
              <div
                key={run.run_id}
                onClick={() => {
                  setSelectedRun(run);
                  fetchRunDetails(run.run_id);
                }}
                className={`p-4 cursor-pointer hover:bg-gray-50 ${isSelected ? 'bg-blue-50 border-l-2 border-blue-500' : ''}`}
              >
                <div className="flex items-center justify-between mb-1">
                  <span className={`px-2 py-0.5 text-xs rounded-full ${status.bg} ${status.text}`}>
                    {run.status.replace('_', ' ')}
                  </span>
                  <span className="text-xs text-gray-400">
                    {run.completed_at ? formatDuration(run.started_at, run.completed_at) : '-'}
                  </span>
                </div>
                <p className="font-mono text-sm text-gray-700 truncate">{run.run_id}</p>
                <p className="text-xs text-gray-500 mt-1">{formatDate(run.started_at)}</p>
              </div>
            );
          })}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto bg-gray-50">
        {selectedRun ? (
          <div className="p-6 space-y-6">
            <div className="bg-white rounded-lg border border-gray-200 p-6">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-lg font-medium text-gray-900">Run Details</h3>
                <span className={`px-3 py-1 text-sm rounded-full ${statusColors[selectedRun.status]?.bg} ${statusColors[selectedRun.status]?.text}`}>
                  {selectedRun.status.replace('_', ' ')}
                </span>
              </div>
              <dl className="grid grid-cols-2 gap-4">
                <div>
                  <dt className="text-sm text-gray-500">Run ID</dt>
                  <dd className="font-mono text-gray-900 text-sm">{selectedRun.run_id}</dd>
                </div>
                <div>
                  <dt className="text-sm text-gray-500">Duration</dt>
                  <dd className="text-gray-900">
                    {selectedRun.completed_at 
                      ? formatDuration(selectedRun.started_at, selectedRun.completed_at)
                      : selectedRun.status === 'running' 
                        ? formatDuration(selectedRun.started_at)
                        : '-'
                    }
                  </dd>
                </div>
                <div>
                  <dt className="text-sm text-gray-500">Started</dt>
                  <dd className="text-gray-900">{formatDate(selectedRun.started_at)}</dd>
                </div>
                {selectedRun.completed_at && (
                  <div>
                    <dt className="text-sm text-gray-500">Completed</dt>
                    <dd className="text-gray-900">{formatDate(selectedRun.completed_at)}</dd>
                  </div>
                )}
                {selectedRun.current_state && (
                  <div className="col-span-2">
                    <dt className="text-sm text-gray-500">Current State</dt>
                    <dd className="font-mono text-gray-900">{selectedRun.current_state}</dd>
                  </div>
                )}
                {selectedRun.error && (
                  <div className="col-span-2">
                    <dt className="text-sm text-gray-500">Error</dt>
                    <dd className="text-red-600 bg-red-50 p-2 rounded text-sm">{selectedRun.error}</dd>
                  </div>
                )}
              </dl>
            </div>

            {approvals.filter(a => a.status === 'pending').length > 0 && (
              <div className="bg-white rounded-lg border border-purple-200 p-6">
                <h3 className="text-lg font-medium text-purple-900 mb-4 flex items-center gap-2">
                  <Clock className="h-5 w-5" />
                  Pending Approvals
                </h3>
                <div className="space-y-3">
                  {approvals.filter(a => a.status === 'pending').map((approval) => (
                    <div key={approval.approval_id} className="flex items-center justify-between p-4 bg-purple-50 rounded-lg">
                      <div>
                        <p className="font-medium text-gray-900">{approval.state_name}</p>
                        <p className="text-sm text-gray-500 font-mono">{approval.approval_id}</p>
                      </div>
                      <div className="flex gap-2">
                        <button
                          onClick={() => handleApprove(approval.approval_id)}
                          className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 flex items-center gap-2"
                        >
                          <CheckCircle2 className="h-4 w-4" />
                          Approve
                        </button>
                        <button
                          onClick={() => handleReject(approval.approval_id)}
                          className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 flex items-center gap-2"
                        >
                          <X className="h-4 w-4" />
                          Reject
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            <div className="bg-white rounded-lg border border-gray-200 p-6">
              <h3 className="text-lg font-medium text-gray-900 mb-4">Execution Steps</h3>
              {runSteps.length === 0 ? (
                <p className="text-gray-500">No steps recorded yet</p>
              ) : (
                <div className="space-y-2">
                  {runSteps.map((step, index) => (
                    <div
                      key={step.id}
                      className={`flex items-start gap-3 p-3 rounded-lg border ${statusColors[step.status]?.border || 'border-gray-200'} bg-white`}
                    >
                      <div className="mt-0.5">
                        {stepStatusIcons[step.status] || stepStatusIcons.pending}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="font-medium text-gray-900">{step.step_id}</span>
                          <span className="text-xs text-gray-500 px-2 py-0.5 bg-gray-100 rounded">
                            {step.metadata?.step_type || 'unknown'}
                          </span>
                          {step.output?.agent_name && (
                            <span className="text-xs text-blue-600 px-2 py-0.5 bg-blue-50 rounded font-medium">
                              {step.output.agent_name}
                            </span>
                          )}
                          {step.output?.agent_id && (
                            <a
                              href={`/runs?agent_id=${step.output.agent_id}`}
                              onClick={(e) => e.stopPropagation()}
                              className="text-xs text-blue-500 hover:text-blue-700 hover:underline"
                            >
                              View agent run →
                            </a>
                          )}
                        </div>
                        {step.started_at && (
                          <p className="text-sm text-gray-500 mt-1">
                            {step.completed_at 
                              ? `Completed in ${formatDuration(step.started_at, step.completed_at)}`
                              : `Started ${formatDate(step.started_at)}`
                            }
                          </p>
                        )}
                        {step.error && (
                          <p className="text-sm text-red-600 mt-1">{step.error}</p>
                        )}
                        {step.output && (
                          <details className="mt-2">
                            <summary className="text-xs text-gray-500 cursor-pointer hover:text-gray-700">
                              View output
                            </summary>
                            <pre className="mt-1 text-xs bg-gray-50 p-2 rounded overflow-x-auto max-h-32">
                              {JSON.stringify(step.output, null, 2)}
                            </pre>
                          </details>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        ) : (
          <div className="h-full flex items-center justify-center">
            <div className="text-center text-gray-500">
              <List className="h-12 w-12 mx-auto mb-3 text-gray-300" />
              <p>Select a run to view details</p>
            </div>
          </div>
        )}
      </div>
    </div>
  );

  const extractAgentsFromWorkflow = (definition: any): string[] => {
    const agents = new Set<string>();
    
    const extractFromState = (state: any) => {
      if (state.type === 'agent' && state.agent) {
        agents.add(state.agent);
      }
      if (state.type === 'operation' && state.input?.agent) {
        agents.add(state.input.agent);
      }
      if (state.type === 'foreach' && state.agent) {
        agents.add(state.agent);
      }
      if (state.type === 'parallel' && state.branches) {
        state.branches.forEach((branch: any) => {
          if (branch.agent) agents.add(branch.agent);
          if (branch.states) branch.states.forEach(extractFromState);
        });
      }
    };
    
    if (definition?.states) {
      definition.states.forEach(extractFromState);
    }
    
    return Array.from(agents).sort();
  };

  const renderDefinitionTab = () => {
    const agents = extractAgentsFromWorkflow(workflow.definition);
    
    return (
      <div className="p-6 space-y-4">
        {agents.length > 0 && (
          <div className="bg-white rounded-lg border border-gray-200 p-4">
            <h3 className="font-medium text-gray-900 mb-3">Agents Used ({agents.length})</h3>
            <div className="flex flex-wrap gap-2">
              {agents.map((agent) => (
                <span
                  key={agent}
                  className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-emerald-50 text-emerald-700 border border-emerald-200"
                >
                  {agent}
                </span>
              ))}
            </div>
          </div>
        )}
        
        <div className="bg-white rounded-lg border border-gray-200 overflow-hidden">
          <div className="p-4 border-b border-gray-200 bg-gray-50">
            <h3 className="font-medium text-gray-900">Workflow Definition (v{workflow.version})</h3>
          </div>
          <pre className="p-4 bg-gray-900 text-gray-100 overflow-auto text-sm max-h-[600px]">
            <code>{JSON.stringify(workflow.definition, null, 2)}</code>
          </pre>
        </div>
      </div>
    );
  };

  return (
    <div className="h-full flex flex-col bg-gray-50">
      <div className="bg-white border-b border-gray-200 px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <button
              onClick={() => navigate('/workflows')}
              className="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
            >
              <ArrowLeft className="h-5 w-5" />
            </button>
            <div>
              <h1 className="text-xl font-semibold text-gray-900">{workflow.name}</h1>
              <p className="text-sm text-gray-500">{workflow.workflow_id}</p>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <div className="flex bg-gray-100 rounded-lg p-1">
              <button
                onClick={() => setActiveTab('overview')}
                className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-1.5 ${
                  activeTab === 'overview' ? 'bg-white text-blue-600 shadow-sm' : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                <GitBranch className="h-4 w-4" />
                Overview
              </button>
              <button
                onClick={() => setActiveTab('runs')}
                className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-1.5 ${
                  activeTab === 'runs' ? 'bg-white text-blue-600 shadow-sm' : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                <History className="h-4 w-4" />
                Runs
              </button>
              <button
                onClick={() => setActiveTab('definition')}
                className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors flex items-center gap-1.5 ${
                  activeTab === 'definition' ? 'bg-white text-blue-600 shadow-sm' : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                <Code className="h-4 w-4" />
                Definition
              </button>
            </div>

            <button
              onClick={handleStartRunClick}
              disabled={startingRun || workflow.status !== 'active'}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
            >
              {startingRun ? (
                <RefreshCw className="h-4 w-4 animate-spin" />
              ) : (
                <Play className="h-4 w-4" />
              )}
              Start Run
            </button>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-auto">
        {activeTab === 'overview' && renderOverviewTab()}
        {activeTab === 'runs' && renderRunsTab()}
        {activeTab === 'definition' && renderDefinitionTab()}
      </div>

      {showStartModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
          <div className="bg-white rounded-lg shadow-xl w-[600px] flex flex-col max-h-[90vh]">
            <div className="px-6 py-4 border-b border-gray-200 flex justify-between items-center">
              <h3 className="text-lg font-medium text-gray-900">Start Workflow Run</h3>
              <button onClick={() => setShowStartModal(false)} className="text-gray-400 hover:text-gray-500">
                <X className="h-5 w-5" />
              </button>
            </div>
            
            <div className="p-6 flex-1 overflow-y-auto">
              <div className="mb-4">
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Input JSON
                </label>
                <div className="h-64 border border-gray-300 rounded-md overflow-hidden">
                  <Editor
                    height="100%"
                    defaultLanguage="json"
                    value={runInput}
                    onChange={(value) => setRunInput(value || '{}')}
                    options={{
                      minimap: { enabled: false },
                      scrollBeyondLastLine: false,
                      fontSize: 14,
                      lineNumbers: 'on',
                    }}
                  />
                </div>
                {inputError && (
                  <p className="mt-2 text-sm text-red-600">{inputError}</p>
                )}
              </div>
            </div>

            <div className="px-6 py-4 bg-gray-50 border-t border-gray-200 flex justify-end gap-3 rounded-b-lg">
              <button
                onClick={() => setShowStartModal(false)}
                className="px-4 py-2 bg-white border border-gray-300 rounded-lg text-gray-700 hover:bg-gray-50 font-medium"
              >
                Cancel
              </button>
              <button
                onClick={handleStartRunSubmit}
                disabled={startingRun}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2 font-medium"
              >
                {startingRun && <RefreshCw className="h-4 w-4 animate-spin" />}
                Start Run
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default WorkflowDetailPage;

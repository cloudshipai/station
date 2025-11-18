import React, { useState, useEffect, useContext } from 'react';
import { useNavigate } from 'react-router-dom';
import { FileText, Plus, Download, AlertTriangle, CheckCircle, Clock, XCircle, Loader, Zap } from 'lucide-react';
import { reportsApi } from '../../api/station';
import type { Report } from '../../types/station';
import { CreateReportModal } from '../modals/CreateReportModal';
import { BenchmarkExperimentModal } from '../modals/BenchmarkExperimentModal';

interface ReportsPageProps {
  environmentContext?: any;
}

export const ReportsPage: React.FC<ReportsPageProps> = ({ environmentContext }) => {
  const navigate = useNavigate();
  const [reports, setReports] = useState<Report[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filterStatus, setFilterStatus] = useState<string>('all');
  const [selectedEnvironmentId, setSelectedEnvironmentId] = useState<number | null>(null);
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isBenchmarkModalOpen, setIsBenchmarkModalOpen] = useState(false);
  const [generatingReports, setGeneratingReports] = useState<Set<number>>(new Set());

  // Helper to safely extract values from SQL null types
  const getSqlValue = (field: any): any => {
    if (field === null || field === undefined) return undefined;
    if (typeof field === 'object' && 'Valid' in field && 'Float64' in field) {
      return field.Valid ? field.Float64 : undefined;
    }
    if (typeof field === 'object' && 'Valid' in field && 'Int64' in field) {
      return field.Valid ? field.Int64 : undefined;
    }
    if (typeof field === 'object' && 'Valid' in field && 'String' in field) {
      return field.Valid ? field.String : undefined;
    }
    if (typeof field === 'object' && 'Valid' in field && 'Time' in field) {
      return field.Valid ? field.Time : undefined;
    }
    return field;
  };

  // Fetch reports
  const fetchReports = async () => {
    try {
      if (reports.length === 0) setLoading(true);
      const params = selectedEnvironmentId
        ? { environment_id: selectedEnvironmentId }
        : {};
      
      const response = await reportsApi.getAll(params);
      setReports(response.data.reports || []);
    } catch (err) {
      console.error('Failed to fetch reports:', err);
      setError('Failed to load reports');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchReports();
  }, [selectedEnvironmentId, environmentContext?.refreshTrigger]);

  // Poll for generating reports
  useEffect(() => {
    const hasGenerating = reports.some(r => 
      r.status === 'generating_team' || 
      r.status === 'generating_agents' || 
      generatingReports.has(r.id)
    );

    if (!hasGenerating) return;

    const interval = setInterval(() => {
      fetchReports();
    }, 2000); // Poll every 2 seconds

    return () => clearInterval(interval);
  }, [reports, generatingReports]);

  // Filter reports by status
  const filteredReports = reports.filter(report => {
    if (filterStatus === 'all') return true;
    return report.status === filterStatus;
  });

  // Status badge component
  const StatusBadge = ({ status }: { status: Report['status'] }) => {
    const statusConfig = {
      pending: { icon: Clock, color: 'gray', text: 'Pending' },
      generating_team: { icon: Loader, color: 'blue', text: 'Generating Team' },
      generating_agents: { icon: Loader, color: 'blue', text: 'Generating Agents' },
      completed: { icon: CheckCircle, color: 'green', text: 'Completed' },
      failed: { icon: XCircle, color: 'red', text: 'Failed' },
    };

    const config = statusConfig[status] || statusConfig.pending;
    const Icon = config.icon;
    
    const colorClasses = {
      gray: 'bg-gray-500/20 text-gray-300 border-gray-500',
      blue: 'bg-tokyo-blue/20 text-tokyo-blue border-tokyo-blue',
      green: 'bg-tokyo-green/20 text-tokyo-green border-tokyo-green',
      red: 'bg-tokyo-red/20 text-tokyo-red border-tokyo-red',
    };

    return (
      <span className={`inline-flex items-center gap-1 px-2 py-1 text-xs font-mono border rounded ${colorClasses[config.color as keyof typeof colorClasses]}`}>
        <Icon className={`h-3 w-3 ${status.includes('generating') ? 'animate-spin' : ''}`} />
        {config.text}
      </span>
    );
  };

  // Score badge component
  const ScoreBadge = ({ score }: { score: number | undefined }) => {
    if (score === undefined) return null;

    const getScoreColor = (s: number) => {
      if (s >= 9) return 'text-tokyo-green';
      if (s >= 7) return 'text-tokyo-yellow';
      return 'text-tokyo-red';
    };

    return (
      <div className={`text-2xl font-mono font-bold ${getScoreColor(score)}`}>
        {score.toFixed(1)}/10
      </div>
    );
  };

  // Progress bar component
  const ProgressBar = ({ progress }: { progress: number | undefined }) => {
    if (progress === undefined || progress === 0) return null;

    return (
      <div className="w-full bg-tokyo-dark3 rounded-full h-2 overflow-hidden">
        <div
          className="bg-tokyo-blue h-full transition-all duration-300"
          style={{ width: `${progress}%` }}
        />
      </div>
    );
  };

  // Format date
  const formatDate = (dateString: string | undefined) => {
    if (!dateString) return 'N/A';
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 60) return `${diffMins} minutes ago`;
    if (diffHours < 24) return `${diffHours} hours ago`;
    if (diffDays < 7) return `${diffDays} days ago`;
    return date.toLocaleDateString();
  };

  // Handle delete
  const handleDelete = async (reportId: number, reportName: string) => {
    if (!confirm(`Are you sure you want to delete "${reportName}"?`)) return;

    try {
      await reportsApi.delete(reportId);
      setReports(reports.filter(r => r.id !== reportId));
    } catch (err) {
      console.error('Failed to delete report:', err);
      alert('Failed to delete report');
    }
  };

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center bg-tokyo-bg">
        <div className="text-tokyo-fg font-mono">Loading reports...</div>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col bg-tokyo-bg">
      {/* Header */}
      <div className="flex items-center justify-between p-6 border-b border-tokyo-dark3 bg-tokyo-bg-dark">
        <div>
          <h1 className="text-2xl font-mono font-semibold text-tokyo-cyan">Reports</h1>
          <p className="text-sm text-tokyo-comment mt-1">
            LLM-based agent performance evaluation & benchmark experiments
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => setIsBenchmarkModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2 bg-tokyo-purple text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm font-medium transition-colors"
          >
            <Zap className="h-4 w-4" />
            Run Experiment
          </button>
          <button
            onClick={() => setIsCreateModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2 bg-tokyo-blue text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm font-medium transition-colors"
          >
            <Plus className="h-4 w-4" />
            Create Report
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="p-4 border-b border-tokyo-dark3 bg-tokyo-dark1">
        <div className="flex items-center gap-4">
          <label className="text-sm font-mono text-tokyo-comment">Environment:</label>
          <select
            value={selectedEnvironmentId || 'all'}
            onChange={(e) => setSelectedEnvironmentId(e.target.value === 'all' ? null : parseInt(e.target.value))}
            className="px-3 py-1 bg-tokyo-bg border border-tokyo-dark3 text-tokyo-fg font-mono rounded hover:border-tokyo-blue5 transition-colors"
          >
            <option value="all">All Environments</option>
            {environmentContext?.environments?.map((env: any) => (
              <option key={env.id} value={env.id}>
                {env.name}
              </option>
            ))}
          </select>

          <label className="text-sm font-mono text-tokyo-comment ml-4">Status:</label>
          <select
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value)}
            className="px-3 py-1 bg-tokyo-bg border border-tokyo-dark3 text-tokyo-fg font-mono rounded hover:border-tokyo-blue5 transition-colors"
          >
            <option value="all">All</option>
            <option value="pending">Pending</option>
            <option value="generating_team">Generating</option>
            <option value="generating_agents">Generating</option>
            <option value="completed">Completed</option>
            <option value="failed">Failed</option>
          </select>
          
          <div className="ml-auto text-sm font-mono text-tokyo-comment">
            {filteredReports.length} report{filteredReports.length !== 1 ? 's' : ''}
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 p-6 overflow-y-auto">
        {error && (
          <div className="mb-4 p-4 bg-tokyo-red/20 border border-tokyo-red rounded flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-tokyo-red" />
            <span className="text-tokyo-red font-mono">{error}</span>
          </div>
        )}

        {filteredReports.length === 0 ? (
          <div className="h-full flex items-center justify-center">
            <div className="text-center">
              <FileText className="h-16 w-16 text-tokyo-comment mx-auto mb-4" />
              <div className="text-tokyo-fg font-mono text-lg mb-2">No reports found</div>
              <div className="text-tokyo-comment font-mono text-sm mb-4">
                {filterStatus !== 'all'
                  ? `No ${filterStatus} reports`
                  : 'Create your first report to evaluate agent performance'
                }
              </div>
              <button
                onClick={() => setIsCreateModalOpen(true)}
                className="px-4 py-2 bg-tokyo-blue text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm transition-colors"
              >
                Create Report
              </button>
            </div>
          </div>
        ) : (
          <div className="grid gap-4">
            {filteredReports.map((report) => (
              <div
                key={report.id}
                className="p-6 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg shadow-tokyo hover:border-tokyo-blue5 transition-colors"
              >
                {/* Header */}
                <div className="flex items-start justify-between mb-4">
                  <div className="flex-1">
                    <div className="flex items-center gap-3 mb-2">
                      <h3 className="text-lg font-mono font-semibold text-tokyo-cyan">
                        {getSqlValue(report.name) || report.name}
                      </h3>
                      <StatusBadge status={report.status} />
                    </div>
                    {(() => {
                      const desc = getSqlValue(report.description);
                      return desc && (
                        <p className="text-sm text-tokyo-comment font-mono">{desc}</p>
                      );
                    })()}
                  </div>
                  
                  {(() => {
                    const teamScore = getSqlValue(report.team_score);
                    return teamScore !== undefined && teamScore > 0 && (
                      <ScoreBadge score={teamScore} />
                    );
                  })()}
                </div>

                {/* Progress */}
                {(report.status.includes('generating') || report.status === 'pending') && (
                  <div className="mb-4">
                    <ProgressBar progress={getSqlValue(report.progress)} />
                    {(() => {
                      const currentStep = getSqlValue(report.current_step);
                      return currentStep && (
                        <p className="text-xs text-tokyo-comment font-mono mt-2">
                          {currentStep}
                        </p>
                      );
                    })()}
                  </div>
                )}

                {/* Error */}
                {(() => {
                  const errorMessage = getSqlValue(report.error_message);
                  return report.status === 'failed' && errorMessage && (
                    <div className="mb-4 p-3 bg-tokyo-red/10 border border-tokyo-red rounded">
                      <p className="text-sm text-tokyo-red font-mono">{errorMessage}</p>
                    </div>
                  );
                })()}

                {/* Metadata */}
                <div className="flex items-center gap-6 text-sm font-mono text-tokyo-comment mb-4">
                  {(() => {
                    const agentsAnalyzed = getSqlValue(report.total_agents_analyzed);
                    const runsAnalyzed = getSqlValue(report.total_runs_analyzed);
                    const llmCost = getSqlValue(report.total_llm_cost);
                    
                    return (
                      <>
                        {agentsAnalyzed !== undefined && agentsAnalyzed > 0 && (
                          <span>{agentsAnalyzed} agents analyzed</span>
                        )}
                        {runsAnalyzed !== undefined && runsAnalyzed > 0 && (
                          <span>{runsAnalyzed} runs</span>
                        )}
                        {llmCost !== undefined && llmCost > 0 && (
                          <span>${llmCost.toFixed(4)} cost</span>
                        )}
                      </>
                    );
                  })()}
                </div>

                <div className="text-xs font-mono text-tokyo-comment mb-4">
                  {(() => {
                    const completedAt = getSqlValue(report.generation_completed_at);
                    const duration = getSqlValue(report.generation_duration_seconds);
                    const createdAt = getSqlValue(report.created_at);
                    
                    return completedAt ? (
                      <>
                        Generated: {formatDate(completedAt)}
                        {duration && <> • Duration: {duration.toFixed(1)}s</>}
                      </>
                    ) : (
                      <>Created: {formatDate(createdAt)}</>
                    );
                  })()}
                </div>

                {/* Actions */}
                <div className="flex items-center gap-2">
                  {report.status === 'completed' && (
                    <>
                      <button
                        onClick={() => navigate(`/reports/${report.id}`)}
                        className="px-3 py-1 bg-tokyo-blue text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-xs transition-colors"
                      >
                        View Report
                      </button>
                      <button
                        className="px-3 py-1 bg-tokyo-green text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-xs transition-colors"
                      >
                        <Download className="h-3 w-3 inline mr-1" />
                        Export PDF
                      </button>
                    </>
                  )}
                  
                  {(report.status === 'pending' || report.status === 'failed') && (
                    <button
                      onClick={async () => {
                        try {
                          setGeneratingReports(prev => new Set(prev).add(report.id));
                          await reportsApi.generate(report.id);
                          // Immediately refresh to show new status
                          await fetchReports();
                        } catch (err) {
                          console.error('Failed to generate report:', err);
                          setGeneratingReports(prev => {
                            const next = new Set(prev);
                            next.delete(report.id);
                            return next;
                          });
                          alert('Failed to start report generation');
                        }
                      }}
                      disabled={generatingReports.has(report.id)}
                      className="px-3 py-1 bg-tokyo-purple text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-xs transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      {generatingReports.has(report.id) ? (
                        <>
                          <Loader className="h-3 w-3 inline mr-1 animate-spin" />
                          Starting...
                        </>
                      ) : (
                        report.status === 'failed' ? 'Retry' : 'Generate Now'
                      )}
                    </button>
                  )}

                  {report.status.includes('generating') && (
                    <button
                      onClick={() => navigate(`/reports/${report.id}`)}
                      className="px-3 py-1 bg-tokyo-cyan text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-xs transition-colors"
                    >
                      View Progress
                    </button>
                  )}

                  <button
                    onClick={() => handleDelete(report.id, report.name)}
                    className="ml-auto px-3 py-1 bg-tokyo-red text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-xs transition-colors"
                  >
                    Delete
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Create Report Modal */}
      <CreateReportModal
        isOpen={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        onSuccess={(reportId) => {
          // Refresh reports list
          const fetchReports = async () => {
            const params = environmentContext?.selectedEnvironment
              ? { environment_id: environmentContext.selectedEnvironment }
              : {};
            const response = await reportsApi.getAll(params);
            setReports(response.data.reports || []);
          };
          fetchReports();
          // Navigate to the new report
          navigate(`/reports/${reportId}`);
        }}
        defaultEnvironmentId={environmentContext?.selectedEnvironment}
      />

      {/* Benchmark Experiment Modal */}
      <BenchmarkExperimentModal
        isOpen={isBenchmarkModalOpen}
        onClose={() => setIsBenchmarkModalOpen(false)}
        environmentId={environmentContext?.selectedEnvironment}
        onComplete={(results) => {
          console.log('Experiment complete:', results);
          // Could show a success toast or refresh reports
        }}
      />
    </div>
  );
};

import React, { useState, useEffect, useContext } from 'react';
import { useNavigate } from 'react-router-dom';
import { FileText, Plus, Download, AlertTriangle, CheckCircle, Clock, XCircle, Loader, Zap, HelpCircle, Target, TrendingUp, Shield, BarChart3 } from 'lucide-react';
import { reportsApi } from '../../api/station';
import type { Report } from '../../types/station';
import { CreateReportModal } from '../modals/CreateReportModal';
import { BenchmarkExperimentModal } from '../modals/BenchmarkExperimentModal';
import { HelpModal } from '../ui/HelpModal';

interface TocItem {
  id: string;
  label: string;
}

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
  const [isHelpModalOpen, setIsHelpModalOpen] = useState(false);

  // Define TOC items for help modal
  const helpTocItems: TocItem[] = [
    { id: 'llm-as-judge', label: 'LLM-as-Judge' },
    { id: 'workflow', label: 'Workflow' },
    { id: 'report-types', label: 'Report Types' },
    { id: 'quality-scores', label: 'Quality Scores' },
    { id: 'team-criteria', label: 'Team Criteria' },
    { id: 'best-practices', label: 'Best Practices' },
    { id: 'debugging', label: 'Debugging' }
  ];

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
    <div className="h-full flex flex-col bg-gray-50">
      {/* Header */}
      <div className="flex items-center justify-between p-6 border-b border-gray-200 bg-white">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Reports</h1>
          <p className="text-sm text-gray-600 mt-1">
            LLM-based agent performance evaluation & benchmark experiments
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => setIsHelpModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2.5 text-gray-600 bg-white hover:bg-gray-50 border border-gray-300 rounded-md font-medium text-sm transition-all shadow-sm"
          >
            <HelpCircle className="h-4 w-4" />
            Help
          </button>
          <button
            onClick={() => setIsBenchmarkModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2.5 bg-white text-gray-700 hover:bg-gray-50 border border-gray-300 rounded-md font-medium text-sm transition-all shadow-sm hover:shadow"
          >
            <Zap className="h-4 w-4" />
            Run Experiment
          </button>
          <button
            onClick={() => setIsCreateModalOpen(true)}
            className="flex items-center gap-2 px-4 py-2.5 bg-white text-gray-700 hover:bg-gray-50 border border-gray-300 rounded-md font-medium text-sm transition-all shadow-sm hover:shadow"
          >
            <Plus className="h-4 w-4" />
            Create Report
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="p-4 border-b border-gray-200 bg-white">
        <div className="flex items-center gap-4">
          <label className="text-sm font-medium text-gray-700">Environment:</label>
          <select
            value={selectedEnvironmentId || 'all'}
            onChange={(e) => setSelectedEnvironmentId(e.target.value === 'all' ? null : parseInt(e.target.value))}
            className="px-3 py-1.5 bg-white border border-gray-300 text-gray-900 rounded-md hover:border-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 transition-colors"
          >
            <option value="all">All Environments</option>
            {environmentContext?.environments?.map((env: any) => (
              <option key={env.id} value={env.id}>
                {env.name}
              </option>
            ))}
          </select>

          <label className="text-sm font-medium text-gray-700 ml-4">Status:</label>
          <select
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value)}
            className="px-3 py-1.5 bg-white border border-gray-300 text-gray-900 rounded-md hover:border-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 transition-colors"
          >
            <option value="all">All</option>
            <option value="pending">Pending</option>
            <option value="generating_team">Generating</option>
            <option value="generating_agents">Generating</option>
            <option value="completed">Completed</option>
            <option value="failed">Failed</option>
          </select>

          <div className="ml-auto text-sm font-medium text-gray-600">
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
          <div className="space-y-4">
            {/* Empty State Message */}
            <div className="text-center py-8">
              <FileText className="h-12 w-12 text-gray-300 mx-auto mb-3" />
              <div className="text-gray-900 text-lg font-medium mb-1">No reports found</div>
              <div className="text-gray-500 text-sm mb-4">
                {filterStatus !== 'all'
                  ? `No ${filterStatus} reports`
                  : 'Create your first report to evaluate agent performance'
                }
              </div>
              <button
                onClick={() => setIsCreateModalOpen(true)}
                className="px-4 py-2.5 bg-white text-gray-700 hover:bg-gray-50 border border-gray-300 rounded-md font-medium text-sm transition-all shadow-sm hover:shadow"
              >
                Create Report
              </button>
            </div>

            {/* Skeleton Placeholders */}
            <div className="grid gap-4 opacity-30">
              {[1, 2].map((i) => (
                <div
                  key={i}
                  className="p-6 bg-white border border-gray-200 rounded-lg shadow-sm"
                >
                  <div className="flex items-start justify-between mb-4">
                    <div className="flex-1">
                      <div className="flex items-center gap-3 mb-2">
                        <div className="h-6 w-48 bg-gray-200 rounded"></div>
                        <div className="h-5 w-20 bg-gray-200 rounded-full"></div>
                      </div>
                      <div className="h-4 w-96 bg-gray-200 rounded"></div>
                    </div>
                    <div className="h-12 w-12 bg-gray-200 rounded"></div>
                  </div>

                  <div className="grid grid-cols-4 gap-4 mb-4">
                    <div className="h-16 bg-gray-100 rounded-lg"></div>
                    <div className="h-16 bg-gray-100 rounded-lg"></div>
                    <div className="h-16 bg-gray-100 rounded-lg"></div>
                    <div className="h-16 bg-gray-100 rounded-lg"></div>
                  </div>

                  <div className="flex gap-2">
                    <div className="h-9 w-28 bg-gray-200 rounded-md"></div>
                    <div className="h-9 w-28 bg-gray-200 rounded-md"></div>
                    <div className="h-9 w-32 bg-gray-200 rounded-md"></div>
                  </div>
                </div>
              ))}
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
                        className="px-3 py-1.5 bg-blue-600 text-white hover:bg-blue-700 rounded font-medium text-xs transition-all shadow-sm"
                      >
                        View Report
                      </button>
                      <button
                        className="px-3 py-1.5 bg-green-600 text-white hover:bg-green-700 rounded font-medium text-xs transition-all shadow-sm"
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
                      className="px-3 py-1.5 bg-purple-600 text-white hover:bg-purple-700 rounded font-medium text-xs transition-all shadow-sm disabled:opacity-50 disabled:cursor-not-allowed"
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
                      className="px-3 py-1.5 bg-cyan-600 text-white hover:bg-cyan-700 rounded font-medium text-xs transition-all shadow-sm"
                    >
                      View Progress
                    </button>
                  )}

                  <button
                    onClick={() => handleDelete(report.id, report.name)}
                    className="ml-auto px-3 py-1.5 bg-red-600 text-white hover:bg-red-700 rounded font-medium text-xs transition-all shadow-sm"
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

      {/* Help Modal */}
      <HelpModal
        isOpen={isHelpModalOpen}
        onClose={() => setIsHelpModalOpen(false)}
        title="Reports & Evaluation"
        pageDescription="Automated LLM-as-judge evaluation that tests agents with generated scenarios, scores performance against business criteria, and provides production readiness assessment with objective quality metrics."
        tocItems={helpTocItems}
      >
        <div className="space-y-6">
          {/* What is LLM-as-Judge */}
          <div id="llm-as-judge">
            <h3 className="text-base font-semibold text-gray-900 mb-3">What is LLM-as-Judge Evaluation?</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-6">
              <div className="text-sm text-gray-900 mb-4">
                <strong>LLM-as-judge</strong> uses an AI model to objectively evaluate agent performance instead of manual testing.
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-3">
                  <div className="bg-white border border-gray-200 rounded-lg p-3">
                    <div className="font-mono text-sm text-gray-900 mb-1">Automated Testing</div>
                    <div className="text-xs text-gray-600">Generate 100+ test scenarios automatically. No more manual "does this look good?" checks.</div>
                  </div>
                  <div className="bg-white border border-gray-200 rounded-lg p-3">
                    <div className="font-mono text-sm text-gray-900 mb-1">Objective Scoring</div>
                    <div className="text-xs text-gray-600">Quality scores (0-10) based on accuracy, completeness, and relevance - consistent across runs.</div>
                  </div>
                </div>
                <div className="space-y-3">
                  <div className="bg-white border border-gray-200 rounded-lg p-3">
                    <div className="font-mono text-sm text-gray-900 mb-1">Business Criteria</div>
                    <div className="text-xs text-gray-600">Define team goals like "reduce MTTR by 30%" or "identify $100K+ savings" - judge scores against goals.</div>
                  </div>
                  <div className="bg-white border border-gray-200 rounded-lg p-3">
                    <div className="font-mono text-sm text-gray-900 mb-1">Production Ready</div>
                    <div className="text-xs text-gray-600">Get clear pass/fail assessment for deploying agents to production with confidence.</div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Evaluation Workflow */}
          <div id="workflow">
            <h3 className="text-base font-semibold text-gray-900 mb-3">Evaluation Workflow</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-4">
              <div className="space-y-3">
                <div className="flex items-start gap-3">
                  <div className="w-6 h-6 rounded bg-[#0084FF] flex items-center justify-center font-mono text-xs text-white flex-shrink-0">1</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">Generate Test Scenarios</div>
                    <div className="text-xs text-gray-600">AI generates 100+ test scenarios based on agent purpose. Example: "FinOps agent" → scenarios for cost spikes, budget forecasts, savings opportunities.</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="w-6 h-6 rounded bg-[#0084FF] flex items-center justify-center font-mono text-xs text-white flex-shrink-0">2</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">Execute All Scenarios</div>
                    <div className="text-xs text-gray-600">Run agent against all scenarios with full execution tracking. Tool calls, token usage, timing all captured for analysis.</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="w-6 h-6 rounded bg-[#0084FF] flex items-center justify-center font-mono text-xs text-white flex-shrink-0">3</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">LLM-as-Judge Scoring</div>
                    <div className="text-xs text-gray-600">Judge AI analyzes agent output against criteria: accuracy, completeness, relevance. Assigns quality score 0-10.</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="w-6 h-6 rounded bg-[#0084FF] flex items-center justify-center font-mono text-xs text-white flex-shrink-0">4</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">Generate Report</div>
                    <div className="text-xs text-gray-600">Aggregate scores, identify weaknesses, produce PDF with recommendations. Clear production readiness decision.</div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Report Types */}
          <div id="report-types">
            <h3 className="text-base font-semibold text-gray-900 mb-3">Report Types</h3>
            <div className="space-y-3">
              <div className="bg-white border border-gray-200 rounded p-4">
                <div className="flex items-center gap-2 mb-2">
                  <Target className="h-4 w-4 text-[#0084FF]" />
                  <div className="font-mono text-sm text-gray-900">Individual Agent Reports</div>
                </div>
                <div className="text-xs text-gray-600 leading-relaxed">
                  <strong>Test single agents with comprehensive scenarios.</strong> Generates 100+ test cases, executes all, scores quality/accuracy/completeness. Shows tool effectiveness, identifies failure patterns, provides specific improvement recommendations. Use during development to iterate on prompts until quality score &gt; 8.0.
                </div>
              </div>

              <div className="bg-white border border-gray-200 rounded p-4">
                <div className="flex items-center gap-2 mb-2">
                  <TrendingUp className="h-4 w-4 text-[#0084FF]" />
                  <div className="font-mono text-sm text-gray-900">Team Performance Reports</div>
                </div>
                <div className="text-xs text-gray-600 leading-relaxed">
                  <strong>Evaluate multi-agent teams against business goals.</strong> Define criteria like "reduce MTTR by 30%" or "identify $100K savings". Station tests coordinator + specialists, measures team performance, calculates weighted score. Shows which agents excel and which need improvement. Perfect for quarterly reviews and optimization.
                </div>
              </div>

              <div className="bg-white border border-gray-200 rounded p-4">
                <div className="flex items-center gap-2 mb-2">
                  <Zap className="h-4 w-4 text-[#0084FF]" />
                  <div className="font-mono text-sm text-gray-900">Benchmark Experiments</div>
                </div>
                <div className="text-xs text-gray-600 leading-relaxed">
                  <strong>Compare model performance (GPT-4o vs GPT-4o-mini).</strong> Run same scenarios on multiple agents with different models. Compare quality, speed, cost, accuracy. Determine optimal model choice - often GPT-4o-mini performs 90% as well at 30% cost. Data-driven model selection instead of guessing.
                </div>
              </div>
            </div>
          </div>

          {/* Understanding Scores */}
          <div id="quality-scores">
            <h3 className="text-base font-semibold text-gray-900 mb-3">Understanding Quality Scores</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-4">
              <div className="space-y-3">
                <div className="flex items-start gap-3">
                  <div className="px-3 py-1 bg-green-100 text-green-800 rounded font-bold text-sm">9-10</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">Production Ready</div>
                    <div className="text-xs text-gray-600">Excellent performance, ready for production deployment. Consistently accurate and complete responses.</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="px-3 py-1 bg-blue-100 text-blue-800 rounded font-bold text-sm">7-8</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">Good with Minor Issues</div>
                    <div className="text-xs text-gray-600">Good performance, minor improvements needed. Review recommendations and refine prompts.</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="px-3 py-1 bg-yellow-100 text-yellow-800 rounded font-bold text-sm">5-6</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">Needs Refinement</div>
                    <div className="text-xs text-gray-600">Acceptable but inconsistent. Significant prompt refinement needed before production use.</div>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="px-3 py-1 bg-red-100 text-red-800 rounded font-bold text-sm">&lt;5</div>
                  <div>
                    <div className="font-mono text-sm text-gray-900 mb-1">Not Production Ready</div>
                    <div className="text-xs text-gray-600">Significant issues. Agent requires major rework - review tool selection, prompt design, and examples.</div>
                  </div>
                </div>
              </div>
              <div className="mt-4 text-xs text-gray-700 bg-white border border-gray-300 px-3 py-2 rounded">
                <strong>Component Metrics:</strong> Accuracy (correctness), Completeness (thoroughness), Relevance (focus on task), Efficiency (optimal tool usage)
              </div>
            </div>
          </div>

          {/* Team Criteria Examples */}
          <div id="team-criteria">
            <h3 className="text-base font-semibold text-gray-900 mb-3">Team Criteria Examples</h3>
            <div className="space-y-2">
              <div className="bg-white border border-gray-200 rounded p-3">
                <div className="font-mono text-sm text-gray-900 mb-1">SRE Team - Incident Response</div>
                <div className="text-xs text-gray-600">
                  <strong>Goal:</strong> Minimize MTTR and prevent recurring issues.<br/>
                  <strong>Criteria:</strong> MTTR reduction (40% weight), Root cause accuracy (30%), Prevention rate (30%)<br/>
                  <strong>Agents:</strong> Coordinator, K8s Expert, Log Analyzer, Metrics Analyzer, Network Diagnostics
                </div>
              </div>
              <div className="bg-white border border-gray-200 rounded p-3">
                <div className="font-mono text-sm text-gray-900 mb-1">FinOps Team - Cost Optimization</div>
                <div className="text-xs text-gray-600">
                  <strong>Goal:</strong> Maximize cost savings and forecast accuracy.<br/>
                  <strong>Criteria:</strong> Savings identified (40%), Forecast accuracy (30%), Execution cost (30%)<br/>
                  <strong>Agents:</strong> Cost Analyzer, Resource Optimizer, Budget Forecaster, Anomaly Detector
                </div>
              </div>
              <div className="bg-white border border-gray-200 rounded p-3">
                <div className="font-mono text-sm text-gray-900 mb-1">Security Team - Vulnerability Detection</div>
                <div className="text-xs text-gray-600">
                  <strong>Goal:</strong> Detect vulnerabilities and maintain compliance.<br/>
                  <strong>Criteria:</strong> Vulnerability detection (50%), False positive rate (30%), Compliance coverage (20%)<br/>
                  <strong>Agents:</strong> Container Scanner, Code Analyzer, Infrastructure Auditor, Compliance Checker
                </div>
              </div>
            </div>
          </div>

          {/* Best Practices */}
          <div id="best-practices">
            <h3 className="text-base font-semibold text-gray-900 mb-3">Best Practices</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-4">
              <ul className="space-y-2 text-xs text-gray-700">
                <li className="flex items-start gap-2">
                  <span className="text-green-600 font-bold">✓</span>
                  <div><strong>Test Early and Often</strong> - Run 10-20 scenario tests after each prompt change during development for fast feedback loop.</div>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-green-600 font-bold">✓</span>
                  <div><strong>Business-Focused Criteria</strong> - Define criteria like "identify $100K savings" not "uses correct tools". Align evaluation with actual business goals.</div>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-green-600 font-bold">✓</span>
                  <div><strong>Comprehensive Pre-Production Testing</strong> - Run 100+ scenarios with comprehensive strategy before production. Ensure quality score &gt; 8.0 for critical agents.</div>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-green-600 font-bold">✓</span>
                  <div><strong>Monitor Production Performance</strong> - Weekly reports on production agents. Track quality trends, response times, user satisfaction over time.</div>
                </li>
                <li className="flex items-start gap-2">
                  <span className="text-green-600 font-bold">✓</span>
                  <div><strong>Compare Models with Data</strong> - Don't assume GPT-4o is always better. Test both GPT-4o and GPT-4o-mini - often mini performs 90% as well at 70% cost savings.</div>
                </li>
              </ul>
            </div>
          </div>

          {/* Debugging Low Scores */}
          <div id="debugging">
            <h3 className="text-base font-semibold text-gray-900 mb-3">Debugging Low Quality Scores</h3>
            <div className="bg-gray-50 rounded-lg border border-gray-200 p-4">
              <div className="text-sm text-gray-900 mb-3">If agent scores &lt; 6.0, investigate:</div>
              <div className="space-y-2">
                <div className="bg-white border border-gray-200 rounded p-2 text-xs text-gray-700">
                  <strong>1. Inspect failing runs</strong> - Click into low-scoring runs, examine tool calls and LLM responses for patterns
                </div>
                <div className="bg-white border border-gray-200 rounded p-2 text-xs text-gray-700">
                  <strong>2. Check tool effectiveness</strong> - Report shows per-tool scores. Are tools returning useful data? Misconfigured?
                </div>
                <div className="bg-white border border-gray-200 rounded p-2 text-xs text-gray-700">
                  <strong>3. Review LLM-as-judge feedback</strong> - Judge provides specific issues: incomplete analysis, incorrect conclusions, off-task responses
                </div>
                <div className="bg-white border border-gray-200 rounded p-2 text-xs text-gray-700">
                  <strong>4. Use Jaeger traces</strong> - View execution timeline to find timeouts, errors, or slow tool calls causing failures
                </div>
              </div>
            </div>
          </div>
        </div>
      </HelpModal>
    </div>
  );
};

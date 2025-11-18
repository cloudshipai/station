import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Download, CheckCircle, XCircle, AlertTriangle, TrendingUp, TrendingDown, Minus, ChevronDown, ChevronRight, ChevronUp } from 'lucide-react';
import { reportsApi, benchmarksApi } from '../../api/station';
import type { Report, AgentReportDetail, CriterionScore } from '../../types/station';
import { EnterpriseAgentAnalysis } from '../reports/EnterpriseAgentAnalysis';

interface TeamCriteria {
  goal: string;
  criteria: Record<string, {
    weight: number;
    threshold: number;
    description: string;
  }>;
}

interface AgentCriteria {
  goal: string;
  criteria: Record<string, {
    weight: number;
    threshold: number;
    description: string;
  }>;
}

export const ReportDetailPage: React.FC = () => {
  const { reportId } = useParams<{ reportId: string }>();
  const navigate = useNavigate();
  
  const [report, setReport] = useState<Report | null>(null);
  const [agentDetails, setAgentDetails] = useState<AgentReportDetail[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  // Parsed data
  const [teamCriteria, setTeamCriteria] = useState<TeamCriteria | null>(null);
  const [teamCriteriaScores, setTeamCriteriaScores] = useState<Record<string, CriterionScore> | null>(null);
  const [agentCriteria, setAgentCriteria] = useState<AgentCriteria | null>(null);
  
  // UI state
  const [showFullCriteria, setShowFullCriteria] = useState(false);
  const [showAgentCriteria, setShowAgentCriteria] = useState(false);
  const [exportingPDF, setExportingPDF] = useState(false);
  
  // Detailed test results
  const [expandedRun, setExpandedRun] = useState<number | null>(null);
  const [runMetrics, setRunMetrics] = useState<Record<number, any[]>>({});
  const [loadingMetrics, setLoadingMetrics] = useState<Set<number>>(new Set());

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

  useEffect(() => {
    const fetchReport = async () => {
      if (!reportId) return;

      // Validate reportId is a number
      const reportIdNum = parseInt(reportId);
      if (isNaN(reportIdNum)) {
        console.error('Invalid reportId:', reportId);
        setError(`Invalid report ID: "${reportId}". Report IDs must be numbers.`);
        setLoading(false);
        return;
      }

      try {
        setLoading(true);
        const response = await reportsApi.getById(reportIdNum);
        const { report: reportData, agent_details } = response.data;
        
        setReport(reportData);
        setAgentDetails(agent_details || []);
        
        // Parse JSON fields
        try {
          const teamCriteriaStr = getSqlValue(reportData.team_criteria);
          if (teamCriteriaStr) {
            setTeamCriteria(JSON.parse(teamCriteriaStr));
          }
          
          const teamCriteriaScoresStr = getSqlValue(reportData.team_criteria_scores);
          if (teamCriteriaScoresStr) {
            setTeamCriteriaScores(JSON.parse(teamCriteriaScoresStr));
          }
          
          const agentCriteriaStr = getSqlValue(reportData.agent_criteria);
          if (agentCriteriaStr) {
            setAgentCriteria(JSON.parse(agentCriteriaStr));
          }
        } catch (parseErr) {
          console.error('Failed to parse JSON fields:', parseErr);
        }
      } catch (err) {
        console.error('Failed to fetch report:', err);
        setError('Failed to load report');
      } finally {
        setLoading(false);
      }
    };

    fetchReport();
  }, [reportId]);

  // Helper functions
  const getScoreColor = (score: number) => {
    if (score >= 9) return 'text-tokyo-green';
    if (score >= 7) return 'text-tokyo-yellow';
    return 'text-tokyo-red';
  };

  const getScoreBgColor = (score: number) => {
    if (score >= 9) return 'bg-tokyo-green/20 border-tokyo-green';
    if (score >= 7) return 'bg-tokyo-yellow/20 border-tokyo-yellow';
    return 'bg-tokyo-red/20 border-tokyo-red';
  };

  const getScoreLabel = (score: number) => {
    if (score >= 9) return 'Excellent';
    if (score >= 7) return 'Good';
    return 'Needs Improvement';
  };

  const formatDate = (dateString: string | undefined) => {
    if (!dateString) return 'N/A';
    const date = new Date(dateString);
    return date.toLocaleString();
  };

  // Export PDF handler with react-pdf (dynamic import)
  const handleExportPDF = async () => {
    if (!report) return;
    
    try {
      setExportingPDF(true);
      
      // Dynamically import PDF dependencies
      const { pdf } = await import('@react-pdf/renderer');
      const { EnterpriseReportPDF } = await import('../reports/EnterpriseReportPDF');
      
      // Generate PDF blob
      const blob = await pdf(
        React.createElement(EnterpriseReportPDF, {
          report,
          agentDetails,
          teamCriteria,
          teamCriteriaScores,
          agentCriteria,
        })
      ).toBlob();
      
      // Create download link
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `${report.name.replace(/\s+/g, '_')}_Report.pdf`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } catch (err) {
      console.error('Failed to generate PDF:', err);
      alert('Failed to generate PDF. Please try again.');
    } finally {
      setExportingPDF(false);
    }
  };

  // Fetch detailed metrics for a run
  const fetchRunMetrics = async (runId: number) => {
    if (runMetrics[runId]) {
      setExpandedRun(expandedRun === runId ? null : runId);
      return;
    }
    
    try {
      setLoadingMetrics(prev => new Set(prev).add(runId));
      console.log(`Fetching metrics for run ${runId}...`);
      
      const response = await benchmarksApi.getMetrics(runId);
      console.log(`Received metrics for run ${runId}:`, response.data);
      
      const metrics = response.data.metrics || [];
      console.log(`Storing ${metrics.length} metrics for run ${runId}`);
      
      setRunMetrics(prev => ({ ...prev, [runId]: metrics }));
      setExpandedRun(runId);
    } catch (err: any) {
      console.error(`Failed to fetch metrics for run ${runId}:`, err);
      
      // Store empty array to indicate no metrics available
      setRunMetrics(prev => ({ ...prev, [runId]: [] }));
      
      // Show user-friendly message
      const errorMsg = err?.response?.data?.error || 'This run has not been individually benchmarked yet. Individual run metrics are created when you run "stn benchmark evaluate <run_id>".';
      alert(`Run #${runId}: ${errorMsg}`);
    } finally {
      setLoadingMetrics(prev => {
        const next = new Set(prev);
        next.delete(runId);
        return next;
      });
    }
  };

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center bg-tokyo-bg">
        <div className="text-tokyo-fg font-mono">Loading report...</div>
      </div>
    );
  }

  if (error || !report) {
    return (
      <div className="h-full flex items-center justify-center bg-tokyo-bg">
        <div className="text-center">
          <AlertTriangle className="h-16 w-16 text-tokyo-red mx-auto mb-4" />
          <div className="text-tokyo-fg font-mono text-lg mb-2">
            {error || 'Report not found'}
          </div>
          <button
            onClick={() => navigate('/reports')}
            className="px-4 py-2 bg-tokyo-blue text-tokyo-bg rounded font-mono text-sm"
          >
            Back to Reports
          </button>
        </div>
      </div>
    );
  }

  const passedAgents = agentDetails.filter(a => a.passed).length;
  const failedAgents = agentDetails.length - passedAgents;

  return (
    <>
      <style>{`
        @media print {
          body { background: white !important; color: black !important; }
          .no-print { display: none !important; }
          h1, h2, h3 { color: #1a1b26 !important; }
          .tokyo-bg, .tokyo-bg-dark { background: white !important; }
          .tokyo-fg, .tokyo-cyan { color: #1a1b26 !important; }
          .border-tokyo-blue7 { border-color: #ccc !important; }
          button { display: none !important; }
        }
      `}</style>
      
      <div className="h-full flex flex-col bg-tokyo-bg overflow-y-auto">
      {/* Header */}
      <div className="sticky top-0 z-10 bg-tokyo-bg-dark border-b border-tokyo-dark3">
        <div className="p-6">
          <button
            onClick={() => navigate('/reports')}
            className="flex items-center gap-2 text-tokyo-blue hover:text-tokyo-blue5 mb-4 font-mono text-sm"
          >
            <ArrowLeft className="h-4 w-4" />
            Back to Reports
          </button>
          
          <div className="flex items-start justify-between">
            <div className="flex-1">
              <h1 className="text-2xl font-mono font-semibold text-tokyo-cyan mb-2">
                {report.name}
              </h1>
              {(() => {
                const desc = getSqlValue(report.description);
                return desc && (
                  <p className="text-sm text-tokyo-comment font-mono mb-4">{desc}</p>
                );
              })()}
              
              <div className="flex items-center gap-6 text-sm font-mono text-tokyo-comment">
                <span>Status: <span className={report.status === 'completed' ? 'text-tokyo-green' : 'text-tokyo-yellow'}>{report.status}</span></span>
                <span>Generated: {formatDate(getSqlValue(report.generation_completed_at))}</span>
                {(() => {
                  const duration = getSqlValue(report.generation_duration_seconds);
                  return duration && (
                    <span>Duration: {duration.toFixed(1)}s</span>
                  );
                })()}
                <span>Judge: {getSqlValue(report.judge_model) || 'gpt-4o-mini'}</span>
                {(() => {
                  const cost = getSqlValue(report.total_llm_cost);
                  return cost && cost > 0 && (
                    <span>Cost: ${cost.toFixed(4)}</span>
                  );
                })()}
              </div>
            </div>
            
            <button 
              onClick={handleExportPDF}
              disabled={exportingPDF}
              className="flex items-center gap-2 px-4 py-2 bg-tokyo-green text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm transition-colors disabled:opacity-50 disabled:cursor-wait"
            >
              {exportingPDF ? (
                <>
                  <div className="h-4 w-4 border-2 border-tokyo-bg border-t-transparent rounded-full animate-spin" />
                  Generating...
                </>
              ) : (
                <>
                  <Download className="h-4 w-4" />
                  Export PDF
                </>
              )}
            </button>
          </div>
        </div>
      </div>

      <div className="p-6 space-y-6">
        {/* Benchmark Context Card */}
        {teamCriteria && (
          <div className="p-6 bg-gradient-to-r from-tokyo-blue/10 to-tokyo-purple/10 border-2 border-tokyo-blue rounded-lg">
            <div className="flex items-start gap-4">
              <div className="flex-shrink-0">
                <div className="h-12 w-12 rounded-lg bg-tokyo-blue/20 flex items-center justify-center">
                  <span className="text-2xl">🎯</span>
                </div>
              </div>
              <div className="flex-1">
                <div className="flex items-center justify-between mb-2">
                  <h2 className="text-xl font-semibold text-tokyo-cyan">
                    Team Benchmark Criteria
                  </h2>
                  <button
                    onClick={() => setShowFullCriteria(!showFullCriteria)}
                    className="flex items-center gap-2 px-3 py-1 text-sm font-mono text-tokyo-blue hover:text-tokyo-blue5 transition-colors"
                  >
                    {showFullCriteria ? (
                      <>
                        <ChevronUp className="h-4 w-4" />
                        Hide Details
                      </>
                    ) : (
                      <>
                        <ChevronDown className="h-4 w-4" />
                        Show Details
                      </>
                    )}
                  </button>
                </div>
                <p className="text-base text-tokyo-fg leading-relaxed mb-4">
                  {teamCriteria.goal}
                </p>
                
                {/* Compact View */}
                {!showFullCriteria && (
                  <div className="bg-tokyo-bg/50 rounded-lg p-4 border border-tokyo-dark3">
                    <h3 className="text-sm font-semibold text-tokyo-comment mb-3 uppercase tracking-wide">
                      Success Criteria Summary
                    </h3>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                      {Object.entries(teamCriteria.criteria).map(([name, config]) => {
                        const score = teamCriteriaScores?.[name]?.score || 0;
                        const passed = score >= config.threshold;
                        
                        return (
                          <div key={name} className="flex items-center gap-3 bg-tokyo-bg-dark/50 rounded p-3 border border-tokyo-dark3">
                            <div className="flex-shrink-0">
                              <div className="h-10 w-10 rounded bg-tokyo-blue/20 flex items-center justify-center">
                                <span className="text-lg font-bold text-tokyo-blue">
                                  {(config.weight * 100).toFixed(0)}%
                                </span>
                              </div>
                            </div>
                            <div className="flex-1 min-w-0">
                              <div className="text-sm font-semibold text-tokyo-fg capitalize truncate">
                                {name.replace(/_/g, ' ')}
                              </div>
                              <div className="text-xs text-tokyo-comment flex items-center gap-2">
                                <span>Target: {config.threshold.toFixed(1)}</span>
                                {passed ? (
                                  <CheckCircle className="h-3 w-3 text-tokyo-green" />
                                ) : (
                                  <XCircle className="h-3 w-3 text-tokyo-red" />
                                )}
                              </div>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}

                {/* Expanded View */}
                {showFullCriteria && (
                  <div className="space-y-3">
                    {Object.entries(teamCriteria.criteria).map(([name, config]) => {
                      const score = teamCriteriaScores?.[name]?.score || 0;
                      const passed = score >= config.threshold;
                      const reasoning = teamCriteriaScores?.[name]?.reasoning;
                      
                      return (
                        <div key={name} className="bg-tokyo-bg/50 rounded-lg p-4 border-2 border-tokyo-dark3">
                          <div className="flex items-start justify-between mb-3">
                            <div className="flex-1">
                              <div className="flex items-center gap-3 mb-2">
                                <h4 className="text-base font-mono font-bold text-tokyo-fg capitalize">
                                  {name.replace(/_/g, ' ')}
                                </h4>
                                {passed ? (
                                  <CheckCircle className="h-5 w-5 text-tokyo-green" />
                                ) : (
                                  <XCircle className="h-5 w-5 text-tokyo-red" />
                                )}
                              </div>
                              <p className="text-sm text-tokyo-comment mb-2">
                                {config.description}
                              </p>
                              <div className="flex items-center gap-4 text-xs font-mono">
                                <span className="text-tokyo-blue">
                                  Weight: {(config.weight * 100).toFixed(0)}%
                                </span>
                                <span className="text-tokyo-comment">
                                  Threshold: {config.threshold.toFixed(1)}/10
                                </span>
                                <span className={passed ? 'text-tokyo-green' : 'text-tokyo-red'}>
                                  Score: {score.toFixed(1)}/10
                                </span>
                              </div>
                            </div>
                            <div className="text-right">
                              <div className={`text-3xl font-mono font-bold ${score >= 9 ? 'text-tokyo-green' : score >= 7 ? 'text-tokyo-yellow' : 'text-tokyo-red'}`}>
                                {score.toFixed(1)}
                              </div>
                              <div className="text-xs text-tokyo-comment mt-1">
                                / 10
                              </div>
                            </div>
                          </div>
                          
                          {/* Progress bar */}
                          <div className="relative w-full h-2 bg-tokyo-dark3 rounded-full overflow-hidden mb-3">
                            <div
                              className={`h-full ${score >= 9 ? 'bg-tokyo-green' : score >= 7 ? 'bg-tokyo-yellow' : 'bg-tokyo-red'}`}
                              style={{ width: `${(score / 10) * 100}%` }}
                            />
                          </div>

                          {reasoning && (
                            <div className="mt-3 p-3 bg-tokyo-bg-dark/30 rounded border border-tokyo-dark3">
                              <p className="text-xs font-mono text-tokyo-comment leading-relaxed">
                                <span className="font-bold text-tokyo-fg">Reasoning: </span>
                                {reasoning}
                              </p>
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Agent Criteria Section (if exists) */}
        {agentCriteria && (
          <div className="p-6 bg-gradient-to-r from-tokyo-purple/10 to-tokyo-pink/10 border-2 border-tokyo-purple rounded-lg">
            <div className="flex items-start gap-4">
              <div className="flex-shrink-0">
                <div className="h-12 w-12 rounded-lg bg-tokyo-purple/20 flex items-center justify-center">
                  <span className="text-2xl">🤖</span>
                </div>
              </div>
              <div className="flex-1">
                <div className="flex items-center justify-between mb-2">
                  <h2 className="text-xl font-semibold text-tokyo-purple">
                    Agent Performance Criteria
                  </h2>
                  <button
                    onClick={() => setShowAgentCriteria(!showAgentCriteria)}
                    className="flex items-center gap-2 px-3 py-1 text-sm font-mono text-tokyo-purple hover:opacity-80 transition-colors"
                  >
                    {showAgentCriteria ? (
                      <>
                        <ChevronUp className="h-4 w-4" />
                        Hide Details
                      </>
                    ) : (
                      <>
                        <ChevronDown className="h-4 w-4" />
                        Show Details
                      </>
                    )}
                  </button>
                </div>
                <p className="text-base text-tokyo-fg leading-relaxed mb-4">
                  {agentCriteria.goal}
                </p>
                
                {showAgentCriteria && (
                  <div className="bg-tokyo-bg/50 rounded-lg p-4 border border-tokyo-dark3">
                    <h3 className="text-sm font-semibold text-tokyo-comment mb-3 uppercase tracking-wide">
                      Individual Agent Evaluation Criteria
                    </h3>
                    <div className="space-y-2">
                      {Object.entries(agentCriteria.criteria).map(([name, config]) => (
                        <div key={name} className="flex items-center justify-between p-3 bg-tokyo-bg-dark/50 rounded border border-tokyo-dark3">
                          <div className="flex-1">
                            <div className="text-sm font-semibold text-tokyo-fg capitalize">
                              {name.replace(/_/g, ' ')}
                            </div>
                            <div className="text-xs text-tokyo-comment mt-1">
                              {config.description}
                            </div>
                            <div className="flex items-center gap-3 mt-1 text-xs font-mono">
                              <span className="text-tokyo-purple">
                                Weight: {(config.weight * 100).toFixed(0)}%
                              </span>
                              <span className="text-tokyo-comment">
                                Threshold: {config.threshold.toFixed(1)}/10
                              </span>
                            </div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>
        )}

        {/* Executive Summary Card */}
        {(() => {
          const teamScore = getSqlValue(report.team_score);
          return report.status === 'completed' && teamScore !== undefined && teamScore > 0 && (
          <div className="p-6 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
            <h2 className="text-2xl font-semibold text-tokyo-cyan mb-6">
              Benchmark Results
            </h2>
            
            {/* Quality Metrics Info */}
            <div className="mb-6 p-4 bg-tokyo-blue/10 border border-tokyo-blue/30 rounded">
              <h3 className="text-sm font-mono font-semibold text-tokyo-blue mb-3 uppercase tracking-wide">
                🔬 Quality Metrics Evaluated
              </h3>
              <div className="grid grid-cols-1 md:grid-cols-5 gap-3">
                <div className="flex items-start gap-2">
                  <div className="flex-shrink-0 w-2 h-2 rounded-full bg-tokyo-green mt-1.5" />
                  <div>
                    <div className="text-xs font-mono font-semibold text-tokyo-fg">Hallucination</div>
                    <div className="text-xs text-tokyo-comment">Detects fabricated information (≤10%)</div>
                  </div>
                </div>
                <div className="flex items-start gap-2">
                  <div className="flex-shrink-0 w-2 h-2 rounded-full bg-tokyo-cyan mt-1.5" />
                  <div>
                    <div className="text-xs font-mono font-semibold text-tokyo-fg">Relevancy</div>
                    <div className="text-xs text-tokyo-comment">Response addresses task (≥80%)</div>
                  </div>
                </div>
                <div className="flex items-start gap-2">
                  <div className="flex-shrink-0 w-2 h-2 rounded-full bg-tokyo-purple mt-1.5" />
                  <div>
                    <div className="text-xs font-mono font-semibold text-tokyo-fg">Task Completion</div>
                    <div className="text-xs text-tokyo-comment">Fully completes request (≥85%)</div>
                  </div>
                </div>
                <div className="flex items-start gap-2">
                  <div className="flex-shrink-0 w-2 h-2 rounded-full bg-tokyo-yellow mt-1.5" />
                  <div>
                    <div className="text-xs font-mono font-semibold text-tokyo-fg">Faithfulness</div>
                    <div className="text-xs text-tokyo-comment">Grounded in context (≤10% drift)</div>
                  </div>
                </div>
                <div className="flex items-start gap-2">
                  <div className="flex-shrink-0 w-2 h-2 rounded-full bg-tokyo-red mt-1.5" />
                  <div>
                    <div className="text-xs font-mono font-semibold text-tokyo-fg">Toxicity</div>
                    <div className="text-xs text-tokyo-comment">No harmful content (≤5%)</div>
                  </div>
                </div>
              </div>
              <div className="mt-3 text-xs text-tokyo-comment font-mono">
                ⚡ Judge Model: {getSqlValue(report.judge_model) || 'gpt-4o-mini'} | 
                💰 Total Cost: ${getSqlValue(report.total_llm_cost)?.toFixed(4) || '0.0000'} | 
                🔄 Runs Analyzed: {getSqlValue(report.total_runs_analyzed) || 0}
              </div>
            </div>
            
            {/* Overall Score */}
            {(() => {
              const teamScore = getSqlValue(report.team_score);
              return teamScore && (
                <>
                  <div className="flex items-center justify-center mb-8">
                    <div className={`text-center p-12 border-4 rounded-xl ${getScoreBgColor(teamScore)} shadow-lg`}>
                      <div className="text-xs uppercase tracking-widest text-tokyo-comment mb-2">
                        Overall Performance
                      </div>
                      <div className={`text-7xl font-bold mb-3 ${getScoreColor(teamScore)}`}>
                        {teamScore.toFixed(1)}
                      </div>
                      <div className="text-3xl font-semibold text-tokyo-comment mb-2">
                        out of 10
                      </div>
                      <div className={`text-xl font-bold px-6 py-2 rounded-full ${teamScore >= 9 ? 'bg-tokyo-green/20 text-tokyo-green' : teamScore >= 7 ? 'bg-tokyo-yellow/20 text-tokyo-yellow' : 'bg-tokyo-red/20 text-tokyo-red'}`}>
                        {getScoreLabel(teamScore)}
                      </div>
                    </div>
                  </div>

                  {/* Progress Bar */}
                  <div className="mb-8">
                    <div className="relative w-full h-12 bg-tokyo-dark3 rounded-xl overflow-hidden shadow-inner">
                      <div
                        className={`h-full transition-all ${teamScore >= 9 ? 'bg-tokyo-green' : teamScore >= 7 ? 'bg-tokyo-yellow' : 'bg-tokyo-red'}`}
                        style={{ width: `${(teamScore / 10) * 100}%` }}
                      />
                      <div className="absolute inset-0 flex items-center justify-between px-6 text-sm font-semibold text-tokyo-fg">
                        <span>0</span>
                        <span>7 (Passing)</span>
                        <span>9 (Excellent)</span>
                        <span>10</span>
                      </div>
                    </div>
                  </div>
                </>
              );
            })()}

            {/* Summary Text */}
            {(() => {
              const summary = getSqlValue(report.executive_summary);
              return summary && (
              <div className="mb-8">
                <h3 className="text-lg font-semibold text-tokyo-fg mb-3">Analysis</h3>
                <p className="text-base text-tokyo-comment leading-relaxed whitespace-pre-wrap">
                  {summary}
                </p>
              </div>
              );
            })()}

            {/* Key Findings */}
            <div className="grid grid-cols-3 gap-6">
              <div className="p-6 bg-tokyo-bg border-2 border-tokyo-green/30 rounded-lg shadow-lg">
                <div className="flex items-center gap-3 mb-3">
                  <CheckCircle className="h-7 w-7 text-tokyo-green" />
                  <span className="text-base font-semibold text-tokyo-green">Passed</span>
                </div>
                <div className="text-4xl font-bold text-tokyo-fg mb-1">
                  {passedAgents}
                </div>
                <div className="text-sm text-tokyo-comment">
                  out of {agentDetails.length} agents
                </div>
              </div>
              
              <div className="p-6 bg-tokyo-bg border-2 border-tokyo-red/30 rounded-lg shadow-lg">
                <div className="flex items-center gap-3 mb-3">
                  <XCircle className="h-7 w-7 text-tokyo-red" />
                  <span className="text-base font-semibold text-tokyo-red">Failed</span>
                </div>
                <div className="text-4xl font-bold text-tokyo-fg mb-1">
                  {failedAgents}
                </div>
                <div className="text-sm text-tokyo-comment">
                  out of {agentDetails.length} agents
                </div>
              </div>
              
              <div className="p-6 bg-tokyo-bg border-2 border-tokyo-blue/30 rounded-lg shadow-lg">
                <div className="flex items-center gap-3 mb-3">
                  <TrendingUp className="h-7 w-7 text-tokyo-blue" />
                  <span className="text-base font-semibold text-tokyo-blue">Success Rate</span>
                </div>
                <div className="text-4xl font-bold text-tokyo-fg mb-1">
                  {agentDetails.length > 0 ? Math.round((passedAgents / agentDetails.length) * 100) : 0}%
                </div>
                <div className="text-sm text-tokyo-comment">
                  overall performance
                </div>
              </div>
            </div>
          </div>
          );
        })()}

        {/* Criteria Performance */}
        {teamCriteriaScores && Object.keys(teamCriteriaScores).length > 0 && (
          <div className="p-6 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
            <h2 className="text-2xl font-semibold text-tokyo-cyan mb-6">
              Detailed Criteria Breakdown
            </h2>
            
            <div className="space-y-4">
              {Object.entries(teamCriteriaScores).map(([criterionName, criterionScore]) => {
                const criterion = teamCriteria?.criteria[criterionName];
                const weight = criterion?.weight || 0;
                const threshold = criterion?.threshold || 7.0;
                const score = criterionScore.score;
                const passed = score >= threshold;

                return (
                  <div key={criterionName} className="p-4 bg-tokyo-bg border border-tokyo-dark3 rounded">
                    {/* Header */}
                    <div className="flex items-start justify-between mb-3">
                      <div className="flex-1">
                        <div className="flex items-center gap-3 mb-1">
                          <h3 className="text-base font-mono font-semibold text-tokyo-fg capitalize">
                            {criterionName.replace(/_/g, ' ')}
                          </h3>
                          <span className="text-xs font-mono text-tokyo-comment">
                            ({(weight * 100).toFixed(0)}% weight)
                          </span>
                          {passed ? (
                            <CheckCircle className="h-4 w-4 text-tokyo-green" />
                          ) : (
                            <XCircle className="h-4 w-4 text-tokyo-red" />
                          )}
                        </div>
                        {criterion?.description && (
                          <p className="text-xs text-tokyo-comment font-mono">{criterion.description}</p>
                        )}
                      </div>
                      
                      <div className={`text-xl font-mono font-bold ${getScoreColor(score)}`}>
                        {score.toFixed(1)}/10
                      </div>
                    </div>

                    {/* Progress Bar */}
                    <div className="relative w-full h-2 bg-tokyo-dark3 rounded-full overflow-hidden mb-3">
                      <div
                        className={`h-full ${score >= 9 ? 'bg-tokyo-green' : score >= 7 ? 'bg-tokyo-yellow' : 'bg-tokyo-red'}`}
                        style={{ width: `${(score / 10) * 100}%` }}
                      />
                    </div>

                    <div className="flex items-center gap-2 text-xs font-mono mb-3">
                      <span className="text-tokyo-comment">Threshold:</span>
                      <span className={passed ? 'text-tokyo-green' : 'text-tokyo-red'}>
                        {threshold.toFixed(1)}/10
                      </span>
                      <span className={`px-2 py-0.5 rounded ${passed ? 'bg-tokyo-green/20 text-tokyo-green' : 'bg-tokyo-red/20 text-tokyo-red'}`}>
                        {passed ? 'Exceeded' : 'Below Target'}
                      </span>
                    </div>

                    {/* Reasoning */}
                    {criterionScore.reasoning && (
                      <p className="text-sm text-tokyo-comment font-mono leading-relaxed">
                        {criterionScore.reasoning}
                      </p>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* Agent Performance Table */}
        {agentDetails.length > 0 && (
          <div className="p-6 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
            <h2 className="text-xl font-mono font-semibold text-tokyo-cyan mb-4">
              Agent Performance ({agentDetails.length} agents)
            </h2>
            
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="border-b border-tokyo-dark3">
                    <th className="text-left py-3 px-4 text-sm font-mono text-tokyo-comment">Status</th>
                    <th className="text-left py-3 px-4 text-sm font-mono text-tokyo-comment">Agent Name</th>
                    <th className="text-center py-3 px-4 text-sm font-mono text-tokyo-comment">Score</th>
                    <th className="text-center py-3 px-4 text-sm font-mono text-tokyo-comment">Runs</th>
                    <th className="text-center py-3 px-4 text-sm font-mono text-tokyo-comment">Success Rate</th>
                    <th className="text-center py-3 px-4 text-sm font-mono text-tokyo-comment">Avg Duration</th>
                  </tr>
                </thead>
                <tbody>
                  {agentDetails
                    .sort((a, b) => b.score - a.score)
                    .map((agent) => (
                      <tr key={agent.id} className="border-b border-tokyo-dark3 hover:bg-tokyo-bg-highlight transition-colors">
                        <td className="py-3 px-4">
                          {agent.passed ? (
                            <CheckCircle className="h-4 w-4 text-tokyo-green" />
                          ) : (
                            <XCircle className="h-4 w-4 text-tokyo-red" />
                          )}
                        </td>
                        <td className="py-3 px-4">
                          <div className="font-mono text-sm text-tokyo-fg">{agent.agent_name}</div>
                        </td>
                        <td className="py-3 px-4 text-center">
                          <span className={`text-sm font-mono font-bold ${getScoreColor(agent.score)}`}>
                            {agent.score.toFixed(1)}
                          </span>
                        </td>
                        <td className="py-3 px-4 text-center">
                          <span className="text-sm font-mono text-tokyo-fg">
                            {getSqlValue(agent.runs_analyzed) || 0}
                          </span>
                        </td>
                        <td className="py-3 px-4 text-center">
                          <span className="text-sm font-mono text-tokyo-fg">
                            {(() => {
                              const rate = getSqlValue(agent.success_rate);
                              return rate !== undefined ? `${(rate * 100).toFixed(0)}%` : 'N/A';
                            })()}
                          </span>
                        </td>
                        <td className="py-3 px-4 text-center">
                          <span className="text-sm font-mono text-tokyo-fg">
                            {(() => {
                              const duration = getSqlValue(agent.avg_duration_seconds);
                              return duration !== undefined ? `${duration.toFixed(1)}s` : 'N/A';
                            })()}
                          </span>
                        </td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </div>
          </div>
        )}

        {/* Enterprise Agent Analysis - Temporarily disabled to isolate PDF issue */}
        {agentDetails.length > 0 && (
          <div className="space-y-6">
            <h2 className="text-2xl font-mono font-bold text-tokyo-cyan flex items-center gap-3">
              <span>📊</span>
              <span>Enterprise Performance Analysis</span>
            </h2>
            <div className="p-6 bg-tokyo-bg-dark border border-tokyo-blue rounded-lg">
              <p className="text-tokyo-fg text-lg mb-4">📄 Enterprise Analysis Available in PDF Export</p>
              <p className="text-tokyo-comment">
                The detailed enterprise analysis section is being optimized. 
                Click the <span className="text-tokyo-green font-bold">"Export PDF"</span> button above to view the complete report with:
              </p>
              <ul className="list-disc list-inside mt-4 space-y-2 text-tokyo-comment">
                <li>Cost projection charts across multiple frequencies</li>
                <li>Agent performance comparison charts</li>
                <li>Best vs worst run examples with full details</li>
                <li>Tool usage statistics and success rates</li>
                <li>Failure pattern analysis</li>
                <li>Actionable improvement recommendations</li>
              </ul>
            </div>
          </div>
        )}

        {/* Detailed Test Results Breakdown - Grouped by Agent */}
        {agentDetails.length > 0 && (
          <div className="p-6 bg-tokyo-bg-dark border border-tokyo-blue7 rounded-lg">
            <h2 className="text-xl font-mono font-semibold text-tokyo-cyan mb-4">
              🔬 Detailed Test Results by Agent
            </h2>
            <p className="text-sm text-tokyo-comment font-mono mb-6">
              Expand any agent to see all runs and their complete quality metric breakdowns
            </p>

            <div className="space-y-6">
              {agentDetails
                .filter(agent => getSqlValue(agent.runs_analyzed) > 0)
                .map((agent) => {
                  const runIdsStr = getSqlValue(agent.run_ids);
                  if (!runIdsStr) return null;
                  
                  // Parse run IDs
                  let runIds: number[] = [];
                  try {
                    if (runIdsStr.startsWith('[')) {
                      runIds = runIdsStr
                        .replace(/[\[\]]/g, '')
                        .split(' ')
                        .map((id: string) => parseInt(id.trim()))
                        .filter((id: number) => !isNaN(id));
                    } else {
                      runIds = JSON.parse(runIdsStr);
                    }
                  } catch (e) {
                    console.error('Failed to parse run IDs:', e);
                    return null;
                  }

                  return (
                    <div key={agent.id} className="border-2 border-tokyo-blue/30 rounded-lg bg-tokyo-bg">
                      {/* Agent Header */}
                      <div className="p-4 bg-tokyo-blue/10 border-b border-tokyo-blue/30">
                        <div className="flex items-center justify-between">
                          <div>
                            <h3 className="text-lg font-mono font-bold text-tokyo-cyan">
                              {agent.agent_name}
                            </h3>
                            <div className="text-sm font-mono text-tokyo-comment mt-1">
                              {runIds.length} runs evaluated • Overall Score: {agent.score.toFixed(1)}/10
                            </div>
                          </div>
                          <div className={`px-4 py-2 rounded ${agent.passed ? 'bg-tokyo-green/20 text-tokyo-green' : 'bg-tokyo-red/20 text-tokyo-red'}`}>
                            <div className="text-xs font-mono font-semibold">
                              {agent.passed ? '✓ PASSED' : '✗ FAILED'}
                            </div>
                          </div>
                        </div>
                      </div>

                      {/* Runs List */}
                      <div className="p-4 space-y-2">
                        {runIds.map((runId) => {
                          const isExpanded = expandedRun === runId;
                          const metrics = runMetrics[runId] || [];
                          const isLoading = loadingMetrics.has(runId);

                          return (
                            <div key={runId} className="border border-tokyo-dark3 rounded bg-tokyo-bg-dark/30">
                              <button
                                onClick={() => fetchRunMetrics(runId)}
                                disabled={isLoading}
                                className="w-full p-3 flex items-center justify-between hover:bg-tokyo-bg-highlight transition-colors disabled:opacity-50 disabled:cursor-wait"
                              >
                                <div className="flex items-center gap-3">
                                  {isLoading ? (
                                    <div className="h-4 w-4 border-2 border-tokyo-cyan border-t-transparent rounded-full animate-spin" />
                                  ) : isExpanded ? (
                                    <ChevronDown className="h-4 w-4 text-tokyo-cyan" />
                                  ) : (
                                    <ChevronRight className="h-4 w-4 text-tokyo-comment" />
                                  )}
                                  <div className="text-left">
                                    <div className="text-sm font-mono font-semibold text-tokyo-fg">
                                      Run #{runId}
                                    </div>
                                    <div className="text-xs font-mono text-tokyo-comment mt-0.5">
                                      {isLoading ? (
                                        'Loading quality metrics...'
                                      ) : metrics.length > 0 ? (
                                        <>
                                          {metrics.filter((m: any) => m.passed).length}/{metrics.length} tests passed •
                                          ${metrics.reduce((sum: number, m: any) => sum + (m.judge_cost || 0), 0).toFixed(6)} cost
                                        </>
                                      ) : (
                                        'Click to load 5 quality metrics'
                                      )}
                                    </div>
                                  </div>
                                </div>
                                <div className="text-xs font-mono text-tokyo-cyan">
                                  {isLoading ? 'Loading...' : isExpanded ? 'Hide' : 'Show Tests'}
                                </div>
                              </button>

                        {isExpanded && (
                          <div className="p-4 border-t border-tokyo-dark3 bg-tokyo-bg-dark/50">
                            {metrics.length === 0 ? (
                              <div className="text-center py-8">
                                <div className="text-tokyo-comment mb-2">
                                  ℹ️ No individual benchmark metrics available for this run
                                </div>
                                <div className="text-xs text-tokyo-comment">
                                  To evaluate this run individually, use: <code className="bg-tokyo-bg px-2 py-1 rounded">stn benchmark evaluate {runId}</code>
                                </div>
                              </div>
                            ) : (
                              <>
                            <div className="grid grid-cols-1 gap-4">
                              {metrics.map((metric: any, idx: number) => (
                                <div
                                  key={idx}
                                  className={`p-4 rounded border-2 ${
                                    metric.passed
                                      ? 'bg-tokyo-green/10 border-tokyo-green/30'
                                      : 'bg-tokyo-red/10 border-tokyo-red/30'
                                  }`}
                                >
                                  <div className="flex items-start justify-between mb-3">
                                    <div className="flex items-center gap-3">
                                      {metric.passed ? (
                                        <CheckCircle className="h-6 w-6 text-tokyo-green flex-shrink-0" />
                                      ) : (
                                        <XCircle className="h-6 w-6 text-tokyo-red flex-shrink-0" />
                                      )}
                                      <div>
                                        <h4 className="text-base font-mono font-bold text-tokyo-fg uppercase">
                                          {metric.metric_name}
                                        </h4>
                                        <div className="text-xs text-tokyo-comment font-mono mt-1">
                                          {metric.metric_name === 'hallucination' && 'Detects fabricated information not grounded in context'}
                                          {metric.metric_name === 'relevancy' && 'Measures how directly response addresses the task'}
                                          {metric.metric_name === 'task_completion' && 'Evaluates if agent fully completed the request'}
                                          {metric.metric_name === 'faithfulness' && 'Ensures responses grounded in available context'}
                                          {metric.metric_name === 'toxicity' && 'Detects harmful, offensive, or inappropriate content'}
                                        </div>
                                      </div>
                                    </div>
                                    <div className="text-right">
                                      <div className={`text-3xl font-mono font-bold ${metric.passed ? 'text-tokyo-green' : 'text-tokyo-red'}`}>
                                        {(metric.score * 100).toFixed(0)}%
                                      </div>
                                      <div className="text-xs font-mono text-tokyo-comment mt-1">
                                        Threshold: {(metric.threshold * 100).toFixed(0)}%
                                      </div>
                                    </div>
                                  </div>

                                  <div className="bg-tokyo-bg/50 rounded p-3 mb-3">
                                    <div className="text-xs font-mono text-tokyo-comment uppercase tracking-wide mb-2">
                                      LLM Judge Reasoning:
                                    </div>
                                    <div className="text-sm text-tokyo-fg font-mono leading-relaxed">
                                      {metric.reason || 'No reason provided'}
                                    </div>
                                  </div>

                                  <div className="grid grid-cols-3 gap-3 text-xs font-mono">
                                    <div className="bg-tokyo-bg/30 rounded p-2">
                                      <div className="text-tokyo-comment mb-1">Judge Tokens</div>
                                      <div className="text-tokyo-fg font-semibold">
                                        {metric.judge_tokens || 0}
                                        {metric.judge_tokens === 0 && (
                                          <span className="text-tokyo-comment text-xs ml-1">*</span>
                                        )}
                                      </div>
                                    </div>
                                    <div className="bg-tokyo-bg/30 rounded p-2">
                                      <div className="text-tokyo-comment mb-1">Judge Cost</div>
                                      <div className="text-tokyo-fg font-semibold">
                                        ${(metric.judge_cost || 0).toFixed(6)}
                                        {metric.judge_cost === 0 && (
                                          <span className="text-tokyo-comment text-xs ml-1">*</span>
                                        )}
                                      </div>
                                    </div>
                                    <div className="bg-tokyo-bg/30 rounded p-2">
                                      <div className="text-tokyo-comment mb-1">Status</div>
                                      <div className={`font-semibold ${metric.passed ? 'text-tokyo-green' : 'text-tokyo-red'}`}>
                                        {metric.passed ? 'PASSED ✓' : 'FAILED ✗'}
                                      </div>
                                    </div>
                                  </div>
                                  
                                  {(metric.judge_tokens === 0 || metric.judge_cost === 0) && (
                                    <div className="mt-2 text-xs text-tokyo-comment font-mono italic">
                                      * Zero cost: No LLM evaluation needed (no tool outputs to analyze for hallucination/faithfulness)
                                    </div>
                                  )}
                                </div>
                              ))}
                            </div>

                            <div className="mt-4 p-3 bg-tokyo-blue/10 border border-tokyo-blue/30 rounded">
                              <div className="text-xs font-mono text-tokyo-comment">
                                💡 <strong>Summary:</strong> This run was evaluated across {metrics.length} quality dimensions. 
                                {metrics.filter((m: any) => m.passed).length === metrics.length
                                  ? ' All tests passed! ✅'
                                  : ` ${metrics.filter((m: any) => !m.passed).length} test(s) failed. ⚠️`}
                                {' '}
                                Total cost: ${metrics.reduce((sum: number, m: any) => sum + (m.judge_cost || 0), 0).toFixed(6)}
                              </div>
                            </div>
                              </>
                            )}
                          </div>
                        )}
                            </div>
                          );
                        })}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
      </div>
    </div>
    </>
  );
};

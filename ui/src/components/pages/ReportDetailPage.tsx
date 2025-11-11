import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Download, CheckCircle, XCircle, AlertTriangle, TrendingUp, TrendingDown, Minus } from 'lucide-react';
import { reportsApi } from '../../api/station';
import type { Report, AgentReportDetail, CriterionScore } from '../../types/station';

interface TeamCriteria {
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

      try {
        setLoading(true);
        const response = await reportsApi.getById(parseInt(reportId));
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
            
            <button className="flex items-center gap-2 px-4 py-2 bg-tokyo-green text-tokyo-bg hover:bg-opacity-90 rounded font-mono text-sm">
              <Download className="h-4 w-4" />
              Export PDF
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
                <h2 className="text-xl font-semibold text-tokyo-cyan mb-2">
                  Benchmark Goal
                </h2>
                <p className="text-base text-tokyo-fg leading-relaxed mb-4">
                  {teamCriteria.goal}
                </p>
                
                <div className="bg-tokyo-bg/50 rounded-lg p-4 border border-tokyo-dark3">
                  <h3 className="text-sm font-semibold text-tokyo-comment mb-3 uppercase tracking-wide">
                    Success Criteria
                  </h3>
                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
                    {Object.entries(teamCriteria.criteria).map(([name, config]) => (
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
                          <div className="text-xs text-tokyo-comment">
                            Threshold: {config.threshold.toFixed(1)}/10
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
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
      </div>
    </div>
  );
};

import React, { useState } from 'react';
import { ChevronDown, ChevronRight, TrendingUp, TrendingDown, AlertTriangle, CheckCircle, Clock, DollarSign, Zap, Wrench } from 'lucide-react';

interface RunExample {
  run_id: number;
  input: string;
  output: string;
  tool_calls: string[];
  duration: number;
  token_count: number;
  status: string;
  explanation: string;
}

interface ToolUsageStats {
  tool_name: string;
  use_count: number;
  success_rate: number;
  avg_duration: number;
}

interface FailurePattern {
  pattern: string;
  frequency: number;
  examples: string[];
  impact: string;
}

interface ImprovementAction {
  issue: string;
  recommendation: string;
  priority: string;
  expected_impact: string;
  concrete_example: string;
}

interface EnterpriseAgentData {
  agent_name: string;
  score: number;
  runs_analyzed: number;
  best_run_example?: string;
  worst_run_example?: string;
  tool_usage_analysis?: string;
  failure_patterns?: string;
  improvement_plan?: string;
  avg_cost?: number;
  avg_duration_seconds?: number;
  avg_tokens?: number;
  success_rate?: number;
}

interface EnterpriseAgentAnalysisProps {
  agent: EnterpriseAgentData;
}

export const EnterpriseAgentAnalysis: React.FC<EnterpriseAgentAnalysisProps> = ({ agent }) => {
  const [expandedSection, setExpandedSection] = useState<string | null>('performance');

  // Helper to parse SQL NullString fields
  const parseSqlJson = (field: any): any => {
    if (!field) return null;
    if (typeof field === 'object' && 'Valid' in field && field.Valid && 'String' in field) {
      try {
        return JSON.parse(field.String);
      } catch {
        return null;
      }
    }
    if (typeof field === 'string') {
      try {
        return JSON.parse(field);
      } catch {
        return null;
      }
    }
    return null;
  };

  // Helper to safely convert to number
  const toNumber = (value: any, defaultValue: number = 0): number => {
    if (value === undefined || value === null) return defaultValue;
    const num = Number(value);
    return isNaN(num) ? defaultValue : num;
  };

  const bestRun: RunExample | null = parseSqlJson(agent.best_run_example);
  const worstRun: RunExample | null = parseSqlJson(agent.worst_run_example);
  const toolUsage: ToolUsageStats[] | null = parseSqlJson(agent.tool_usage_analysis);
  const failures: FailurePattern[] | null = parseSqlJson(agent.failure_patterns);
  const improvements: ImprovementAction[] | null = parseSqlJson(agent.improvement_plan);

  const toggleSection = (section: string) => {
    setExpandedSection(expandedSection === section ? null : section);
  };

  const getPriorityColor = (priority: string) => {
    switch (priority.toLowerCase()) {
      case 'high': return 'text-tokyo-red bg-tokyo-red/20';
      case 'medium': return 'text-tokyo-yellow bg-tokyo-yellow/20';
      case 'low': return 'text-tokyo-green bg-tokyo-green/20';
      default: return 'text-tokyo-comment bg-tokyo-comment/20';
    }
  };

  const getImpactColor = (impact: string) => {
    switch (impact.toLowerCase()) {
      case 'high': return 'text-tokyo-red';
      case 'medium': return 'text-tokyo-yellow';
      case 'low': return 'text-tokyo-green';
      default: return 'text-tokyo-comment';
    }
  };

  return (
    <div className="border-2 border-tokyo-blue/30 rounded-lg bg-tokyo-bg overflow-hidden">
      {/* Agent Header */}
      <div className="p-6 bg-gradient-to-r from-tokyo-blue/10 to-tokyo-purple/10 border-b border-tokyo-blue/30">
        <div className="flex items-start justify-between">
          <div>
            <h3 className="text-2xl font-mono font-bold text-tokyo-cyan mb-2">
              {agent.agent_name}
            </h3>
            <div className="flex items-center gap-4 text-sm font-mono text-tokyo-comment">
              <span>{agent.runs_analyzed} runs analyzed</span>
              <span>•</span>
              <span className="text-tokyo-cyan font-bold">{agent.score.toFixed(1)}/10</span>
              <span>•</span>
              <span>{(toNumber(agent.success_rate) * 100).toFixed(0)}% success rate</span>
            </div>
          </div>
          <div className="flex gap-3">
            <div className="px-4 py-2 rounded-lg bg-tokyo-bg border border-tokyo-blue/40">
              <div className="text-xs font-mono text-tokyo-comment">Avg Cost</div>
              <div className="text-lg font-mono font-bold text-tokyo-green">
                ${toNumber(agent.avg_cost).toFixed(4)}
              </div>
            </div>
            <div className="px-4 py-2 rounded-lg bg-tokyo-bg border border-tokyo-blue/40">
              <div className="text-xs font-mono text-tokyo-comment">Avg Duration</div>
              <div className="text-lg font-mono font-bold text-tokyo-cyan">
                {toNumber(agent.avg_duration_seconds).toFixed(1)}s
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="p-6 space-y-4">
        {/* Performance Overview Section */}
        <div className="border border-tokyo-blue/30 rounded-lg overflow-hidden">
          <button
            onClick={() => toggleSection('performance')}
            className="w-full p-4 flex items-center justify-between bg-tokyo-bg-dark hover:bg-tokyo-bg-highlight transition-colors"
          >
            <div className="flex items-center gap-3">
              <TrendingUp className="h-5 w-5 text-tokyo-cyan" />
              <span className="text-lg font-mono font-semibold text-tokyo-fg">Performance Metrics</span>
            </div>
            {expandedSection === 'performance' ? (
              <ChevronDown className="h-5 w-5 text-tokyo-cyan" />
            ) : (
              <ChevronRight className="h-5 w-5 text-tokyo-comment" />
            )}
          </button>
          {expandedSection === 'performance' && (
            <div className="p-4 bg-tokyo-bg border-t border-tokyo-blue/30">
              <div className="grid grid-cols-2 gap-4">
                <div className="p-4 bg-tokyo-bg-dark rounded-lg border border-tokyo-dark3">
                  <div className="flex items-center gap-2 mb-2">
                    <Zap className="h-4 w-4 text-tokyo-yellow" />
                    <span className="text-xs font-mono text-tokyo-comment">Avg Tokens</span>
                  </div>
                  <div className="text-2xl font-mono font-bold text-tokyo-fg">
                    {toNumber(agent.avg_tokens).toLocaleString()}
                  </div>
                </div>
                <div className="p-4 bg-tokyo-bg-dark rounded-lg border border-tokyo-dark3">
                  <div className="flex items-center gap-2 mb-2">
                    <DollarSign className="h-4 w-4 text-tokyo-green" />
                    <span className="text-xs font-mono text-tokyo-comment">Cost per Run</span>
                  </div>
                  <div className="text-2xl font-mono font-bold text-tokyo-fg">
                    ${toNumber(agent.avg_cost).toFixed(4)}
                  </div>
                </div>
                <div className="p-4 bg-tokyo-bg-dark rounded-lg border border-tokyo-dark3">
                  <div className="flex items-center gap-2 mb-2">
                    <Clock className="h-4 w-4 text-tokyo-cyan" />
                    <span className="text-xs font-mono text-tokyo-comment">Execution Time</span>
                  </div>
                  <div className="text-2xl font-mono font-bold text-tokyo-fg">
                    {toNumber(agent.avg_duration_seconds).toFixed(2)}s
                  </div>
                </div>
                <div className="p-4 bg-tokyo-bg-dark rounded-lg border border-tokyo-dark3">
                  <div className="flex items-center gap-2 mb-2">
                    <CheckCircle className="h-4 w-4 text-tokyo-green" />
                    <span className="text-xs font-mono text-tokyo-comment">Success Rate</span>
                  </div>
                  <div className="text-2xl font-mono font-bold text-tokyo-fg">
                    {(toNumber(agent.success_rate) * 100).toFixed(0)}%
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* Best/Worst Run Comparison */}
        {(bestRun || worstRun) && (
          <div className="border border-tokyo-blue/30 rounded-lg overflow-hidden">
            <button
              onClick={() => toggleSection('comparison')}
              className="w-full p-4 flex items-center justify-between bg-tokyo-bg-dark hover:bg-tokyo-bg-highlight transition-colors"
            >
              <div className="flex items-center gap-3">
                <TrendingUp className="h-5 w-5 text-tokyo-green" />
                <span className="text-lg font-mono font-semibold text-tokyo-fg">Best vs Worst Performance</span>
              </div>
              {expandedSection === 'comparison' ? (
                <ChevronDown className="h-5 w-5 text-tokyo-cyan" />
              ) : (
                <ChevronRight className="h-5 w-5 text-tokyo-comment" />
              )}
            </button>
            {expandedSection === 'comparison' && (
              <div className="p-4 bg-tokyo-bg border-t border-tokyo-blue/30">
                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  {/* Best Run */}
                  {bestRun && (
                    <div className="border-2 border-tokyo-green/30 rounded-lg p-4 bg-tokyo-green/5">
                      <div className="flex items-center gap-2 mb-3">
                        <CheckCircle className="h-5 w-5 text-tokyo-green" />
                        <span className="text-sm font-mono font-bold text-tokyo-green">BEST PERFORMING RUN</span>
                      </div>
                      <div className="space-y-3">
                        <div>
                          <div className="text-xs font-mono text-tokyo-comment mb-1">Run ID</div>
                          <div className="text-sm font-mono text-tokyo-fg">#{bestRun.run_id}</div>
                        </div>
                        <div>
                          <div className="text-xs font-mono text-tokyo-comment mb-1">Performance</div>
                          <div className="flex gap-4 text-sm font-mono text-tokyo-fg">
                            <span>{bestRun.duration ? bestRun.duration.toFixed(1) : '0'}s</span>
                            <span>•</span>
                            <span>{bestRun.token_count ? bestRun.token_count.toLocaleString() : '0'} tokens</span>
                          </div>
                        </div>
                        <div>
                          <div className="text-xs font-mono text-tokyo-comment mb-1">Tools Used</div>
                          <div className="flex flex-wrap gap-1">
                            {bestRun.tool_calls && bestRun.tool_calls.length > 0 ? bestRun.tool_calls.map((tool, idx) => (
                              <span key={idx} className="px-2 py-1 text-xs font-mono bg-tokyo-blue/20 text-tokyo-cyan rounded">
                                {tool.replace('__', '')}
                              </span>
                            )) : <span className="text-xs text-tokyo-comment">No tools used</span>}
                          </div>
                        </div>
                        <div>
                          <div className="text-xs font-mono text-tokyo-comment mb-1">Why It Succeeded</div>
                          <div className="text-xs font-mono text-tokyo-fg italic">"{bestRun.explanation}"</div>
                        </div>
                        <div>
                          <div className="text-xs font-mono text-tokyo-comment mb-1">Input</div>
                          <div className="text-xs font-mono text-tokyo-fg bg-tokyo-bg-dark p-2 rounded max-h-20 overflow-y-auto">
                            {bestRun.input}
                          </div>
                        </div>
                      </div>
                    </div>
                  )}

                  {/* Worst Run */}
                  {worstRun && (
                    <div className="border-2 border-tokyo-red/30 rounded-lg p-4 bg-tokyo-red/5">
                      <div className="flex items-center gap-2 mb-3">
                        <AlertTriangle className="h-5 w-5 text-tokyo-red" />
                        <span className="text-sm font-mono font-bold text-tokyo-red">NEEDS IMPROVEMENT</span>
                      </div>
                      <div className="space-y-3">
                        <div>
                          <div className="text-xs font-mono text-tokyo-comment mb-1">Run ID</div>
                          <div className="text-sm font-mono text-tokyo-fg">#{worstRun.run_id}</div>
                        </div>
                        <div>
                          <div className="text-xs font-mono text-tokyo-comment mb-1">Performance</div>
                          <div className="flex gap-4 text-sm font-mono text-tokyo-fg">
                            <span>{worstRun.duration ? worstRun.duration.toFixed(1) : '0'}s</span>
                            <span>•</span>
                            <span>{worstRun.token_count ? worstRun.token_count.toLocaleString() : '0'} tokens</span>
                          </div>
                        </div>
                        <div>
                          <div className="text-xs font-mono text-tokyo-comment mb-1">Tools Used</div>
                          <div className="flex flex-wrap gap-1">
                            {worstRun.tool_calls && worstRun.tool_calls.length > 0 ? worstRun.tool_calls.map((tool, idx) => (
                              <span key={idx} className="px-2 py-1 text-xs font-mono bg-tokyo-blue/20 text-tokyo-cyan rounded">
                                {tool.replace('__', '')}
                              </span>
                            )) : <span className="text-xs text-tokyo-comment">No tools used</span>}
                          </div>
                        </div>
                        <div>
                          <div className="text-xs font-mono text-tokyo-comment mb-1">What Went Wrong</div>
                          <div className="text-xs font-mono text-tokyo-fg italic">"{worstRun.explanation}"</div>
                        </div>
                        <div>
                          <div className="text-xs font-mono text-tokyo-comment mb-1">Input</div>
                          <div className="text-xs font-mono text-tokyo-fg bg-tokyo-bg-dark p-2 rounded max-h-20 overflow-y-auto">
                            {worstRun.input}
                          </div>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Tool Usage Analysis */}
        {toolUsage && toolUsage.length > 0 && (
          <div className="border border-tokyo-blue/30 rounded-lg overflow-hidden">
            <button
              onClick={() => toggleSection('tools')}
              className="w-full p-4 flex items-center justify-between bg-tokyo-bg-dark hover:bg-tokyo-bg-highlight transition-colors"
            >
              <div className="flex items-center gap-3">
                <Wrench className="h-5 w-5 text-tokyo-purple" />
                <span className="text-lg font-mono font-semibold text-tokyo-fg">Tool Usage Statistics</span>
              </div>
              {expandedSection === 'tools' ? (
                <ChevronDown className="h-5 w-5 text-tokyo-cyan" />
              ) : (
                <ChevronRight className="h-5 w-5 text-tokyo-comment" />
              )}
            </button>
            {expandedSection === 'tools' && (
              <div className="p-4 bg-tokyo-bg border-t border-tokyo-blue/30">
                <div className="space-y-2">
                  {toolUsage.map((tool, idx) => (
                    <div key={idx} className="p-3 bg-tokyo-bg-dark rounded-lg border border-tokyo-dark3">
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-sm font-mono font-semibold text-tokyo-cyan">
                          {tool.tool_name.replace('__', '')}
                        </span>
                        <span className="text-xs font-mono text-tokyo-comment">
                          Used {tool.use_count} times
                        </span>
                      </div>
                      <div className="flex items-center gap-4 text-xs font-mono text-tokyo-fg">
                        <div className="flex items-center gap-1">
                          <span className="text-tokyo-comment">Success Rate:</span>
                          <span className={tool.success_rate >= 0.8 ? 'text-tokyo-green' : 'text-tokyo-yellow'}>
                            {(tool.success_rate * 100).toFixed(0)}%
                          </span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Failure Patterns */}
        {failures && failures.length > 0 && (
          <div className="border border-tokyo-red/30 rounded-lg overflow-hidden">
            <button
              onClick={() => toggleSection('failures')}
              className="w-full p-4 flex items-center justify-between bg-tokyo-red/10 hover:bg-tokyo-red/20 transition-colors"
            >
              <div className="flex items-center gap-3">
                <AlertTriangle className="h-5 w-5 text-tokyo-red" />
                <span className="text-lg font-mono font-semibold text-tokyo-fg">Failure Patterns Detected</span>
              </div>
              {expandedSection === 'failures' ? (
                <ChevronDown className="h-5 w-5 text-tokyo-cyan" />
              ) : (
                <ChevronRight className="h-5 w-5 text-tokyo-comment" />
              )}
            </button>
            {expandedSection === 'failures' && (
              <div className="p-4 bg-tokyo-bg border-t border-tokyo-red/30">
                <div className="space-y-3">
                  {failures.map((failure, idx) => (
                    <div key={idx} className="p-4 bg-tokyo-red/5 border border-tokyo-red/30 rounded-lg">
                      <div className="flex items-start justify-between mb-2">
                        <div className="flex-1">
                          <div className="text-sm font-mono font-semibold text-tokyo-red mb-1">
                            {failure.pattern}
                          </div>
                          <div className="text-xs font-mono text-tokyo-comment">
                            Occurred {failure.frequency} times • Impact: <span className={getImpactColor(failure.impact)}>{failure.impact}</span>
                          </div>
                        </div>
                      </div>
                      <div className="mt-2">
                        <div className="text-xs font-mono text-tokyo-comment mb-1">Affected Runs:</div>
                        <div className="flex flex-wrap gap-1">
                          {failure.examples.map((runId, i) => (
                            <span key={i} className="px-2 py-1 text-xs font-mono bg-tokyo-bg-dark text-tokyo-red rounded">
                              #{runId}
                            </span>
                          ))}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Improvement Plan */}
        {improvements && improvements.length > 0 && (
          <div className="border border-tokyo-yellow/30 rounded-lg overflow-hidden">
            <button
              onClick={() => toggleSection('improvements')}
              className="w-full p-4 flex items-center justify-between bg-tokyo-yellow/10 hover:bg-tokyo-yellow/20 transition-colors"
            >
              <div className="flex items-center gap-3">
                <TrendingUp className="h-5 w-5 text-tokyo-yellow" />
                <span className="text-lg font-mono font-semibold text-tokyo-fg">Actionable Improvement Plan</span>
              </div>
              {expandedSection === 'improvements' ? (
                <ChevronDown className="h-5 w-5 text-tokyo-cyan" />
              ) : (
                <ChevronRight className="h-5 w-5 text-tokyo-comment" />
              )}
            </button>
            {expandedSection === 'improvements' && (
              <div className="p-4 bg-tokyo-bg border-t border-tokyo-yellow/30">
                <div className="space-y-3">
                  {improvements.map((action, idx) => (
                    <div key={idx} className="p-4 bg-tokyo-bg-dark border border-tokyo-yellow/30 rounded-lg">
                      <div className="flex items-start justify-between mb-3">
                        <div className="flex-1">
                          <div className="flex items-center gap-2 mb-2">
                            <span className={`px-2 py-1 text-xs font-mono font-bold rounded ${getPriorityColor(action.priority)}`}>
                              {action.priority} PRIORITY
                            </span>
                            <span className="text-xs font-mono text-tokyo-green">
                              Expected: {action.expected_impact}
                            </span>
                          </div>
                          <div className="text-sm font-mono font-semibold text-tokyo-yellow mb-2">
                            Issue: {action.issue}
                          </div>
                          <div className="text-sm font-mono text-tokyo-fg mb-2">
                            → {action.recommendation}
                          </div>
                          <div className="text-xs font-mono text-tokyo-comment italic">
                            Example: {action.concrete_example}
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

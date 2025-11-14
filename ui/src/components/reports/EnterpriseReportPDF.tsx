import React from 'react';
import { Document, Page, Text, View, StyleSheet, Svg, Path, Rect, Line } from '@react-pdf/renderer';
import type { Report, AgentReportDetail, CriterionScore } from '../../types/station';

// Professional corporate colors
const colors = {
  primary: '#1e3a8a',
  secondary: '#475569',
  accent: '#3b82f6',
  success: '#059669',
  warning: '#d97706',
  danger: '#dc2626',
  text: '#1e293b',
  textLight: '#64748b',
  border: '#cbd5e1',
  bgLight: '#f8fafc',
  chartBlue: '#3b82f6',
  chartGreen: '#10b981',
  chartYellow: '#f59e0b',
  chartRed: '#ef4444',
  chartPurple: '#8b5cf6',
};

const styles = StyleSheet.create({
  page: {
    padding: 50,
    backgroundColor: '#FFFFFF',
    fontFamily: 'Helvetica',
    fontSize: 10,
    color: colors.text,
  },
  header: {
    marginBottom: 20,
    borderBottom: 2,
    borderBottomColor: colors.primary,
    paddingBottom: 10,
  },
  title: {
    fontSize: 24,
    fontWeight: 'bold',
    color: colors.primary,
    marginBottom: 4,
  },
  subtitle: {
    fontSize: 9,
    color: colors.textLight,
    marginBottom: 2,
  },
  sectionTitle: {
    fontSize: 16,
    fontWeight: 'bold',
    color: colors.primary,
    marginTop: 20,
    marginBottom: 10,
  },
  subsectionTitle: {
    fontSize: 12,
    fontWeight: 'bold',
    color: colors.secondary,
    marginTop: 12,
    marginBottom: 6,
  },
  text: {
    fontSize: 10,
    lineHeight: 1.5,
    marginBottom: 4,
  },
  scoreBox: {
    padding: 15,
    backgroundColor: colors.bgLight,
    borderRadius: 4,
    borderWidth: 1,
    borderColor: colors.border,
    marginVertical: 10,
    alignItems: 'center',
  },
  scoreValue: {
    fontSize: 32,
    fontWeight: 'bold',
    color: colors.primary,
  },
  table: {
    marginVertical: 10,
  },
  tableHeader: {
    flexDirection: 'row',
    backgroundColor: colors.primary,
    padding: 8,
    color: '#FFFFFF',
    fontWeight: 'bold',
  },
  tableRow: {
    flexDirection: 'row',
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
    padding: 8,
  },
  tableCell: {
    fontSize: 9,
  },
  metricCard: {
    flex: 1,
    padding: 10,
    backgroundColor: colors.bgLight,
    borderRadius: 4,
    borderWidth: 1,
    borderColor: colors.border,
    margin: 5,
  },
  metricLabel: {
    fontSize: 8,
    color: colors.textLight,
    marginBottom: 3,
  },
  metricValue: {
    fontSize: 16,
    fontWeight: 'bold',
    color: colors.text,
  },
  chartContainer: {
    marginVertical: 15,
    padding: 10,
    backgroundColor: colors.bgLight,
    borderRadius: 4,
    borderWidth: 1,
    borderColor: colors.border,
  },
  chartTitle: {
    fontSize: 11,
    fontWeight: 'bold',
    color: colors.text,
    marginBottom: 8,
  },
  legendItem: {
    flexDirection: 'row',
    alignItems: 'center',
    marginRight: 15,
    marginBottom: 5,
  },
  legendColor: {
    width: 12,
    height: 12,
    marginRight: 5,
    borderRadius: 2,
  },
  legendText: {
    fontSize: 8,
    color: colors.text,
  },
  footer: {
    position: 'absolute',
    bottom: 30,
    left: 50,
    right: 50,
    borderTopWidth: 1,
    borderTopColor: colors.border,
    paddingTop: 8,
    flexDirection: 'row',
    justifyContent: 'space-between',
    fontSize: 8,
    color: colors.textLight,
  },
});

// Helper functions
const getSqlValue = (field: any): any => {
  if (field === null || field === undefined) return undefined;
  if (typeof field === 'object' && 'Valid' in field) {
    if ('Float64' in field) return field.Valid ? field.Float64 : undefined;
    if ('Int64' in field) return field.Valid ? field.Int64 : undefined;
    if ('String' in field) return field.Valid ? field.String : undefined;
    if ('Time' in field) return field.Valid ? field.Time : undefined;
  }
  return field;
};

const toNumber = (value: any, defaultValue: number = 0): number => {
  const extracted = getSqlValue(value);
  if (extracted === undefined || extracted === null) return defaultValue;
  const num = Number(extracted);
  return isNaN(num) ? defaultValue : num;
};

const formatDate = (dateString: string) => {
  if (!dateString) return 'N/A';
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', { 
    year: 'numeric', 
    month: 'long', 
    day: 'numeric',
  });
};

// Cost Projection Bar Chart Component (using View/Text only)
const CostProjectionChart: React.FC<{ projections: any[] }> = ({ projections }) => {
  const maxCost = Math.max(...projections.map(p => p.total_cost));
  
  const colors_arr = [
    colors.chartBlue,
    colors.chartGreen,
    colors.chartYellow,
    colors.chartRed,
    colors.chartPurple,
    colors.accent,
  ];
  
  return (
    <View style={styles.chartContainer}>
      <Text style={styles.chartTitle}>Monthly Cost Projections at Different Frequencies</Text>
      <View style={{ flexDirection: 'row', gap: 8, marginTop: 10, height: 150, alignItems: 'flex-end' }}>
        {projections.slice(0, 6).map((proj, idx) => {
          const heightPercent = (proj.total_cost / maxCost) * 100;
          
          return (
            <View key={idx} style={{ flex: 1, alignItems: 'center' }}>
              <Text style={{ fontSize: 7, marginBottom: 2 }}>${proj.total_cost.toFixed(2)}</Text>
              <View 
                style={{ 
                  width: '100%', 
                  height: `${heightPercent}%`,
                  backgroundColor: colors_arr[idx % colors_arr.length],
                  borderRadius: 2
                }} 
              />
              <Text style={{ fontSize: 7, marginTop: 4, textAlign: 'center' }}>
                {proj.frequency.split(' ')[1] || proj.frequency.substring(0, 5)}
              </Text>
            </View>
          );
        })}
      </View>
    </View>
  );
};

// Agent Score Comparison Chart
const AgentScoreChart: React.FC<{ agents: any[] }> = ({ agents }) => {
  const chartWidth = 450;
  const chartHeight = 200;
  const barWidth = chartWidth / Math.min(agents.length, 8) - 10;
  
  return (
    <View style={styles.chartContainer}>
      <Text style={styles.chartTitle}>Agent Performance Scores (0-10 scale)</Text>
      <Svg height={chartHeight + 40} width={chartWidth + 50}>
        {/* Y-axis */}
        <Line x1="40" y1="20" x2="40" y2={chartHeight + 20} stroke={colors.border} strokeWidth="1" />
        <Line x1="40" y1={chartHeight + 20} x2={chartWidth + 40} y2={chartHeight + 20} stroke={colors.border} strokeWidth="1" />
        
        {/* Y-axis labels */}
        <Text x="5" y="25" fontSize="8" fill={colors.textLight}>10</Text>
        <Text x="10" y={chartHeight / 2 + 25} fontSize="8" fill={colors.textLight}>5</Text>
        <Text x="10" y={chartHeight + 25} fontSize="8" fill={colors.textLight}>0</Text>
        
        {/* Bars */}
        {agents.slice(0, 8).map((agent, idx) => {
          const barHeight = (agent.score / 10) * chartHeight;
          const x = 50 + idx * (barWidth + 10);
          const y = chartHeight - barHeight + 20;
          
          const barColor = agent.score >= 8 ? colors.chartGreen : 
                          agent.score >= 6 ? colors.chartYellow : colors.chartRed;
          
          return (
            <React.Fragment key={idx}>
              <Rect
                x={x}
                y={y}
                width={barWidth}
                height={barHeight}
                fill={barColor}
              />
              <Text
                x={x + barWidth / 2}
                y={y - 5}
                fontSize="9"
                fill={colors.text}
                textAnchor="middle"
                fontWeight="bold"
              >
                {agent.score.toFixed(1)}
              </Text>
            </React.Fragment>
          );
        })}
      </Svg>
      
      {/* Agent names legend below chart */}
      <View style={{ marginTop: 5, flexDirection: 'row', flexWrap: 'wrap' }}>
        {agents.slice(0, 8).map((agent, idx) => (
          <Text key={idx} style={{ fontSize: 7, color: colors.text, marginRight: 10, marginTop: 3 }}>
            {agent.agent_name.substring(0, 12)}
          </Text>
        ))}
      </View>
    </View>
  );
};

// Tool Usage Pie Chart (simplified as bars due to SVG limitations)
const ToolUsageChart: React.FC<{ toolUsage: any[] }> = ({ toolUsage }) => {
  if (!toolUsage || toolUsage.length === 0) return null;
  
  const chartWidth = 450;
  const barHeight = 25;
  const maxCount = Math.max(...toolUsage.map(t => t.use_count));
  
  return (
    <View style={styles.chartContainer}>
      <Text style={styles.chartTitle}>Tool Usage Frequency</Text>
      <View>
        {toolUsage.slice(0, 6).map((tool, idx) => {
          const barWidth = (tool.use_count / maxCount) * (chartWidth - 100);
          return (
            <View key={idx} style={{ marginBottom: 8, flexDirection: 'row', alignItems: 'center' }}>
              <Text style={{ fontSize: 8, width: 100, color: colors.text }}>
                {tool.tool_name.replace('__', '').substring(0, 15)}
              </Text>
              <Svg height={barHeight} width={chartWidth - 100}>
                <Rect
                  x="0"
                  y="0"
                  width={barWidth}
                  height={barHeight - 5}
                  fill={colors.chartBlue}
                />
                <Text
                  x={barWidth + 5}
                  y={barHeight / 2 + 3}
                  fontSize="8"
                  fill={colors.text}
                >
                  {tool.use_count} uses ({(tool.success_rate * 100).toFixed(0)}% success)
                </Text>
              </Svg>
            </View>
          );
        })}
      </View>
    </View>
  );
};

interface EnterpriseReportPDFProps {
  report: Report;
  agentDetails: AgentReportDetail[];
  teamCriteria: any;
  teamCriteriaScores: Record<string, CriterionScore> | null;
}

export const EnterpriseReportPDF: React.FC<EnterpriseReportPDFProps> = ({
  report,
  agentDetails,
  teamCriteria,
  teamCriteriaScores,
}) => {
  const teamScore = getSqlValue(report.team_score) || 0;
  const summary = getSqlValue(report.executive_summary) || '';
  const totalRuns = getSqlValue(report.total_runs_analyzed) || 0;
  const totalAgents = getSqlValue(report.total_agents_analyzed) || 0;
  
  // Parse enterprise data
  const parseSqlJson = (field: any): any => {
    const str = getSqlValue(field);
    if (!str) return null;
    try {
      return JSON.parse(str);
    } catch {
      return null;
    }
  };
  
  // Calculate team cost projections
  const avgCostPerRun = agentDetails.reduce((sum, a) => sum + toNumber(a.avg_cost), 0) / Math.max(agentDetails.length, 1);
  const avgTokensPerRun = agentDetails.reduce((sum, a) => sum + toNumber(a.avg_tokens), 0) / Math.max(agentDetails.length, 1);
  
  // Generate cost projections
  const costProjections = [
    { frequency: 'Every 5 Minutes', runs: 8640 * totalAgents, total_cost: avgCostPerRun * 8640 * totalAgents },
    { frequency: 'Hourly', runs: 720 * totalAgents, total_cost: avgCostPerRun * 720 * totalAgents },
    { frequency: 'Every 4 Hours', runs: 180 * totalAgents, total_cost: avgCostPerRun * 180 * totalAgents },
    { frequency: 'Daily', runs: 30 * totalAgents, total_cost: avgCostPerRun * 30 * totalAgents },
    { frequency: 'Weekly', runs: 4 * totalAgents, total_cost: avgCostPerRun * 4 * totalAgents },
    { frequency: 'Monthly', runs: 1 * totalAgents, total_cost: avgCostPerRun * 1 * totalAgents },
  ];
  
  return (
    <Document>
      {/* Page 1: Executive Summary */}
      <Page size="A4" style={styles.page}>
        <View style={styles.header}>
          <Text style={styles.title}>{report.name}</Text>
          <Text style={styles.subtitle}>{getSqlValue(report.description)}</Text>
          <Text style={styles.subtitle}>
            Generated: {formatDate(getSqlValue(report.created_at))} • 
            Duration: {toNumber(report.generation_duration_seconds).toFixed(1)}s • 
            Model: {getSqlValue(report.judge_model)}
          </Text>
        </View>
        
        {/* Team Score */}
        <View style={styles.scoreBox}>
          <Text style={{ fontSize: 10, color: colors.textLight, marginBottom: 5 }}>
            OVERALL TEAM SCORE
          </Text>
          <Text style={styles.scoreValue}>{teamScore.toFixed(1)}/10</Text>
        </View>
        
        {/* Executive Summary */}
        <Text style={styles.sectionTitle}>Executive Summary</Text>
        <Text style={styles.text}>{summary}</Text>
        
        {/* Key Metrics Grid */}
        <Text style={styles.sectionTitle}>Performance Overview</Text>
        <View style={{ flexDirection: 'row', flexWrap: 'wrap', marginVertical: 10 }}>
          <View style={styles.metricCard}>
            <Text style={styles.metricLabel}>Total Agents</Text>
            <Text style={styles.metricValue}>{totalAgents}</Text>
          </View>
          <View style={styles.metricCard}>
            <Text style={styles.metricLabel}>Total Runs Analyzed</Text>
            <Text style={styles.metricValue}>{totalRuns}</Text>
          </View>
          <View style={styles.metricCard}>
            <Text style={styles.metricLabel}>Avg Cost/Run</Text>
            <Text style={styles.metricValue}>${avgCostPerRun.toFixed(4)}</Text>
          </View>
          <View style={styles.metricCard}>
            <Text style={styles.metricLabel}>Avg Tokens/Run</Text>
            <Text style={styles.metricValue}>{Math.round(avgTokensPerRun).toLocaleString()}</Text>
          </View>
        </View>
        
        {/* Team Criteria Scores */}
        {teamCriteriaScores && (
          <>
            <Text style={styles.sectionTitle}>Business Criteria Performance</Text>
            {Object.entries(teamCriteriaScores).map(([key, value]) => (
              <View key={key} style={{ marginBottom: 8 }}>
                <View style={{ flexDirection: 'row', justifyContent: 'space-between', marginBottom: 3 }}>
                  <Text style={{ fontSize: 10, fontWeight: 'bold', color: colors.text }}>
                    {key.replace(/_/g, ' ').toUpperCase()}
                  </Text>
                  <Text style={{ fontSize: 10, fontWeight: 'bold', color: value.score >= 8 ? colors.success : value.score >= 6 ? colors.warning : colors.danger }}>
                    {value.score.toFixed(1)}/10
                  </Text>
                </View>
                <Text style={{ fontSize: 9, color: colors.textLight }}>
                  {value.reasoning}
                </Text>
              </View>
            ))}
          </>
        )}
        
        <Text style={styles.footer} fixed>
          <Text>Page 1</Text>
          <Text>Confidential • {report.name}</Text>
        </Text>
      </Page>
      
      {/* Page 2: Cost Analysis & Projections */}
      <Page size="A4" style={styles.page}>
        <Text style={styles.sectionTitle}>Cost Analysis & Financial Projections</Text>
        
        <Text style={styles.subsectionTitle}>Monthly Cost Projections</Text>
        <Text style={styles.text}>
          Based on current average cost of ${avgCostPerRun.toFixed(4)} per run across {totalAgents} agents
        </Text>
        
        <CostProjectionChart projections={costProjections} />
        
        {/* Cost Projection Table */}
        <View style={styles.table}>
          <View style={styles.tableHeader}>
            <Text style={[styles.tableCell, { flex: 2 }]}>Frequency</Text>
            <Text style={[styles.tableCell, { flex: 1, textAlign: 'right' }]}>Runs/Month</Text>
            <Text style={[styles.tableCell, { flex: 1, textAlign: 'right' }]}>Monthly Cost</Text>
            <Text style={[styles.tableCell, { flex: 1, textAlign: 'right' }]}>Annual Cost</Text>
          </View>
          {costProjections.map((proj, idx) => (
            <View key={idx} style={styles.tableRow}>
              <Text style={[styles.tableCell, { flex: 2 }]}>{proj.frequency}</Text>
              <Text style={[styles.tableCell, { flex: 1, textAlign: 'right' }]}>
                {proj.runs.toLocaleString()}
              </Text>
              <Text style={[styles.tableCell, { flex: 1, textAlign: 'right' }]}>
                ${proj.total_cost.toFixed(2)}
              </Text>
              <Text style={[styles.tableCell, { flex: 1, textAlign: 'right' }]}>
                ${(proj.total_cost * 12).toFixed(2)}
              </Text>
            </View>
          ))}
        </View>
        
        {/* ROI Analysis */}
        <Text style={styles.subsectionTitle}>Return on Investment</Text>
        <Text style={styles.text}>
          At daily execution frequency, the team costs approximately ${costProjections[3].total_cost.toFixed(2)} per month.
          {'\n'}
          Assuming $100 value per successful agent execution, the estimated monthly value is ${(100 * costProjections[3].runs * 0.8).toFixed(2)} (80% success rate).
          {'\n'}
          ROI: {((100 * costProjections[3].runs * 0.8 / costProjections[3].total_cost) - 1).toFixed(0)}x return on investment.
        </Text>
        
        <Text style={styles.footer} fixed>
          <Text>Page 2</Text>
          <Text>Confidential • Cost Analysis</Text>
        </Text>
      </Page>
      
      {/* Page 3: Agent Performance Analysis */}
      <Page size="A4" style={styles.page}>
        <Text style={styles.sectionTitle}>Agent Performance Comparison</Text>
        
        <AgentScoreChart agents={agentDetails.sort((a, b) => b.score - a.score)} />
        
        {/* Agent Performance Table */}
        <View style={styles.table}>
          <View style={styles.tableHeader}>
            <Text style={[styles.tableCell, { flex: 2 }]}>Agent Name</Text>
            <Text style={[styles.tableCell, { flex: 1, textAlign: 'center' }]}>Score</Text>
            <Text style={[styles.tableCell, { flex: 1, textAlign: 'center' }]}>Runs</Text>
            <Text style={[styles.tableCell, { flex: 1, textAlign: 'center' }]}>Success Rate</Text>
            <Text style={[styles.tableCell, { flex: 1, textAlign: 'right' }]}>Avg Cost</Text>
          </View>
          {agentDetails.sort((a, b) => b.score - a.score).map((agent, idx) => (
            <View key={idx} style={styles.tableRow}>
              <Text style={[styles.tableCell, { flex: 2 }]}>{agent.agent_name}</Text>
              <Text style={[styles.tableCell, { flex: 1, textAlign: 'center', fontWeight: 'bold', color: agent.score >= 8 ? colors.success : agent.score >= 6 ? colors.warning : colors.danger }]}>
                {agent.score.toFixed(1)}
              </Text>
              <Text style={[styles.tableCell, { flex: 1, textAlign: 'center' }]}>
                {getSqlValue(agent.runs_analyzed) || 0}
              </Text>
              <Text style={[styles.tableCell, { flex: 1, textAlign: 'center' }]}>
                {(toNumber(agent.success_rate) * 100).toFixed(0)}%
              </Text>
              <Text style={[styles.tableCell, { flex: 1, textAlign: 'right' }]}>
                ${toNumber(agent.avg_cost).toFixed(4)}
              </Text>
            </View>
          ))}
        </View>
        
        {/* Top Performers */}
        <Text style={styles.subsectionTitle}>Top Performers</Text>
        {agentDetails.slice(0, 3).map((agent, idx) => {
          const bestRun = parseSqlJson(agent.best_run_example);
          return (
            <View key={idx} style={{ marginBottom: 10, padding: 8, backgroundColor: colors.bgLight, borderRadius: 4 }}>
              <Text style={{ fontSize: 10, fontWeight: 'bold', color: colors.success, marginBottom: 3 }}>
                #{idx + 1} {agent.agent_name} - {agent.score.toFixed(1)}/10
              </Text>
              {bestRun && (
                <Text style={{ fontSize: 8, color: colors.text }}>
                  Best Run: {bestRun.duration.toFixed(1)}s, {bestRun.token_count.toLocaleString()} tokens • {bestRun.explanation}
                </Text>
              )}
            </View>
          );
        })}
        
        <Text style={styles.footer} fixed>
          <Text>Page 3</Text>
          <Text>Confidential • Agent Performance</Text>
        </Text>
      </Page>
      
      {/* Page 4+: Individual Agent Details */}
      {agentDetails.map((agent, agentIdx) => {
        const bestRun = parseSqlJson(agent.best_run_example);
        const worstRun = parseSqlJson(agent.worst_run_example);
        const toolUsage = parseSqlJson(agent.tool_usage_analysis);
        const improvements = parseSqlJson(agent.improvement_plan);
        const failures = parseSqlJson(agent.failure_patterns);
        
        return (
          <Page key={agentIdx} size="A4" style={styles.page}>
            <Text style={styles.sectionTitle}>{agent.agent_name}</Text>
            <Text style={{ fontSize: 10, color: colors.textLight, marginBottom: 10 }}>
              Score: {agent.score.toFixed(1)}/10 • 
              Runs: {getSqlValue(agent.runs_analyzed)} • 
              Success Rate: {(toNumber(agent.success_rate) * 100).toFixed(0)}%
            </Text>
            
            {/* Performance Metrics */}
            <View style={{ flexDirection: 'row', flexWrap: 'wrap', marginBottom: 15 }}>
              <View style={[styles.metricCard, { flex: 1, minWidth: 120 }]}>
                <Text style={styles.metricLabel}>Avg Duration</Text>
                <Text style={styles.metricValue}>
                  {toNumber(agent.avg_duration_seconds).toFixed(1)}s
                </Text>
              </View>
              <View style={[styles.metricCard, { flex: 1, minWidth: 120 }]}>
                <Text style={styles.metricLabel}>Avg Cost</Text>
                <Text style={styles.metricValue}>
                  ${toNumber(agent.avg_cost).toFixed(4)}
                </Text>
              </View>
              <View style={[styles.metricCard, { flex: 1, minWidth: 120 }]}>
                <Text style={styles.metricLabel}>Avg Tokens</Text>
                <Text style={styles.metricValue}>
                  {toNumber(agent.avg_tokens).toLocaleString()}
                </Text>
              </View>
            </View>
            
            {/* Tool Usage */}
            {toolUsage && toolUsage.length > 0 && (
              <ToolUsageChart toolUsage={toolUsage} />
            )}
            
            {/* Best/Worst Comparison */}
            {(bestRun || worstRun) && (
              <>
                <Text style={styles.subsectionTitle}>Performance Comparison</Text>
                <View style={{ flexDirection: 'row', gap: 10 }}>
                  {bestRun && (
                    <View style={{ flex: 1, padding: 8, backgroundColor: '#dcfce7', borderRadius: 4, borderWidth: 1, borderColor: colors.success }}>
                      <Text style={{ fontSize: 9, fontWeight: 'bold', color: colors.success, marginBottom: 4 }}>
                        BEST RUN #{bestRun.run_id}
                      </Text>
                      <Text style={{ fontSize: 8, color: colors.text, marginBottom: 2 }}>
                        Duration: {bestRun.duration.toFixed(1)}s
                      </Text>
                      <Text style={{ fontSize: 8, color: colors.text, marginBottom: 2 }}>
                        Tokens: {bestRun.token_count.toLocaleString()}
                      </Text>
                      <Text style={{ fontSize: 7, color: colors.textLight, fontStyle: 'italic' }}>
                        {bestRun.explanation}
                      </Text>
                    </View>
                  )}
                  {worstRun && (
                    <View style={{ flex: 1, padding: 8, backgroundColor: '#fee2e2', borderRadius: 4, borderWidth: 1, borderColor: colors.danger }}>
                      <Text style={{ fontSize: 9, fontWeight: 'bold', color: colors.danger, marginBottom: 4 }}>
                        NEEDS IMPROVEMENT #{worstRun.run_id}
                      </Text>
                      <Text style={{ fontSize: 8, color: colors.text, marginBottom: 2 }}>
                        Duration: {worstRun.duration.toFixed(1)}s
                      </Text>
                      <Text style={{ fontSize: 8, color: colors.text, marginBottom: 2 }}>
                        Tokens: {worstRun.token_count.toLocaleString()}
                      </Text>
                      <Text style={{ fontSize: 7, color: colors.textLight, fontStyle: 'italic' }}>
                        {worstRun.explanation}
                      </Text>
                    </View>
                  )}
                </View>
              </>
            )}
            
            {/* Failure Patterns */}
            {failures && failures.length > 0 && (
              <>
                <Text style={styles.subsectionTitle}>Failure Patterns</Text>
                {failures.map((failure: any, idx: number) => (
                  <View key={idx} style={{ marginBottom: 6, padding: 6, backgroundColor: '#fef2f2', borderRadius: 3, borderLeftWidth: 3, borderLeftColor: colors.danger }}>
                    <Text style={{ fontSize: 9, fontWeight: 'bold', color: colors.danger }}>
                      {failure.pattern} ({failure.frequency}x, {failure.impact} impact)
                    </Text>
                    <Text style={{ fontSize: 8, color: colors.textLight }}>
                      Affected runs: {failure.examples.join(', ')}
                    </Text>
                  </View>
                ))}
              </>
            )}
            
            {/* Improvement Plan */}
            {improvements && improvements.length > 0 && (
              <>
                <Text style={styles.subsectionTitle}>Actionable Improvement Plan</Text>
                {improvements.map((action: any, idx: number) => (
                  <View key={idx} style={{ marginBottom: 8, padding: 8, backgroundColor: colors.bgLight, borderRadius: 3, borderLeftWidth: 3, borderLeftColor: action.priority === 'High' ? colors.danger : action.priority === 'Medium' ? colors.warning : colors.success }}>
                    <View style={{ flexDirection: 'row', justifyContent: 'space-between', marginBottom: 3 }}>
                      <Text style={{ fontSize: 9, fontWeight: 'bold', color: colors.text }}>
                        {action.issue}
                      </Text>
                      <Text style={{ fontSize: 8, color: action.priority === 'High' ? colors.danger : action.priority === 'Medium' ? colors.warning : colors.success }}>
                        {action.priority} Priority
                      </Text>
                    </View>
                    <Text style={{ fontSize: 8, color: colors.text, marginBottom: 2 }}>
                      → {action.recommendation}
                    </Text>
                    <Text style={{ fontSize: 7, color: colors.textLight }}>
                      Expected: {action.expected_impact}
                    </Text>
                  </View>
                ))}
              </>
            )}
            
            <Text style={styles.footer} fixed>
              <Text>Page {4 + agentIdx}</Text>
              <Text>Confidential • {agent.agent_name}</Text>
            </Text>
          </Page>
        );
      })}
    </Document>
  );
};

import React from 'react';
import { Document, Page, Text, View, StyleSheet, Image } from '@react-pdf/renderer';
import type { Report, AgentReportDetail, CriterionScore } from '../../types/station';

// Tokyo Night color palette for PDF
const colors = {
  bg: '#1a1b26',
  bgDark: '#16161e',
  fg: '#c0caf5',
  comment: '#565f89',
  cyan: '#7dcfff',
  blue: '#7aa2f7',
  purple: '#bb9af7',
  green: '#9ece6a',
  yellow: '#e0af68',
  red: '#f7768e',
  orange: '#ff9e64',
};

// Professional PDF styles
const styles = StyleSheet.create({
  page: {
    padding: 40,
    backgroundColor: '#FFFFFF',
    fontFamily: 'Helvetica',
  },
  
  // Header styles
  header: {
    marginBottom: 30,
    borderBottom: 2,
    borderBottomColor: colors.blue,
    paddingBottom: 15,
  },
  title: {
    fontSize: 28,
    fontWeight: 'bold',
    color: colors.blue,
    marginBottom: 8,
  },
  subtitle: {
    fontSize: 12,
    color: colors.comment,
    marginBottom: 4,
  },
  
  // Executive summary
  summarySection: {
    marginTop: 20,
    marginBottom: 20,
    padding: 15,
    backgroundColor: '#f8f9fa',
    borderRadius: 4,
    borderLeft: 4,
    borderLeftColor: colors.blue,
  },
  sectionTitle: {
    fontSize: 18,
    fontWeight: 'bold',
    color: colors.blue,
    marginBottom: 10,
  },
  
  // Score display
  scoreContainer: {
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    marginVertical: 20,
    padding: 20,
    backgroundColor: '#f0f4f8',
    borderRadius: 8,
  },
  scoreLabel: {
    fontSize: 12,
    color: colors.comment,
    marginBottom: 5,
  },
  scoreValue: {
    fontSize: 48,
    fontWeight: 'bold',
    marginBottom: 5,
  },
  scoreOutOf: {
    fontSize: 20,
    color: colors.comment,
  },
  scoreRating: {
    fontSize: 14,
    fontWeight: 'bold',
    marginTop: 8,
    padding: 6,
    borderRadius: 4,
  },
  
  // Key metrics grid
  metricsGrid: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginVertical: 15,
    gap: 10,
  },
  metricCard: {
    flex: 1,
    padding: 12,
    backgroundColor: '#f8f9fa',
    borderRadius: 4,
    borderLeft: 3,
  },
  metricLabel: {
    fontSize: 10,
    color: colors.comment,
    marginBottom: 4,
  },
  metricValue: {
    fontSize: 20,
    fontWeight: 'bold',
    marginBottom: 2,
  },
  metricSubtext: {
    fontSize: 9,
    color: colors.comment,
  },
  
  // Benchmark goal section
  goalSection: {
    marginVertical: 15,
    padding: 15,
    backgroundColor: '#f0f4ff',
    borderRadius: 4,
    borderLeft: 4,
    borderLeftColor: colors.purple,
  },
  goalTitle: {
    fontSize: 14,
    fontWeight: 'bold',
    color: colors.purple,
    marginBottom: 8,
  },
  goalText: {
    fontSize: 11,
    color: '#333',
    lineHeight: 1.6,
    marginBottom: 12,
  },
  
  // Criteria breakdown
  criteriaContainer: {
    marginTop: 10,
  },
  criterionItem: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    paddingVertical: 8,
    paddingHorizontal: 12,
    marginBottom: 6,
    backgroundColor: '#ffffff',
    borderRadius: 4,
    borderLeft: 3,
  },
  criterionLeft: {
    flex: 1,
  },
  criterionName: {
    fontSize: 11,
    fontWeight: 'bold',
    color: '#1a1b26',
    marginBottom: 2,
  },
  criterionMeta: {
    fontSize: 9,
    color: colors.comment,
  },
  criterionScore: {
    fontSize: 16,
    fontWeight: 'bold',
    textAlign: 'right',
    marginLeft: 10,
  },
  
  // Agent performance table
  agentTable: {
    marginVertical: 15,
  },
  tableHeader: {
    flexDirection: 'row',
    backgroundColor: colors.blue,
    padding: 10,
    borderTopLeftRadius: 4,
    borderTopRightRadius: 4,
  },
  tableHeaderText: {
    fontSize: 10,
    fontWeight: 'bold',
    color: '#FFFFFF',
  },
  tableRow: {
    flexDirection: 'row',
    padding: 10,
    borderBottom: 1,
    borderBottomColor: '#e0e0e0',
  },
  tableRowAlt: {
    backgroundColor: '#f8f9fa',
  },
  tableCell: {
    fontSize: 9,
  },
  
  // Agent details
  agentSection: {
    marginTop: 15,
    marginBottom: 15,
    padding: 15,
    backgroundColor: '#f8f9fa',
    borderRadius: 4,
    borderLeft: 4,
  },
  agentName: {
    fontSize: 14,
    fontWeight: 'bold',
    marginBottom: 8,
  },
  agentStats: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginTop: 8,
    gap: 10,
  },
  agentStatBox: {
    flex: 1,
    padding: 8,
    backgroundColor: '#ffffff',
    borderRadius: 4,
  },
  agentStatLabel: {
    fontSize: 8,
    color: colors.comment,
    marginBottom: 2,
  },
  agentStatValue: {
    fontSize: 12,
    fontWeight: 'bold',
  },
  
  // Footer
  footer: {
    position: 'absolute',
    bottom: 30,
    left: 40,
    right: 40,
    borderTop: 1,
    borderTopColor: '#e0e0e0',
    paddingTop: 10,
    flexDirection: 'row',
    justifyContent: 'space-between',
  },
  footerText: {
    fontSize: 8,
    color: colors.comment,
  },
  
  // Page break helper
  pageBreak: {
    marginTop: 20,
  },
});

interface ReportPDFProps {
  report: Report;
  agentDetails: AgentReportDetail[];
  teamCriteria: {
    goal: string;
    criteria: Record<string, {
      weight: number;
      threshold: number;
      description: string;
    }>;
  } | null;
  teamCriteriaScores: Record<string, CriterionScore> | null;
  agentCriteria?: {
    goal: string;
    criteria: Record<string, {
      weight: number;
      threshold: number;
      description: string;
    }>;
  } | null;
}

// Helper to safely extract SQL null values
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

const getScoreColor = (score: number) => {
  if (score >= 9) return colors.green;
  if (score >= 7) return colors.yellow;
  return colors.red;
};

const getScoreRating = (score: number) => {
  if (score >= 9) return 'Excellent';
  if (score >= 7) return 'Good';
  return 'Needs Improvement';
};

export const ReportPDF: React.FC<ReportPDFProps> = ({
  report,
  agentDetails,
  teamCriteria,
  teamCriteriaScores,
  agentCriteria,
}) => {
  const teamScore = getSqlValue(report.team_score) || 0;
  const summary = getSqlValue(report.executive_summary) || '';
  const description = getSqlValue(report.description) || '';
  const completedAt = getSqlValue(report.generation_completed_at);
  const duration = getSqlValue(report.generation_duration_seconds);
  const judgeModel = getSqlValue(report.judge_model) || 'gpt-4o-mini';
  const totalCost = getSqlValue(report.total_llm_cost) || 0;
  const totalRuns = getSqlValue(report.total_runs_analyzed) || 0;
  
  const passedAgents = agentDetails.filter(a => a.passed).length;
  const failedAgents = agentDetails.length - passedAgents;
  const successRate = agentDetails.length > 0 ? Math.round((passedAgents / agentDetails.length) * 100) : 0;

  return (
    <Document>
      {/* Cover Page */}
      <Page size="A4" style={styles.page}>
        {/* Header */}
        <View style={styles.header}>
          <Text style={styles.title}>{report.name}</Text>
          {description && (
            <Text style={styles.subtitle}>{description}</Text>
          )}
          <Text style={styles.subtitle}>
            Generated: {completedAt ? new Date(completedAt).toLocaleString() : 'N/A'}
          </Text>
          <Text style={styles.subtitle}>
            Environment: {report.environment_name} • Judge Model: {judgeModel}
          </Text>
        </View>

        {/* Overall Score */}
        <View style={styles.summarySection}>
          <Text style={styles.sectionTitle}>Executive Summary</Text>
          
          <View style={styles.scoreContainer}>
            <View style={{ alignItems: 'center' }}>
              <Text style={styles.scoreLabel}>OVERALL PERFORMANCE</Text>
              <View style={{ flexDirection: 'row', alignItems: 'baseline' }}>
                <Text style={[styles.scoreValue, { color: getScoreColor(teamScore) }]}>
                  {teamScore.toFixed(1)}
                </Text>
                <Text style={styles.scoreOutOf}> / 10</Text>
              </View>
              <Text style={[
                styles.scoreRating,
                {
                  color: getScoreColor(teamScore),
                  backgroundColor: getScoreColor(teamScore) + '20',
                }
              ]}>
                {getScoreRating(teamScore)}
              </Text>
            </View>
          </View>

          {/* Key Metrics Grid */}
          <View style={styles.metricsGrid}>
            <View style={[styles.metricCard, { borderLeftColor: colors.green }]}>
              <Text style={styles.metricLabel}>Passed</Text>
              <Text style={[styles.metricValue, { color: colors.green }]}>
                {passedAgents}
              </Text>
              <Text style={styles.metricSubtext}>of {agentDetails.length} agents</Text>
            </View>

            <View style={[styles.metricCard, { borderLeftColor: colors.red }]}>
              <Text style={styles.metricLabel}>Failed</Text>
              <Text style={[styles.metricValue, { color: colors.red }]}>
                {failedAgents}
              </Text>
              <Text style={styles.metricSubtext}>of {agentDetails.length} agents</Text>
            </View>

            <View style={[styles.metricCard, { borderLeftColor: colors.blue }]}>
              <Text style={styles.metricLabel}>Success Rate</Text>
              <Text style={[styles.metricValue, { color: colors.blue }]}>
                {successRate}%
              </Text>
              <Text style={styles.metricSubtext}>overall performance</Text>
            </View>
          </View>

          {/* Executive Summary Text */}
          {summary && (
            <View style={{ marginTop: 15 }}>
              <Text style={{ fontSize: 11, lineHeight: 1.6, color: '#333' }}>
                {summary}
              </Text>
            </View>
          )}

          {/* Evaluation Metadata */}
          <View style={{ marginTop: 15, paddingTop: 10, borderTop: 1, borderTopColor: '#e0e0e0' }}>
            <Text style={{ fontSize: 9, color: colors.comment }}>
              Evaluation Details: {totalRuns} runs analyzed • 
              Duration: {duration ? `${duration.toFixed(1)}s` : 'N/A'} • 
              Total Cost: ${totalCost.toFixed(4)}
            </Text>
          </View>
        </View>

        {/* Benchmark Goal & Criteria */}
        {teamCriteria && (
          <View style={styles.goalSection}>
            <Text style={styles.goalTitle}>🎯 Benchmark Goal</Text>
            <Text style={styles.goalText}>{teamCriteria.goal}</Text>
            
            <Text style={{ fontSize: 11, fontWeight: 'bold', color: colors.purple, marginBottom: 8 }}>
              Success Criteria
            </Text>
            
            <View style={styles.criteriaContainer}>
              {Object.entries(teamCriteria.criteria).map(([name, config]) => {
                const score = teamCriteriaScores?.[name]?.score || 0;
                const passed = score >= config.threshold;
                
                return (
                  <View 
                    key={name} 
                    style={[
                      styles.criterionItem,
                      { borderLeftColor: passed ? colors.green : colors.red }
                    ]}
                  >
                    <View style={styles.criterionLeft}>
                      <Text style={styles.criterionName}>
                        {name.replace(/_/g, ' ').toUpperCase()}
                      </Text>
                      <Text style={styles.criterionMeta}>
                        Weight: {(config.weight * 100).toFixed(0)}% • 
                        Threshold: {config.threshold.toFixed(1)}/10 • 
                        {passed ? ' ✓ PASSED' : ' ✗ FAILED'}
                      </Text>
                      {config.description && (
                        <Text style={{ fontSize: 9, color: '#555', marginTop: 2 }}>
                          {config.description}
                        </Text>
                      )}
                    </View>
                    <Text style={[styles.criterionScore, { color: getScoreColor(score) }]}>
                      {score.toFixed(1)}
                    </Text>
                  </View>
                );
              })}
            </View>
          </View>
        )}

        {/* Footer */}
        <View style={styles.footer}>
          <Text style={styles.footerText}>Station Agent Performance Report</Text>
          <Text style={styles.footerText}>Page 1</Text>
        </View>
      </Page>

      {/* Agent Performance Details Page */}
      <Page size="A4" style={styles.page}>
        <View style={styles.header}>
          <Text style={styles.sectionTitle}>Agent Performance Breakdown</Text>
          <Text style={styles.subtitle}>
            Detailed analysis of {agentDetails.length} agents across {totalRuns} total runs
          </Text>
        </View>

        {/* Agent Performance Table */}
        <View style={styles.agentTable}>
          <View style={styles.tableHeader}>
            <Text style={[styles.tableHeaderText, { width: '30%' }]}>Agent Name</Text>
            <Text style={[styles.tableHeaderText, { width: '15%', textAlign: 'center' }]}>Score</Text>
            <Text style={[styles.tableHeaderText, { width: '15%', textAlign: 'center' }]}>Runs</Text>
            <Text style={[styles.tableHeaderText, { width: '20%', textAlign: 'center' }]}>Success Rate</Text>
            <Text style={[styles.tableHeaderText, { width: '20%', textAlign: 'center' }]}>Avg Duration</Text>
          </View>

          {agentDetails
            .sort((a, b) => b.score - a.score)
            .map((agent, index) => {
              const runsAnalyzed = getSqlValue(agent.runs_analyzed) || 0;
              const successRate = getSqlValue(agent.success_rate);
              const avgDuration = getSqlValue(agent.avg_duration_seconds);
              
              return (
                <View 
                  key={agent.id} 
                  style={[
                    styles.tableRow,
                    index % 2 === 1 && styles.tableRowAlt
                  ]}
                >
                  <Text style={[styles.tableCell, { width: '30%', fontWeight: 'bold' }]}>
                    {agent.passed ? '✓ ' : '✗ '}{agent.agent_name}
                  </Text>
                  <Text style={[
                    styles.tableCell,
                    { width: '15%', textAlign: 'center', color: getScoreColor(agent.score), fontWeight: 'bold' }
                  ]}>
                    {agent.score.toFixed(1)}
                  </Text>
                  <Text style={[styles.tableCell, { width: '15%', textAlign: 'center' }]}>
                    {runsAnalyzed}
                  </Text>
                  <Text style={[styles.tableCell, { width: '20%', textAlign: 'center' }]}>
                    {successRate !== undefined ? `${(successRate * 100).toFixed(0)}%` : 'N/A'}
                  </Text>
                  <Text style={[styles.tableCell, { width: '20%', textAlign: 'center' }]}>
                    {avgDuration !== undefined ? `${avgDuration.toFixed(1)}s` : 'N/A'}
                  </Text>
                </View>
              );
            })}
        </View>

        {/* Quality Metrics Info */}
        <View style={{ marginTop: 20, padding: 12, backgroundColor: '#f0f4ff', borderRadius: 4 }}>
          <Text style={{ fontSize: 11, fontWeight: 'bold', color: colors.blue, marginBottom: 6 }}>
            🔬 Quality Metrics Evaluated
          </Text>
          <Text style={{ fontSize: 9, color: '#333', lineHeight: 1.5 }}>
            Each agent run was evaluated across 5 quality dimensions using LLM-as-judge ({judgeModel}):
          </Text>
          <View style={{ marginTop: 6 }}>
            <Text style={{ fontSize: 9, color: '#333', marginBottom: 2 }}>
              • <Text style={{ fontWeight: 'bold' }}>Hallucination</Text>: Detects fabricated information (≤10%)
            </Text>
            <Text style={{ fontSize: 9, color: '#333', marginBottom: 2 }}>
              • <Text style={{ fontWeight: 'bold' }}>Relevancy</Text>: Response addresses task (≥80%)
            </Text>
            <Text style={{ fontSize: 9, color: '#333', marginBottom: 2 }}>
              • <Text style={{ fontWeight: 'bold' }}>Task Completion</Text>: Fully completes request (≥85%)
            </Text>
            <Text style={{ fontSize: 9, color: '#333', marginBottom: 2 }}>
              • <Text style={{ fontWeight: 'bold' }}>Faithfulness</Text>: Grounded in context (≤10% drift)
            </Text>
            <Text style={{ fontSize: 9, color: '#333', marginBottom: 2 }}>
              • <Text style={{ fontWeight: 'bold' }}>Toxicity</Text>: No harmful content (≤5%)
            </Text>
          </View>
        </View>

        {/* Footer */}
        <View style={styles.footer}>
          <Text style={styles.footerText}>Station Agent Performance Report</Text>
          <Text style={styles.footerText}>Page 2</Text>
        </View>
      </Page>

      {/* Agent Details Pages (one per agent with detailed stats) */}
      {agentDetails.slice(0, 10).map((agent, pageIndex) => {
        const runsAnalyzed = getSqlValue(agent.runs_analyzed) || 0;
        const successRate = getSqlValue(agent.success_rate);
        const avgDuration = getSqlValue(agent.avg_duration_seconds);
        const strengths = getSqlValue(agent.strengths);
        const weaknesses = getSqlValue(agent.weaknesses);
        
        return (
          <Page key={agent.id} size="A4" style={styles.page}>
            <View style={styles.header}>
              <Text style={[styles.sectionTitle, { color: agent.passed ? colors.green : colors.red }]}>
                {agent.passed ? '✓' : '✗'} {agent.agent_name}
              </Text>
              <Text style={styles.subtitle}>Detailed Performance Analysis</Text>
            </View>

            {/* Agent Score Card */}
            <View style={[
              styles.agentSection,
              { borderLeftColor: agent.passed ? colors.green : colors.red }
            ]}>
              <Text style={[styles.agentName, { color: getScoreColor(agent.score) }]}>
                Overall Score: {agent.score.toFixed(1)}/10
              </Text>
              
              <View style={styles.agentStats}>
                <View style={styles.agentStatBox}>
                  <Text style={styles.agentStatLabel}>Runs Analyzed</Text>
                  <Text style={[styles.agentStatValue, { color: colors.blue }]}>
                    {runsAnalyzed}
                  </Text>
                </View>

                <View style={styles.agentStatBox}>
                  <Text style={styles.agentStatLabel}>Success Rate</Text>
                  <Text style={[styles.agentStatValue, { color: colors.green }]}>
                    {successRate !== undefined ? `${(successRate * 100).toFixed(0)}%` : 'N/A'}
                  </Text>
                </View>

                <View style={styles.agentStatBox}>
                  <Text style={styles.agentStatLabel}>Avg Duration</Text>
                  <Text style={[styles.agentStatValue, { color: colors.purple }]}>
                    {avgDuration !== undefined ? `${avgDuration.toFixed(1)}s` : 'N/A'}
                  </Text>
                </View>
              </View>
            </View>

            {/* Strengths */}
            {strengths && (
              <View style={{ marginTop: 15 }}>
                <Text style={{ fontSize: 12, fontWeight: 'bold', color: colors.green, marginBottom: 8 }}>
                  💪 Strengths
                </Text>
                <View style={{ padding: 12, backgroundColor: '#f0fff4', borderRadius: 4, borderLeft: 3, borderLeftColor: colors.green }}>
                  <Text style={{ fontSize: 10, lineHeight: 1.6, color: '#333' }}>
                    {strengths}
                  </Text>
                </View>
              </View>
            )}

            {/* Weaknesses */}
            {weaknesses && (
              <View style={{ marginTop: 15 }}>
                <Text style={{ fontSize: 12, fontWeight: 'bold', color: colors.red, marginBottom: 8 }}>
                  ⚠️ Areas for Improvement
                </Text>
                <View style={{ padding: 12, backgroundColor: '#fff5f5', borderRadius: 4, borderLeft: 3, borderLeftColor: colors.red }}>
                  <Text style={{ fontSize: 10, lineHeight: 1.6, color: '#333' }}>
                    {weaknesses}
                  </Text>
                </View>
              </View>
            )}

            {/* Footer */}
            <View style={styles.footer}>
              <Text style={styles.footerText}>Station Agent Performance Report</Text>
              <Text style={styles.footerText}>Page {pageIndex + 3}</Text>
            </View>
          </Page>
        );
      })}
    </Document>
  );
};

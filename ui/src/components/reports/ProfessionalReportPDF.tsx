import React from 'react';
import { Document, Page, Text, View, StyleSheet } from '@react-pdf/renderer';
import type { Report, AgentReportDetail, CriterionScore } from '../../types/station';

// Professional corporate color palette - blues and grays
const colors = {
  primary: '#1e3a8a',      // Deep blue
  secondary: '#475569',    // Slate gray
  accent: '#3b82f6',       // Bright blue
  success: '#059669',      // Green
  warning: '#d97706',      // Amber
  danger: '#dc2626',       // Red
  text: '#1e293b',         // Dark gray
  textLight: '#64748b',    // Medium gray
  border: '#cbd5e1',       // Light gray
  bgLight: '#f8fafc',      // Very light gray
};

// Professional PDF styles - corporate and clean
const styles = StyleSheet.create({
  page: {
    padding: 50,
    backgroundColor: '#FFFFFF',
    fontFamily: 'Helvetica',
    fontSize: 10,
    color: colors.text,
  },
  
  // Headers
  header: {
    marginBottom: 25,
    borderBottom: 2,
    borderBottomColor: colors.primary,
    paddingBottom: 12,
  },
  title: {
    fontSize: 24,
    fontWeight: 'bold',
    color: colors.primary,
    marginBottom: 6,
  },
  subtitle: {
    fontSize: 10,
    color: colors.textLight,
    marginBottom: 3,
  },
  
  // Section headers
  sectionTitle: {
    fontSize: 14,
    fontWeight: 'bold',
    color: colors.primary,
    marginBottom: 10,
    marginTop: 15,
  },
  
  sectionSubtitle: {
    fontSize: 11,
    fontWeight: 'bold',
    color: colors.secondary,
    marginBottom: 8,
    marginTop: 10,
  },
  
  // Score display
  scoreBox: {
    padding: 20,
    backgroundColor: colors.bgLight,
    borderRadius: 4,
    borderWidth: 1,
    borderColor: colors.border,
    marginVertical: 15,
    alignItems: 'center',
  },
  scoreLabel: {
    fontSize: 9,
    color: colors.textLight,
    marginBottom: 4,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
  },
  scoreValue: {
    fontSize: 36,
    fontWeight: 'bold',
    color: colors.primary,
  },
  scoreOutOf: {
    fontSize: 14,
    color: colors.textLight,
  },
  
  // Tables
  table: {
    marginVertical: 10,
  },
  tableHeader: {
    flexDirection: 'row',
    backgroundColor: colors.primary,
    padding: 8,
    color: '#FFFFFF',
    fontWeight: 'bold',
    fontSize: 9,
  },
  tableRow: {
    flexDirection: 'row',
    padding: 8,
    borderBottom: 1,
    borderBottomColor: colors.border,
    fontSize: 9,
  },
  tableRowAlt: {
    backgroundColor: colors.bgLight,
  },
  
  // Criteria
  criterionBox: {
    marginBottom: 10,
    padding: 12,
    backgroundColor: colors.bgLight,
    borderLeft: 3,
    borderRadius: 2,
  },
  criterionHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 6,
  },
  criterionName: {
    fontSize: 11,
    fontWeight: 'bold',
    color: colors.text,
  },
  criterionScore: {
    fontSize: 16,
    fontWeight: 'bold',
  },
  criterionMeta: {
    fontSize: 8,
    color: colors.textLight,
    marginBottom: 4,
  },
  criterionDescription: {
    fontSize: 9,
    color: colors.secondary,
    lineHeight: 1.4,
  },
  
  // Summary text
  summaryText: {
    fontSize: 10,
    lineHeight: 1.6,
    color: colors.text,
    marginBottom: 10,
  },
  
  // Metrics grid
  metricsGrid: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    marginVertical: 12,
    gap: 10,
  },
  metricCard: {
    flex: 1,
    padding: 12,
    backgroundColor: colors.bgLight,
    borderRadius: 4,
    borderWidth: 1,
    borderColor: colors.border,
  },
  metricLabel: {
    fontSize: 8,
    color: colors.textLight,
    marginBottom: 4,
    textTransform: 'uppercase',
  },
  metricValue: {
    fontSize: 18,
    fontWeight: 'bold',
    color: colors.primary,
  },
  metricSubtext: {
    fontSize: 7,
    color: colors.textLight,
    marginTop: 2,
  },
  
  // Appendix styles
  appendixSection: {
    marginTop: 15,
    marginBottom: 15,
  },
  appendixBox: {
    padding: 10,
    backgroundColor: colors.bgLight,
    borderRadius: 3,
    marginBottom: 10,
    borderWidth: 1,
    borderColor: colors.border,
  },
  appendixLabel: {
    fontSize: 8,
    color: colors.textLight,
    marginBottom: 3,
    fontWeight: 'bold',
  },
  appendixValue: {
    fontSize: 9,
    color: colors.text,
    lineHeight: 1.4,
  },
  codeBlock: {
    backgroundColor: '#f1f5f9',
    padding: 8,
    borderRadius: 3,
    fontFamily: 'Courier',
    fontSize: 8,
    color: colors.text,
    marginVertical: 4,
  },
  
  // Footer
  footer: {
    position: 'absolute',
    bottom: 30,
    left: 50,
    right: 50,
    borderTop: 1,
    borderTopColor: colors.border,
    paddingTop: 8,
    flexDirection: 'row',
    justifyContent: 'space-between',
    fontSize: 8,
    color: colors.textLight,
  },
});

interface ProfessionalReportPDFProps {
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

const getScoreColor = (score: number) => {
  if (score >= 9) return colors.success;
  if (score >= 7) return colors.warning;
  return colors.danger;
};

const formatDate = (dateString: string) => {
  if (!dateString) return 'N/A';
  const date = new Date(dateString);
  return date.toLocaleDateString('en-US', { 
    year: 'numeric', 
    month: 'long', 
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  });
};

export const ProfessionalReportPDF: React.FC<ProfessionalReportPDFProps> = ({
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
  const successRate = agentDetails.length > 0 ? (passedAgents / agentDetails.length * 100) : 0;

  // Parse run IDs for appendix
  const allRunDetails: Array<{agentName: string, runIds: number[]}> = [];
  agentDetails.forEach(agent => {
    const runIdsStr = getSqlValue(agent.run_ids);
    if (runIdsStr) {
      let runIds: number[] = [];
      try {
        if (runIdsStr.startsWith('[')) {
          runIds = runIdsStr.replace(/[\[\]]/g, '').split(' ').map((id: string) => parseInt(id.trim())).filter((id: number) => !isNaN(id));
        } else {
          runIds = JSON.parse(runIdsStr);
        }
        if (runIds.length > 0) {
          allRunDetails.push({ agentName: agent.agent_name, runIds: runIds.slice(0, 5) }); // First 5 runs per agent
        }
      } catch (e) {
        console.error('Failed to parse run IDs:', e);
      }
    }
  });

  return (
    <Document>
      {/* PAGE 1: Cover & Executive Summary */}
      <Page size="A4" style={styles.page}>
        <View style={styles.header}>
          <Text style={styles.title}>{report.name}</Text>
          {description && <Text style={styles.subtitle}>{description}</Text>}
          <Text style={styles.subtitle}>Generated: {formatDate(completedAt)}</Text>
          <Text style={styles.subtitle}>Environment: {report.environment_name} | Judge Model: {judgeModel}</Text>
        </View>

        {/* Executive Summary Section */}
        <Text style={styles.sectionTitle}>Executive Summary</Text>
        
        <View style={styles.scoreBox}>
          <Text style={styles.scoreLabel}>Overall Performance Score</Text>
          <View style={{ flexDirection: 'row', alignItems: 'baseline' }}>
            <Text style={[styles.scoreValue, { color: getScoreColor(teamScore) }]}>
              {teamScore.toFixed(1)}
            </Text>
            <Text style={styles.scoreOutOf}> / 10</Text>
          </View>
        </View>

        <View style={styles.metricsGrid}>
          <View style={styles.metricCard}>
            <Text style={styles.metricLabel}>Agents Passed</Text>
            <Text style={[styles.metricValue, { color: colors.success }]}>{passedAgents}</Text>
            <Text style={styles.metricSubtext}>of {agentDetails.length} total</Text>
          </View>
          
          <View style={styles.metricCard}>
            <Text style={styles.metricLabel}>Agents Failed</Text>
            <Text style={[styles.metricValue, { color: colors.danger }]}>{failedAgents}</Text>
            <Text style={styles.metricSubtext}>of {agentDetails.length} total</Text>
          </View>
          
          <View style={styles.metricCard}>
            <Text style={styles.metricLabel}>Success Rate</Text>
            <Text style={[styles.metricValue, { color: colors.primary }]}>{successRate.toFixed(0)}%</Text>
            <Text style={styles.metricSubtext}>overall performance</Text>
          </View>
        </View>

        {summary && (
          <View style={{ marginTop: 10, marginBottom: 10 }}>
            <Text style={styles.summaryText}>{summary}</Text>
          </View>
        )}

        {/* Evaluation Metadata */}
        <View style={{ marginTop: 15, padding: 10, backgroundColor: colors.bgLight, borderRadius: 3 }}>
          <Text style={styles.appendixLabel}>EVALUATION DETAILS</Text>
          <Text style={styles.appendixValue}>
            Runs Analyzed: {totalRuns} | Duration: {duration ? `${duration.toFixed(1)}s` : 'N/A'} | Cost: ${totalCost.toFixed(4)}
          </Text>
        </View>

        <View style={styles.footer}>
          <Text>Station Agent Performance Report</Text>
          <Text>Page 1</Text>
        </View>
      </Page>

      {/* PAGE 2: Team Criteria Breakdown */}
      {teamCriteria && (
        <Page size="A4" style={styles.page}>
          <View style={styles.header}>
            <Text style={styles.sectionTitle}>Benchmark Criteria & Results</Text>
            <Text style={styles.subtitle}>Detailed evaluation against defined success criteria</Text>
          </View>

          <Text style={styles.sectionSubtitle}>Benchmark Goal</Text>
          <Text style={styles.summaryText}>{teamCriteria.goal}</Text>

          <Text style={styles.sectionSubtitle}>Success Criteria Assessment</Text>
          
          {Object.entries(teamCriteria.criteria)
            .sort((a, b) => b[1].weight - a[1].weight) // Sort by weight descending
            .map(([name, config]) => {
            const score = teamCriteriaScores?.[name]?.score || 0;
            const passed = score >= config.threshold;
            const reasoning = teamCriteriaScores?.[name]?.reasoning;
            
            return (
              <View 
                key={name} 
                style={[
                  styles.criterionBox,
                  { borderLeftColor: passed ? colors.success : colors.danger }
                ]}
              >
                <View style={styles.criterionHeader}>
                  <View style={{ flex: 1 }}>
                    <Text style={styles.criterionName}>
                      {name.toUpperCase().replace(/_/g, ' ')}
                    </Text>
                    <Text style={styles.criterionMeta}>
                      Weight: {(config.weight * 100).toFixed(0)}% | Threshold: {config.threshold.toFixed(1)}/10 | Status: {passed ? 'PASSED' : 'FAILED'}
                    </Text>
                  </View>
                  <Text style={[styles.criterionScore, { color: getScoreColor(score) }]}>
                    {score.toFixed(1)}
                  </Text>
                </View>
                
                <Text style={styles.criterionDescription}>{config.description}</Text>
                
                {reasoning && (
                  <View style={{ marginTop: 6, paddingTop: 6, borderTop: 1, borderTopColor: colors.border }}>
                    <Text style={styles.criterionMeta}>ASSESSMENT</Text>
                    <Text style={[styles.criterionDescription, { fontSize: 8 }]}>{reasoning}</Text>
                  </View>
                )}
              </View>
            );
          })}

          <View style={styles.footer}>
            <Text>Station Agent Performance Report</Text>
            <Text>Page 2</Text>
          </View>
        </Page>
      )}

      {/* PAGE 3: Agent Performance Summary */}
      <Page size="A4" style={styles.page}>
        <View style={styles.header}>
          <Text style={styles.sectionTitle}>Agent Performance Summary</Text>
          <Text style={styles.subtitle}>{agentDetails.length} agents analyzed across {totalRuns} runs</Text>
        </View>

        <View style={styles.table}>
          <View style={styles.tableHeader}>
            <Text style={{ width: '25%' }}>Agent Name</Text>
            <Text style={{ width: '15%', textAlign: 'center' }}>Score</Text>
            <Text style={{ width: '15%', textAlign: 'center' }}>Status</Text>
            <Text style={{ width: '15%', textAlign: 'center' }}>Runs</Text>
            <Text style={{ width: '15%', textAlign: 'center' }}>Success Rate</Text>
            <Text style={{ width: '15%', textAlign: 'center' }}>Avg Duration</Text>
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
                  style={[styles.tableRow, index % 2 === 1 && styles.tableRowAlt]}
                >
                  <Text style={{ width: '25%', fontSize: 8 }}>{agent.agent_name}</Text>
                  <Text style={{ width: '15%', textAlign: 'center', fontWeight: 'bold', color: getScoreColor(agent.score) }}>
                    {agent.score.toFixed(1)}
                  </Text>
                  <Text style={{ width: '15%', textAlign: 'center', color: agent.passed ? colors.success : colors.danger }}>
                    {agent.passed ? 'PASS' : 'FAIL'}
                  </Text>
                  <Text style={{ width: '15%', textAlign: 'center' }}>{runsAnalyzed}</Text>
                  <Text style={{ width: '15%', textAlign: 'center' }}>
                    {successRate !== undefined ? `${(successRate * 100).toFixed(0)}%` : 'N/A'}
                  </Text>
                  <Text style={{ width: '15%', textAlign: 'center' }}>
                    {avgDuration !== undefined ? `${avgDuration.toFixed(2)}s` : 'N/A'}
                  </Text>
                </View>
              );
            })}
        </View>

        <View style={{ marginTop: 15, padding: 10, backgroundColor: colors.bgLight, borderRadius: 3 }}>
          <Text style={styles.appendixLabel}>QUALITY METRICS EVALUATED</Text>
          <Text style={[styles.appendixValue, { fontSize: 8, lineHeight: 1.5 }]}>
            Each agent run was evaluated using LLM-as-judge ({judgeModel}) across 5 quality dimensions:{'\n'}
            • Hallucination: Detects fabricated information not grounded in context (threshold ≤10%){'\n'}
            • Relevancy: Measures how directly the response addresses the task (threshold ≥80%){'\n'}
            • Task Completion: Evaluates if agent fully completed the request (threshold ≥85%){'\n'}
            • Faithfulness: Ensures responses are grounded in available context (threshold ≤10% drift){'\n'}
            • Toxicity: Detects harmful, offensive, or inappropriate content (threshold ≤5%)
          </Text>
        </View>

        <View style={styles.footer}>
          <Text>Station Agent Performance Report</Text>
          <Text>Page 3</Text>
        </View>
      </Page>

      {/* PAGES 4+: Agent Details */}
      {agentDetails.slice(0, 8).map((agent, pageIndex) => {
        const runsAnalyzed = getSqlValue(agent.runs_analyzed) || 0;
        const successRate = getSqlValue(agent.success_rate);
        const avgDuration = getSqlValue(agent.avg_duration_seconds);
        const strengths = getSqlValue(agent.strengths);
        const weaknesses = getSqlValue(agent.weaknesses);
        
        return (
          <Page key={agent.id} size="A4" style={styles.page}>
            <View style={styles.header}>
              <Text style={[styles.sectionTitle, { color: agent.passed ? colors.success : colors.danger }]}>
                {agent.agent_name}
              </Text>
              <Text style={styles.subtitle}>Detailed Performance Analysis</Text>
            </View>

            <View style={[styles.criterionBox, { borderLeftColor: agent.passed ? colors.success : colors.danger }]}>
              <View style={styles.criterionHeader}>
                <Text style={styles.criterionName}>Overall Score</Text>
                <Text style={[styles.criterionScore, { color: getScoreColor(agent.score) }]}>
                  {agent.score.toFixed(1)}/10
                </Text>
              </View>
              <Text style={styles.criterionMeta}>
                Status: {agent.passed ? 'PASSED' : 'FAILED'}
              </Text>
            </View>

            <View style={styles.metricsGrid}>
              <View style={styles.metricCard}>
                <Text style={styles.metricLabel}>Runs Analyzed</Text>
                <Text style={styles.metricValue}>{runsAnalyzed}</Text>
              </View>
              <View style={styles.metricCard}>
                <Text style={styles.metricLabel}>Success Rate</Text>
                <Text style={[styles.metricValue, { color: successRate && successRate > 0.7 ? colors.success : colors.warning }]}>
                  {successRate !== undefined ? `${(successRate * 100).toFixed(0)}%` : 'N/A'}
                </Text>
              </View>
              <View style={styles.metricCard}>
                <Text style={styles.metricLabel}>Avg Duration</Text>
                <Text style={styles.metricValue}>
                  {avgDuration !== undefined ? `${avgDuration.toFixed(2)}s` : 'N/A'}
                </Text>
              </View>
            </View>

            {strengths && (
              <View style={{ marginTop: 15 }}>
                <Text style={[styles.sectionSubtitle, { color: colors.success }]}>Strengths</Text>
                <View style={[styles.criterionBox, { borderLeftColor: colors.success, backgroundColor: '#f0fdf4' }]}>
                  <Text style={styles.criterionDescription}>{strengths}</Text>
                </View>
              </View>
            )}

            {weaknesses && (
              <View style={{ marginTop: 10 }}>
                <Text style={[styles.sectionSubtitle, { color: colors.danger }]}>Areas for Improvement</Text>
                <View style={[styles.criterionBox, { borderLeftColor: colors.danger, backgroundColor: '#fef2f2' }]}>
                  <Text style={styles.criterionDescription}>{weaknesses}</Text>
                </View>
              </View>
            )}

            <View style={styles.footer}>
              <Text>Station Agent Performance Report</Text>
              <Text>Page {pageIndex + 4}</Text>
            </View>
          </Page>
        );
      })}

      {/* APPENDIX: Detailed Test Data */}
      <Page size="A4" style={styles.page}>
        <View style={styles.header}>
          <Text style={styles.sectionTitle}>Appendix: Detailed Test Data</Text>
          <Text style={styles.subtitle}>Comprehensive test execution details for reproducibility and transparency</Text>
        </View>

        <Text style={styles.sectionSubtitle}>Test Execution Summary</Text>
        
        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>EVALUATION METHODOLOGY</Text>
          <Text style={styles.appendixValue}>
            This report evaluates agent performance using LLM-as-judge methodology with {judgeModel}. Each agent was tested across multiple runs, with each run evaluated against 5 quality dimensions. Scores are aggregated using weighted criteria to produce an overall team performance score.
          </Text>
        </View>

        <Text style={styles.sectionSubtitle}>Agent Run Samples</Text>
        <Text style={[styles.appendixValue, { marginBottom: 10 }]}>
          The following shows a sample of run IDs analyzed for each agent. Full run details including inputs, outputs, tool calls, and quality metrics can be retrieved using: stn runs inspect [run_id] -v
        </Text>

        {allRunDetails.slice(0, 10).map((detail, idx) => (
          <View key={idx} style={styles.appendixBox}>
            <Text style={styles.appendixLabel}>{detail.agentName.toUpperCase()}</Text>
            <Text style={[styles.appendixValue, { fontFamily: 'Courier', fontSize: 8 }]}>
              Run IDs: {detail.runIds.join(', ')}
            </Text>
            <Text style={[styles.appendixValue, { fontSize: 8, marginTop: 4, color: colors.textLight }]}>
              Command: stn runs inspect {detail.runIds[0]} -v
            </Text>
          </View>
        ))}

        <Text style={styles.sectionSubtitle}>Quality Metrics Details</Text>
        
        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>HALLUCINATION DETECTION</Text>
          <Text style={styles.appendixValue}>
            Measures whether the agent fabricates information not present in the provided context. Evaluated by comparing agent outputs against available tool outputs and execution context. Threshold: ≤10%.
          </Text>
        </View>

        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>RELEVANCY ASSESSMENT</Text>
          <Text style={styles.appendixValue}>
            Evaluates how directly the agent's response addresses the original task request. Measures alignment between user intent and agent output. Threshold: ≥80%.
          </Text>
        </View>

        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>TASK COMPLETION</Text>
          <Text style={styles.appendixValue}>
            Determines if the agent fully completed the requested task or only provided partial results. Threshold: ≥85%.
          </Text>
        </View>

        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>FAITHFULNESS TO CONTEXT</Text>
          <Text style={styles.appendixValue}>
            Ensures responses remain grounded in the execution context and tool outputs. Measures semantic drift from source data. Threshold: ≤10% drift.
          </Text>
        </View>

        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>TOXICITY DETECTION</Text>
          <Text style={styles.appendixValue}>
            Screens for harmful, offensive, or inappropriate content in agent responses. Threshold: ≤5%.
          </Text>
        </View>

        <View style={styles.footer}>
          <Text>Station Agent Performance Report - Appendix</Text>
          <Text>Page {agentDetails.slice(0, 8).length + 4}</Text>
        </View>
      </Page>

      {/* APPENDIX PAGE 2: Data Collection & Reproducibility */}
      <Page size="A4" style={styles.page}>
        <View style={styles.header}>
          <Text style={styles.sectionTitle}>Appendix: Data Collection & Reproducibility</Text>
          <Text style={styles.subtitle}>Technical details for verification and audit purposes</Text>
        </View>

        <Text style={styles.sectionSubtitle}>Data Collection Process</Text>
        
        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>AGENT EXECUTION DATA</Text>
          <Text style={styles.appendixValue}>
            Each agent run captures:{'\n'}
            • Input task/prompt provided to the agent{'\n'}
            • Complete agent response/output{'\n'}
            • All tool calls made during execution{'\n'}
            • Tool call parameters and results{'\n'}
            • Execution duration and token usage{'\n'}
            • Error messages (if any)
          </Text>
        </View>

        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>LLM JUDGE EVALUATION</Text>
          <Text style={styles.appendixValue}>
            Model: {judgeModel}{'\n'}
            Total Tokens Used: {getSqlValue(report.total_llm_tokens) || 0}{'\n'}
            Total Cost: ${totalCost.toFixed(6)}{'\n'}
            Evaluation Duration: {duration ? `${duration.toFixed(2)}s` : 'N/A'}
          </Text>
        </View>

        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>ACCESSING DETAILED RUN DATA</Text>
          <Text style={styles.appendixValue}>
            To inspect individual run details:{'\n'}
          </Text>
          <View style={styles.codeBlock}>
            <Text>stn runs inspect [run_id] -v</Text>
          </View>
          <Text style={[styles.appendixValue, { marginTop: 6 }]}>
            This command displays:{'\n'}
            • Full execution trace{'\n'}
            • Tool call details with parameters{'\n'}
            • Token usage breakdown{'\n'}
            • Timing information{'\n'}
            • Success/failure status
          </Text>
        </View>

        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>BENCHMARKING INDIVIDUAL RUNS</Text>
          <Text style={styles.appendixValue}>
            To evaluate a specific run with quality metrics:{'\n'}
          </Text>
          <View style={styles.codeBlock}>
            <Text>stn benchmark evaluate [run_id]</Text>
          </View>
          <Text style={[styles.appendixValue, { marginTop: 6 }]}>
            This generates detailed quality assessments including:{'\n'}
            • Individual metric scores and reasoning{'\n'}
            • LLM judge evaluation rationale{'\n'}
            • Pass/fail determination per metric{'\n'}
            • Production readiness recommendation
          </Text>
        </View>

        <Text style={styles.sectionSubtitle}>Report Configuration</Text>
        
        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>TEAM CRITERIA</Text>
          <View style={styles.codeBlock}>
            <Text>{JSON.stringify({ goal: teamCriteria?.goal }, null, 2)}</Text>
          </View>
        </View>

        {agentCriteria && (
          <View style={styles.appendixBox}>
            <Text style={styles.appendixLabel}>AGENT CRITERIA</Text>
            <Text style={styles.appendixValue}>
              Goal: {agentCriteria.goal}{'\n'}
              Evaluation includes value-per-cost analysis, reliability metrics, and response quality assessment.
            </Text>
          </View>
        )}

        <View style={styles.appendixBox}>
          <Text style={styles.appendixLabel}>CONFIDENCE & TRANSPARENCY</Text>
          <Text style={styles.appendixValue}>
            All evaluation data is stored locally in Station's database and can be queried for audit purposes. Run details, tool calls, and quality metrics provide complete transparency into agent behavior and performance characteristics. This enables teams to build confidence in agent capabilities and identify specific areas for improvement.
          </Text>
        </View>

        <View style={styles.footer}>
          <Text>Station Agent Performance Report - Appendix</Text>
          <Text>Page {agentDetails.slice(0, 8).length + 5}</Text>
        </View>
      </Page>
    </Document>
  );
};

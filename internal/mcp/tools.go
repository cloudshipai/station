package mcp

import (
	"context"
	"fmt"
	"log"

	"station/internal/db/repositories"
	"station/internal/services"
	"station/pkg/models"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ToolsServer handles enhanced MCP tools for agent management
type ToolsServer struct {
	repos        *repositories.Repositories
	mcpServer    *server.MCPServer
	agentService services.AgentServiceInterface
	localMode    bool
}

func NewToolsServer(repos *repositories.Repositories, mcpServer *server.MCPServer, agentService services.AgentServiceInterface, localMode bool) *ToolsServer {
	ts := &ToolsServer{
		repos:        repos,
		mcpServer:    mcpServer,
		agentService: agentService,
		localMode:    localMode,
	}
	ts.setupEnhancedTools()
	return ts
}

// setupEnhancedTools adds enhanced call_agent tools and prompts
func (ts *ToolsServer) setupEnhancedTools() {
	// Note: create_agent is now consolidated in the main MCP server (mcp.go)
	
	// Note: call_agent is now consolidated in the main MCP server (tools_setup.go) with advanced functionality
	
	// Add prompts for AI-assisted agent creation
	ts.setupAgentCreationPrompts()
	
	// Add specialized prompts for common use cases 
	ts.setupSpecializedPrompts()
}

// setupSpecializedPrompts adds prompts for common agent creation use cases
func (ts *ToolsServer) setupSpecializedPrompts() {
	// AWS logs analysis agent prompt
	logsAnalysisPrompt := mcp.NewPrompt("create_logs_analysis_agent",
		mcp.WithPromptDescription("Guide for creating an agent that searches and analyzes AWS logs for urgent issues"),
		mcp.WithArgument("log_sources", mcp.ArgumentDescription("AWS log sources to analyze (CloudWatch, S3, etc.)")),
		mcp.WithArgument("urgency_criteria", mcp.ArgumentDescription("What makes a log entry urgent or high priority")),
		mcp.WithArgument("analysis_depth", mcp.ArgumentDescription("Level of analysis needed (summary, detailed, root-cause)")),
	)
	ts.mcpServer.AddPrompt(logsAnalysisPrompt, ts.handleLogsAnalysisPrompt)

	// DevOps monitoring agent prompt
	devopsMonitorPrompt := mcp.NewPrompt("create_devops_monitor_agent",
		mcp.WithPromptDescription("Guide for creating an agent that monitors infrastructure and deployment health"),
		mcp.WithArgument("infrastructure_type", mcp.ArgumentDescription("Type of infrastructure (kubernetes, docker, ec2, etc.)")),
		mcp.WithArgument("monitoring_scope", mcp.ArgumentDescription("What to monitor (performance, availability, errors, etc.)")),
		mcp.WithArgument("alert_thresholds", mcp.ArgumentDescription("When to alert (critical, warning, info levels)")),
	)
	ts.mcpServer.AddPrompt(devopsMonitorPrompt, ts.handleDevOpsMonitorPrompt)

	// Security scanning agent prompt
	securityScanPrompt := mcp.NewPrompt("create_security_scan_agent",
		mcp.WithPromptDescription("Guide for creating an agent that performs security scans and vulnerability assessments"),
		mcp.WithArgument("scan_targets", mcp.ArgumentDescription("What to scan (repositories, infrastructure, applications)")),
		mcp.WithArgument("scan_types", mcp.ArgumentDescription("Types of scans (vulnerability, compliance, code analysis)")),
		mcp.WithArgument("compliance_frameworks", mcp.ArgumentDescription("Compliance requirements (SOC2, GDPR, HIPAA, etc.)")),
	)
	ts.mcpServer.AddPrompt(securityScanPrompt, ts.handleSecurityScanPrompt)

	// Data processing agent prompt
	dataProcessingPrompt := mcp.NewPrompt("create_data_processing_agent",
		mcp.WithPromptDescription("Guide for creating an agent that processes, transforms, and analyzes data"),
		mcp.WithArgument("data_sources", mcp.ArgumentDescription("Where data comes from (APIs, files, databases, etc.)")),
		mcp.WithArgument("processing_type", mcp.ArgumentDescription("Type of processing (ETL, analysis, reporting, etc.)")),
		mcp.WithArgument("output_format", mcp.ArgumentDescription("Desired output format (reports, alerts, dashboards, etc.)")),
	)
	ts.mcpServer.AddPrompt(dataProcessingPrompt, ts.handleDataProcessingPrompt)
}

// setupAgentCreationPrompts adds prompts that guide AI clients in creating agents
func (ts *ToolsServer) setupAgentCreationPrompts() {
	// Agent creation assistant prompt
	agentCreationPrompt := mcp.NewPrompt("create_comprehensive_agent",
		mcp.WithPromptDescription("Guide for creating well-structured AI agents using Station's tools and environments"),
		mcp.WithArgument("user_intent", mcp.ArgumentDescription("What the user wants to accomplish with this agent"), mcp.RequiredArgument()),
		mcp.WithArgument("domain", mcp.ArgumentDescription("Area of work (devops, data-science, marketing, etc.)")),
		mcp.WithArgument("schedule_preference", mcp.ArgumentDescription("When should this run? (on-demand, daily, weekly, custom cron)")),
	)
	
	ts.mcpServer.AddPrompt(agentCreationPrompt, ts.handleAgentCreationPrompt)
}

// handleAgentCreationPrompt provides structured guidance for agent creation
func (ts *ToolsServer) handleAgentCreationPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	// Get available environments and tools for context
	environments, err := ts.repos.Environments.List()
	if err != nil {
		log.Printf("Failed to get environments for prompt: %v", err)
		environments = []*models.Environment{} // Continue with empty list
	}
	
	// Get actual tool categories from discovered MCP tools
	toolsWithDetails, err := ts.repos.MCPTools.GetAllWithDetails()
	if err != nil {
		log.Printf("Failed to get tools for prompt: %v", err)
		toolsWithDetails = []*models.MCPToolWithDetails{} // Continue with empty list
	}
	
	// Extract unique tool categories/names
	toolCategoryMap := make(map[string]bool)
	for _, tool := range toolsWithDetails {
		toolCategoryMap[tool.Name] = true
	}
	
	var toolCategories []string
	for category := range toolCategoryMap {
		toolCategories = append(toolCategories, category)
	}
	
	userIntent := ""
	domain := ""
	schedulePreference := ""
	
	// Extract arguments if provided
	if args := request.Params.Arguments; args != nil {
		if intent, ok := args["user_intent"]; ok {
			userIntent = intent
		}
		if d, ok := args["domain"]; ok {
			domain = d
		}
		if sched, ok := args["schedule_preference"]; ok {
			schedulePreference = sched
		}
	}
	
	promptContent := ts.buildAgentCreationPrompt(userIntent, domain, schedulePreference, toolCategories, getEnvironmentNames(environments))

	return mcp.NewGetPromptResult("Station AI Agent Creation Assistant", []mcp.PromptMessage{
		{
			Role: mcp.RoleUser,
			Content: mcp.TextContent{
				Type: "text",
				Text: promptContent,
			},
		},
	}), nil
}

// buildAgentCreationPrompt creates the comprehensive agent creation prompt
func (ts *ToolsServer) buildAgentCreationPrompt(userIntent, domain, schedulePreference string, toolCategories []string, environmentNames []string) string {
	return fmt.Sprintf(`# Station AI Agent Creation Assistant

You are helping to create a sophisticated AI agent in Station - a revolutionary AI infrastructure platform. This prompt will guide you through creating a well-structured agent that takes advantage of Station's key benefits:

## Why Station for AI Agents?

1. **Background Agent Excellence**: Station is the easiest way to create background agents that work seamlessly with your development flow
2. **Environment-Based Tool Organization**: Organize tools by environments without cluttering your personal MCP setup
3. **Smart Context Management**: Filter subtools (not just servers) so agents don't get context poisoning from MCP servers with too many tools
4. **Team AI Infrastructure**: Build agents that can be shared and managed across teams

## User Intent Analysis
%s

## Current Context
- Domain: %s
- Schedule Preference: %s
- Available Environments: %v
- Available Tool Categories: %v

## Agent Creation Framework

### 1. Intent Understanding & Agent Purpose
Based on the user intent, determine:
- **Primary Goal**: What specific problem does this agent solve?
- **Success Metrics**: How will we know the agent is working well?
- **Automation Level**: On-demand, scheduled, or event-driven?

### 2. Tool Selection Strategy
Instead of overwhelming the agent with every available tool, use Station's smart filtering:
- **Core Tools**: Essential tools for the main workflow (2-4 tools max)
- **Context Tools**: Environment-specific tools that provide necessary context
- **Fallback Tools**: Additional tools for edge cases (use sparingly)

### 3. Environment Alignment
Choose the optimal environment based on:
- **Resource Access**: Does the agent need specific databases, APIs, or credentials?
- **Security Boundary**: What level of access is appropriate?
- **Team Scope**: Should this be personal or shared?

### 4. Prompt Engineering
Create a system prompt that includes:
- **Clear Role Definition**: What is the agent's primary responsibility?
- **Workflow Steps**: Step-by-step process the agent should follow
- **Quality Gates**: How should the agent validate its work?
- **Error Handling**: What to do when things go wrong?

### 5. Execution Strategy
Configure the agent for optimal performance:
- **Max Steps**: Balance thoroughness with efficiency (recommended: 3-7 steps)
- **Schedule**: Match execution frequency to business needs
- **Dependencies**: What other agents or systems does this interact with?

## Output Format

Please provide your agent creation plan in this structure:

{
  "agent_name": "descriptive-agent-name",
  "agent_description": "One-line description of agent purpose",
  "environment_selection": {
    "recommended_environment": "environment_name",
    "rationale": "Why this environment is optimal"
  },
  "tool_selection": {
    "core_tools": ["tool1", "tool2"],
    "rationale": "Why these specific tools were chosen"
  },
  "system_prompt": "Detailed system prompt for the agent...",
  "execution_config": {
    "max_steps": 5,
    "schedule": "cron_expression_or_on_demand",
    "rationale": "Why this execution pattern fits the use case"
  },
  "success_criteria": "How to measure if this agent is successful",
  "potential_improvements": ["future enhancement 1", "future enhancement 2"]
}

## Next Steps

After you provide this plan, I'll:
1. Present the complete agent configuration for your review
2. Ask for your explicit confirmation before creating the agent
3. Validate tool availability in the selected environment
4. Create the agent using Station's enhanced MCP tools
5. Set up monitoring and scheduling if needed

## ⚠️ Important: User Confirmation Required

**I will NOT create any agent without your explicit approval.** After analyzing your requirements and presenting the plan above, I will:

1. **Show you the complete agent details** including name, description, system prompt, environment, tools, and configuration
2. **Ask: "Do you want me to create this agent with these exact specifications?"**
3. **Wait for your "yes" or confirmation** before proceeding with agent creation
4. **Allow you to modify** any aspect of the agent before creation

This ensures you have full control over what agents are created in your Station environment.

Remember: Station's power comes from smart agent design, not tool proliferation. Focus on solving the specific user problem efficiently!`, 
		userIntent, domain, schedulePreference, environmentNames, toolCategories)
}

// Helper function to extract environment names
func getEnvironmentNames(environments []*models.Environment) []string {
	names := make([]string, len(environments))
	for i, env := range environments {
		names[i] = env.Name
	}
	return names
}

// Enhanced tool handlers

// Note: handleCreateAgentAdvanced has been consolidated into the main MCP server (mcp.go)

// Note: handleCallAgentAdvanced functionality moved to tools_setup.go handleCallAgent

// handleLogsAnalysisPrompt provides specialized guidance for AWS logs analysis agent creation
func (ts *ToolsServer) handleLogsAnalysisPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	logSources := ""
	urgencyCriteria := ""
	analysisDepth := ""
	
	// Extract arguments if provided
	if args := request.Params.Arguments; args != nil {
		if sources, ok := args["log_sources"]; ok {
			logSources = sources
		}
		if criteria, ok := args["urgency_criteria"]; ok {
			urgencyCriteria = criteria
		}
		if depth, ok := args["analysis_depth"]; ok {
			analysisDepth = depth
		}
	}
	
	promptContent := fmt.Sprintf(`# AWS Logs Analysis Agent Creation Guide

## Specialized Agent Configuration

**Agent Purpose**: Intelligent AWS log analysis and urgent issue prioritization

### Recommended Configuration

**Agent Name**: "aws-logs-analyzer"
**Description**: "Analyzes AWS logs across multiple sources and prioritizes urgent issues for immediate attention"

### Tool Requirements for AWS Logs Analysis

**Essential Tools** (select from available MCP tools):
- aws-cli or aws-sdk tools for accessing CloudWatch, S3, ECS logs
- search or grep tools for pattern matching in log files
- json parsing tools for structured log analysis
- file operations for log file access
- notification tools for urgent issue alerts

### System Prompt Template

You are an AWS Logs Analysis Agent specialized in searching and prioritizing urgent issues across AWS infrastructure.

Your Mission: 
1. Search specified AWS log sources: %s
2. Identify urgent issues based on: %s
3. Provide analysis depth: %s

Workflow:
1. Log Collection: Access logs from CloudWatch, S3 buckets, ECS containers, Lambda functions
2. Pattern Recognition: Look for error patterns, performance anomalies, security alerts
3. Urgency Classification: 
   - CRITICAL: Service outages, security breaches, data loss
   - HIGH: Performance degradation, failed deployments, resource exhaustion
   - MEDIUM: Warnings, deprecated API usage, configuration issues
   - LOW: Info messages, debug traces, routine operations

4. Prioritization: Rank issues by business impact and time sensitivity
5. Summary Report: Provide actionable insights with timestamps and affected resources

Output Format:
- CRITICAL issues (immediate action required)
- HIGH priority issues (address within hours)
- Summary of patterns and trends
- Recommended next steps

Error Handling: If log access fails, report the specific AWS service/region and suggest troubleshooting steps.

### Execution Configuration
- Max Steps: 7 (allows thorough log analysis across multiple sources)
- Schedule: On-demand (for immediate analysis) or hourly (for continuous monitoring)
- Environment: Production environment with AWS credentials

### Success Criteria
- Identifies critical issues within 5 minutes of execution
- Provides clear priority ranking of all discovered issues
- Includes specific log entries and timestamps for evidence
- Suggests concrete remediation steps

Ready to create this agent? Use the 'create_agent' tool with these specifications.
`, logSources, urgencyCriteria, analysisDepth)

	return mcp.NewGetPromptResult("AWS Logs Analysis Agent Creator", []mcp.PromptMessage{
		{
			Role: mcp.RoleUser,
			Content: mcp.TextContent{
				Type: "text",
				Text: promptContent,
			},
		},
	}), nil
}

// handleDevOpsMonitorPrompt provides specialized guidance for DevOps monitoring agent creation
func (ts *ToolsServer) handleDevOpsMonitorPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	infrastructureType := ""
	monitoringScope := ""
	alertThresholds := ""
	
	// Extract arguments if provided
	if args := request.Params.Arguments; args != nil {
		if infra, ok := args["infrastructure_type"]; ok {
			infrastructureType = infra
		}
		if scope, ok := args["monitoring_scope"]; ok {
			monitoringScope = scope
		}
		if thresholds, ok := args["alert_thresholds"]; ok {
			alertThresholds = thresholds
		}
	}
	
	promptContent := fmt.Sprintf(`# DevOps Infrastructure Monitoring Agent Guide

## Specialized Agent Configuration

**Agent Purpose**: Continuous monitoring of infrastructure health and deployment status

### Recommended Configuration

**Agent Name**: "devops-monitor"
**Description**: "Monitors infrastructure health, deployment status, and system performance metrics"

### Tool Requirements for DevOps Monitoring

**Essential Tools** (select from available MCP tools):
- docker or kubernetes tools for container monitoring
- http-client or curl tools for health check endpoints
- file operations for log file monitoring
- json parsing tools for metrics analysis
- notification tools for alert delivery

### System Prompt Template

You are a DevOps Infrastructure Monitoring Agent specialized in maintaining system health and deployment visibility.

Your Mission:
1. Infrastructure Type: %s
2. Monitoring Scope: %s
3. Alert Thresholds: %s

Workflow:
1. Health Checks: Verify service endpoints, container status, resource utilization
2. Performance Monitoring: Track response times, error rates, resource consumption
3. Deployment Tracking: Monitor deployment status, rollback capabilities, version consistency
4. Alert Classification:
   - CRITICAL: Service down, deployment failed, resource exhausted
   - HIGH: Performance degradation, high error rates, capacity warnings
   - MEDIUM: Configuration drift, deprecated API usage, minor performance issues
   - LOW: Informational updates, successful deployments, routine maintenance

5. Automated Actions: Restart unhealthy services, scale resources, trigger rollbacks
6. Reporting: Generate status dashboards, trend analysis, incident summaries

Output Format:
- System health overview with color-coded status
- Critical issues requiring immediate attention
- Performance trends and capacity planning insights
- Deployment status and rollback recommendations

Error Handling: If monitoring tools fail, switch to backup monitoring methods and alert on monitoring system health.

### Execution Configuration
- Max Steps: 8 (comprehensive infrastructure analysis)
- Schedule: Every 15 minutes (continuous monitoring) or on-demand
- Environment: Production environment with infrastructure access

### Success Criteria
- Detects critical issues within 2 minutes of occurrence
- Maintains <5%% false positive rate on alerts
- Provides actionable remediation steps for all alerts
- Tracks deployment success rates and rollback metrics

Ready to create this monitoring agent? Use the 'create_agent' tool with these specifications.
`, infrastructureType, monitoringScope, alertThresholds)

	return mcp.NewGetPromptResult("DevOps Monitoring Agent Creator", []mcp.PromptMessage{
		{
			Role: mcp.RoleUser,
			Content: mcp.TextContent{
				Type: "text",
				Text: promptContent,
			},
		},
	}), nil
}

// handleSecurityScanPrompt provides specialized guidance for security scanning agent creation
func (ts *ToolsServer) handleSecurityScanPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	scanTargets := ""
	scanTypes := ""
	complianceFrameworks := ""
	
	// Extract arguments if provided
	if args := request.Params.Arguments; args != nil {
		if targets, ok := args["scan_targets"]; ok {
			scanTargets = targets
		}
		if types, ok := args["scan_types"]; ok {
			scanTypes = types
		}
		if frameworks, ok := args["compliance_frameworks"]; ok {
			complianceFrameworks = frameworks
		}
	}
	
	promptContent := fmt.Sprintf(`# Security Scanning Agent Creation Guide

## Specialized Agent Configuration

**Agent Purpose**: Comprehensive security scanning and vulnerability assessment across infrastructure and applications

### Recommended Configuration

**Agent Name**: "security-scanner"
**Description**: "Performs automated security scans, vulnerability assessments, and compliance checks"

### Tool Requirements for Security Scanning

**Essential Tools** (select from available MCP tools):
- git or repository tools for code scanning
- file operations for configuration analysis
- http-client tools for web application scanning
- json/yaml parsing for configuration assessment
- search tools for secret/credential detection

### System Prompt Template

You are a Security Scanning Agent specialized in identifying vulnerabilities and ensuring compliance across systems.

Your Mission:
1. Scan Targets: %s
2. Scan Types: %s
3. Compliance Frameworks: %s

Workflow:
1. Target Discovery: Identify repositories, applications, infrastructure components
2. Vulnerability Scanning:
   - Static code analysis for security flaws
   - Dependency vulnerability checking
   - Configuration security assessment
   - Infrastructure security posture review

3. Compliance Verification:
   - SOC2 controls validation
   - GDPR data protection measures
   - HIPAA security requirements
   - Industry-specific standards

4. Risk Classification:
   - CRITICAL: Active exploits, exposed credentials, unpatched vulnerabilities
   - HIGH: Known vulnerabilities, misconfigurations, weak authentication
   - MEDIUM: Security warnings, outdated dependencies, policy violations
   - LOW: Best practice recommendations, informational findings

5. Remediation Guidance: Provide specific fix instructions and timeline recommendations
6. Compliance Reporting: Generate audit-ready reports with evidence and controls mapping

Output Format:
- Executive summary with risk metrics
- Critical vulnerabilities requiring immediate patching
- Compliance status with control mappings
- Remediation roadmap with prioritized actions
- Trend analysis and security posture improvements

Error Handling: If scan tools fail, document the limitation and suggest manual verification steps.

### Execution Configuration
- Max Steps: 10 (thorough security analysis across multiple vectors)
- Schedule: Weekly (regular assessment) or on-demand (incident response)
- Environment: Security environment with appropriate scanning permissions

### Success Criteria
- Identifies critical vulnerabilities within scan scope
- Provides clear remediation steps for all findings
- Maps findings to compliance requirements
- Maintains low false positive rate (<10%%)
- Generates audit-ready compliance reports

Ready to create this security agent? Use the 'create_agent' tool with these specifications.
`, scanTargets, scanTypes, complianceFrameworks)

	return mcp.NewGetPromptResult("Security Scanning Agent Creator", []mcp.PromptMessage{
		{
			Role: mcp.RoleUser,
			Content: mcp.TextContent{
				Type: "text",
				Text: promptContent,
			},
		},
	}), nil
}

// handleDataProcessingPrompt provides specialized guidance for data processing agent creation
func (ts *ToolsServer) handleDataProcessingPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	dataSources := ""
	processingType := ""
	outputFormat := ""
	
	// Extract arguments if provided
	if args := request.Params.Arguments; args != nil {
		if sources, ok := args["data_sources"]; ok {
			dataSources = sources
		}
		if pType, ok := args["processing_type"]; ok {
			processingType = pType
		}
		if format, ok := args["output_format"]; ok {
			outputFormat = format
		}
	}
	
	promptContent := fmt.Sprintf(`# Data Processing Agent Creation Guide

## Specialized Agent Configuration

**Agent Purpose**: Intelligent data processing, transformation, and analysis pipeline automation

### Recommended Configuration

**Agent Name**: "data-processor"
**Description**: "Processes, transforms, and analyzes data from multiple sources with intelligent insights"

### Tool Requirements for Data Processing

**Essential Tools** (select from available MCP tools):
- file operations for data file access
- json/csv parsing tools for structured data
- http-client tools for API data sources
- search tools for data discovery and filtering
- database tools if available for data storage

### System Prompt Template

You are a Data Processing Agent specialized in ETL operations, data analysis, and intelligent reporting.

Your Mission:
1. Data Sources: %s
2. Processing Type: %s
3. Output Format: %s

Workflow:
1. Data Ingestion: Collect data from specified sources (APIs, files, databases)
2. Data Validation: Check data quality, completeness, and format consistency
3. Data Transformation:
   - Clean and normalize data formats
   - Handle missing or invalid data points
   - Apply business rules and transformations
   - Merge and enrich data from multiple sources

4. Analysis Operations:
   - Statistical analysis and trend identification
   - Anomaly detection in data patterns  
   - Key metrics calculation and aggregation
   - Data correlation and relationship analysis

5. Quality Assurance:
   - Data accuracy validation
   - Completeness verification
   - Consistency checks across sources
   - Error rate monitoring and reporting

6. Output Generation: Create reports, dashboards, alerts, or processed datasets

Quality Gates:
- Validate data completeness (>95%% coverage expected)
- Check for data anomalies and outliers
- Verify processing accuracy against known benchmarks
- Ensure output format meets specifications

Output Format Options:
- Reports: Executive summaries with key insights and recommendations
- Dashboards: Real-time metrics and trend visualizations
- Alerts: Automated notifications for threshold breaches or anomalies
- Datasets: Cleaned and transformed data for downstream systems

Error Handling: Log data quality issues, skip corrupted records with reporting, implement fallback data sources when possible.

### Execution Configuration
- Max Steps: 12 (comprehensive data pipeline processing)
- Schedule: Hourly (real-time processing) or daily (batch processing)
- Environment: Data environment with access to required data sources

### Success Criteria
- Processes data within defined SLA timeframes
- Maintains data quality standards (>95%% accuracy)
- Provides actionable insights and trend analysis
- Generates timely alerts for critical data patterns
- Produces consistent, reliable output formats

Ready to create this data processing agent? Use the 'create_agent' tool with these specifications.
`, dataSources, processingType, outputFormat)

	return mcp.NewGetPromptResult("Data Processing Agent Creator", []mcp.PromptMessage{
		{
			Role: mcp.RoleUser,
			Content: mcp.TextContent{
				Type: "text",
				Text: promptContent,
			},
		},
	}), nil
}
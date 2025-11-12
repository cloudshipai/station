package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"station/internal/db/queries"
	"station/internal/services"

	"github.com/mark3labs/mcp-go/mcp"
)

// Report Management Handlers

func (s *Server) handleCreateReport(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'name' parameter: %v", err)), nil
	}

	envIDStr, err := request.RequireString("environment_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'environment_id' parameter: %v", err)), nil
	}

	envID, err := strconv.ParseInt(envIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid environment_id format: %v", err)), nil
	}

	teamCriteria, err := request.RequireString("team_criteria")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'team_criteria' parameter: %v", err)), nil
	}

	// Validate team_criteria JSON
	var criteriaTest interface{}
	if err := json.Unmarshal([]byte(teamCriteria), &criteriaTest); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid team_criteria JSON: %v", err)), nil
	}

	description := request.GetString("description", "")
	agentCriteria := request.GetString("agent_criteria", "")

	// Create report in database
	var descSQL sql.NullString
	if description != "" {
		descSQL = sql.NullString{String: description, Valid: true}
	}

	var agentCriteriaSQL sql.NullString
	if agentCriteria != "" {
		agentCriteriaSQL = sql.NullString{String: agentCriteria, Valid: true}
	}

	report, err := s.repos.Reports.CreateReport(ctx, queries.CreateReportParams{
		Name:          name,
		Description:   descSQL,
		EnvironmentID: envID,
		TeamCriteria:  teamCriteria,
		AgentCriteria: agentCriteriaSQL,
		JudgeModel:    sql.NullString{String: "gpt-4o-mini", Valid: true},
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create report: %v", err)), nil
	}

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Report '%s' created successfully", name),
		"report": map[string]interface{}{
			"id":             report.ID,
			"name":           report.Name,
			"environment_id": report.EnvironmentID,
			"status":         report.Status,
			"created_at":     report.CreatedAt,
		},
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleGenerateReport(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reportIDStr, err := request.RequireString("report_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'report_id' parameter: %v", err)), nil
	}

	reportID, err := strconv.ParseInt(reportIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid report_id format: %v", err)), nil
	}

	// Get report
	report, err := s.repos.Reports.GetByID(ctx, reportID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Report not found: %v", err)), nil
	}

	// Start async report generation
	go func() {
		genCtx := context.Background()

		// Create report generator
		generator := services.NewReportGenerator(s.repos, nil)

		// Generate report (this runs benchmarks on all matching runs)
		if err := generator.GenerateReport(genCtx, reportID); err != nil {
			// Report will be marked as failed in the database
			fmt.Printf("Report generation failed: %v\n", err)
		}
	}()

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Report generation started for '%s'", report.Name),
		"report": map[string]interface{}{
			"id":     report.ID,
			"name":   report.Name,
			"status": "generating",
		},
		"note": "Use get_report to check progress and view results",
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleListReports(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := int64(50)
	limitStr := request.GetString("limit", "")
	if limitStr != "" {
		if parsed, err := strconv.ParseInt(limitStr, 10, 64); err == nil {
			limit = parsed
		}
	}

	offset := int64(0)
	offsetStr := request.GetString("offset", "")
	if offsetStr != "" {
		if parsed, err := strconv.ParseInt(offsetStr, 10, 64); err == nil {
			offset = parsed
		}
	}

	envIDStr := request.GetString("environment_id", "")

	var reports []interface{}

	if envIDStr != "" {
		envID, parseErr := strconv.ParseInt(envIDStr, 10, 64)
		if parseErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid environment_id format: %v", parseErr)), nil
		}

		reportList, err := s.repos.Reports.ListByEnvironment(ctx, envID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list reports: %v", err)), nil
		}

		for _, r := range reportList {
			reports = append(reports, r)
		}
	} else {
		reportList, err := s.repos.Reports.ListReports(ctx, queries.ListReportsParams{
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list reports: %v", err)), nil
		}

		for _, r := range reportList {
			reports = append(reports, r)
		}
	}

	response := map[string]interface{}{
		"success": true,
		"reports": reports,
		"count":   len(reports),
		"limit":   limit,
		"offset":  offset,
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (s *Server) handleGetReport(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	reportIDStr, err := request.RequireString("report_id")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Missing 'report_id' parameter: %v", err)), nil
	}

	reportID, err := strconv.ParseInt(reportIDStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid report_id format: %v", err)), nil
	}

	// Get report
	report, err := s.repos.Reports.GetByID(ctx, reportID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Report not found: %v", err)), nil
	}

	// Get agent details for this report
	agentDetails, err := s.repos.Reports.GetAgentReportDetails(ctx, reportID)
	if err != nil {
		// Non-fatal - report might not have agent details yet
		agentDetails = nil
	}

	response := map[string]interface{}{
		"success":       true,
		"report":        report,
		"agent_details": agentDetails,
	}

	resultJSON, _ := json.MarshalIndent(response, "", "  ")
	return mcp.NewToolResultText(string(resultJSON)), nil
}

# Reports System - Quick Reference

## CLI Commands

```bash
# Create a report
stn report create --env production --name "Q1 Review" --description "Quarterly review"

# Generate report (runs LLM evaluation)
stn report generate <report_id>

# List all reports
stn report list
stn report list --env production

# Show detailed report
stn report show <report_id>
```

## API Endpoints

```bash
# List reports
GET /api/v1/reports?environment_id=X

# Get report
GET /api/v1/reports/:id

# Create report
POST /api/v1/reports
{
  "name": "Report Name",
  "environment_id": 1,
  "team_criteria": { ... }
}

# Generate report (async)
POST /api/v1/reports/:id/generate

# Delete report
DELETE /api/v1/reports/:id
```

## Database Tables

- **reports**: Main report metadata, team scores, status
- **agent_report_details**: Per-agent evaluation results

## Key Files

- Service: `internal/services/report_generator.go`
- CLI: `cmd/main/handlers/report/handlers.go`
- API: `internal/api/v1/reports.go`
- DB: `internal/db/repositories/reports.go`
- Tests: `internal/services/report_generator_e2e_test.go`

## Next Steps

1. Build UI components for report visualization
2. Add WebSocket for real-time progress updates
3. Create report templates and scheduling

See `SESSION_SUMMARY_REPORTS.md` for full documentation.

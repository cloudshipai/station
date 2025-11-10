# GCP FinOps Faker Environment

Production-ready standalone faker environment for **GCP Cloud FinOps cost investigations**. This environment provides AI-generated MCP tools that simulate Google Cloud Platform's billing, resource management, and monitoring APIs without requiring real GCP credentials or access.

## Overview

This environment contains **3 specialized standalone fakers**:

1. **`gcp-billing-faker`** - GCP Cloud Billing & Cost Management API
   - Billing data queries and export analysis
   - Cost trend analysis and anomaly detection
   - Resource cost attribution and allocation
   - Budget management and variance analysis
   - Cost optimization recommendations
   - Detailed cost reporting and breakdowns

2. **`gcp-resources-faker`** - GCP Resource Management & Asset Inventory API
   - Resource inventory across compute, storage, databases
   - Resource utilization metrics and details
   - Tagging and organizational hierarchy
   - Right sizing and idle resource recommendations
   - Quota management and limit tracking

3. **`gcp-monitoring-faker`** - GCP Cloud Monitoring & Logging API
   - Cost-related metrics and alerting
   - Resource performance monitoring
   - Cost event log analysis
   - Custom cost dashboards
   - Expensive operation tracing

## Use Cases

### FinOps Investigations
- **Cost Spike Analysis**: Investigate unexpected cost increases
- **Budget Variance**: Analyze budget overruns and forecasting
- **Resource Optimization**: Identify idle and underutilized resources
- **Cost Attribution**: Track costs by project, service, label, region
- **Commitment Analysis**: Evaluate sustained use and committed use discounts
- **Cost Anomaly Detection**: Find unusual spending patterns

### DevOps Cost Management
- **Resource Right sizing**: Find over-provisioned VMs and disks
- **Idle Resource Cleanup**: Identify unused resources costing money
- **Quota Management**: Track quota usage and prevent limits
- **Cost-Performance Correlation**: Link resource performance to spend
- **API Usage Analysis**: Understand expensive API operations

## Quick Start

### 1. Sync the Environment

```bash
cd /home/epuerta/projects/hack/station
./bin/stn sync gcp-finops-faker
```

**Expected output** (first run, ~30-60s for AI generation):
```
[FAKER] 🚀 NewStandaloneFaker starting for ID: gcp-billing-faker
[FAKER] Initializing GenKit for standalone mode with provider: openai
[FAKER] GenKit initialized successfully
[FAKER] Generating tools with AI for instruction: Generate comprehensive GCP Cloud Billing...
[FAKER] Using GenerateData with model: openai/gpt-4o-mini
[FAKER] Generated 8 tools
[FAKER]   - query_billing_export: Query BigQuery billing export tables for cost data
[FAKER]   - analyze_cost_spike: Analyze cost spikes and identify root causes
[FAKER]   - get_cost_forecast: Get cost forecasts based on historical trends
[FAKER]   - get_service_costs: Get cost breakdown by GCP service
[FAKER]   - list_budgets: List all budgets for a billing account
[FAKER]   - get_cost_recommendations: Get cost optimization recommendations
[FAKER]   - get_cost_breakdown_by_service: Get detailed service cost breakdown
[FAKER]   - generate_cost_report: Generate comprehensive cost reports

[FAKER] 🚀 NewStandaloneFaker starting for ID: gcp-resources-faker
...
[FAKER] Generated 9 tools
...

[FAKER] 🚀 NewStandaloneFaker starting for ID: gcp-monitoring-faker
...
[FAKER] Generated 7 tools
...

✅ Environment 'gcp-finops-faker' synced successfully
```

**Subsequent runs** (<5s, tools loaded from cache):
```
[FAKER] Loading cached tools for faker ID: gcp-billing-faker
[FAKER] Loaded 8 tools from cache
...
```

### 2. List MCP Servers

```bash
./bin/stn mcp list --env gcp-finops-faker
```

**Output**:
```
┌────────────────────────┬────────────────────┬──────────┐
│ SERVER NAME            │ COMMAND            │ TOOLS    │
├────────────────────────┼────────────────────┼──────────┤
│ gcp-billing-faker      │ stn faker          │ 8        │
│ gcp-resources-faker    │ stn faker          │ 9        │
│ gcp-monitoring-faker   │ stn faker          │ 7        │
└────────────────────────┴────────────────────┴──────────┘
```

### 3. Discover Tools

```bash
./bin/stn mcp discover --env gcp-finops-faker
```

**Sample Tools Generated**:

**Billing Tools** (`gcp-billing-faker`):
- `query_billing_export` - Query BigQuery billing export tables
- `analyze_cost_spike` - Analyze cost spikes with root cause analysis
- `get_cost_forecast` - Get cost forecasts based on trends
- `compare_period_costs` - Compare costs between time periods
- `detect_cost_anomalies` - Detect unusual cost patterns
- `get_service_costs` - Get costs by GCP service
- `get_project_costs` - Get costs by project
- `list_budgets` - List all budgets
- `get_budget_status` - Get current budget status
- `get_cost_recommendations` - Get optimization recommendations
- `get_cost_breakdown_by_service` - Detailed service breakdown
- `generate_cost_report` - Generate comprehensive reports

**Resource Tools** (`gcp-resources-faker`):
- `list_compute_instances` - List GCE instances
- `list_gke_clusters` - List GKE clusters
- `list_cloud_sql_instances` - List Cloud SQL databases
- `list_storage_buckets` - List GCS buckets
- `get_instance_details` - Get VM instance details
- `get_instance_metrics` - Get VM utilization metrics
- `list_resources_by_label` - Find resources by labels
- `get_rightsizing_recommendations` - Get VM sizing recommendations
- `list_idle_vms` - Find idle compute instances
- `get_project_quotas` - Get project quotas and limits

**Monitoring Tools** (`gcp-monitoring-faker`):
- `query_cost_metrics` - Query cost-related metrics
- `list_cost_alerts` - List cost alert policies
- `get_vm_performance` - Get VM performance metrics
- `search_cost_logs` - Search logs for cost events
- `get_billing_logs` - Get billing event logs
- `find_resource_creation_events` - Track resource creation
- `list_expensive_traces` - Find expensive trace operations
- `analyze_api_cost_patterns` - Analyze API usage costs

### 4. Create FinOps Agent

Create an agent to perform cost investigations:

```bash
./bin/stn agent create \
  --name "GCP Cost Investigator" \
  --description "Investigates GCP cost spikes, analyzes spending patterns, and provides optimization recommendations" \
  --env gcp-finops-faker \
  --model gpt-4o-mini \
  --prompt "You are a GCP FinOps expert specialized in cost investigations. When analyzing costs:
1. Start by querying billing data for the specified time period
2. Identify cost spikes and anomalies
3. Drill down into specific services, projects, or resources
4. Correlate costs with resource usage and performance metrics
5. Provide actionable optimization recommendations
6. Generate comprehensive cost reports with insights

Always explain your findings clearly and suggest specific next steps."
```

### 5. Run Cost Investigation

```bash
./bin/stn agent run "GCP Cost Investigator" \
  "Investigate the cost spike in project 'prod-webapp' for the last 7 days. Analyze which GCP services are driving the increase, identify any idle or underutilized resources, and provide cost optimization recommendations." \
  --tail
```

**Expected Workflow**:
1. Agent calls `query_billing_export` with project filter
2. Agent calls `analyze_cost_spike` to identify causes
3. Agent calls `get_service_costs` for service breakdown
4. Agent calls `list_compute_instances` to check resources
5. Agent calls `list_idle_vms` to find optimization opportunities
6. Agent calls `get_cost_recommendations` for suggestions
7. Agent generates summary with actionable insights

## Example Investigation Scenarios

### Scenario 1: Cost Spike Root Cause Analysis
```bash
./bin/stn agent run "GCP Cost Investigator" \
  "We had a 300% cost spike on November 8th. Investigate what caused it, which services were affected, and if there were any unusual resource creation or API usage patterns."
```

### Scenario 2: Budget Variance Investigation
```bash
./bin/stn agent run "GCP Cost Investigator" \
  "Our Q4 budget for project 'ml-training' is tracking 45% over forecast. Analyze the variance, identify the top cost drivers, and recommend adjustments to stay within budget."
```

### Scenario 3: Idle Resource Audit
```bash
./bin/stn agent run "GCP Cost Investigator" \
  "Perform a comprehensive audit of all idle and underutilized resources across all projects. Calculate potential monthly savings and provide a prioritized cleanup plan."
```

### Scenario 4: Multi-Project Cost Attribution
```bash
./bin/stn agent run "GCP Cost Investigator" \
  "Compare costs across all projects for the last 30 days. Identify which projects have the highest spend, fastest growth, and best cost efficiency. Flag any anomalies."
```

### Scenario 5: Commitment Analysis
```bash
./bin/stn agent run "GCP Cost Investigator" \
  "Analyze our sustained use discounts and committed use discount coverage. Are we over-committed or under-committed? What changes would optimize our discount strategy?"
```

## Tool Cache Management

### View Cached Tools

```bash
sqlite3 ~/.config/station/station.db "
SELECT 
  faker_id, 
  COUNT(*) as tool_count,
  datetime(MAX(created_at)) as last_generated
FROM faker_tool_cache 
WHERE faker_id LIKE 'gcp-%'
GROUP BY faker_id;"
```

**Output**:
```
gcp-billing-faker|8|2025-11-10 18:50:23
gcp-resources-faker|9|2025-11-10 18:50:45
gcp-monitoring-faker|7|2025-11-10 18:51:02
```

### Regenerate Tools

To get fresh tool schemas (if you want different tools):

```bash
sqlite3 ~/.config/station/station.db "DELETE FROM faker_tool_cache WHERE faker_id = 'gcp-billing-faker';"
./bin/stn sync gcp-finops-faker
```

## Faker Sessions

### View Tool Call History

```bash
# List all faker sessions
./bin/stn faker sessions list

# View detailed session with all tool calls
./bin/stn faker sessions view <session-id>

# View aggregated metrics
./bin/stn faker metrics
```

### Export Session for Analysis

```bash
./bin/stn faker sessions replay <session-id> > investigation.json
```

This exports the complete investigation workflow including:
- All tool calls with parameters
- AI-generated responses
- Timing information
- Tool call sequence

Use this to:
- Share investigation methodologies
- Create replayable test scenarios
- Analyze agent behavior
- Document cost findings

## Architecture

```
┌─────────────────────────────────────────┐
│  GCP FinOps Faker Environment           │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  gcp-billing-faker (8 tools)       │ │
│  │  - Billing queries & exports       │ │
│  │  - Cost analysis & forecasting     │ │
│  │  - Budget management               │ │
│  │  - Optimization recommendations    │ │
│  └────────────────────────────────────┘ │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  gcp-resources-faker (9 tools)     │ │
│  │  - Resource inventory              │ │
│  │  - Utilization metrics             │ │
│  │  - Tagging & organization          │ │
│  │  - Rightsizing recommendations     │ │
│  └────────────────────────────────────┘ │
│                                          │
│  ┌────────────────────────────────────┐ │
│  │  gcp-monitoring-faker (7 tools)    │ │
│  │  - Cost metrics & alerts           │ │
│  │  - Performance monitoring          │ │
│  │  - Log analysis                    │ │
│  │  - Trace cost correlation          │ │
│  └────────────────────────────────────┘ │
└─────────────────────────────────────────┘
                  │
                  │ MCP Protocol
                  ▼
┌─────────────────────────────────────────┐
│  GCP Cost Investigator Agent            │
│  - Analyzes cost spikes                 │
│  - Identifies optimization opportunities│
│  - Generates actionable recommendations │
│  - Produces comprehensive reports       │
└─────────────────────────────────────────┘
```

## Benefits of Standalone Fakers for FinOps

1. **No GCP Credentials Required** - Test FinOps workflows without production access
2. **Realistic Data** - AI generates plausible cost data and metrics
3. **Fast Iterations** - Instantly test investigation scenarios
4. **Cost-Free** - No actual GCP API charges
5. **Reproducible** - Tools cached for consistent testing
6. **Safe Exploration** - Experiment with complex queries safely
7. **Agent Training** - Perfect for developing FinOps agent capabilities

## Integration with Real GCP APIs

Once your FinOps agents are working well with fakers, you can:

1. **Switch to Real MCP Servers**: Replace standalone fakers with real GCP MCP servers
2. **Hybrid Approach**: Use fakers for development, real APIs for production
3. **Validation Testing**: Compare faker responses with real API responses
4. **Cost Estimation**: Test cost analysis logic before running on real data

## Advanced Usage

### Custom AI Instructions

Edit `template.json` to customize tool generation:

```json
{
  "mcpServers": {
    "gcp-billing-faker": {
      "args": [
        "--ai-instruction", "Your custom instruction focusing on specific GCP services or cost scenarios"
      ]
    }
  }
}
```

Then regenerate:
```bash
rm ~/.config/station/station.db
./bin/stn sync gcp-finops-faker
```

### Multi-Environment Testing

Create variations for different scenarios:
- `gcp-finops-faker-prod` - Production-scale cost data
- `gcp-finops-faker-dev` - Development environment costs
- `gcp-finops-faker-spike` - Cost spike simulation

### CloudShip Integration

This environment can feed data into CloudShip for:
- FinOps inventory dashboards
- Cost opportunity tracking
- Budget projection analysis
- Optimization event streams

## Troubleshooting

### Tools Not Generating
```bash
# Check faker debug logs
./bin/stn sync gcp-finops-faker 2>&1 | grep FAKER

# Verify AI provider is configured
cat ~/.config/station/config.yaml | grep -A 3 ai
```

### Inconsistent Tool Names
```bash
# Clear cache and regenerate
sqlite3 ~/.config/station/station.db "DELETE FROM faker_tool_cache WHERE faker_id LIKE 'gcp-%';"
./bin/stn sync gcp-finops-faker
```

### Agent Not Finding Tools
```bash
# Verify tools are registered
./bin/stn mcp discover --env gcp-finops-faker | wc -l

# Check agent tool assignment
./bin/stn agent get "GCP Cost Investigator" | grep tools
```

## Next Steps

1. ✅ **Environment Created** - You're here!
2. 🔄 **Sync Environment** - Run `stn sync gcp-finops-faker`
3. 🤖 **Create Agent** - Build your FinOps investigation agent
4. 🔍 **Run Investigation** - Test cost analysis scenarios
5. 📊 **Analyze Results** - Review faker sessions and agent findings
6. 🚀 **Production Ready** - Migrate to real GCP MCP servers when ready

---

**Environment Status**: ✅ Ready for Testing  
**Tools**: ~24 AI-generated GCP FinOps tools  
**Cost**: $0 (no real GCP charges)  
**Setup Time**: ~60s (first run), ~5s (cached)

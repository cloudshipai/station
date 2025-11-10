# AWS FinOps Standalone Faker Scenario

## Overview
Complex multi-faker AWS FinOps investigation scenario with 3 standalone fakers and 2 specialized agents.

## Configuration

**Environment**: `aws-finops-faker`  
**Location**: `~/.config/station/environments/aws-finops-faker/`

### Fakers (3 total)

#### 1. aws-cost-faker
**AI Generated Tools** (5):
- `get_anomalies` - Detect cost anomalies and spikes
- `get_cost_and_usage` - Query AWS Cost Explorer data
- `get_ec2_costs` - EC2-specific cost breakdowns
- `get_ri_utilization` - Reserved Instance utilization metrics
- `list_budgets` - AWS budget management

#### 2. aws-resources-faker  
**AI Generated Tools** (8):
- `ec2_instance_inventory` - List all EC2 instances
- `list_reserved_instances` - List RI commitments
- `describe_db_instances` - RDS database inventory
- `analyze_db_sizing` - RDS rightsizing analysis
- `list_ebs_volumes` - EBS volume inventory
- `find_idle_resources` - Identify unused resources
- `get_rightsizing_recommendations` - Instance optimization
- `get_tag_coverage_report` - Tagging compliance

#### 3. aws-cloudwatch-faker
**AI Generated Tools** (5):
- `get_cost_metrics` - CloudWatch cost-related metrics
- `list_cost_alarms` - Cost budget alarms
- `analyze_metric_anomalies` - Metric anomaly detection
- `query_cloudtrail_events` - Audit log queries
- `find_high_cost_api_calls` - Expensive API operations

### Agents (2 total)

#### 1. AWS Cost Investigator
- **Max Steps**: 15
- **Focus**: Cost anomaly detection, root cause analysis, optimization
- **Tools**: 18 (all tools from all 3 fakers)
- **Workflow**: Anomaly detection → Service attribution → Resource investigation → Root cause → Recommendations

#### 2. AWS Reserved Instance Optimizer  
- **Max Steps**: 12
- **Focus**: RI/Savings Plans utilization, purchase recommendations
- **Tools**: 12 (cost + resource tools)
- **Workflow**: Current commitment analysis → On-demand exposure → Purchase recommendations → Financial projections

## Test Scenarios

### Scenario 1: EC2 Cost Spike Investigation
**Trigger**: "Investigate why EC2 costs increased 300% in the last week"

**Expected Workflow**:
1. `get_anomalies` - Detect 300% spike in us-east-1
2. `get_ec2_costs` - Identify spike in m5.large instances
3. `ec2_instance_inventory` - Find 150 new instances launched
4. `query_cloudtrail_events` - Identify autoscaling group change
5. `find_idle_resources` - Detect 80 instances with <5% CPU
6. `get_rightsizing_recommendations` - Suggest downsizing to t3.medium
7. **Output**: Root cause (bad autoscaling config), immediate actions (terminate idle), savings ($45k/month)

### Scenario 2: Reserved Instance Optimization
**Trigger**: "Analyze our RI utilization and recommend purchases"

**Expected Workflow**:
1. `get_ri_utilization` - Current RI utilization: 72%
2. `list_reserved_instances` - 50 RIs expiring in 90 days
3. `ec2_instance_inventory` - 200 on-demand m5.xlarge instances
4. `get_cost_and_usage` - $85k/month on-demand spend
5. `get_rightsizing_recommendations` - Purchase 150 m5.xlarge 1-year RIs
6. **Output**: Projected savings $32k/month, break-even in 3 months

### Scenario 3: Comprehensive Cost Audit  
**Trigger**: "Run a full cost optimization audit for our AWS account"

**Expected Workflow**:
1. `get_cost_and_usage` - Total monthly spend: $250k
2. `get_ec2_costs` - EC2: $120k (48%)
3. `get_anomalies` - Detect S3 transfer cost spike
4. `list_budgets` - Engineering team 130% over budget
5. `ec2_instance_inventory` - 500 total instances
6. `find_idle_resources` - 75 idle instances ($18k/month waste)
7. `describe_db_instances` - 20 RDS instances
8. `analyze_db_sizing` - 8 RDS instances oversized
9. `get_tag_coverage_report` - 40% untagged resources
10. `get_rightsizing_recommendations` - $67k/month potential savings
11. **Output**: Multi-layered optimization plan with $85k/month total savings

## Key Metrics

- **Tools Generated**: 18 total (5 + 8 + 5)
- **Tool Generation Time**: <5s per faker (cached)
- **Agent Execution Time Target**: <90s for full investigation
- **Data Realism**: Agents believe data is from real AWS APIs

## Limitations & Future Enhancements

**Current Limitations**:
1. AI generates only 5-8 tools per faker (not comprehensive 20+ as requested)
2. No response caching yet (each tool call = new OpenAI request)
3. No cross-call state management (each tool call is independent)

**Future Enhancements**:
1. Response caching by parameters to reduce OpenAI calls
2. Stateful tool calls (remember previous queries)
3. More tools per faker (target: 20-30 tools each)
4. Multi-cloud scenario (AWS + GCP + Azure)
5. Security incident + cost impact correlation

## Usage

```bash
# Sync environment (creates fakers + agents)
stn sync aws-finops-faker

# List agents
stn agent list --env aws-finops-faker

# Run cost investigation
stn agent run "AWS Cost Investigator" \
  "Investigate why EC2 costs spiked 300% last week in us-east-1" \
  --env aws-finops-faker --tail

# Run RI optimization
stn agent run "AWS Reserved Instance Optimizer" \
  "Analyze our current RI utilization and recommend optimal purchases for steady-state workloads" \
  --env aws-finops-faker --tail
```

## Success Criteria

✅ **Faker Generation**: <5s per faker initialization  
✅ **Tool Caching**: Tools persist across agent runs  
✅ **Agent Execution**: Complete investigation in <90s  
🔄 **Data Realism**: Agents accept AI data as real (testing required)  
🔄 **Actionable Output**: Clear recommendations with $ amounts (testing required)

## Related Documentation

- `test-environments/README_COMPLEX_SCENARIOS.md` - Complex scenario designs
- `FAKER_AGENT_SUCCESS.md` - GCP FinOps faker success story  
- `STANDALONE_FAKER_SUCCESS.md` - Standalone faker architecture

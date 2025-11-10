# Complex Faker Scenario Designs

## Overview
This document outlines complex multi-faker, multi-agent scenarios for testing Station's standalone faker capabilities with realistic workflow simulations.

## Scenario 1: Multi-Cloud Cost Anomaly Investigation
**Complexity**: Hard (5 fakers, 4 agents, cross-cloud correlation)

### Fakers:
1. **AWS Billing Faker**: CloudWatch billing metrics, Cost Explorer data
2. **GCP Billing Faker**: BigQuery billing exports, cost breakdowns
3. **Azure Billing Faker**: Azure Cost Management data
4. **Kubernetes Resource Faker**: Multi-cluster resource usage (AWS EKS, GCP GKE, Azure AKS)
5. **Observability Faker**: Datadog/Prometheus metrics showing traffic patterns

### Agents:
1. **Multi-Cloud Cost Orchestrator**: Coordinates investigation across clouds
2. **AWS Cost Specialist**: Deep dive into AWS billing anomalies
3. **GCP Cost Specialist**: Analyzes GCP billing patterns
4. **Kubernetes Resource Analyzer**: Correlates K8s resource usage with cloud costs

### Scenario Details:
- **Trigger**: 200% cost spike across all clouds in last 7 days
- **Root Cause**: Kubernetes cluster autoscaling bug causing over-provisioning
- **Evidence Chain**:
  - AWS: EC2 costs +$50k (500 new nodes)
  - GCP: GCE costs +$35k (350 new nodes)  
  - Azure: VM costs +$28k (280 new nodes)
  - K8s: HPA scaled to 10x capacity due to bad custom metric
  - Datadog: Traffic unchanged (proof of over-provisioning)

---

## Scenario 2: Security Incident + Cost Impact Analysis
**Complexity**: Very Hard (6 fakers, 5 agents, security + finops correlation)

### Fakers:
1. **AWS Security Faker**: GuardDuty findings, CloudTrail audit logs
2. **GCP Security Faker**: Security Command Center, Cloud Logging
3. **Cryptocurrency Mining Faker**: Simulated cryptominer resource usage patterns
4. **Billing Impact Faker**: Real-time cost tracking during incident
5. **Network Traffic Faker**: VPC flow logs, egress traffic patterns
6. **Compute Resource Faker**: EC2/GCE instance lifecycle events

### Agents:
1. **Security Incident Commander**: Orchestrates security + cost response
2. **Threat Analyst**: Identifies cryptomining activity
3. **Cost Impact Analyst**: Quantifies financial damage in real-time
4. **Remediation Specialist**: Terminates malicious resources
5. **Post-Mortem Generator**: Creates comprehensive incident report

### Scenario Details:
- **Trigger**: GuardDuty alerts for cryptomining activity + $85k cost spike
- **Attack Vector**: Compromised CI/CD credentials launched 2,000 GPU instances
- **Investigation Timeline**:
  - T+0: GuardDuty detects outbound traffic to mining pool
  - T+15min: Cost spike detected ($12k/hour burn rate)
  - T+30min: CloudTrail reveals compromised service account
  - T+45min: 2,000 p3.8xlarge instances discovered across 5 regions
  - T+1hr: Instances terminated, credentials rotated
  - T+24hr: Final cost impact: $85,234
  
---

## Scenario 3: Multi-Region DR Failover + Cost Optimization
**Complexity**: Hard (5 fakers, 4 agents, stateful resource tracking)

### Fakers:
1. **Primary Region Faker (us-east-1)**: Simulates region outage, failing health checks
2. **DR Region Faker (us-west-2)**: Simulates DR resources spinning up
3. **Database Replication Faker**: RDS/Aurora replication lag, failover events
4. **Cost Optimization Faker**: Reserved Instance utilization, Savings Plans
5. **SRE Runbook Faker**: Incident procedures, escalation policies

### Agents:
1. **DR Orchestrator**: Manages failover procedure
2. **Database Specialist**: Handles data layer failover
3. **Cost Optimizer**: Analyzes DR resource efficiency
4. **Compliance Auditor**: Verifies RTO/RPO requirements met

### Scenario Details:
- **Trigger**: AWS us-east-1 region degradation (simulated)
- **Response**: Automated failover to us-west-2
- **Cost Analysis**:
  - Running dual regions: +$150k/month
  - Under-utilized Reserved Instances in DR: -$45k waste
  - Recommendation: Switch to on-demand + Savings Plans in DR
  - Potential savings: $45k/month while maintaining RTO/RPO

---

## Scenario 4: Real-Time FinOps for Black Friday Traffic
**Complexity**: Hard (4 fakers, 3 agents, time-series simulation)

### Fakers:
1. **E-Commerce Traffic Faker**: Simulates 50x traffic surge over 72 hours
2. **Auto-Scaling Faker**: ECS/EKS scaling events every 5 minutes
3. **Database Performance Faker**: RDS read replica lag, connection pools
4. **Real-Time Cost Faker**: Per-minute cost tracking during surge

### Agents:
1. **Traffic Forecaster**: Predicts next 6hr traffic patterns
2. **Capacity Planner**: Pre-scales resources before surges
3. **Cost Efficiency Monitor**: Monitors cost-per-transaction in real-time

### Scenario Details:
- **Timeline**: Black Friday 72-hour event (Thu 6pm - Sun 6pm)
- **Traffic Pattern**:
  - Thu 6pm: 10k req/s baseline
  - Fri 12am: Surge to 500k req/s (50x)
  - Fri-Sun: Sustained 200-400k req/s
  - Sun 6pm: Return to 15k req/s
- **Cost Optimization**:
  - Baseline: $1,200/day ($0.10 per 1k requests)
  - Peak without optimization: $180k/day ($0.36 per 1k requests)
  - With capacity planning: $85k/day ($0.17 per 1k requests)
  - **Savings**: $95k/day = $285k over weekend

---

## Scenario 5: SaaS Multi-Tenant Cost Allocation
**Complexity**: Very Hard (3 fakers, 4 agents, complex attribution logic)

### Fakers:
1. **Multi-Tenant Resource Faker**: Simulates 500 customer workloads with tagging
2. **Shared Services Faker**: Load balancers, databases, caching layers
3. **Cost Allocation Faker**: Per-tenant cost breakdowns with attribution models

### Agents:
1. **Cost Allocation Orchestrator**: Coordinates tenant billing
2. **Resource Tagger**: Ensures proper cost tagging
3. **Shared Cost Allocator**: Distributes shared service costs
4. **Customer Billing Generator**: Creates per-customer invoices

### Scenario Details:
- **Challenge**: 500 SaaS customers, shared infrastructure, need accurate per-customer costs
- **Cost Attribution Models**:
  - Direct costs: EC2/RDS tagged by customer_id (easy)
  - Shared ALB: Distribute by request count (medium)
  - Shared RDS: Distribute by connection time + query count (hard)
  - Shared Redis: Distribute by key namespace size (hard)
- **Complexity**: Some customers cost 100x more than others due to usage patterns
- **Goal**: Identify unprofitable customers, recommend pricing changes

---

## Implementation Priority

### Phase 1 (Current): Single-Cloud FinOps
- ✅ GCP FinOps Faker (3 fakers, 2 agents) - **COMPLETE**
- 🔄 AWS FinOps Faker (3 fakers, 2 agents) - **NEXT**

### Phase 2: Multi-Cloud Scenarios
- Scenario 1: Multi-Cloud Cost Anomaly (5 fakers, 4 agents)
- Scenario 3: DR Failover + Cost Optimization (5 fakers, 4 agents)

### Phase 3: Advanced Scenarios
- Scenario 2: Security Incident + Cost Impact (6 fakers, 5 agents)
- Scenario 4: Black Friday Real-Time FinOps (4 fakers, 3 agents)
- Scenario 5: SaaS Multi-Tenant Allocation (3 fakers, 4 agents)

---

## Key Metrics for Success

Each scenario should demonstrate:
1. **Tool Generation Speed**: <5s per faker initialization
2. **Agent Execution Time**: <90s for full investigation
3. **Data Realism**: Agents convinced data is real (no questioning)
4. **Actionable Output**: Clear root causes and recommendations
5. **Cross-Faker Correlation**: Agents successfully correlate data across fakers

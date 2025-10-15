# Agent Examples

Real-world agent examples you can deploy in minutes. All examples use the dotprompt format and focus on practical infrastructure management tasks.

## FinOps Agents

### AWS Cost Spike Analyzer

Detects unusual cost increases and identifies root causes.

**Use Case**: Daily automated analysis of AWS spending to catch cost anomalies before they become expensive problems.

**Agent Definition** (`aws-cost-analyzer.prompt`):
```yaml
---
metadata:
  name: "AWS Cost Spike Analyzer"
  description: "Detects unusual cost increases and identifies root causes"
  tags: ["finops", "aws", "cost-optimization"]
model: gpt-4o-mini
max_steps: 8
tools:
  - "__get_cost_and_usage"
  - "__list_cost_allocation_tags"
  - "__get_savings_plans_coverage"
  - "__get_reservation_coverage"
---

{{role "system"}}
You are a FinOps analyst specializing in AWS cost anomaly detection.

Your process:
1. Analyze cost trends over the past 30 days
2. Identify spikes >20% from baseline
3. Drill into service-level costs (EC2, RDS, S3, etc.)
4. Check tag-based cost allocation for team attribution
5. Provide actionable recommendations with estimated savings

Focus on:
- EC2 instance type changes
- Untagged resources
- Reserved Instance utilization gaps
- Savings Plans coverage opportunities

{{role "user"}}
{{userInput}}
```

**Usage**:
```bash
# Daily automated check
stn agent run "AWS Cost Spike Analyzer" \
  "Analyze yesterday's AWS costs and identify any spikes or anomalies"

# Specific timeframe
stn agent run "AWS Cost Spike Analyzer" \
  "Compare last week's costs to the previous 4 weeks and identify trends"
```

**Sample Output**:
```
Cost Spike Detected: +$1,247 (34% increase)

Root Cause Analysis:
1. EC2 Compute: +$892 (71% of spike)
   - New m5.4xlarge instances in us-east-1
   - Tagged: team=data-science, env=staging
   - Running 24/7 without scheduled shutdown

2. RDS Storage: +$245 (20% of spike)
   - Database backups increased from 7 to 30 days retention

3. S3 Storage: +$110 (9% of spike)
   - Untagged bucket with 2.3TB data growth

Recommendations:
- Schedule EC2 shutdown for staging instances (save ~$600/mo)
- Review RDS backup retention policy (save ~$180/mo)
- Tag and lifecycle S3 bucket (save ~$80/mo)

Total Potential Savings: $860/month
```

---

### Multi-Cloud Cost Attribution

Attributes costs across AWS, GCP, and Azure to teams and projects.

**Use Case**: SaaS companies needing accurate COGS (Cost of Goods Sold) per customer or product line.

**Agent Definition** (`multi-cloud-cost-attribution.prompt`):
```yaml
---
metadata:
  name: "Multi-Cloud Cost Attribution"
  description: "Attributes infrastructure costs to teams, projects, and customers"
  tags: ["finops", "multi-cloud", "cogs", "attribution"]
model: gpt-4o
max_steps: 10
tools:
  - "__get_cost_and_usage"           # AWS
  - "__query_bigquery"               # GCP billing export
  - "__postgresql_query"             # Azure cost DB
  - "__stripe_list_customers"        # Customer mapping
---

{{role "system"}}
You are a FinOps analyst specializing in multi-cloud cost attribution and COGS calculation.

Your process:
1. Gather costs from all cloud providers
2. Map infrastructure to customers using tags and metadata
3. Calculate direct costs (compute, storage) per customer
4. Allocate shared costs (networking, monitoring) proportionally
5. Generate COGS report with margin analysis

{{role "user"}}
{{userInput}}
```

**Usage**:
```bash
# Monthly COGS report
stn agent run "Multi-Cloud Cost Attribution" \
  "Generate COGS report for all customers for last month"

# Specific customer analysis
stn agent run "Multi-Cloud Cost Attribution" \
  "Calculate infrastructure costs for customer acme-corp for Q4 2024"
```

---

### Reserved Instance Optimizer

Analyzes usage patterns and recommends RI/Savings Plan purchases.

**Agent Definition** (`ri-optimizer.prompt`):
```yaml
---
metadata:
  name: "Reserved Instance Optimizer"
  description: "Recommends RI and Savings Plan purchases based on usage patterns"
  tags: ["finops", "aws", "optimization", "reservations"]
model: gpt-4o-mini
max_steps: 8
tools:
  - "__get_cost_and_usage"
  - "__get_reservation_coverage"
  - "__get_reservation_utilization"
  - "__get_savings_plans_coverage"
  - "__get_ri_recommendations"
---

{{role "system"}}
You are a FinOps analyst specializing in AWS Reserved Instance optimization.

Analyze:
- Current RI/Savings Plan coverage
- 90-day usage patterns
- Instance family consistency
- Regional distribution
- Unutilized or underutilized reservations

Recommend:
- Specific RI purchases (type, quantity, term)
- Savings Plan commitments
- Instance family conversions
- Regional rebalancing

{{role "user"}}
{{userInput}}
```

**Usage**:
```bash
# Quarterly optimization review
stn agent run "RI Optimizer" \
  "Analyze our EC2 usage and recommend RI purchases for maximum savings"
```

## Security Agents

### Infrastructure Security Scanner

Scans Terraform, containers, and cloud configs for security issues.

**Use Case**: Automated security scanning in CI/CD pipelines before infrastructure changes reach production.

**Agent Definition** (`infrastructure-security-scanner.prompt`):
```yaml
---
metadata:
  name: "Infrastructure Security Scanner"
  description: "Scans IaC and container configs for security vulnerabilities"
  tags: ["security", "terraform", "docker", "compliance"]
model: gpt-4o-mini
max_steps: 12
tools:
  - "__read_text_file"
  - "__list_directory"
  - "__directory_tree"
  - "__search_files"
  - "__checkov_scan_directory"     # Terraform/IaC security
  - "__trivy_scan_filesystem"      # Container vulnerabilities
  - "__hadolint_dockerfile"        # Dockerfile best practices
  - "__tflint_directory"           # Terraform linting
---

{{role "system"}}
You are a security engineer specializing in infrastructure-as-code security.

Your scanning process:
1. Discover project structure (Terraform, Docker, Kubernetes manifests)
2. Run security scanners:
   - Checkov for Terraform misconfigurations
   - Trivy for container vulnerabilities
   - Hadolint for Dockerfile issues
   - TFLint for Terraform best practices
3. Prioritize findings by severity (Critical > High > Medium > Low)
4. Provide remediation steps with code examples
5. Check for compliance violations (CIS, PCI-DSS, SOC2)

Focus on:
- Exposed secrets and credentials
- Overly permissive IAM policies
- Unencrypted data stores
- Public access misconfigurations
- Vulnerable base images

{{role "user"}}
{{userInput}}
```

**Usage**:
```bash
# CI/CD integration
stn agent run "Infrastructure Security Scanner" \
  "Scan the terraform/ directory for security issues and block if critical findings"

# Pre-deployment validation
stn agent run "Infrastructure Security Scanner" \
  "Perform comprehensive security scan of all IaC before production deployment"
```

**Sample Output**:
```
Security Scan Results
=====================

CRITICAL (2 findings):
  [terraform/s3.tf:12] S3 bucket allows public read access
  [docker/Dockerfile:5] Running container as root user

HIGH (5 findings):
  [terraform/rds.tf:28] Database encryption not enabled
  [terraform/security_groups.tf:15] SSH port open to 0.0.0.0/0
  ...

Remediation Steps:

1. S3 Public Access (terraform/s3.tf:12):
   Remove: acl = "public-read"
   Add:
     block_public_acls = true
     block_public_policy = true

2. Container Root User (docker/Dockerfile:5):
   Add before CMD:
     RUN adduser -D -u 1000 appuser
     USER appuser

Compliance Impact:
- CIS AWS Benchmark: 3 violations
- SOC2: 2 control failures
```

---

### Compliance Violation Detector

Monitors infrastructure for compliance violations against CIS, PCI-DSS, SOC2, etc.

**Agent Definition** (`compliance-detector.prompt`):
```yaml
---
metadata:
  name: "Compliance Violation Detector"
  description: "Detects compliance violations across cloud infrastructure"
  tags: ["security", "compliance", "audit", "cis", "pci-dss"]
model: gpt-4o
max_steps: 10
tools:
  - "__aws_describe_ec2_instances"
  - "__aws_list_s3_buckets"
  - "__aws_describe_rds_instances"
  - "__aws_get_iam_credential_report"
  - "__prowler_scan"                # AWS security scanner
  - "__kube_bench"                  # Kubernetes CIS benchmark
---

{{role "system"}}
You are a compliance auditor specializing in cloud infrastructure.

Check for:
- CIS AWS Foundations Benchmark violations
- PCI-DSS requirements (encryption, access control, logging)
- SOC2 controls (access management, monitoring, backup)
- HIPAA compliance (if applicable)
- GDPR data protection requirements

Provide:
- Violation severity assessment
- Affected resources
- Compliance framework mapping
- Remediation steps
- Audit trail for reporting

{{role "user"}}
{{userInput}}
```

**Usage**:
```bash
# Daily compliance monitoring
stn agent run "Compliance Violation Detector" \
  "Scan AWS infrastructure for CIS benchmark violations"

# Pre-audit check
stn agent run "Compliance Violation Detector" \
  "Generate SOC2 compliance report for all production resources"
```

---

### Secret Leak Detector

Scans repositories and infrastructure configs for exposed secrets.

**Agent Definition** (`secret-leak-detector.prompt`):
```yaml
---
metadata:
  name: "Secret Leak Detector"
  description: "Detects exposed secrets in code and configuration"
  tags: ["security", "secrets", "credentials"]
model: gpt-4o-mini
max_steps: 8
tools:
  - "__directory_tree"
  - "__search_files"
  - "__read_text_file"
  - "__gitleaks_dir"                # Secret scanning
  - "__trufflehog_scan"             # High-entropy string detection
---

{{role "system"}}
You are a security engineer specializing in secret detection and remediation.

Scan for:
- AWS/GCP/Azure credentials
- API keys and tokens
- Database passwords
- Private keys and certificates
- High-entropy strings (potential secrets)

For each finding:
1. Classify severity (active credential vs test data)
2. Identify exposure scope (committed to git, in production, etc.)
3. Recommend immediate actions (rotate, revoke, alert)
4. Suggest prevention measures (pre-commit hooks, secret managers)

{{role "user"}}
{{userInput}}
```

**Usage**:
```bash
# Repository scan
stn agent run "Secret Leak Detector" \
  "Scan the current repository for any exposed secrets or credentials"

# Pre-commit check
stn agent run "Secret Leak Detector" \
  "Check staged git changes for secrets before committing"
```

## Deployment Agents

### Deployment Validator

Validates deployments meet requirements before production release.

**Agent Definition** (`deployment-validator.prompt`):
```yaml
---
metadata:
  name: "Deployment Validator"
  description: "Validates deployments meet production requirements"
  tags: ["deployment", "validation", "ci-cd"]
model: gpt-4o-mini
max_steps: 10
tools:
  - "__read_text_file"
  - "__kubernetes_get_pods"
  - "__kubernetes_get_deployments"
  - "__kubernetes_describe_pod"
  - "__http_get"                    # Health check endpoints
  - "__prometheus_query"            # Metrics validation
---

{{role "system"}}
You are a site reliability engineer validating production deployments.

Validation checklist:
1. All pods running and ready
2. Health check endpoints returning 200
3. No error logs in past 5 minutes
4. Resource limits properly configured
5. Replicas match desired count
6. Recent metrics within normal range (CPU, memory, latency)

If validation fails:
- Identify root cause
- Recommend rollback if critical
- Provide debugging steps

{{role "user"}}
{{userInput}}
```

**Usage**:
```bash
# Post-deployment validation
stn agent run "Deployment Validator" \
  "Validate the api-service deployment in production namespace"

# Canary validation
stn agent run "Deployment Validator" \
  "Check if canary deployment is healthy before promoting to 100%"
```

---

### Performance Regression Detector

Detects performance regressions by comparing metrics across deployments.

**Agent Definition** (`performance-regression-detector.prompt`):
```yaml
---
metadata:
  name: "Performance Regression Detector"
  description: "Detects performance regressions across deployments"
  tags: ["performance", "monitoring", "regression"]
model: gpt-4o
max_steps: 10
tools:
  - "__prometheus_query"
  - "__grafana_get_dashboard"
  - "__datadog_query_metrics"
---

{{role "system"}}
You are a performance engineer analyzing deployment impact.

Compare metrics:
- Request latency (p50, p95, p99)
- Error rates
- Throughput (requests/second)
- Resource utilization (CPU, memory)
- Database query times

Analysis:
1. Baseline: Previous deployment metrics
2. Current: New deployment metrics
3. Statistical significance of changes
4. Regression threshold: >10% degradation

Report:
- Performance changes (improved, degraded, unchanged)
- Regression severity
- Affected endpoints/operations
- Rollback recommendation if severe

{{role "user"}}
{{userInput}}
```

**Usage**:
```bash
# Post-deployment performance check
stn agent run "Performance Regression Detector" \
  "Compare performance metrics before and after deployment v1.2.3"
```

## CI/CD Integration Examples

### GitHub Actions

```yaml
name: Security Scan
on: [push, pull_request]

jobs:
  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install Station
        run: curl -fsSL https://install.station.dev | bash

      - name: Run Security Scanner
        run: |
          stn agent run "Infrastructure Security Scanner" \
            "Scan for security issues and fail if critical findings" \
            --format json > scan-results.json
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}

      - name: Check Results
        run: |
          CRITICAL=$(jq '.findings[] | select(.severity=="CRITICAL") | length' scan-results.json)
          if [ "$CRITICAL" -gt 0 ]; then
            echo "Critical security issues found!"
            exit 1
          fi
```

### GitLab CI

```yaml
security_scan:
  stage: test
  script:
    - curl -fsSL https://install.station.dev | bash
    - stn agent run "Infrastructure Security Scanner" "Scan terraform/ for issues"
  only:
    - merge_requests
```

## Scheduled Agents

### Daily Cost Analysis (Cron)

```bash
# /etc/cron.d/station-cost-analysis
0 8 * * * station stn agent run "AWS Cost Spike Analyzer" "Daily cost analysis" >> /var/log/station-costs.log
```

### Weekly Compliance Check

```bash
# /etc/cron.weekly/station-compliance
#!/bin/bash
stn agent run "Compliance Violation Detector" \
  "Weekly SOC2 compliance scan" \
  --format json | mail -s "Weekly Compliance Report" compliance@company.com
```

## Next Steps

- [Agent Development](./agent-development.md) - Create custom agents
- [MCP Tools](./mcp-tools.md) - Available tools for agents
- [Deployment Modes](./deployment-modes.md) - How to run agents
- [Bundles](./bundles.md) - Package and share agents

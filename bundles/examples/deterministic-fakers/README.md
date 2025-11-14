# Deterministic Faker Configurations

This directory contains example faker MCP configurations with `FAKER_TOOL_NAMES` environment variables to ensure deterministic tool generation across Station installations.

## Purpose

These configurations demonstrate how to use the `FAKER_TOOL_NAMES` feature to make faker tools reproducible for bundle portability.

## Usage

1. Copy desired config files to your environment directory:
   ```bash
   cp bundles/examples/deterministic-fakers/*.json ~/.config/station/environments/your-env/
   ```

2. Run sync to generate tools:
   ```bash
   stn sync your-env
   ```

3. Tools will be generated with exact names specified in `FAKER_TOOL_NAMES`

## Verified Configurations

All configurations in this directory have been tested and verified to generate the correct tool names:

- **datadog-apm.json** (10 tools) - Datadog APM trace analysis
- **elasticsearch-logs.json** (8 tools) - Elasticsearch log analysis  
- **gcp.json** (9 tools) - GCP Cloud Billing and Cost Management
- **grafana-dashboards.json** (10 tools) - Grafana dashboard visualization
- **kubernetes-metrics.json** (9 tools) - Kubernetes deployment rollout analysis
- **pagerduty-incidents.json** (13 tools) - PagerDuty incident correlation
- **prometheus-metrics.json** (9 tools) - Prometheus metrics analysis

## Test Results

All configs tested with:
1. Fresh database (cache cleared)
2. Sync to generate tools
3. Verification that tool names match `FAKER_TOOL_NAMES` exactly

Result: ✅ 100% match on all 7 fakers (68 total tools)

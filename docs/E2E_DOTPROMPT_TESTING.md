# End-to-End Dotprompt Testing Guide

This document provides a comprehensive guide for testing the Station dotprompt architecture from initialization to agent execution and template management.

## Overview

The Station dotprompt architecture enables:
- ✅ Name-based agent addressing (instead of IDs)
- ✅ Portable .prompt file configurations with YAML frontmatter
- ✅ Dual model support (OpenAI GPT-4 + Google Gemini 2.0 Flash Exp)
- ✅ Declarative MCP tool synchronization
- ✅ GitOps-ready agent deployment
- ✅ Custom frontmatter with runtime extraction
- ✅ File-to-database synchronization

## Prerequisites

### Required Dependencies
```bash
# Go 1.21+ for Station development
go version

# Node.js for MCP servers
node --version

# Python 3.8+ for MCP servers
python3 --version

# UV for Python package management
uv --version
```

### API Keys (for production testing)
```bash
export GOOGLE_API_KEY="your-gemini-api-key"
export OPENAI_API_KEY="your-openai-api-key" 
export AWS_ACCESS_KEY_ID="your-aws-key"
export AWS_SECRET_ACCESS_KEY="your-aws-secret"
export GRAFANA_URL="http://localhost:3000"
export GRAFANA_TOKEN="your-grafana-token"
```

## E2E Testing Workflow

### Phase 1: Clean Installation and Setup

#### 1.1 Database Reset and Initialization
```bash
# Remove existing database
rm -f ~/.config/station/station.db

# Initialize Station fresh
./main init

# Verify initialization
ls -la ~/.config/station/
```

Expected output:
```
📁 Config file: ~/.config/station/config.yaml
🗄️  Database: ~/.config/station/station.db
📁 File config structure: ~/.config/station/environments/default
```

#### 1.2 Move MCP Configurations to Default Environment
```bash
# Move AWS and Grafana configs (if not already in place)
mv ~/.config/station/aws.json ~/.config/station/environments/default/
mv ~/.config/station/mcp_grafana_config.json ~/.config/station/environments/default/

# Verify file structure
tree ~/.config/station/environments/default/
```

Expected structure:
```
~/.config/station/environments/default/
├── agents/
├── aws.json
└── mcp_grafana_config.json
```

### Phase 2: MCP Configuration Synchronization

#### 2.1 Run MCP Sync
```bash
# Sync all file-based configs to database
./main mcp sync default

# Verify sync with dry-run
./main mcp sync default --dry-run
```

Expected output:
```
🔄 MCP Configuration Sync
📥 Configs to sync:
  • aws
  • mcp_grafana_config
✅ Sync completed successfully!
```

#### 2.2 Verify Available Tools
```bash
# List all available MCP tools
./main mcp tools --environment default

# Filter specific tools
./main mcp tools --environment default --filter "aws"
./main mcp tools --environment default --filter "grafana"
```

Expected AWS tools: `__call_aws`, `__suggest_aws_commands`
Expected Grafana tools: `__search_dashboards`, `__list_alert_rules`, etc.

### Phase 3: Agent Creation with Dotprompt

#### 3.1 Create AWS S3 Monitoring Agent

**File**: `~/.config/station/environments/default/agents/aws-s3-monitor.prompt`

```yaml
---
model: "gemini-2.0-flash-exp"
config:
  temperature: 0.3
  max_tokens: 2000
metadata:
  name: "aws-s3-monitor"
  description: "AWS S3 bucket monitoring agent"
  version: "1.0.0"
tools:
  - "__call_aws"
  - "__suggest_aws_commands"
station:
  mcp_dependencies:
    awslabs.aws-api-mcp-server:
      assigned_tools: ["__call_aws", "__suggest_aws_commands"]
      server_command: "uvx awslabs.aws-api-mcp-server@latest"
---

You are an AWS S3 monitoring specialist.
Task: {{TASK}}
Environment: {{ENVIRONMENT}}

When asked to "list all my S3 buckets and give me the one with the largest size":
1. List all S3 buckets using AWS CLI
2. Calculate sizes for each bucket
3. Identify the largest bucket
4. Provide detailed analysis
```

#### 3.2 Create Grafana Dashboard Monitor Agent

**File**: `~/.config/station/environments/default/agents/grafana-dashboard-monitor.prompt`

```yaml
---
model: "gpt-4"
config:
  temperature: 0.3
  max_tokens: 2000
metadata:
  name: "grafana-dashboard-monitor"
  description: "Grafana monitoring agent"
  version: "1.0.0"
tools:
  - "__search_dashboards"
  - "__list_alert_rules"
station:
  mcp_dependencies:
    mcp-grafana:
      assigned_tools: ["__search_dashboards", "__list_alert_rules"]
---

You are a Grafana operational monitoring specialist.
Task: {{TASK}}

When asked to "list me all my grafana dashboards and show me the alerts":
1. Search for all dashboards
2. List all alert rules
3. Analyze critical alerts
4. Provide operational insights
```

### Phase 4: Agent Execution Testing

#### 4.1 Test Name-Based Agent Execution (New Architecture)
```bash
# Execute AWS S3 monitoring agent by name
./main agent run aws-s3-monitor "List all my S3 buckets and give me the one with the largest size" --env default

# Execute Grafana dashboard monitor by name  
./main agent run grafana-dashboard-monitor "List me all my grafana dashboards and show me the alerts" --env default

# Test with tail flag for real-time output
./main agent run aws-s3-monitor "Get S3 bucket sizes" --env default --tail
```

Expected output format:
```
🎯 Execution Results
⏱️  Duration: [duration]
✅ Success: true
🤖 Model: gemini-2.0-flash-exp
📊 Steps Used: [count]
🔧 Tools Used: [count]

📄 Response:
[Agent response with tool calls and analysis]
```

#### 4.2 Test Dual Model Support
```bash
# Test Gemini model execution
./main agent run aws-s3-monitor "List S3 buckets" --env default

# Test OpenAI model execution  
./main agent run grafana-dashboard-monitor "Show dashboards" --env default
```

### Phase 5: Agent Synchronization Testing

#### 5.1 Test File-to-Database Sync
```bash
# Modify an agent .prompt file
# Change temperature from 0.3 to 0.5 in aws-s3-monitor.prompt

# Sync changes to database
./main mcp sync default

# Verify changes were applied
./main agent run aws-s3-monitor "test task" --env default
```

#### 5.2 Test Agent Export/Import
```bash
# Export agent to dotprompt format
./main agent export aws-s3-monitor default

# Modify exported file
# Re-import changes
./main mcp sync default
```

### Phase 6: Template Bundle Testing

#### 6.1 Create Agent Bundle
```bash
# Export agent as template bundle
./main agent bundle export aws-s3-monitor /tmp/aws-monitor-bundle

# Validate bundle structure
./main agent bundle validate /tmp/aws-monitor-bundle

# Install bundle in different environment
./main agent bundle install /tmp/aws-monitor-bundle staging --vars "env=staging"
```

#### 6.2 Test Bundle Portability
```bash
# Create new environment
./main env create staging

# Install bundle with different variables
./main agent bundle install /tmp/aws-monitor-bundle staging \
  --vars "aws_region=us-west-2,environment=staging"
```

## Test Scenarios and Expected Outcomes

### Scenario 1: AWS S3 Analysis Task
**Input**: `"List all my S3 buckets and give me the one with the largest size"`

**Expected Agent Behavior**:
1. ✅ Use `__call_aws` tool to execute `aws s3 ls`
2. ✅ For each bucket, call `aws s3api list-objects-v2` or use CloudWatch metrics
3. ✅ Calculate total size per bucket
4. ✅ Sort by size and identify largest
5. ✅ Format results with bucket name, size, object count, cost estimate

**Expected Output**:
```
🪣 S3 Bucket Analysis Results

📊 LARGEST BUCKET:
   Bucket: my-backup-bucket-prod
   Size: 2.4 TB
   Objects: 15,432 objects
   Estimated Cost: $56.88/month

📋 ALL BUCKETS (by size):
   1. my-backup-bucket-prod - 2.4 TB - 15,432 objects
   2. data-lake-raw - 1.8 TB - 8,921 objects
   3. application-logs - 340 GB - 45,123 objects
```

### Scenario 2: Grafana Dashboard Analysis
**Input**: `"List me all my grafana dashboards and show me the alerts"`

**Expected Agent Behavior**:
1. ✅ Use `__search_dashboards` to get all dashboards
2. ✅ Use `__list_alert_rules` to get all alerts with states
3. ✅ Categorize alerts by severity and status
4. ✅ Highlight firing or problematic alerts
5. ✅ Provide operational recommendations

**Expected Output**:
```
📊 GRAFANA OPERATIONAL DASHBOARD

🎛️ DASHBOARDS SUMMARY:
   Total Found: 23
   Critical Dashboards: Infrastructure, Application Performance, Database
   
🚨 ALERT RULES SUMMARY:
   Total Rules: 45
   🔴 Firing: 2 
   🟡 Pending: 1
   🟢 Inactive: 42

🔥 CRITICAL ALERTS:
   • High CPU Usage (Infrastructure Dashboard)
     - State: FIRING
     - Severity: critical
```

## Validation Checklist

### ✅ Core Functionality
- [ ] Database initialization successful
- [ ] MCP sync loads configurations correctly
- [ ] Agent .prompt files parse without errors
- [ ] Name-based agent execution works
- [ ] MCP tool integration functions properly

### ✅ Dual Model Support
- [ ] Gemini 2.0 Flash Exp model executes successfully
- [ ] OpenAI GPT-4 model executes successfully
- [ ] Model switching works based on .prompt file config
- [ ] Different temperature/token settings respected

### ✅ dotprompt Features
- [ ] YAML frontmatter parsing works
- [ ] Custom station.* fields extracted correctly
- [ ] Template variable substitution functions
- [ ] MCP dependency mapping displays properly
- [ ] Execution metadata shown in results

### ✅ File Synchronization
- [ ] Changes to .prompt files sync to database
- [ ] Checksums detect file modifications
- [ ] Force sync overrides unchanged files
- [ ] Dry-run mode shows planned changes

### ✅ Template Bundles
- [ ] Agent export creates valid bundles
- [ ] Bundle validation catches errors
- [ ] Bundle installation works across environments
- [ ] Variable substitution in bundles functions

## Troubleshooting Common Issues

### Issue: "Model not found" errors
**Solution**: Ensure API keys are set and models are correctly specified in .prompt files

### Issue: MCP tools not available
**Solution**: Verify MCP server configurations in JSON files and check tool sync

### Issue: Agent name resolution fails
**Solution**: Ensure .prompt file metadata.name matches filename

### Issue: File sync doesn't detect changes
**Solution**: Use `--force` flag or check file permissions

### Issue: Template variables not substituting
**Solution**: Verify variable names match between template and context

## Performance Benchmarks

### Expected Response Times
- **Agent parsing**: < 100ms
- **MCP tool execution**: 1-5 seconds (depending on external API)
- **File synchronization**: < 500ms for typical environments
- **End-to-end agent execution**: 2-10 seconds

### Resource Usage
- **Memory**: ~50MB for agent execution
- **CPU**: Low impact during parsing, higher during LLM inference
- **Disk**: ~1MB per agent .prompt file

## Documentation and Examples

### Template Gallery
Create additional agent templates for common use cases:

- **Database Monitoring**: PostgreSQL/MySQL performance analysis
- **Infrastructure Monitoring**: Server health and resource usage  
- **Security Analysis**: Log analysis and threat detection
- **Cost Optimization**: Cloud resource cost analysis
- **Performance Testing**: Application performance monitoring

### Integration Examples
- **CI/CD Integration**: Automated agent deployment via GitOps
- **Monitoring Integration**: Agent execution via cron jobs
- **Dashboard Integration**: Grafana dashboards showing agent results
- **Alert Integration**: Webhook notifications for agent failures

## Conclusion

This E2E testing guide ensures the complete dotprompt architecture functions correctly from initialization through production usage. The name-based agent system with dual model support and file synchronization provides a robust foundation for GitOps-ready agent deployment.

Key innovations tested:
1. ✅ **Name-based addressing** eliminates ID dependency
2. ✅ **Dual model support** enables best-of-breed AI usage
3. ✅ **File synchronization** enables GitOps workflows
4. ✅ **Custom frontmatter** supports enterprise metadata
5. ✅ **Template bundles** enable agent portability

The architecture successfully bridges the gap between development and production AI agent deployment while maintaining full backward compatibility with existing Station functionality.
# Building and Deploying Station CI/CD Environments

This guide shows how to build Station environments as Docker containers and deploy them in CI/CD pipelines for automated security scanning, infrastructure auditing, and other DevOps tasks.

## Overview

Station environments can be packaged as self-contained Docker containers that include:
- **AI agents** with specific expertise (security, terraform, etc.)
- **MCP tools** for interacting with systems (Ship CLI security tools, filesystem operations)  
- **Runtime dependencies** (Node.js, Ship CLI, Docker CLI for tool execution)

## Building an Environment Container

### 1. Create Your Environment

First, create a Station environment with agents and tool configurations:

```bash
# Create environment directory structure
mkdir -p ~/.config/station/environments/devops-simple/{agents}

# Create template.json with MCP server configurations
cat > ~/.config/station/environments/devops-simple/template.json << 'EOF'
{
  "name": "devops-simple",
  "description": "Simple DevOps bundle with 2 focused agents - Security Scanner and Terraform Auditor",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest", 
        "{{ .PROJECT_ROOT }}"
      ]
    },
    "security-tools": {
      "command": "ship",
      "args": [
        "mcp",
        "gitleaks",
        "--stdio"
      ]
    },
    "terraform-tools": {
      "command": "ship",
      "args": [
        "mcp",
        "checkov",
        "--stdio"
      ]
    }
  }
}
EOF

# Create variables.yml for template variable resolution
cat > ~/.config/station/environments/devops-simple/variables.yml << 'EOF'
PROJECT_ROOT: /workspace
EOF
```

### 2. Create Agent Configurations

Create `.prompt` files for your agents with proper tool assignments:

```bash
# Security Scanner Agent
cat > ~/.config/station/environments/devops-simple/agents/Security\ Scanner.prompt << 'EOF'
---
name: Security Scanner
description: Focused security scanning agent - detects secrets and basic security issues in repositories
model: gpt-5
temperature: 0.1
max_steps: 5
tools:
  - __list_directory
  - __read_file
  - __directory_tree
  - __gitleaks_dir
  - __gitleaks_git
  - __search_files
---

You are a focused security scanning agent with the following MCP tools available:

## Available Tools:
1. **Filesystem Tools**: `__list_directory`, `__read_file`, `__directory_tree` - explore repository structure
2. **Gitleaks Tools**: `__gitleaks_dir` - scan directory for secrets, `__gitleaks_git` - scan git history
3. **File Search**: `__search_files` - find specific file patterns

## Your Mission:
Perform targeted security scanning using your available tools:

1. **Explore Repository**: Use `__list_directory` to see all files in the current directory
2. **Secret Detection**: Use `__gitleaks_dir` with the current directory path to scan for secrets
3. **File Analysis**: Use `__read_file` to examine .env, config files, and scripts
4. **Pattern Search**: Use `__search_files` to find sensitive file patterns

## Process:
1. FIRST: Use `__list_directory` with "." to see what files exist
2. THEN: Use `__gitleaks_dir` with "." to scan the directory for secrets  
3. FINALLY: Use `__read_file` to examine any suspicious files found
4. Report all findings with file paths and line numbers

## Report Format:
```markdown
# Security Scan Results

## 🚨 Critical Issues Found: X
[List critical security issues with file:line references]

## ⚠️ Medium Issues Found: Y  
[List medium priority security issues]

## ✅ Security Checks Passed: Z
[List successful security validations]

## 🛠️ Remediation Steps
1. [Specific action items with commands/code examples]
```

Focus on actionable findings that developers can fix immediately. Keep scans fast and targeted.
EOF

# Terraform Auditor Agent  
cat > ~/.config/station/environments/devops-simple/agents/Terraform\ Auditor.prompt << 'EOF'
---
name: Terraform Auditor
description: Infrastructure security auditor - analyzes Terraform configurations for security misconfigurations and best practices
model: gpt-5
temperature: 0.1
max_steps: 5
tools:
  - __list_directory
  - __read_file
  - __directory_tree
  - __search_files
  - __checkov_scan_directory
  - __checkov_scan_file
---

You are a Terraform security specialist with access to checkov for infrastructure as code security analysis and filesystem access for configuration review.

## Your Mission:
Analyze Terraform configurations for security and compliance:

1. **IaC Security Scanning**: Use checkov to scan Terraform files for security misconfigurations
2. **Configuration Review**: Examine terraform files for best practices violations  
3. **Compliance Assessment**: Check against common security frameworks (CIS, AWS Security)

## Process:
1. Discover Terraform files in the repository (.tf, .tfvars)
2. Run checkov security scans on discovered configurations
3. Analyze results and provide prioritized remediation guidance

## Report Format:
```markdown  
# Terraform Security Audit

## 📁 Files Analyzed: X
[List of terraform files scanned]

## 🚨 Critical Security Issues: Y
[High-severity misconfigurations that create security risks]

## ⚠️ Security Recommendations: Z
[Best practices improvements and compliance issues]

## ✅ Compliant Configurations: A
[Security checks that passed successfully]

## 🛠️ Remediation Priority
1. **High Priority**: [Critical security fixes]
2. **Medium Priority**: [Best practice improvements]
```

Focus on practical security improvements for infrastructure configurations. Provide specific code examples for fixes.
EOF
```

### 3. Build the Container

Use Station's build command to create a Docker container:

```bash
# Build environment container with Ship CLI security tools
stn build env devops-simple --provider openai --model gpt-5 --ship
```

This creates:
- ✅ **Docker container**: `station-devops-simple:latest`
- ✅ **Workspace directory**: `/workspace` for mounting repositories
- ✅ **All dependencies**: Station binary, Ship CLI, Docker CLI, Node.js
- ✅ **Agent configurations**: 2 agents with proper tool assignments
- ✅ **Baseline tool discovery**: Filesystem tools discovered during build

> **Important**: The container needs Docker socket access at runtime to discover and use Ship security tools (gitleaks, checkov, etc.).

## Deploying in CI/CD

### Runtime Workflow Pattern

Since containers are stateless, you need to run two commands in sequence:

1. **`stn sync`** - Discovers all available tools (requires Docker socket)
2. **`stn agent run`** - Executes agents with full tool access

### GitHub Actions Example

```yaml
name: Station Security Audit

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  security-audit:
    runs-on: ubuntu-latest
    
    steps:
    - name: Checkout code
      uses: actions/checkout@v4
      
    - name: Run Station Security Analysis  
      env:
        OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
      run: |
        echo "🚀 Starting Station DevOps Security Analysis"
        
        # Pull the Station container (or use your registry)
        docker pull station-devops-simple:latest
        
        # Run sync to discover all tools with Docker available
        echo "=== SYNC: Discovering Security Tools ==="
        docker run --rm \\
          -v /var/run/docker.sock:/var/run/docker.sock \\
          -v ${{ github.workspace }}:/workspace \\
          -e OPENAI_API_KEY="${OPENAI_API_KEY}" \\
          station-devops-simple:latest \\
          stn sync default -i=false
        
        echo "=== SECURITY SCANNER ==="
        # Run Security Scanner agent
        docker run --rm \\
          -v /var/run/docker.sock:/var/run/docker.sock \\
          -v ${{ github.workspace }}:/workspace \\
          -e OPENAI_API_KEY="${OPENAI_API_KEY}" \\
          station-devops-simple:latest \\
          stn agent run "Security Scanner" \\
          "Scan this repository for security issues, secrets, and vulnerabilities. Focus on .env files, configuration files, and any exposed credentials." \\
          --env default
          
        echo "=== TERRAFORM AUDITOR ==="
        # Run Terraform Auditor agent  
        docker run --rm \\
          -v /var/run/docker.sock:/var/run/docker.sock \\
          -v ${{ github.workspace }}:/workspace \\
          -e OPENAI_API_KEY="${OPENAI_API_KEY}" \\
          station-devops-simple:latest \\
          stn agent run "Terraform Auditor" \\
          "Analyze all Terraform files in this repository for security misconfigurations and compliance issues." \\
          --env default
```

### Alternative: Combined Command Pattern

For simpler workflows, combine both operations:

```bash
docker run --rm \\
  -v /var/run/docker.sock:/var/run/docker.sock \\
  -v $(pwd):/workspace \\
  -e OPENAI_API_KEY="${OPENAI_API_KEY}" \\
  station-devops-simple:latest \\
  bash -c "stn sync default -i=false && stn agent run 'Security Scanner' 'Scan for security issues' --env default"
```

## Container Registry Distribution

### Tagging and Pushing

```bash
# Tag for your registry
docker tag station-devops-simple:latest your-registry.com/station-devops-simple:latest

# Push to registry
docker push your-registry.com/station-devops-simple:latest
```

### Using in CI/CD

```yaml
- name: Run Station Analysis
  run: |
    docker pull your-registry.com/station-devops-simple:latest
    docker run --rm \\
      -v /var/run/docker.sock:/var/run/docker.sock \\
      -v ${{ github.workspace }}:/workspace \\
      -e OPENAI_API_KEY="${OPENAI_API_KEY}" \\
      your-registry.com/station-devops-simple:latest \\
      bash -c "stn sync default -i=false && stn agent run 'Security Scanner' 'Security audit' --env default"
```

## Testing Locally with Act

Test your GitHub Actions workflows locally using [nektos/act](https://github.com/nektos/act):

```bash
# Install act
curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash

# Run workflow locally
act workflow_dispatch -W .github/workflows/station-security-test.yml \\
  -s OPENAI_API_KEY=your_key_here \\
  --container-architecture linux/amd64
```

## Container Architecture Details

### What's Inside the Container

- **Base**: Ubuntu 22.04 with Node.js 20
- **Station Binary**: Latest version with embedded UI
- **Dependencies**: 
  - Ship CLI (307+ security tools via Dagger containers)
  - Docker CLI (static binary for running Ship tools)
  - SQLite3 for local agent database
- **Working Directory**: `/workspace` (mount your repo here)
- **Configuration**: Complete environment in `/root/.config/station/`

### Tool Discovery Process

1. **Build Time**: Filesystem tools discovered and saved to database
2. **Runtime**: When Docker socket is available:
   - Ship security tools discovered (gitleaks, checkov, etc.)  
   - All tools (28+) assigned to agents
   - Agents execute with full capabilities

### Required Environment Variables

- **`OPENAI_API_KEY`** (or your AI provider's key)
- **Docker Socket**: Mounted via `-v /var/run/docker.sock:/var/run/docker.sock`
- **Repository**: Mounted via `-v $(pwd):/workspace`

## Troubleshooting

### Tools Not Found

If agents report missing tools, ensure:
1. Docker socket is mounted for Ship tools
2. `stn sync` runs before agent execution  
3. API key is provided for tool discovery

### Agent Timeouts

Security scanning can take time. Use timeouts appropriately:
```bash
timeout 120s stn agent run "Security Scanner" "task" --env default
```

### Container Size

The container is ~2GB due to security tools. For smaller deployments:
- Build without `--ship` flag (filesystem tools only)
- Use multi-stage builds to reduce size
- Consider tool-specific containers

## Advanced Patterns

### Multi-Agent Workflows

Run multiple agents in sequence for comprehensive analysis:

```bash
#!/bin/bash
AGENTS=("Security Scanner" "Terraform Auditor")
TASKS=("Security audit" "Infrastructure compliance audit")

# Sync once
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \\
  -v $(pwd):/workspace -e OPENAI_API_KEY="$API_KEY" \\
  station-devops-simple:latest stn sync default -i=false

# Run each agent
for i in "${!AGENTS[@]}"; do
  echo "Running: ${AGENTS[i]}"
  docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \\
    -v $(pwd):/workspace -e OPENAI_API_KEY="$API_KEY" \\
    station-devops-simple:latest \\
    stn agent run "${AGENTS[i]}" "${TASKS[i]}" --env default
done
```

### Results Extraction

Capture agent outputs for reporting:

```bash
# Run with output capture
OUTPUT=$(docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \\
  -v $(pwd):/workspace -e OPENAI_API_KEY="$API_KEY" \\
  station-devops-simple:latest \\
  stn agent run "Security Scanner" "Security audit" --env default 2>&1)

# Process results
echo "$OUTPUT" | grep "🚨\\|⚠️\\|✅" > security-report.txt
```

This guide provides a complete foundation for building, deploying, and operating Station environments in CI/CD pipelines. The containerized approach ensures consistent, reproducible security and infrastructure analysis across all your repositories.
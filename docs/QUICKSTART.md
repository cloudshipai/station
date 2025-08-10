# Station Quickstart Guide

Get Station running in under 5 minutes with intelligent AI agents managing your infrastructure.

## Prerequisites

- Linux, macOS, or Windows
- 256MB RAM, 100MB disk space  
- Node.js (for MCP servers)
- Go 1.21+ (for building from source)

## Installation

### Option 1: Build from Source (Current)
```bash
git clone https://github.com/cloudshipai/station
cd station
go build -o stn ./cmd/main
```

### Option 2: Binary Release (Coming Soon)
```bash
curl -sSL https://getstation.cloudshipai.com/install | bash
```

## Quick Setup (2 minutes)

### 1. Initialize Station
```bash
# Initialize with encryption keys and database
./stn init

# Verify installation
./stn --version
./stn config show
```

### 2. Create Environments
```bash
# Create development environment
./stn env create development "Development environment"

# Create staging environment  
./stn env create staging "Staging environment"

# List environments
./stn env list
```

### 3. Load MCP Server Templates
```bash
# Load filesystem tools (most common)
./stn load examples/mcps/filesystem.json

# Load GitHub integration
./stn load examples/mcps/github.json

# Load database tools
./stn load examples/mcps/postgresql.json

# See all 22+ available templates
ls examples/mcps/
```

### 4. Create Your First Agent
```bash
# Create an intelligent filesystem agent
./stn agent create \
  --name "File System Explorer" \
  --description "Analyze and manage file systems with intelligent recommendations" \
  --domain "devops" \
  --environment "development"

# Create a GitHub management agent
./stn agent create \
  --name "GitHub Repository Manager" \
  --description "Manage repositories, review PRs, and handle GitHub workflows" \
  --domain "software-engineering" \
  --environment "development"
```

### 5. Run Agents
```bash
# List your agents
./stn agent list

# Run filesystem analysis
./stn agent run 1 "Analyze the project directory structure and identify any issues"

# Run GitHub operations  
./stn agent run 2 "List repositories and check for any pending pull requests"

# View execution history
./stn runs list
./stn runs show 1
```

## Common Use Cases

### Development Workflow
```bash
# Load development tools
./stn load examples/mcps/filesystem.json
./stn load examples/mcps/git-advanced.json
./stn load examples/mcps/github.json

# Create development agent
./stn agent create \
  --name "Development Assistant" \
  --description "Help with code analysis, git operations, and GitHub management" \
  --domain "software-engineering"

# Use the agent
./stn agent run 1 "Check git status, analyze recent commits, and suggest next steps"
```

### Infrastructure Management
```bash
# Load infrastructure tools
./stn load examples/mcps/aws-cli.json
./stn load examples/mcps/kubernetes.json  
./stn load examples/mcps/monitoring-prometheus.json

# Create infrastructure agent
./stn agent create \
  --name "Infrastructure Monitor" \
  --description "Monitor AWS resources, Kubernetes clusters, and system metrics" \
  --domain "devops" \
  --schedule "0 */6 * * * *"  # Every 6 hours

# Run infrastructure check
./stn agent run 2 "Check AWS resource utilization and Kubernetes cluster health"
```

### Database Operations
```bash
# Load database tools
./stn load examples/mcps/postgresql.json
./stn load examples/mcps/mysql.json

# Create database agent
./stn agent create \
  --name "Database Administrator" \
  --description "Monitor database performance, run queries, and manage schemas" \
  --domain "data-engineering"

# Use database agent
./stn agent run 3 "Check database connection status and identify any performance issues"
```

## Advanced Configuration

### Team Setup
```bash
# Start Station server for team access
./stn server &

# Create team members
./stn user create alice --role admin
./stn user create bob --role developer

# Team members can connect remotely
export STATION_API_KEY=xxx
./stn agent list --endpoint https://station.company.com
```

### Webhook Notifications
```bash
# Set up Slack notifications
./stn webhook create \
  --name "Slack Alerts" \
  --url "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK" \
  --events "agent_run_completed,agent_run_failed"

# View webhook history
./stn webhook deliveries
```

### Environment-Specific Configuration
```bash
# Load different configs per environment
./stn load examples/mcps/postgresql.json --env development
./stn load examples/mcps/postgresql.json --env staging
./stn load examples/mcps/postgresql.json --env production

# Create environment-specific agents
./stn agent create "Dev DB Monitor" "Monitor development database" --env development
./stn agent create "Prod DB Monitor" "Monitor production database" --env production
```

## Troubleshooting

### Common Issues

**"No tools available for execution"**
```bash
# Check loaded MCP servers
./stn mcp list

# Load basic filesystem tools
./stn load examples/mcps/filesystem.json

# Verify tools are available
./stn mcp tools
```

**"Agent creation failed"**
```bash
# Check Station's stdio MCP server
./stn mcp test station-stdio

# Verify AI provider configuration
./stn config show | grep -i ai

# Set AI provider if needed
export STN_AI_PROVIDER=openai
export STN_AI_API_KEY=your-key-here
```

**"Database connection error"**
```bash
# Check database status
./stn config show | grep -i database

# Reinitialize if needed
./stn init --force
```

### Getting Help

```bash
# General help
./stn --help

# Command-specific help
./stn agent --help
./stn load --help

# Check system status
./stn status
```

## Next Steps

- [📚 Read the Architecture Guide](ARCHITECTURE.md)
- [🔒 Review Security Best Practices](SECURITY.md) 
- [🏗️ Set Up Production Deployment](../PRODUCTION_READINESS.md)
- [🧪 Run Comprehensive Tests](../TESTING_SCENARIOS.md)
- [📖 Browse All MCP Templates](../examples/mcps/README.md)

## Need Help?

- **Documentation**: Check the [docs/](.) folder
- **Issues**: Open an issue on GitHub
- **Community**: Join our Discord community
- **Enterprise**: Contact us for enterprise support

---

**You're ready to use Station!** 🎉

Start with simple file system operations, then expand to more complex infrastructure management as you get comfortable with the platform.
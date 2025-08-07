# Station Architecture

Station is designed as a self-bootstrapping AI agent runtime that manages itself through its own MCP (Model Context Protocol) interface.

## Core Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Claude    │────▶│   Station    │────▶│   Your Tools    │
│ (MCP Client)│ MCP │  (Runtime)   │     │ FS, GH, AWS     │
└─────────────┘     └──────────────┘     └─────────────────┘
                           │
                    ┌──────▼───────┐     ┌─────────────────┐
                    │Self-Bootstrap│────▶│ Station's Own   │
                    │Intelligence  │stdio│   MCP Server    │
                    │   (Genkit)   │     │  (13 tools)     │ 
                    └──────────────┘     └─────────────────┘
```

## Key Components

### 1. Runtime Engine
The core Station binary provides:
- **Agent Execution Queue**: Manages concurrent agent runs with worker pools
- **MCP Server Management**: Handles connections to external MCP servers
- **Environment Isolation**: Separates configurations across dev/staging/prod
- **Security Layer**: Encrypts secrets and manages permissions

### 2. Self-Bootstrapping Intelligence
Station uses Google's Genkit framework with its own MCP server:
- **AI-Powered Tool Selection**: Analyzes requirements and assigns optimal tools
- **Dynamic Execution Planning**: Adjusts iteration limits (1-25) based on task complexity
- **Self-Management**: Station creates and manages agents using its own MCP interface
- **Multi-Provider Support**: OpenAI, Anthropic, Google, Ollama with smart fallbacks

### 3. MCP Integration Layer
Station acts as both MCP client and server:
- **MCP Client**: Connects to external servers (filesystem, GitHub, AWS, etc.)
- **MCP Server**: Provides 13 management tools via stdio interface
- **Tool Discovery**: Auto-detects available tools and capabilities
- **Protocol Translation**: Handles different MCP server implementations

## Self-Bootstrapping Flow

### Agent Creation Process
1. **User Request**: `./stn agent create "name" "description" --domain "devops"`
2. **MCP Connection**: Station connects to its own stdio MCP server
3. **AI Analysis**: Genkit analyzes requirements using available MCP tools
4. **Tool Selection**: AI selects optimal tools from agent's environment
5. **Agent Creation**: Station creates agent with intelligent configuration
6. **Tool Assignment**: Assigns specific tools based on domain and requirements

### Agent Execution Process
1. **Execution Request**: `./stn agent run 1 "task description"`
2. **Environment Setup**: Load agent's assigned tools from environment MCP servers
3. **Tool Filtering**: Only provide tools assigned to the specific agent
4. **AI Execution**: Genkit executes with filtered tools and dynamic iteration limits
5. **Result Processing**: Parse steps, tool calls, and execution metadata
6. **Storage**: Store run results with detailed execution information

## Environment Architecture

### Multi-Environment Isolation
```
~/.config/station/
├── config.yaml              # Global configuration
├── station.db              # Main database
└── environments/            # Environment-specific configs
    ├── default/
    │   ├── variables.yml    # Environment variables
    │   ├── agents/          # Exported agent configs
    │   └── mcp-servers/     # MCP server configurations
    ├── staging/
    │   ├── variables.yml
    │   ├── agents/
    │   └── mcp-servers/
    └── production/
        ├── variables.yml
        ├── agents/
        └── mcp-servers/
```

### GitOps-Ready Configuration
- **File-Based Storage**: All configurations stored as files
- **Template Variables**: Support for environment-specific values
- **Version Control**: Ready for Git-based configuration management
- **Import/Export**: Agents can be exported to files and imported across environments

## Database Schema

### Core Tables
- **agents**: Agent configurations and metadata
- **agent_tools**: Tool assignments per agent
- **agent_runs**: Execution history and results
- **environments**: Environment definitions
- **mcp_servers**: MCP server configurations
- **mcp_tools**: Available tools per server
- **users**: User management and API keys
- **webhooks**: Notification webhooks

### Relationships
```sql
agents
├── belongs_to: environment
├── has_many: agent_tools
├── has_many: agent_runs
└── belongs_to: user

agent_tools
├── belongs_to: agent
└── belongs_to: mcp_tool

mcp_tools
└── belongs_to: mcp_server

mcp_servers
└── belongs_to: environment
```

## Security Architecture

### Encryption at Rest
- **Database Encryption**: SQLite encrypted with AES-256
- **Secrets Management**: Environment variables and secrets encrypted
- **Key Management**: 32-byte encryption keys with secure generation

### Network Security
- **Local-First**: Runs entirely within your infrastructure
- **No External Dependencies**: All AI requests go through your chosen provider
- **Credential Isolation**: Secrets never leave your environment
- **TLS Support**: HTTPS for web interface and API endpoints

### Permission Model
```yaml
User Permissions:
  - admin: Full access to all environments and agents
  - user: Access to assigned environments only
  - viewer: Read-only access

Environment Isolation:
  - Separate MCP server configurations
  - Isolated tool assignments
  - Environment-specific variables

Tool Permissions:
  - Agent-specific tool assignments
  - Fine-grained tool filtering
  - Server-level access control
```

## Execution Architecture

### Agent Execution Queue
```go
type ExecutionQueue struct {
    workers     int           // Concurrent worker count
    buffer      int           // Queue buffer size
    timeout     time.Duration // Execution timeout
    ctx         context.Context
    workChan    chan AgentWork
    resultChan  chan AgentResult
}
```

### Worker Pool Management
- **Concurrent Execution**: Multiple agents can run simultaneously
- **Resource Management**: Worker pools prevent resource exhaustion
- **Timeout Handling**: Configurable timeouts prevent hung executions
- **Graceful Shutdown**: Clean termination of running agents

### MCP Connection Pooling
- **Connection Reuse**: MCP connections pooled for efficiency
- **Health Monitoring**: Automatic connection health checks
- **Failover**: Automatic retry and reconnection logic
- **Resource Cleanup**: Proper connection lifecycle management

## Deployment Architectures

### Single-Node Deployment
```yaml
# Recommended for development and small teams
Resources:
  - CPU: 2-4 cores
  - Memory: 4-8GB RAM
  - Storage: 20-100GB SSD
  - Network: Standard connectivity

Components:
  - Station binary
  - SQLite database
  - MCP servers (Node.js processes)
```

### High-Availability Deployment
```yaml
# Recommended for production environments
Components:
  - Multiple Station instances (load balanced)
  - PostgreSQL database (with replication)
  - Redis cache (for session management)
  - Shared file storage (for configuration)

Load Balancer:
  - API requests: Round-robin
  - Agent execution: Sticky sessions
  - Health checks: /health endpoint
```

### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: station
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
  template:
    spec:
      containers:
      - name: station
        image: station:latest
        resources:
          requests:
            memory: "2Gi"
            cpu: "1000m"
          limits:
            memory: "4Gi"
            cpu: "2000m"
```

## Performance Characteristics

### Benchmarks
| Operation | Latency | Throughput |
|-----------|---------|------------|
| Agent Creation | 6.5s avg | 10/min |
| Tool Discovery | 1.5s avg | 40/min |
| MCP Loading | 3.2s avg | 20/min |
| Agent Execution | 10.8s avg | 6/min |

### Scalability
- **Agents**: 1000+ agents per instance
- **Concurrent Executions**: 10-50 simultaneous (configurable)
- **MCP Servers**: 100+ servers per environment
- **Environments**: Unlimited environments

### Resource Usage
- **Memory**: 45MB baseline, 67MB peak during execution
- **CPU**: Low idle usage, burst during AI inference
- **Storage**: 20MB binary + data + logs
- **Network**: Only during AI API calls and MCP operations

## Monitoring and Observability

### Health Endpoints
- **`/health`**: Basic health check
- **`/ready`**: Readiness check (database connectivity)
- **`/metrics`**: Prometheus metrics

### Key Metrics
```
# System metrics
station_agents_total
station_agent_runs_total
station_mcp_servers_total
station_execution_queue_size

# Performance metrics  
station_agent_execution_duration_seconds
station_mcp_request_duration_seconds
station_database_query_duration_seconds

# Error metrics
station_agent_execution_errors_total
station_mcp_connection_errors_total
station_database_errors_total
```

### Logging
- **Structured Logging**: JSON format for machine parsing
- **Log Levels**: DEBUG, INFO, WARN, ERROR
- **Request Tracing**: Correlation IDs for request tracking
- **Audit Logging**: Security-relevant events

## GitOps Deployment Architecture

### SQLite State Persistence with Litestream

**Challenge**: GitOps deployments use ephemeral containers, but Station needs persistent database state across deployments.

**Solution**: Litestream integration provides automatic SQLite replication and restoration:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Git Repo      │───▶│  Station Pod    │───▶│  S3/GCS/Azure   │
│ Agent Templates │    │  + Litestream   │    │  DB Backups     │
│ Configurations  │    │                 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │ Ephemeral SQLite │
                       │ Auto-Restored   │
                       └─────────────────┘
```

### State Persistence Flow

1. **Container Start**: Litestream restores database from cloud replica
2. **Runtime**: Continuous 10-second replication to cloud storage
3. **Deployment**: New container automatically restores latest state
4. **Recovery**: Point-in-time restoration from backup history

### Production Deployment Options

#### Kubernetes with Litestream
```yaml
# Single replica deployment with automatic state restoration
replicas: 1
strategy:
  type: Recreate  # Ensure clean database transitions
  
containers:
- name: station
  image: station:production
  env:
  - name: LITESTREAM_S3_BUCKET
    value: "station-production-backups"
```

#### Docker Compose with Litestream
```yaml
# Production deployment with GitOps configuration mounts
volumes:
  - ./agent-templates:/app/agent-templates:ro
  - ./environments:/app/environments:ro
  - station-data:/data  # Ephemeral - persisted via Litestream
```

### GitOps Workflow Integration

- **Infrastructure as Code**: All configurations in version control
- **Automated Deployments**: CI/CD pipelines with agent template validation
- **Environment Promotion**: Dev → Staging → Production with encrypted secrets
- **Audit Trail**: Full deployment history with database backup verification

### Backup and Recovery

- **Automatic Backups**: Continuous replication every 10 seconds
- **Retention Policy**: 24-hour retention with configurable cleanup
- **Point-in-Time Recovery**: Restore to any backup timestamp
- **Multi-Cloud Support**: S3, Google Cloud Storage, Azure Blob Storage

## Future Architecture Considerations

### Planned Enhancements
- **Multi-Region Deployments**: Cross-region Litestream replication
- **Advanced Scheduling**: Cron-based and event-driven triggers  
- **Plugin System**: Custom MCP server plugins
- **Federation**: Multi-Station orchestration with shared state

### Integration Points
- **CI/CD Pipelines**: GitHub Actions, GitLab CI, Jenkins with agent deployment
- **Monitoring Systems**: Prometheus, Grafana, DataDog with Litestream metrics
- **Alert Systems**: PagerDuty, OpsGenie, Slack for deployment notifications
- **Identity Systems**: LDAP, SAML, OAuth for enterprise authentication
- **Secret Management**: HashiCorp Vault, AWS Secrets Manager, Azure Key Vault

This architecture enables Station to be both simple to deploy and powerful enough for enterprise GitOps environments while maintaining security, reliability, and zero-downtime deployments with full state persistence.
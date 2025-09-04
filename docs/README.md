# Station Documentation

Station is a secure, self-hosted platform for building and deploying intelligent multi-environment MCP agents that integrate with AI assistants like Claude.

## 📖 Core Documentation

### Getting Started
- [Installation & Setup](./INSTALLATION.md) - Install Station and get started
- [Quick Start Guide](./QUICKSTART.md) - Get up and running in minutes
- [Architecture Overview](./ARCHITECTURE.md) - Understanding how Station works

### Agent Development
- [Creating Agents](./agents/CREATING_AGENTS.md) - Build your first agent
- [Agent Configuration](./agents/AGENT_CONFIG.md) - Configure agents with JSON Schema
- [Environment Management](./agents/ENVIRONMENTS.md) - Multi-environment agent isolation
- [MCP Integration](./agents/MCP_INTEGRATION.md) - Using MCP tools in agents

### Bundle Management
- [Bundle System](./bundles/BUNDLE_SYSTEM.md) - Understanding Station bundles
- [Creating Bundles](./bundles/CREATING_BUNDLES.md) - Package agents for distribution
- [Bundle Registry](./bundles/BUNDLE_REGISTRY.md) - Publishing and sharing bundles

### Development & Operations
- [CLI Reference](./cli/CLI_REFERENCE.md) - Complete Station CLI commands
- [API Reference](./api/API_REFERENCE.md) - REST API documentation
- [Database Schema](./database/SCHEMA.md) - Database structure and migrations
- [Security Model](./SECURITY.md) - Station's security architecture

## 🏗️ Architecture Documentation

### System Architecture
- [Layered Architecture](./LAYERED_ARCHITECTURE_SUMMARY.md) - New execution visibility design
- [Agent Execution Flow](./execution/AGENT_EXECUTION_FLOW.md) - How agents run
- [MCP Server Pool](./mcp/MCP_SERVER_POOL.md) - MCP connection management
- [File Configuration System](./config/FILE_BASED_CONFIG.md) - GitOps configuration

### Advanced Topics
- [Performance Monitoring](./monitoring/PERFORMANCE.md) - Tracking agent performance
- [Troubleshooting](./TROUBLESHOOTING.md) - Common issues and solutions
- [Deployment Patterns](./deployment/DEPLOYMENT_PATTERNS.md) - Production deployment

## 🤝 Contributing

- [Contributing Guide](./CONTRIBUTING.md) - How to contribute to Station
- [Development Setup](./development/DEVELOPMENT_SETUP.md) - Setting up dev environment
- [Testing Guide](./development/TESTING.md) - Running and writing tests

## 🔄 Migration Guides

- [V1.0 Migration](./migration/V1_MIGRATION.md) - Upgrading to Station v1.0
- [Breaking Changes](./BREAKING_CHANGES.md) - Version compatibility notes

---

## Documentation Organization

This documentation follows a structured approach:

- **User-focused**: Getting started, tutorials, and guides
- **Reference**: Complete API, CLI, and configuration references  
- **Architecture**: Deep-dive technical documentation for developers
- **Operations**: Deployment, monitoring, and troubleshooting

Each section includes both conceptual explanations and practical examples to help users at all levels work effectively with Station.
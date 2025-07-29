# Station 🚂

**Station** is a secure, self-hosted platform for managing AI agents and Model Context Protocol (MCP) servers. It provides a unified interface for deploying, configuring, and executing AI agents across multiple environments with enterprise-grade security and encryption.

## 🎯 Features

- **🤖 AI Agent Management**: Create, deploy, and execute AI agents with customizable prompts and tool access
- **🔒 MCP Server Integration**: Secure management of Model Context Protocol servers with encrypted configuration storage
- **🛡️ Enterprise Security**: API key authentication, encrypted data storage, and secure agent execution
- **🌍 Multi-Environment Support**: Isolated environments for different projects, teams, or use cases  
- **📡 SSH Admin Interface**: Terminal-based administration with intuitive menus and real-time monitoring
- **🔧 Multi-LLM Support**: Compatible with OpenAI, Anthropic, and other leading AI model providers
- **📊 Execution Tracking**: Detailed logging and monitoring of agent runs and tool usage

## 🚀 Quick Start

### For Users (MCP Client)

Station acts as an MCP server that you can connect to from any MCP-compatible client:

1. **Get your API key** from your Station administrator
2. **Configure your MCP client** to connect to Station:

```json
{
  "mcpServers": {
    "station": {
      "command": "curl",
      "args": [
        "-X", "POST",
        "-H", "Authorization: Bearer YOUR_API_KEY_HERE",
        "-H", "Content-Type: application/json",
        "http://your-station-host:3000/mcp"
      ]
    }
  }
}
```

3. **Available MCP Tools**:
   - `create_agent`: Create new AI agents with custom configurations
   - `call_agent`: Execute agents with specific tasks
   - `list_mcp_configs`: View available MCP server configurations
   - `discover_tools`: Find tools from configured MCP servers
   - `call_mcp_tool`: Execute tools from external MCP servers

### For Administrators

#### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/station.git
cd station

# Install dependencies
go mod download

# Set up environment
cp .env.example .env
# Edit .env with your configuration (see Configuration section)

# Build and run
go build -o station cmd/main.go
./station
```

#### Initial Setup

1. **Generate encryption key**:
   ```bash
   openssl rand -hex 32
   ```

2. **Configure environment** (`.env`):
   ```bash
   ENCRYPTION_KEY=your-64-character-hex-key-here
   DATABASE_URL=station.db
   SSH_PORT=2222
   MCP_PORT=3000
   API_PORT=8080
   ```

3. **Start Station**:
   ```bash
   ./station
   ```

4. **Access admin interface**:
   ```bash
   ssh admin@localhost -p 2222
   ```

## 📋 Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `ENCRYPTION_KEY` | 32-byte hex key for encrypting sensitive data | - | ✅ |
| `DATABASE_URL` | SQLite database file path | `station.db` | ❌ |
| `SSH_PORT` | SSH admin interface port | `2222` | ❌ |
| `MCP_PORT` | MCP server port | `3000` | ❌ |
| `API_PORT` | HTTP API port | `8080` | ❌ |
| `SSH_HOST_KEY_PATH` | SSH host key file path | `./ssh_host_key` | ❌ |
| `ADMIN_USERNAME` | Default admin username | `admin` | ❌ |

### Model Providers

Station supports multiple AI model providers. Configure them through the admin interface:

- **OpenAI**: GPT-3.5, GPT-4, GPT-4 Turbo models
- **Anthropic**: Claude 3 family models  
- **Local/Custom**: Any OpenAI-compatible API endpoint

## 🏗️ Architecture

Station is built with a modular architecture:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   MCP Clients   │────│   Station MCP   │────│   External MCP  │
│   (Claude, etc) │    │     Server      │    │     Servers     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                               │
                    ┌─────────────────┐
                    │   Agent Engine  │
                    │   (Eino/ReAct)  │
                    └─────────────────┘
                               │
                    ┌─────────────────┐    ┌─────────────────┐
                    │  Encrypted DB   │    │  SSH Admin UI   │
                    │   (SQLite)      │    │  (Bubble Tea)   │
                    └─────────────────┘    └─────────────────┘
```

### Key Components

- **MCP Server**: Secure HTTP server implementing MCP protocol
- **Agent Engine**: ReAct-based agent execution with tool integration
- **Configuration Manager**: Encrypted storage for MCP server configs and API keys
- **Authentication System**: API key-based auth with role-based access
- **SSH Admin Interface**: Terminal UI for system administration

## 🔐 Security

Station implements enterprise-grade security practices:

- **🔑 API Key Authentication**: All MCP requests require valid API keys
- **🔒 Encryption at Rest**: Sensitive data encrypted using NaCl secretbox
- **🛡️ Secure Contexts**: User identity inferred from authenticated context
- **🚫 No Hardcoded Secrets**: All keys managed through environment variables
- **🔍 Audit Logging**: Comprehensive tracking of agent executions and tool usage

## 🛠️ Development

### Prerequisites

- Go 1.21+
- SQLite3
- OpenSSL (for key generation)

### Building from Source

```bash
# Clone and build
git clone https://github.com/your-org/station.git
cd station
go build -o station cmd/main.go

# Run tests
go test ./...

# Or use the test script
./scripts/run_tests.sh
```

### Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📚 API Reference

### MCP Tools

#### `create_agent`

Create a new AI agent with specified configuration.

**Parameters:**
- `name` (string, required): Agent name
- `description` (string, required): Agent description  
- `prompt` (string, required): System prompt for the agent
- `environment_id` (string, required): Environment ID where agent operates
- `max_steps` (number, optional): Maximum execution steps (default: 5)
- `assigned_tools` (array, optional): List of tool names to assign

#### `call_agent`

Execute an AI agent with a given task.

**Parameters:**
- `agent_id` (string, required): ID of the agent to execute
- `task` (string, required): Task or input for the agent

### REST API

Station also exposes a REST API for programmatic access:

- `GET /api/agents` - List all agents
- `POST /api/agents` - Create new agent
- `GET /api/agents/{id}/runs` - Get agent execution history
- `POST /api/environments` - Create new environment

## 🔧 Troubleshooting

### Common Issues

**Connection Refused (MCP)**
- Check that Station is running on the configured MCP port
- Verify API key is correctly formatted in Authorization header

**Authentication Failed**
- Ensure API key is valid and not expired
- Check that user account is active in the admin interface

**Agent Execution Timeout**
- Increase `max_steps` parameter for complex tasks
- Check that required tools are properly configured

### Debug Mode

Enable debug logging:
```bash
STATION_DEBUG=true ./station
```

## 📄 License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0) - see the [LICENSE](LICENSE) file for details.

The AGPL-3.0 ensures that any modifications or improvements to Station remain open source, even when deployed as a web service. For commercial licensing options and enterprise support, please contact us.

## 🤝 Support

- **Documentation**: [Wiki](https://github.com/your-org/station/wiki)
- **Issues**: [GitHub Issues](https://github.com/your-org/station/issues)
- **Discussions**: [GitHub Discussions](https://github.com/your-org/station/discussions)

---

**Station** - Secure AI Agent Management Platform • Built with ❤️ in Go
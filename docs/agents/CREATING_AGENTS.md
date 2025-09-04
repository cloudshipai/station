# Creating Station Agents

Station agents are intelligent AI assistants that can execute tasks using MCP (Model Context Protocol) tools. This guide covers creating, configuring, and deploying agents.

## Agent Fundamentals

### What is a Station Agent?

A Station agent consists of:
- **Dotprompt Configuration**: YAML frontmatter with metadata and input schema
- **Multi-role Prompt Structure**: System and user role definitions  
- **Tool Access**: MCP tools available to the agent
- **Execution Context**: Environment and variable configuration

### Agent File Structure

Agents are defined in `.prompt` files with this structure:

```yaml
---
metadata:
  name: "File Analyzer Agent"
  description: "Analyzes files and directories for insights"
  tags: ["filesystem", "analysis"]
model: gpt-4o-mini
max_steps: 8
tools:
  - "__read_text_file"
  - "__list_directory"
  - "__get_file_info"
  - "__search_files"
input:
  schema:
    type: object
    properties:
      userInput:
        type: string
        description: User input for the agent
      file_path:
        type: string
        description: Path to file or directory to analyze
      analysis_type:
        type: string
        enum: ["basic", "detailed", "security"]
        default: "basic"
    required:
      - userInput
---

{{role "system"}}
You are an expert file analysis agent. When given a file or directory path,
you analyze its contents and provide insights about:
- File types and structure
- Content summaries  
- Potential issues or improvements
- Organization suggestions

Use the provided tools to examine files and directories thoroughly.
Be thorough but concise in your analysis.

{{role "user"}}
{{userInput}}

**Analysis Target:** {{file_path}}
**Analysis Type:** {{analysis_type}}
```

## Creating Agents

### Method 1: Interactive CLI Creation

```bash
# Start interactive agent creation
stn agent create

# Follow prompts for:
# - Agent name and description
# - Environment selection
# - Tool selection
# - Prompt definition
```

The CLI will guide you through:
1. **Basic Configuration**: Name, description, environment
2. **Tool Selection**: Choose from available MCP tools
3. **Prompt Creation**: Define agent behavior and capabilities
4. **Testing**: Run initial test to verify functionality

### Method 2: Manual File Creation

Create agent files directly in your environment:

```bash
# Navigate to environment agents directory
cd ~/.config/station/environments/default/agents

# Create agent file
touch "My Custom Agent.prompt"

# Edit with your preferred editor
editor "My Custom Agent.prompt"
```

### Method 3: Template-based Creation

Use existing agents as templates:

```bash
# Copy an existing agent
cp ~/.config/station/environments/default/agents/Hello\ World\ Agent.prompt \
   ~/.config/station/environments/default/agents/My\ New\ Agent.prompt

# Customize the copied agent
editor ~/.config/station/environments/default/agents/My\ New\ Agent.prompt
```

## Agent Configuration

### Metadata Section

```yaml
metadata:
  name: "Agent Name"           # Display name (required)
  description: "Description"   # Brief description (required)  
  tags: ["tag1", "tag2"]      # Categories for organization
```

### Model and Execution Settings

```yaml
model: gpt-4o-mini             # AI model to use
max_steps: 8                   # Maximum execution steps
```

Supported models:
- `gpt-4o` - Most capable, higher cost
- `gpt-4o-mini` - Fast and efficient (recommended)
- `gpt-4-turbo` - Good balance of capability and speed

### Tool Configuration

```yaml
tools:
  - "__read_text_file"         # Read file contents
  - "__list_directory"         # List directory contents
  - "__search_files"           # Search for files
  - "__get_file_info"          # Get file metadata
```

Available tool categories:
- **Filesystem**: `__read_text_file`, `__list_directory`, `__search_files`
- **Web Automation**: `__browser_navigate`, `__browser_screenshot`, `__browser_click`
- **Security**: `__checkov_scan_directory`, `__trivy_scan_filesystem`
- **Custom**: Your own MCP tools

### Input Schema (JSON Schema)

Define custom input parameters:

```yaml
input:
  schema:
    type: object
    properties:
      userInput:
        type: string
        description: User input for the agent
      # Custom parameters
      target_directory:
        type: string
        description: Directory to analyze
      analysis_depth:
        type: string
        enum: ["surface", "deep", "comprehensive"]
        default: "surface"
      include_hidden:
        type: boolean
        default: false
    required:
      - userInput
      - target_directory
```

## Prompt Engineering

### System Role Definition

The system role defines agent behavior:

```yaml
{{role "system"}}
You are a specialized code review agent with expertise in:
- Code quality analysis
- Security vulnerability detection  
- Performance optimization suggestions
- Best practice recommendations

Guidelines:
1. Always scan the entire codebase first to understand structure
2. Focus on critical issues before minor style improvements
3. Provide specific, actionable recommendations
4. Include code examples in your suggestions
5. Prioritize security and performance issues

Available tools allow you to read files, search codebases, and analyze directory structures.
```

### User Role and Variables

The user role processes input and variables:

```yaml
{{role "user"}}
{{userInput}}

**Target Repository:** {{repository_path}}
**Review Focus:** {{review_focus}}
**Output Format:** {{output_format}}
```

### Best Practices for Prompts

1. **Be Specific**: Clearly define agent capabilities and limitations
2. **Include Context**: Explain available tools and how to use them
3. **Set Expectations**: Define output format and quality standards
4. **Handle Edge Cases**: Account for error conditions and unexpected inputs
5. **Iterate**: Test and refine prompts based on agent performance

## Testing Agents

### Basic Testing

```bash
# Test agent with simple input
stn agent run "My Agent" "Test message"

# Test with custom parameters (if agent uses custom schema)
stn agent run "File Analyzer" "Analyze my project" \
  --file-path "/path/to/project" \
  --analysis-type "detailed"
```

### Advanced Testing

```bash
# Follow real-time execution
stn agent run "My Agent" "Complex task" --tail

# Test with different environments
stn agent run "My Agent" "Task" --env production

# Inspect detailed execution logs
stn runs list
stn runs inspect <run-id> -v
```

### Performance Testing

```bash
# Test execution time and token usage
time stn agent run "My Agent" "Performance test task"

# Monitor resource usage
stn status --verbose

# Test with large inputs
stn agent run "File Analyzer" "Analyze large directory" \
  --file-path "/very/large/directory"
```

## Agent Examples

### 1. Simple File Analyzer

```yaml
---
metadata:
  name: "File Analyzer"
  description: "Analyzes file contents and provides insights"
  tags: ["filesystem", "analysis"]
model: gpt-4o-mini
max_steps: 5
tools:
  - "__read_text_file"
  - "__get_file_info"
input:
  schema:
    type: object
    properties:
      userInput:
        type: string
        description: User input for the agent
    required:
      - userInput
---

{{role "system"}}
You are a file analysis expert. When given a file path, analyze its contents
and provide insights about file type, structure, and potential improvements.

{{role "user"}}
{{userInput}}
```

### 2. Web Testing Agent

```yaml
---
metadata:
  name: "Web Page Tester"
  description: "Tests web pages for functionality and performance"
  tags: ["web", "testing", "automation"]
model: gpt-4o-mini
max_steps: 10
tools:
  - "__browser_navigate"
  - "__browser_screenshot"
  - "__browser_click"
  - "__browser_evaluate"
input:
  schema:
    type: object
    properties:
      userInput:
        type: string
        description: User input for the agent
      target_url:
        type: string
        description: URL to test
      test_type:
        type: string
        enum: ["functionality", "performance", "accessibility"]
        default: "functionality"
    required:
      - userInput
      - target_url
---

{{role "system"}}
You are a web testing specialist. Test websites for functionality,
performance, and user experience issues. Always take screenshots
to document your findings.

{{role "user"}}
{{userInput}}

**Target URL:** {{target_url}}
**Test Type:** {{test_type}}
```

### 3. Security Scanner Agent

```yaml
---
metadata:
  name: "Security Scanner"
  description: "Scans codebases for security vulnerabilities"
  tags: ["security", "analysis", "devops"]
model: gpt-4o-mini
max_steps: 12
tools:
  - "__checkov_scan_directory"
  - "__trivy_scan_filesystem"
  - "__search_files"
  - "__read_text_file"
input:
  schema:
    type: object
    properties:
      userInput:
        type: string
        description: User input for the agent
      scan_path:
        type: string
        description: Path to scan for security issues
      severity_level:
        type: string
        enum: ["low", "medium", "high", "critical"]
        default: "medium"
    required:
      - userInput
      - scan_path
---

{{role "system"}}
You are a security analysis expert specializing in:
- Infrastructure as Code security (Terraform, Docker, K8s)
- Container vulnerability scanning  
- Source code security analysis
- Compliance checking

Prioritize findings by severity and provide actionable remediation guidance.

{{role "user"}}
{{userInput}}

**Scan Target:** {{scan_path}}
**Minimum Severity:** {{severity_level}}
```

## Deployment and Distribution

### Environment Deployment

```bash
# Deploy agent to specific environment
stn agent create --env production

# Copy agent between environments  
cp ~/.config/station/environments/dev/agents/My\ Agent.prompt \
   ~/.config/station/environments/prod/agents/
```

### Bundle Creation

Package agents for distribution:

```bash
# Create bundle with multiple agents
stn bundle create my-agent-bundle

# Add agents to bundle
stn bundle add "My Agent" my-agent-bundle

# Export bundle for sharing
stn bundle export my-agent-bundle ./my-bundle.tar.gz
```

### Sharing and Versioning

```bash
# Export individual agent
stn agent export "My Agent" ./my-agent.prompt

# Import agent from file
stn agent import ./shared-agent.prompt

# Version control agents with git
cd ~/.config/station/environments/default
git init
git add agents/
git commit -m "Add custom agents"
```

## Troubleshooting

### Common Issues

#### Agent Won't Start
```bash
# Check agent configuration
stn agent validate "My Agent"

# Verify tool availability
stn mcp tools | grep required_tool

# Check environment setup
stn env status
```

#### Tool Execution Failures
```bash
# Test individual tools
stn mcp call tool_name '{"param": "value"}'

# Check MCP server status
stn mcp status

# Refresh tool discovery
stn mcp refresh
```

#### Performance Issues
```bash
# Monitor execution
stn agent run "My Agent" "task" --tail

# Review execution logs
stn runs inspect <run-id> -v

# Check token usage patterns
stn runs list --tokens
```

### Best Practices

1. **Start Simple**: Begin with basic agents and gradually add complexity
2. **Test Thoroughly**: Verify agents work with various input types
3. **Monitor Performance**: Track token usage and execution time
4. **Use Version Control**: Keep agent definitions in git repositories
5. **Document Behavior**: Maintain clear descriptions of agent capabilities
6. **Handle Errors**: Design agents to gracefully handle failures

## Next Steps

- **[Environment Management](./ENVIRONMENTS.md)** - Multi-environment workflows
- **[MCP Integration](./MCP_INTEGRATION.md)** - Advanced tool usage
- **[Bundle System](../bundles/BUNDLE_SYSTEM.md)** - Package and share agents
- **[Performance Monitoring](../monitoring/PERFORMANCE.md)** - Track agent metrics
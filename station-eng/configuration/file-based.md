# File-Based Configuration System

Station uses a **file-based configuration approach** for GitOps-friendly MCP server management. This system replaced the original database-only approach to enable version control and team collaboration.

## 🎯 Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                FILE-BASED CONFIGURATION SYSTEM                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │  Templates  │  │  Variables  │  │   Configs   │            │
│  │             │  │             │  │             │            │
│  │ - JSON/YAML │  │ - Per Env   │  │ - Rendered  │            │
│  │ - {{VAR}}   │  │ - Encrypted │  │ - Final     │            │
│  │ - Reusable  │  │ - Secrets   │  │ - Loaded    │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│         │                │                │                    │
│         └────────────────┼────────────────┘                    │
│                          │                                     │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              TEMPLATE ENGINE                            │   │
│  │                                                         │   │
│  │  Variable Resolution │ Validation │ Environment Mgmt   │   │
│  └─────────────────────────────────────────────────────────┘   │
│                          │                                     │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │               MCP SERVER LOADING                        │   │
│  │                                                         │   │  
│  │  Server Startup │ Tool Discovery │ Health Monitoring   │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## 📁 Directory Structure

**Default Configuration Layout**:
```
~/.config/station/
├── config.yaml                    # Main Station configuration
├── config/
│   └── environments/
│       ├── default/
│       │   ├── filesystem.json    # MCP server templates
│       │   ├── aws-cli.json
│       │   └── template-vars/     # Variable storage
│       │       ├── filesystem.yml
│       │       └── aws-cli.yml
│       ├── production/
│       │   ├── github.json
│       │   └── template-vars/
│       │       └── github.yml
│       └── development/
│           ├── database.json
│           └── template-vars/
│               └── database.yml
```

**File Types**:
- **Templates** (`.json`): MCP server configurations with variables
- **Variables** (`.yml`): Environment-specific variable values
- **Config** (`.yaml`): Main Station configuration

## 🎨 Template System

### **Template Format**
Templates use **Go template syntax** with double braces and support **multiple MCP transport types**:

#### **Stdio Transport** (Subprocess-based MCP servers)
```json
{
  "name": "GitHub Integration", 
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "{{.GITHUB_TOKEN}}",
        "GITHUB_API_BASE_URL": "{{.GITHUB_API_URL}}",
        "GITHUB_REPO_ACCESS": "{{.REPO_ACCESS_LEVEL}}"
      }
    },
    "filesystem": {
      "command": "npx", 
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "{{.PROJECT_PATH}}"],
      "env": {
        "FILESYSTEM_WRITE_ENABLED": "{{.WRITE_ENABLED}}"
      }
    }
  }
}
```

#### **HTTP/SSE Transport** (URL-based MCP servers)
```json
{
  "name": "Cloud MCP Services",
  "mcpServers": {
    "aws-knowledge": {
      "url": "https://knowledge-mcp.global.api.aws",
      "env": {
        "AUTHORIZATION": "Bearer {{.AWS_MCP_TOKEN}}",
        "AWS_REGION": "{{.AWS_REGION}}"
      }
    },
    "custom-api": {
      "url": "{{.CUSTOM_MCP_ENDPOINT}}/sse",
      "env": {
        "API_KEY": "{{.CUSTOM_API_KEY}}",
        "HTTP_X_CLIENT_ID": "station-{{.CLIENT_ID}}"
      }
    }
  }
}
```

#### **Mixed Transport Configuration** (Best of both worlds)
```json
{
  "name": "Hybrid MCP Setup",
  "mcpServers": {
    "local-filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "{{.PROJECT_PATH}}"]
    },
    "cloud-knowledge": {
      "url": "https://knowledge-mcp.global.api.aws",
      "env": {
        "AUTHORIZATION": "Bearer {{.AWS_TOKEN}}"
      }
    },
    "github-api": {
      "command": "npx", 
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_PERSONAL_ACCESS_TOKEN": "{{.GITHUB_TOKEN}}"
      }
    }
  }
}
```

### **Variable Files**
Variables are stored per-environment in YAML format:

```yaml
# ~/.config/station/config/environments/production/template-vars/github.yml
GITHUB_TOKEN: "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxx"
GITHUB_API_URL: "https://api.github.com"  
REPO_ACCESS_LEVEL: "read"
PROJECT_PATH: "/home/user/projects"
WRITE_ENABLED: "false"
```

### **Variable Types and Security**
```yaml
# Variable types and handling
GITHUB_TOKEN: "sensitive_value"      # Encrypted at rest
PROJECT_PATH: "/safe/path"           # Plain text, validated
DEBUG_MODE: "false"                  # Boolean string
PORT_NUMBER: "3000"                  # Numeric string
```

## 🚀 Configuration Loading Process

### **Loading Flow**
```
1. Template Discovery    │ Find .json templates in environment dir
                        │
2. Variable Loading     │ Load corresponding .yml variable files
                        │  
3. Template Rendering   │ Apply variables to template placeholders
                        │
4. Validation           │ Validate rendered configuration
                        │
5. MCP Server Startup   │ Launch MCP servers with final config
                        │
6. Tool Discovery       │ Discover and register available tools using Universal MCP Client
```

## 🌐 Universal MCP Client Architecture

Station implements a **universal MCP client** that can consume any compatible MCP server configuration, supporting all transport types available through the [`mcp-go`](https://github.com/mark3labs/mcp-go) library.

### **Design Philosophy**
> **"Use mcp-go as the primary client for consuming any MCP server configuration, then bridge to genkit if needed"**

This approach provides maximum compatibility and future-proofing by leveraging mcp-go's comprehensive transport support rather than manually parsing MCP configs.

### **Transport Support Matrix**

| Transport Type | Use Case | Configuration | Authentication |
|---------------|----------|---------------|----------------|
| **Stdio** | Local subprocess servers | `command` + `args` | Environment variables |
| **SSE** | HTTP Server-Sent Events | `url` (auto-detected) | HTTP headers via `env` |
| **HTTP** | RESTful MCP services | `url` + explicit type | Bearer tokens, API keys |

### **Universal Client Implementation** (`internal/services/tool_discovery_client.go`)

```go
type MCPClient struct{}

// DiscoverToolsFromServer connects using appropriate transport and discovers tools
func (c *MCPClient) DiscoverToolsFromServer(serverConfig models.MCPServerConfig) ([]mcp.Tool, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
    defer cancel()

    // Create appropriate transport based on server configuration  
    mcpTransport, err := c.createTransport(serverConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create transport: %v", err)
    }

    // Create universal client with the transport
    mcpClient := client.NewClient(mcpTransport)
    
    // Start, initialize, and discover tools
    if err := mcpClient.Start(ctx); err != nil {
        return nil, fmt.Errorf("failed to start client: %v", err)
    }
    defer mcpClient.Close()

    // Standard MCP initialization process
    initRequest := mcp.InitializeRequest{}
    initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
    initRequest.Params.ClientInfo = mcp.Implementation{
        Name:    "Station Tool Discovery",
        Version: "1.0.0",
    }

    serverInfo, err := mcpClient.Initialize(ctx, initRequest)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize: %v", err)
    }

    // Discover and return tools
    toolsRequest := mcp.ListToolsRequest{}
    toolsResult, err := mcpClient.ListTools(ctx, toolsRequest)
    if err != nil {
        return nil, fmt.Errorf("failed to list tools: %v", err)
    }

    return toolsResult.Tools, nil
}
```

### **Smart Transport Selection**

```go
func (c *MCPClient) createTransport(serverConfig models.MCPServerConfig) (transport.Interface, error) {
    // Option 1: Stdio transport (subprocess-based)
    if serverConfig.Command != "" {
        log.Printf("Creating stdio transport for command: %s", serverConfig.Command)
        
        var envSlice []string
        for key, value := range serverConfig.Env {
            envSlice = append(envSlice, fmt.Sprintf("%s=%s", key, value))
        }
        
        return transport.NewStdio(serverConfig.Command, envSlice, serverConfig.Args...), nil
    }
    
    // Option 2: URL-based transports (HTTP/SSE)
    if serverConfig.URL != "" {
        return c.createHTTPTransport(serverConfig.URL, serverConfig.Env)
    }
    
    // Option 3: Backwards compatibility - check args for URLs
    for _, arg := range serverConfig.Args {
        if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
            return c.createHTTPTransport(arg, serverConfig.Env)
        }
    }
    
    return nil, fmt.Errorf("no valid transport configuration found - provide either 'command' for stdio transport or 'url' for HTTP/SSE transport")
}
```

### **HTTP Transport with Authentication**

```go  
func (c *MCPClient) createHTTPTransport(baseURL string, envVars map[string]string) (transport.Interface, error) {
    // Parse and validate URL
    _, err := url.Parse(baseURL)
    if err != nil {
        return nil, fmt.Errorf("invalid URL format: %v", err)
    }
    
    var options []transport.ClientOption
    
    // Convert environment variables to HTTP headers
    if len(envVars) > 0 {
        headers := make(map[string]string)
        for key, value := range envVars {
            switch {
            case strings.HasPrefix(key, "HTTP_"):
                // HTTP_X_API_KEY -> X-API-Key
                headerName := strings.ReplaceAll(strings.TrimPrefix(key, "HTTP_"), "_", "-")
                headers[headerName] = value
            case key == "AUTHORIZATION" || key == "AUTH_TOKEN":
                headers["Authorization"] = value
            case key == "API_KEY":
                headers["X-API-Key"] = value
            }
        }
        if len(headers) > 0 {
            options = append(options, transport.WithHeaders(headers))
        }
    }
    
    // Use SSE transport (most widely supported for MCP)
    return transport.NewSSE(baseURL, options...)
}
```

### **Configuration Parsing Enhancement**

The file config service was updated to support both transport types:

```go
// Parse rendered JSON with both stdio and URL support
var mcpConfig struct {
    MCPServers map[string]struct {
        Command string            `json:"command"`  // Stdio transport
        Args    []string          `json:"args"`     // Stdio transport  
        URL     string            `json:"url"`      // HTTP/SSE transport
        Env     map[string]string `json:"env"`      // Both transports
    } `json:"mcpServers"`
}

// Convert to internal format
for name, serverConfig := range mcpConfig.MCPServers {
    servers[name] = models.MCPServerConfig{
        Command: serverConfig.Command,  // ✅ Stdio support
        Args:    serverConfig.Args,     // ✅ Stdio support
        URL:     serverConfig.URL,      // ✅ NEW: HTTP/SSE support  
        Env:     serverConfig.Env,      // ✅ Both transports
    }
}
```

### **Benefits of Universal Client Approach**

1. **Maximum Compatibility**: Supports any MCP server that mcp-go can connect to
2. **Future-Proof**: Automatically gains support for new transports added to mcp-go
3. **No Manual Parsing**: Leverages mcp-go's robust configuration handling
4. **Consistent Tool Discovery**: Same discovery process regardless of transport
5. **Graceful Error Handling**: Clear errors instead of panics for invalid configs
6. **Authentication Flexibility**: Supports various auth methods via environment mapping

### **Implementation** (`internal/services/file_config_service.go`)
```go
type FileConfigService struct {
    configDir      string
    varsDir        string
    templateEngine *template.Engine
    fileSystem     filesystem.FileSystem
}

func (fcs *FileConfigService) LoadConfig(envName, configName string) (*models.MCPConfig, error) {
    // 1. Locate template file
    templatePath := filepath.Join(fcs.configDir, "environments", envName, configName+".json")
    
    // 2. Load template content
    templateContent, err := fcs.fileSystem.ReadFile(templatePath)
    if err != nil {
        return nil, fmt.Errorf("failed to read template: %w", err)
    }
    
    // 3. Load variables for this environment
    variables, err := fcs.loadVariables(envName, configName)
    if err != nil {
        return nil, fmt.Errorf("failed to load variables: %w", err)
    }
    
    // 4. Render template with variables
    rendered, err := fcs.templateEngine.Render(templateContent, variables)
    if err != nil {
        return nil, fmt.Errorf("failed to render template: %w", err)
    }
    
    // 5. Parse and validate final configuration
    var config models.MCPConfig
    if err := json.Unmarshal(rendered, &config); err != nil {
        return nil, fmt.Errorf("invalid configuration: %w", err)
    }
    
    return &config, nil
}
```

## 🔧 Environment Management

### **Environment Structure**
Each environment is a separate directory with isolated configurations:

```go
type Environment struct {
    ID          int64     `json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    ConfigDir   string    `json:"config_dir"`
    VarsDir     string    `json:"vars_dir"`
    CreatedAt   time.Time `json:"created_at"`
}
```

### **Environment Operations**
```go
// Create new environment
func (fcs *FileConfigService) CreateEnvironment(name, description string) (*Environment, error) {
    envDir := filepath.Join(fcs.configDir, "environments", name)
    
    // Create directory structure
    dirs := []string{
        envDir,
        filepath.Join(envDir, "template-vars"),
    }
    
    for _, dir := range dirs {
        if err := fcs.fileSystem.MkdirAll(dir, 0755); err != nil {
            return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
        }
    }
    
    // Create environment record
    env := &Environment{
        Name:        name,
        Description: description,
        ConfigDir:   envDir,
        VarsDir:     filepath.Join(envDir, "template-vars"),
        CreatedAt:   time.Now(),
    }
    
    return env, nil
}
```

### **Environment Switching**
```bash
# Load config from specific environment
stn load github.json --env production

# Load from default environment  
stn load github.json --env default

# Create new environment
stn env create staging --description "Staging environment"
```

## 🎭 Variable Resolution System

### **Variable Sources** (Priority Order)
1. **Command Line Arguments**: `--var KEY=VALUE`
2. **Environment Variables**: `STATION_VAR_KEY=VALUE`
3. **Template Variables File**: `template-vars/config.yml`
4. **Global Variables File**: `variables.yml`
5. **Default Values**: Template defaults

### **Variable Processing**
```go
func (fcs *FileConfigService) resolveVariables(envName, configName string) (map[string]interface{}, error) {
    variables := make(map[string]interface{})
    
    // 1. Load global variables (lowest priority)
    if globalVars, err := fcs.loadGlobalVariables(); err == nil {
        mergeVariables(variables, globalVars)
    }
    
    // 2. Load environment variables
    if envVars, err := fcs.loadEnvironmentVariables(envName); err == nil {
        mergeVariables(variables, envVars)
    }
    
    // 3. Load template-specific variables (highest priority)
    if templateVars, err := fcs.loadTemplateVariables(envName, configName); err == nil {
        mergeVariables(variables, templateVars)
    }
    
    // 4. Apply environment variable overrides
    applyEnvironmentOverrides(variables)
    
    return variables, nil
}
```

### **Intelligent Variable Detection**
Station can automatically detect placeholders in templates:

```go
// internal/services/intelligent_placeholder_analyzer.go
func (ipa *IntelligentPlaceholderAnalyzer) DetectVariables(template string) ([]VariableInfo, error) {
    patterns := []string{
        `\{\{\.([A-Z_][A-Z0-9_]*)\}\}`,     // {{.VAR_NAME}}
        `\{\{([A-Z_][A-Z0-9_]*)\}\}`,       // {{VAR_NAME}}
        `<([A-Z_][A-Z0-9_]*)>`,             // <VAR_NAME>
        `\[([A-Z_][A-Z0-9_]*)\]`,           // [VAR_NAME]
        `YOUR_([A-Z_][A-Z0-9_]*)`,          // YOUR_API_KEY
    }
    
    var variables []VariableInfo
    for _, pattern := range patterns {
        matches := regexp.MustCompile(pattern).FindAllStringSubmatch(template, -1)
        for _, match := range matches {
            variables = append(variables, VariableInfo{
                Name:        match[1],
                Pattern:     pattern,
                Required:    true,
                Detected:    true,
            })
        }
    }
    
    return deduplicateVariables(variables), nil
}
```

## 🔄 Interactive Configuration Workflow

### **Interactive Loading Process**
When users run `stn load` without arguments, Station provides an interactive experience:

```
1. Template Editor    │ Opens editor for configuration template
                     │
2. AI Detection      │ Automatically detects {{VARIABLES}} 
                     │
3. Variable Forms    │ Interactive prompts for each variable
                     │
4. Validation        │ Validates configuration and variables
                     │
5. Save & Load       │ Saves to environment and loads MCP servers
```

### **Interactive Implementation**
```go
func (lh *LoadHandler) interactiveLoad(envName string) error {
    // 1. Open editor for template input
    template, err := lh.openEditor()
    if err != nil {
        return fmt.Errorf("editor failed: %w", err)
    }
    
    // 2. Detect variables in template
    analyzer := services.NewIntelligentPlaceholderAnalyzer()
    variables, err := analyzer.DetectVariables(template)
    if err != nil {
        return fmt.Errorf("variable detection failed: %w", err)
    }
    
    // 3. Prompt for variable values
    values := make(map[string]string)
    for _, variable := range variables {
        value, err := lh.promptForVariable(variable)
        if err != nil {
            return fmt.Errorf("variable input failed: %w", err)
        }
        values[variable.Name] = value
    }
    
    // 4. Save configuration and variables
    configName := generateConfigName()
    if err := lh.saveConfiguration(envName, configName, template, values); err != nil {
        return fmt.Errorf("save failed: %w", err)
    }
    
    // 5. Load the configuration
    return lh.loadConfiguration(envName, configName)
}
```

## 🛠️ Development Best Practices

### **Template Design**
- **Use descriptive variable names**: `GITHUB_TOKEN` not `TOKEN`
- **Provide defaults where sensible**: `{{.DEBUG_MODE | default "false"}}`
- **Group related variables**: Keep related configs in same template
- **Document variables**: Add comments explaining usage

### **Variable Management**
- **Separate secrets from configs**: Use separate files for sensitive data
- **Environment isolation**: Never share variables between environments
- **Version control templates**: Templates in git, variables local/encrypted
- **Validate variable format**: Check URLs, paths, booleans, etc.

### **Environment Organization**
- **Logical naming**: `production`, `staging`, `development`, `local`
- **Consistent structure**: Same template names across environments
- **Documentation**: Document what each environment is for
- **Access control**: Appropriate permissions for variable files

## 🔒 Security Considerations

### **Variable Encryption**
Variables can be encrypted at rest using Station's encryption system:

```go
// Encrypt sensitive variables before storage
func (fcs *FileConfigService) saveEncryptedVariable(name, value string) error {
    encrypted, err := fcs.cryptoService.Encrypt([]byte(value))
    if err != nil {
        return fmt.Errorf("encryption failed: %w", err)
    }
    
    return fcs.saveVariable(name, base64.StdEncoding.EncodeToString(encrypted))
}
```

### **File Permissions**
```bash
# Template files (shareable)
chmod 644 ~/.config/station/config/environments/*/config.json

# Variable files (sensitive)  
chmod 600 ~/.config/station/config/environments/*/template-vars/*.yml

# Environment directories
chmod 755 ~/.config/station/config/environments/*/
```

### **Git Integration**
```gitignore
# .gitignore for Station configs
# Include templates (shareable)
!config/environments/*/config.json

# Exclude variables (sensitive)
config/environments/*/template-vars/
config/environments/*/variables.yml
*.local.yml
```

---
*Next: See `templates.md` for template engine details and advanced templating features*
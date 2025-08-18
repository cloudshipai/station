# Template Variable System

## Overview

Station's Template Variable System enables dynamic configuration of MCP servers and agents through parameterized templates. This system resolves template variables from environment-specific configuration files, supports interactive prompting for missing variables, and ensures robust template processing across different deployment environments.

## Architecture

### Variable Resolution Flow

```mermaid
graph TB
    subgraph "Template Processing"
        Template[Template File]
        VariableDetection[Variable Detection]
        VariableResolution[Variable Resolution]
        TemplateRendering[Template Rendering]
    end
    
    subgraph "Variable Sources"
        VariablesFile[variables.yml]
        EnvironmentVars[Environment Variables]
        InteractiveInput[Interactive Prompts]
        DefaultValues[Default Values]
    end
    
    subgraph "Validation & Error Handling"
        MissingVarCheck[Missing Variable Check]
        ValidationRules[Validation Rules]
        ErrorReporting[Error Reporting]
    end
    
    Template --> VariableDetection
    VariableDetection --> VariableResolution
    
    VariableResolution --> VariablesFile
    VariableResolution --> EnvironmentVars
    VariableResolution --> InteractiveInput
    VariableResolution --> DefaultValues
    
    VariableResolution --> MissingVarCheck
    MissingVarCheck --> ValidationRules
    ValidationRules --> ErrorReporting
    ErrorReporting --> TemplateRendering
    
    VariableResolution --> TemplateRendering
```

## Implementation

### Core Template Variable Service

```go
// internal/services/template_variable_service.go
type TemplateVariableService struct {
    environmentsPath string
    interactive      bool
    logger          *logging.Logger
}

type VariableContext struct {
    EnvironmentName string
    Variables       map[string]string
    RequiredVars    []string
    MissingVars     []string
}

func NewTemplateVariableService(environmentsPath string, interactive bool) *TemplateVariableService {
    return &TemplateVariableService{
        environmentsPath: environmentsPath,
        interactive:      interactive,
        logger:          logging.GetLogger("template-variables"),
    }
}

func (tvs *TemplateVariableService) ProcessTemplate(
    templateContent string, 
    environmentName string,
) (string, error) {
    // Load variables from file
    variables, err := tvs.loadVariables(environmentName)
    if err != nil {
        return "", fmt.Errorf("failed to load variables: %w", err)
    }
    
    // Create variable context
    context := &VariableContext{
        EnvironmentName: environmentName,
        Variables:       variables,
    }
    
    // Render template with error handling for missing variables
    rendered, err := tvs.renderTemplate(templateContent, context.Variables)
    if err != nil {
        // Check if error is due to missing variables
        if missingVars := tvs.extractMissingVariables(err); len(missingVars) > 0 {
            context.MissingVars = missingVars
            
            // Prompt for missing variables if interactive mode
            if tvs.interactive {
                if err := tvs.promptForMissingVariables(context); err != nil {
                    return "", fmt.Errorf("failed to collect missing variables: %w", err)
                }
                
                // Save updated variables
                if err := tvs.saveVariables(environmentName, context.Variables); err != nil {
                    tvs.logger.Warn("Failed to save variables: %v", err)
                }
                
                // Re-render with complete variables
                return tvs.renderTemplate(templateContent, context.Variables)
            }
            
            return "", fmt.Errorf("missing required variables: %v", missingVars)
        }
        
        return "", fmt.Errorf("template rendering failed: %w", err)
    }
    
    return rendered, nil
}

func (tvs *TemplateVariableService) renderTemplate(
    templateContent string, 
    variables map[string]string,
) (string, error) {
    // Create a new template with the content and configure it to error on missing keys
    tmpl, err := template.New("mcp-config").Option("missingkey=error").Parse(templateContent)
    if err != nil {
        return "", fmt.Errorf("failed to parse template: %w", err)
    }
    
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, variables); err != nil {
        return "", fmt.Errorf("template execution failed: %w", err)
    }
    
    return buf.String(), nil
}
```

### Variable Loading and Persistence

```go
func (tvs *TemplateVariableService) loadVariables(environmentName string) (map[string]string, error) {
    variablesPath := filepath.Join(tvs.environmentsPath, environmentName, "variables.yml")
    
    // Return empty map if variables file doesn't exist
    if _, err := os.Stat(variablesPath); os.IsNotExist(err) {
        tvs.logger.Debug("Variables file not found: %s", variablesPath)
        return make(map[string]string), nil
    }
    
    data, err := os.ReadFile(variablesPath)
    if err != nil {
        return nil, fmt.Errorf("failed to read variables file: %w", err)
    }
    
    variables := make(map[string]string)
    if err := yaml.Unmarshal(data, &variables); err != nil {
        return nil, fmt.Errorf("failed to parse variables YAML: %w", err)
    }
    
    tvs.logger.Debug("Loaded %d variables from %s", len(variables), variablesPath)
    return variables, nil
}

func (tvs *TemplateVariableService) saveVariables(
    environmentName string, 
    variables map[string]string,
) error {
    envDir := filepath.Join(tvs.environmentsPath, environmentName)
    if err := os.MkdirAll(envDir, 0755); err != nil {
        return fmt.Errorf("failed to create environment directory: %w", err)
    }
    
    variablesPath := filepath.Join(envDir, "variables.yml")
    
    data, err := yaml.Marshal(variables)
    if err != nil {
        return fmt.Errorf("failed to marshal variables: %w", err)
    }
    
    if err := os.WriteFile(variablesPath, data, 0644); err != nil {
        return fmt.Errorf("failed to write variables file: %w", err)
    }
    
    tvs.logger.Info("Saved %d variables to %s", len(variables), variablesPath)
    return nil
}
```

### Interactive Variable Collection

```go
func (tvs *TemplateVariableService) promptForMissingVariables(context *VariableContext) error {
    if !tvs.interactive {
        return fmt.Errorf("interactive mode disabled, cannot prompt for variables")
    }
    
    fmt.Printf("\n🔧 Missing template variables for environment '%s'\n\n", context.EnvironmentName)
    
    for _, varName := range context.MissingVars {
        // Check if variable already has a value (might be empty string)
        if _, exists := context.Variables[varName]; exists {
            continue
        }
        
        // Prompt for variable value
        fmt.Printf("Enter value for %s", varName)
        
        // Add helpful hints for common variables
        if hint := tvs.getVariableHint(varName); hint != "" {
            fmt.Printf(" (%s)", hint)
        }
        
        fmt.Print(": ")
        
        reader := bufio.NewReader(os.Stdin)
        input, err := reader.ReadString('\n')
        if err != nil {
            return fmt.Errorf("failed to read input for %s: %w", varName, err)
        }
        
        value := strings.TrimSpace(input)
        
        // Validate required variables
        if value == "" && tvs.isRequiredVariable(varName) {
            fmt.Printf("❌ %s is required and cannot be empty\n", varName)
            return fmt.Errorf("required variable %s cannot be empty", varName)
        }
        
        context.Variables[varName] = value
        fmt.Printf("✅ Set %s = %s\n", varName, value)
    }
    
    fmt.Println()
    return nil
}

func (tvs *TemplateVariableService) getVariableHint(varName string) string {
    hints := map[string]string{
        "ROOT_PATH":     "absolute path to your project root",
        "ALLOWED_PATH":  "directory path for filesystem access",
        "API_KEY":       "your API key for external services",
        "DATABASE_URL":  "connection string for database",
        "AWS_REGION":    "AWS region like us-east-1",
        "SLACK_TOKEN":   "Slack bot token",
        "GITHUB_TOKEN":  "GitHub personal access token",
        "PROJECT_NAME":  "name of your project",
        "ENVIRONMENT":   "environment name like dev, staging, prod",
    }
    
    return hints[varName]
}

func (tvs *TemplateVariableService) isRequiredVariable(varName string) bool {
    requiredVars := []string{
        "ROOT_PATH",
        "ALLOWED_PATH",
        "API_KEY",
        "DATABASE_URL",
    }
    
    for _, required := range requiredVars {
        if varName == required {
            return true
        }
    }
    
    return false
}
```

### Missing Variable Detection

```go
func (tvs *TemplateVariableService) extractMissingVariables(err error) []string {
    var missingVars []string
    
    // Go template errors for missing variables look like:
    // "template: mcp-config:1:23: executing \"mcp-config\" at <.VARIABLE_NAME>: map has no entry for key \"VARIABLE_NAME\""
    errorStr := err.Error()
    
    // Regular expression to match template variable errors
    re := regexp.MustCompile(`map has no entry for key "([^"]+)"`)
    matches := re.FindAllStringSubmatch(errorStr, -1)
    
    for _, match := range matches {
        if len(match) > 1 {
            varName := match[1]
            // Avoid duplicates
            if !contains(missingVars, varName) {
                missingVars = append(missingVars, varName)
            }
        }
    }
    
    return missingVars
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
```

## Integration with MCP Configuration Sync

### Declarative Sync Service Integration

```go
// internal/services/declarative_sync.go
func (ds *DeclarativeSyncService) processTemplate(
    templatePath string, 
    environmentName string,
) (*MCPServerConfig, error) {
    // Read template content
    templateContent, err := os.ReadFile(templatePath)
    if err != nil {
        return nil, fmt.Errorf("failed to read template: %w", err)
    }
    
    // Process template variables
    processedContent, err := ds.templateVariableService.ProcessTemplate(
        string(templateContent),
        environmentName,
    )
    if err != nil {
        return nil, fmt.Errorf("template processing failed: %w", err)
    }
    
    // Parse processed JSON
    var serverConfig MCPServerConfig
    if err := json.Unmarshal([]byte(processedContent), &serverConfig); err != nil {
        return nil, fmt.Errorf("failed to parse processed template: %w", err)
    }
    
    return &serverConfig, nil
}

func (ds *DeclarativeSyncService) syncEnvironment(environmentName string) error {
    envPath := filepath.Join(ds.environmentsPath, environmentName)
    
    // Scan for template files
    templateFiles, err := filepath.Glob(filepath.Join(envPath, "*.json"))
    if err != nil {
        return fmt.Errorf("failed to scan template files: %w", err)
    }
    
    for _, templatePath := range templateFiles {
        templateName := strings.TrimSuffix(filepath.Base(templatePath), ".json")
        
        logging.Info("Processing template: %s", templateName)
        
        // Process template with variable substitution
        serverConfig, err := ds.processTemplate(templatePath, environmentName)
        if err != nil {
            logging.Error("Template processing failed for %s: %v", templateName, err)
            continue
        }
        
        // Create or update MCP server
        if err := ds.createOrUpdateMCPServer(serverConfig, environmentName); err != nil {
            logging.Error("Failed to create MCP server for %s: %v", templateName, err)
            continue
        }
        
        logging.Success("Successfully processed template: %s", templateName)
    }
    
    return nil
}
```

## Template Examples

### Basic MCP Server Template

```json
{
  "description": "Filesystem operations with configurable root path",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest",
        "{{ .ROOT_PATH }}"
      ]
    }
  },
  "name": "filesystem-{{ .ENVIRONMENT }}"
}
```

### Advanced Multi-Server Template

```json
{
  "description": "Complete development environment with multiple tools",
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-filesystem@latest",
        "{{ .PROJECT_ROOT }}"
      ]
    },
    "git": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-git@latest",
        "{{ .PROJECT_ROOT }}"
      ]
    },
    "slack": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-slack@latest"
      ],
      "env": {
        "SLACK_BOT_TOKEN": "{{ .SLACK_TOKEN }}",
        "SLACK_CHANNEL": "{{ .DEFAULT_SLACK_CHANNEL }}"
      }
    },
    "database": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-postgres@latest",
        "{{ .DATABASE_URL }}"
      ]
    }
  },
  "name": "dev-environment-{{ .PROJECT_NAME }}"
}
```

### Variables Configuration Example

```yaml
# variables.yml
PROJECT_ROOT: "/home/user/projects/my-app"
PROJECT_NAME: "my-app"
ENVIRONMENT: "development"
SLACK_TOKEN: "xoxb-your-slack-token-here"
DEFAULT_SLACK_CHANNEL: "#dev-alerts"
DATABASE_URL: "postgres://user:password@localhost:5432/myapp_dev"
AWS_REGION: "us-east-1"
```

## CLI Integration

### Sync Command with Interactive Mode

```go
// cmd/main/handlers/mcp/sync.go
func handleSyncCommand(environmentName string, interactive bool) error {
    // Create template variable service with interactive mode
    templateVariableService := services.NewTemplateVariableService(
        getEnvironmentsPath(),
        interactive,
    )
    
    // Create sync service with template variable support
    syncService := services.NewDeclarativeSyncService(
        db,
        templateVariableService,
        getEnvironmentsPath(),
    )
    
    fmt.Printf("🔄 Syncing environment: %s\n", environmentName)
    
    if interactive {
        fmt.Println("📝 Interactive mode enabled - you will be prompted for missing variables")
    }
    
    err := syncService.SyncEnvironment(environmentName)
    if err != nil {
        return fmt.Errorf("sync failed: %w", err)
    }
    
    fmt.Printf("✅ Sync completed successfully for environment: %s\n", environmentName)
    return nil
}
```

### Command Line Usage

```bash
# Basic sync (interactive by default)
stn sync production

# Non-interactive sync (will fail if variables missing)
stn sync production --no-interactive

# Sync with explicit variable override
stn sync production --set ROOT_PATH=/opt/app --set API_KEY=secret
```

## Error Handling and Validation

### Template Validation

```go
type TemplateValidator struct {
    requiredFields []string
    validators     map[string]func(string) error
}

func (tv *TemplateValidator) ValidateTemplate(content string) []ValidationError {
    var errors []ValidationError
    
    // Parse template to check syntax
    tmpl, err := template.New("validation").Parse(content)
    if err != nil {
        errors = append(errors, ValidationError{
            Type:    "syntax_error",
            Message: fmt.Sprintf("Template syntax error: %v", err),
        })
        return errors
    }
    
    // Extract variables from template
    variables := tv.extractVariables(tmpl)
    
    // Validate variable names
    for _, varName := range variables {
        if !tv.isValidVariableName(varName) {
            errors = append(errors, ValidationError{
                Type:    "invalid_variable",
                Message: fmt.Sprintf("Invalid variable name: %s", varName),
                Field:   varName,
            })
        }
    }
    
    return errors
}

func (tv *TemplateValidator) ValidateVariables(variables map[string]string) []ValidationError {
    var errors []ValidationError
    
    for varName, value := range variables {
        if validator, exists := tv.validators[varName]; exists {
            if err := validator(value); err != nil {
                errors = append(errors, ValidationError{
                    Type:    "validation_error",
                    Message: fmt.Sprintf("Variable %s validation failed: %v", varName, err),
                    Field:   varName,
                })
            }
        }
    }
    
    return errors
}
```

### Variable Type Validation

```go
func setupVariableValidators() map[string]func(string) error {
    return map[string]func(string) error{
        "ROOT_PATH": func(value string) error {
            if !filepath.IsAbs(value) {
                return fmt.Errorf("must be an absolute path")
            }
            if _, err := os.Stat(value); os.IsNotExist(err) {
                return fmt.Errorf("path does not exist")
            }
            return nil
        },
        "DATABASE_URL": func(value string) error {
            if _, err := url.Parse(value); err != nil {
                return fmt.Errorf("must be a valid URL")
            }
            return nil
        },
        "AWS_REGION": func(value string) error {
            validRegions := []string{
                "us-east-1", "us-west-1", "us-west-2", "eu-west-1", 
                "eu-central-1", "ap-southeast-1", "ap-northeast-1",
            }
            for _, region := range validRegions {
                if value == region {
                    return nil
                }
            }
            return fmt.Errorf("must be a valid AWS region")
        },
        "API_KEY": func(value string) error {
            if len(value) < 10 {
                return fmt.Errorf("API key too short (minimum 10 characters)")
            }
            return nil
        },
    }
}
```

## UI Integration

### Variable Management Interface

```typescript
// ui/src/components/VariableManager.tsx
interface VariableManagerProps {
    environmentName: string;
    onSave: (variables: Record<string, string>) => void;
}

export function VariableManager({ environmentName, onSave }: VariableManagerProps) {
    const [variables, setVariables] = useState<Record<string, string>>({});
    const [loading, setLoading] = useState(true);
    const [validationErrors, setValidationErrors] = useState<Record<string, string>>({});
    
    useEffect(() => {
        loadVariables();
    }, [environmentName]);
    
    const loadVariables = async () => {
        try {
            const response = await fetch(`/api/v1/environments/${environmentName}/variables`);
            const data = await response.json();
            setVariables(data);
        } catch (error) {
            console.error('Failed to load variables:', error);
        } finally {
            setLoading(false);
        }
    };
    
    const validateVariable = (name: string, value: string) => {
        // Client-side validation
        const validators: Record<string, (value: string) => string | null> = {
            ROOT_PATH: (value) => {
                if (!value.startsWith('/')) {
                    return 'Must be an absolute path starting with /';
                }
                return null;
            },
            API_KEY: (value) => {
                if (value.length < 10) {
                    return 'API key must be at least 10 characters';
                }
                return null;
            },
        };
        
        const validator = validators[name];
        if (validator) {
            return validator(value);
        }
        
        return null;
    };
    
    const handleVariableChange = (name: string, value: string) => {
        setVariables({ ...variables, [name]: value });
        
        // Validate on change
        const error = validateVariable(name, value);
        setValidationErrors({
            ...validationErrors,
            [name]: error || ''
        });
    };
    
    const handleSave = async () => {
        // Check for validation errors
        const hasErrors = Object.values(validationErrors).some(error => error !== '');
        if (hasErrors) {
            alert('Please fix validation errors before saving');
            return;
        }
        
        try {
            const response = await fetch(`/api/v1/environments/${environmentName}/variables`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(variables)
            });
            
            if (response.ok) {
                onSave(variables);
                alert('Variables saved successfully');
            } else {
                alert('Failed to save variables');
            }
        } catch (error) {
            console.error('Failed to save variables:', error);
            alert('Failed to save variables');
        }
    };
    
    if (loading) {
        return <div className="text-tokyo-comment">Loading variables...</div>;
    }
    
    return (
        <div className="space-y-4">
            <div className="flex justify-between items-center">
                <h3 className="text-lg font-semibold text-tokyo-fg">
                    Environment Variables - {environmentName}
                </h3>
                <button
                    onClick={() => setVariables({ ...variables, '': '' })}
                    className="px-3 py-1 bg-tokyo-blue text-tokyo-bg rounded text-sm hover:bg-opacity-80"
                >
                    + Add Variable
                </button>
            </div>
            
            <div className="space-y-3">
                {Object.entries(variables).map(([name, value], index) => (
                    <div key={index} className="grid grid-cols-2 gap-3">
                        <div>
                            <input
                                type="text"
                                placeholder="VARIABLE_NAME"
                                value={name}
                                onChange={(e) => {
                                    const newVars = { ...variables };
                                    delete newVars[name];
                                    newVars[e.target.value] = value;
                                    setVariables(newVars);
                                }}
                                className="w-full p-2 bg-tokyo-bg-dark border border-tokyo-comment rounded text-tokyo-fg focus:border-tokyo-blue outline-none"
                            />
                        </div>
                        <div className="relative">
                            <input
                                type={name.includes('PASSWORD') || name.includes('TOKEN') || name.includes('KEY') ? 'password' : 'text'}
                                placeholder="value"
                                value={value}
                                onChange={(e) => handleVariableChange(name, e.target.value)}
                                className={`w-full p-2 bg-tokyo-bg-dark border rounded text-tokyo-fg focus:border-tokyo-blue outline-none ${
                                    validationErrors[name] ? 'border-tokyo-red' : 'border-tokyo-comment'
                                }`}
                            />
                            {validationErrors[name] && (
                                <div className="absolute -bottom-5 left-0 text-tokyo-red text-xs">
                                    {validationErrors[name]}
                                </div>
                            )}
                            <button
                                onClick={() => {
                                    const newVars = { ...variables };
                                    delete newVars[name];
                                    setVariables(newVars);
                                }}
                                className="absolute right-2 top-2 text-tokyo-red hover:bg-tokyo-red hover:bg-opacity-20 rounded p-1"
                            >
                                ×
                            </button>
                        </div>
                    </div>
                ))}
            </div>
            
            <div className="flex justify-end">
                <button
                    onClick={handleSave}
                    className="px-4 py-2 bg-tokyo-green text-tokyo-bg rounded hover:bg-opacity-80"
                >
                    Save Variables
                </button>
            </div>
        </div>
    );
}
```

## Performance Optimizations

### Variable Caching

```go
type VariableCache struct {
    cache map[string]CachedVariables
    mutex sync.RWMutex
    ttl   time.Duration
}

type CachedVariables struct {
    Variables map[string]string
    LoadTime  time.Time
}

func (vc *VariableCache) Get(environmentName string) (map[string]string, bool) {
    vc.mutex.RLock()
    defer vc.mutex.RUnlock()
    
    cached, exists := vc.cache[environmentName]
    if !exists {
        return nil, false
    }
    
    // Check if cache is still valid
    if time.Since(cached.LoadTime) > vc.ttl {
        return nil, false
    }
    
    return cached.Variables, true
}

func (vc *VariableCache) Set(environmentName string, variables map[string]string) {
    vc.mutex.Lock()
    defer vc.mutex.Unlock()
    
    vc.cache[environmentName] = CachedVariables{
        Variables: variables,
        LoadTime:  time.Now(),
    }
}
```

## Future Enhancements

### Planned Features

1. **Advanced Variable Types**:
   - Encrypted variables for secrets
   - Complex data types (arrays, objects)
   - Variable inheritance and overrides
   - Conditional variables

2. **Enhanced Validation**:
   - Custom validation rules
   - Cross-variable dependencies
   - External validation services
   - Variable schema definitions

3. **Integration Improvements**:
   - External secret management integration
   - Environment variable inheritance
   - Git-based variable management
   - Variable change tracking

4. **UI Enhancements**:
   - Visual template editor
   - Variable dependency visualization
   - Template preview with variables
   - Bulk variable operations

## Conclusion

The Template Variable System provides a robust foundation for dynamic configuration management in Station. By supporting interactive prompting, comprehensive validation, and seamless integration with MCP configuration sync, it enables flexible and reliable deployment across different environments while maintaining security and usability.
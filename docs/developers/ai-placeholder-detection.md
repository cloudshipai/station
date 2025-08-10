# AI-Powered Placeholder Detection

## Overview

Station's load command features intelligent AI-powered placeholder detection that automatically identifies and generates smart forms for configuration templates. This system uses Genkit + OpenAI to understand context and create user-friendly configuration experiences.

## Quick Start

### Basic Usage

```bash
# Traditional load (unchanged)
stn load config.json

# AI-powered template detection
stn load config.json --detect

# Interactive editor mode with AI detection
stn load -e
```

### Example Configuration

```json
{
    "mcpServers": {
        "SQLite Server": {
            "command": "npx",
            "args": ["-y", "mcp-sqlite", "<path-to-your-sqlite-database.db>"]
        },
        "AWS CLI": {
            "command": "npx",
            "args": ["-y", "@aws-mcp/cli"],
            "env": {
                "AWS_ACCESS_KEY_ID": "YOUR_AWS_ACCESS_KEY_ID",
                "AWS_SECRET_ACCESS_KEY": "YOUR_AWS_SECRET_ACCESS_KEY",
                "AWS_DEFAULT_REGION": "us-east-1"
            }
        },
        "GitHub MCP": {
            "env": {
                "GITHUB_TOKEN": "[GITHUB_TOKEN]",
                "GITHUB_API_URL": "https://your-github-enterprise.com/api/v3"
            }
        }
    }
}
```

## How It Works

### 1. Placeholder Detection Patterns

The AI system recognizes multiple placeholder formats:

| Pattern | Example | Generated Field Type |
|---------|---------|---------------------|
| `<description>` | `<path-to-sqlite-database.db>` | File path picker |
| `YOUR_VARIABLE` | `YOUR_API_KEY` | Sensitive text field |
| `[TOKEN]` | `[GITHUB_TOKEN]` | Password field |
| `your-name` | `your-username` | Text field |
| `/path/to/your/file` | `/path/to/your/config.json` | File path field |

### 2. AI Analysis Process

1. **Pattern Detection**: Scans configuration for various placeholder formats
2. **Context Analysis**: Uses OpenAI to understand semantic meaning
3. **Field Generation**: Creates appropriate form fields with validation
4. **Template Replacement**: Converts placeholders to `{{VARIABLE}}` format

### 3. Smart Field Types

The AI generates context-aware fields:

- **Paths**: File picker with validation
- **API Keys**: Masked input, marked sensitive, encrypted storage
- **URLs**: URL format validation
- **Database Connections**: Smart connection string handling
- **Regions**: Predefined options with smart defaults

## Load Command Modes

### Traditional Mode (Default)
```bash
stn load [file|url]
```
- No AI processing
- Uses existing template system with `{{placeholder}}` format
- Backward compatible with all existing configurations

### AI Detection Mode
```bash
stn load config.json --detect
```
- Enables AI-powered placeholder detection
- Requires `OPENAI_API_KEY` environment variable
- Gracefully falls back to regex detection if AI unavailable

### Editor Mode
```bash
stn load -e
```
- Opens default editor (`$VISUAL`, `$EDITOR`, or fallback to nano/vim)
- Provides helpful template with examples
- Always enables AI detection for pasted content
- Auto-removes instruction comments

### GitHub Discovery Mode (Unchanged)
```bash
stn load https://github.com/user/mcp-repo
```
- Extracts configurations from README files
- Launches TurboTax-style wizard
- No changes to existing behavior

## Architecture

### Core Components

```
cmd/main/handlers/load_handlers.go
├── RunLoad()                    # Main load orchestration
├── detectTemplates()            # AI-powered detection
├── handleEditorMode()           # Editor functionality
├── initializeAI()              # Genkit + OpenAI setup
└── replaceDetectedPlaceholders() # Template conversion

internal/services/intelligent_placeholder_analyzer.go
├── PlaceholderAnalyzer         # Main AI service
├── AnalyzeConfiguration()      # Configuration analysis
├── detectAllPlaceholderPatterns() # Pattern detection
└── buildAnalysisPrompt()       # AI prompt generation
```

### Integration Points

The system integrates seamlessly with existing Station components:

- **Template System**: Converts to existing `{{placeholder}}` format
- **Form Generation**: Uses existing TUI form infrastructure  
- **Credential Storage**: Leverages existing encryption and key management
- **Configuration Upload**: Uses standard upload pipelines

## Configuration Examples

### Database Servers

**Input Template:**
```json
{
    "mcpServers": {
        "Database": {
            "command": "node",
            "args": ["server.js"],
            "env": {
                "DB_HOST": "your-database-host",
                "DB_USER": "your-username", 
                "DB_PASSWORD": "your-secure-password",
                "DB_NAME": "your-database-name"
            }
        }
    }
}
```

**AI-Generated Fields:**
- `DB_HOST`: Text field with hostname validation
- `DB_USER`: Text field for database username
- `DB_PASSWORD`: Masked password field, marked sensitive
- `DB_NAME`: Text field for database name

### File-Based Servers

**Input Template:**
```json
{
    "mcpServers": {
        "Local Server": {
            "command": "python",
            "args": ["/path/to/your/script.py", "<config-file-path>"],
            "env": {
                "DATA_DIR": "/path/to/your/data",
                "LOG_FILE": "/path/to/your/logfile.log"
            }
        }
    }
}
```

**AI-Generated Fields:**
- `SCRIPT_PATH`: File picker for Python scripts
- `CONFIG_FILE_PATH`: File picker for configuration files
- `DATA_DIR`: Directory picker
- `LOG_FILE`: File path for log output

## Developer Guide

### Prerequisites

```bash
# Required environment variable
export OPENAI_API_KEY="your-openai-api-key"
```

### Adding Custom Placeholder Patterns

To extend placeholder detection, modify `detectAllPlaceholderPatterns()`:

```go
patterns := []*regexp.Regexp{
    // Existing patterns...
    
    // Add new pattern
    regexp.MustCompile(`your-custom-pattern`),
}
```

### Customizing AI Analysis

The AI analysis prompt can be customized in `buildAnalysisPrompt()`:

```go
func (pa *PlaceholderAnalyzer) buildAnalysisPrompt(configJSON string, placeholders []string) string {
    return fmt.Sprintf(`You are an expert at analyzing MCP server configurations.
    
    // Add custom instructions here
    
    Configuration: %s
    Placeholders: %s`, configJSON, strings.Join(placeholders, "\n"))
}
```

### Field Type Mapping

The system maps placeholder analysis to form field types:

```go
type PlaceholderAnalysis struct {
    Type        string `json:"type"`        // path, api_key, url, string, password
    Description string `json:"description"` // User-friendly description
    Required    bool   `json:"required"`    // Is field required
    Sensitive   bool   `json:"sensitive"`   // Should be encrypted
    Default     string `json:"default"`     // Default value
    Help        string `json:"help"`        // Help text
    Validation  map[string]string `json:"validation"` // Validation rules
}
```

## Testing

### Manual Testing

1. **Editor Mode**:
   ```bash
   stn load -e
   # Paste configuration with various placeholder formats
   # Verify AI detection and form generation
   ```

2. **Detect Mode**:
   ```bash
   # Create test config with placeholders
   echo '{"mcpServers":{"test":{"args":["<test-file>"]}}}' > test-config.json
   stn load test-config.json --detect
   ```

3. **Fallback Testing**:
   ```bash
   # Test without OPENAI_API_KEY
   unset OPENAI_API_KEY
   stn load test-config.json --detect
   # Should fall back to regex detection
   ```

### Integration Testing

The system includes comprehensive test coverage:

- Pattern detection accuracy
- AI analysis quality
- Form generation correctness
- Template replacement reliability
- Fallback behavior validation

## Performance Considerations

### AI Service Initialization
- AI services are initialized only when needed (`--detect` or `-e` flags)
- OpenAI API calls are cached for repeated analysis
- Graceful degradation when AI services are unavailable

### Memory Usage
- Large configurations are processed in chunks
- Template analysis is performed in-memory
- No persistent AI model storage required

## Security

### Credential Handling
- All sensitive fields are automatically encrypted
- API keys are never logged or exposed
- OpenAI API calls don't include sensitive data

### Input Validation
- JSON validation before processing
- Placeholder pattern sanitization
- Safe template replacement without code execution

## Troubleshooting

### Common Issues

**AI Detection Not Working**
```bash
# Check environment variable
echo $OPENAI_API_KEY

# Verify API key permissions
curl -H "Authorization: Bearer $OPENAI_API_KEY" \
     https://api.openai.com/v1/models
```

**Editor Not Opening**
```bash
# Check editor configuration
echo $VISUAL $EDITOR

# Test editor availability
which code nano vim vi
```

**Placeholder Not Detected**
- Check if pattern matches supported formats
- Use `--detect` flag explicitly
- Verify configuration is valid JSON

### Debug Mode

Enable debug logging:
```bash
export STATION_DEBUG=true
stn load config.json --detect
```

## Migration Guide

### From Manual Templates

**Before** (manual template definition):
```json
{
    "mcpServers": {"server": {"env": {"API_KEY": "{{API_KEY}}"}}},
    "templates": {
        "API_KEY": {
            "description": "Your API key",
            "type": "password",
            "required": true,
            "sensitive": true
        }
    }
}
```

**After** (AI detection):
```json
{
    "mcpServers": {"server": {"env": {"API_KEY": "YOUR_API_KEY"}}}
}
```
The AI automatically generates the template definition.

### Backward Compatibility

- All existing configurations continue to work unchanged
- Manual template definitions take precedence over AI detection
- Mixed configurations (manual + AI) are supported

## Future Enhancements

### Planned Features

1. **Custom AI Models**: Support for local/private AI models
2. **Learning System**: Improve detection based on user feedback
3. **Configuration Validation**: Pre-deployment configuration testing
4. **Smart Defaults**: Environment-specific default value suggestions
5. **Batch Processing**: Multi-configuration template processing

### Contributing

To contribute to the AI placeholder detection system:

1. Review the architecture in `internal/services/intelligent_placeholder_analyzer.go`
2. Add test cases for new placeholder patterns
3. Update documentation for new field types
4. Submit PRs with comprehensive test coverage

## Examples Repository

Additional examples and templates are available in:
- `/test-mcp-servers/example-intelligent-config.json`
- Station's MCP server collection with AI-ready templates
- Community-contributed configuration templates

---

*This feature represents Station's commitment to making MCP server configuration as intuitive and user-friendly as possible, leveraging AI to bridge the gap between complex configuration requirements and user experience.*
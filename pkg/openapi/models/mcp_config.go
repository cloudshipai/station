package models

// MCPConfig represents the top-level MCP server configuration
type MCPConfig struct {
	ToolSet *ToolSetConfig `yaml:"toolSet,omitempty"`
	Server  ServerConfig   `yaml:"server,omitempty"`
	Tools   []Tool         `yaml:"tools,omitempty"`
}

// ToolSetConfig defines the configuration for a toolset.
type ToolSetConfig struct {
	Name        string             `json:"name"`
	ServerTools []ServerToolConfig `json:"serverTools"`
}

// ServerToolConfig specifies which tools from a server to include in a toolset.
type ServerToolConfig struct {
	ServerName string   `json:"serverName"`
	Tools      []string `json:"tools"`
}

// ServerConfig represents the MCP server configuration
type ServerConfig struct {
	Name            string           `yaml:"name"`
	Config          map[string]any   `yaml:"config,omitempty"`
	AllowTools      []string         `yaml:"allowTools,omitempty"`
	SecuritySchemes []SecurityScheme `yaml:"securitySchemes,omitempty"`
}

// SecurityScheme defines a security scheme that can be used by the tools.
type SecurityScheme struct {
	ID                string `yaml:"id"`
	Type              string `yaml:"type"`             // e.g., "http", "apiKey", "oauth2", "openIdConnect"
	Scheme            string `yaml:"scheme,omitempty"` // e.g., "basic", "bearer" for "http" type
	In                string `yaml:"in,omitempty"`     // e.g., "header", "query", "cookie" for "apiKey" type
	Name              string `yaml:"name,omitempty"`   // Name of the header, query parameter or cookie for "apiKey" type
	DefaultCredential string `yaml:"defaultCredential,omitempty"`
}

// Tool represents an MCP tool configuration
type Tool struct {
	Name                  string                   `yaml:"name"`
	Description           string                   `yaml:"description"`
	Args                  []Arg                    `yaml:"args"`
	RequestTemplate       RequestTemplate          `yaml:"requestTemplate"`
	ResponseTemplate      ResponseTemplate         `yaml:"responseTemplate"`
	ErrorResponseTemplate *string                  `yaml:"errorResponseTemplate,omitempty"`
	Security              *ToolSecurityRequirement `yaml:"security,omitempty"`
	OutputSchema          map[string]any           `yaml:"outputSchema,omitempty"` // NEW: Output schema for MCP 2025-06-18
}

// Arg represents an MCP tool argument
type Arg struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Type        string         `yaml:"type,omitempty"`
	Required    bool           `yaml:"required,omitempty"`
	Default     any            `yaml:"default,omitempty"`
	Enum        []any          `yaml:"enum,omitempty"`
	Items       map[string]any `yaml:"items,omitempty"`
	Properties  map[string]any `yaml:"properties,omitempty"`
	Position    string         `yaml:"position,omitempty"`
}

// RequestTemplate represents the MCP request template
type RequestTemplate struct {
	URL            string                   `yaml:"url"`
	Method         string                   `yaml:"method"`
	Headers        []Header                 `yaml:"headers,omitempty"`
	Body           string                   `yaml:"body,omitempty"`
	ArgsToJsonBody bool                     `yaml:"argsToJsonBody,omitempty"`
	ArgsToUrlParam bool                     `yaml:"argsToUrlParam,omitempty"`
	ArgsToFormBody bool                     `yaml:"argsToFormBody,omitempty"`
	Security       *ToolSecurityRequirement `yaml:"security,omitempty"`
}

// ToolSecurityRequirement specifies a security scheme requirement for a tool.
type ToolSecurityRequirement struct {
	ID          string `yaml:"id"`                    // References a SecurityScheme ID defined in ServerConfig.SecuritySchemes
	Passthrough bool   `yaml:"passthrough,omitempty"` // Whether to pass through the security credentials
}

// Header represents an HTTP header
type Header struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

// ResponseTemplate represents the MCP response template
type ResponseTemplate struct {
	Body        string `yaml:"body,omitempty"`
	PrependBody string `yaml:"prependBody,omitempty"`
	AppendBody  string `yaml:"appendBody,omitempty"`
}

// ConvertOptions represents options for the conversion process
type ConvertOptions struct {
	ServerName     string
	ServerConfig   map[string]interface{}
	ToolNamePrefix string
	TemplatePath   string
}

// ToolTemplate represents a template for applying to all tools
type ToolTemplate struct {
	RequestTemplate  *RequestTemplate         `yaml:"requestTemplate,omitempty"`
	ResponseTemplate *ResponseTemplate        `yaml:"responseTemplate,omitempty"`
	Security         *ToolSecurityRequirement `yaml:"security,omitempty"`
	OutputSchema     map[string]any           `yaml:"outputSchema,omitempty"` // NEW: Output schema template
}

// MCPConfigTemplate represents a template for patching the generated config
type MCPConfigTemplate struct {
	Server ServerConfig `yaml:"server"`
	Tools  ToolTemplate `yaml:"tools,omitempty"`
}

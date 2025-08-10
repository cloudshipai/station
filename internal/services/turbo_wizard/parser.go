package turbo_wizard

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ConfigParser handles parsing of MCP server configurations
type ConfigParser struct{}

// NewConfigParser creates a new parser instance
func NewConfigParser() *ConfigParser {
	return &ConfigParser{}
}

// ParseBlockToConfig parses different MCP server block formats
func (p *ConfigParser) ParseBlockToConfig(block MCPServerBlock) *ServerConfig {
	config := &ServerConfig{
		Name:        block.ServerName,
		Description: block.Description,
		RawBlock:    block.RawBlock,
		Env:         make(map[string]string),
		RequiredEnv: []EnvironmentVariable{},
	}

	// Try to parse the JSON block to extract configuration
	var parsedBlock map[string]interface{}
	if err := json.Unmarshal([]byte(block.RawBlock), &parsedBlock); err != nil {
		// If JSON parsing fails, try to extract info from text
		return p.parseTextBlock(block, config)
	}

	// Determine transport type and parse accordingly
	if command, hasCommand := parsedBlock["command"].(string); hasCommand {
		if strings.HasPrefix(command, "docker") {
			config.Transport = TransportDocker
			config = p.parseDockerConfig(parsedBlock, config)
		} else {
			config.Transport = TransportSTDIO
			config.Command = command
		}
	} else if url, hasURL := parsedBlock["url"].(string); hasURL {
		if strings.Contains(url, "/sse") {
			config.Transport = TransportSSE
		} else {
			config.Transport = TransportHTTP
		}
		config.URL = url
	}

	// Parse common fields
	if args, ok := parsedBlock["args"].([]interface{}); ok {
		config.Args = make([]string, len(args))
		for i, arg := range args {
			if argStr, ok := arg.(string); ok {
				config.Args[i] = argStr
			}
		}
	}

	if env, ok := parsedBlock["env"].(map[string]interface{}); ok {
		for k, v := range env {
			if vStr, ok := v.(string); ok {
				config.Env[k] = vStr
				// Detect if this looks like an API key or sensitive value
				envVar := EnvironmentVariable{
					Name:     k,
					Value:    vStr,
					Required: true,
				}
				if p.isAPIKeyField(k, vStr) {
					envVar.Type = "api_key"
					envVar.Description = fmt.Sprintf("API key for %s", k)
				} else if p.isPathField(k, vStr) {
					envVar.Type = "path"
					envVar.Description = fmt.Sprintf("File path for %s", k)
				} else {
					envVar.Type = "string"
					envVar.Description = fmt.Sprintf("Configuration value for %s", k)
				}
				config.RequiredEnv = append(config.RequiredEnv, envVar)
			}
		}
	}

	return config
}

// parseDockerConfig handles Docker-specific parsing
func (p *ConfigParser) parseDockerConfig(parsedBlock map[string]interface{}, config *ServerConfig) *ServerConfig {
	if args, ok := parsedBlock["args"].([]interface{}); ok {
		dockerArgs := make([]string, 0)
		mounts := make([]DockerMount, 0)

		for i, arg := range args {
			if argStr, ok := arg.(string); ok {
				// Parse Docker mount arguments
				if argStr == "--mount" && i+1 < len(args) {
					if mountStr, ok := args[i+1].(string); ok {
						mount := p.parseDockerMount(mountStr)
						if mount != nil {
							mounts = append(mounts, *mount)
						}
					}
				} else if !strings.HasPrefix(argStr, "--mount") {
					dockerArgs = append(dockerArgs, argStr)
				}
			}
		}

		config.Args = dockerArgs
		config.DockerMounts = mounts
	}

	return config
}

// parseDockerMount parses Docker mount string format
func (p *ConfigParser) parseDockerMount(mountStr string) *DockerMount {
	// Parse format: "type=bind,src=/path,dst=/path,ro"
	mount := &DockerMount{}
	parts := strings.Split(mountStr, ",")

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		key, value := kv[0], kv[1]
		switch key {
		case "type":
			mount.Type = value
		case "src", "source":
			mount.Source = value
		case "dst", "destination", "target":
			mount.Target = value
		case "ro", "readonly":
			mount.ReadOnly = true
		}
	}

	if mount.Source != "" && mount.Target != "" {
		return mount
	}
	return nil
}

// parseTextBlock handles non-JSON text blocks
func (p *ConfigParser) parseTextBlock(block MCPServerBlock, config *ServerConfig) *ServerConfig {
	text := block.RawBlock

	// Try to detect common patterns
	if strings.Contains(text, "docker run") {
		config.Transport = TransportDocker
		// Extract docker command
		if dockerMatch := regexp.MustCompile(`docker\s+run\s+(.+)`).FindStringSubmatch(text); len(dockerMatch) > 1 {
			config.Command = "docker"
			args := strings.Fields(dockerMatch[1])
			config.Args = args
		}
	} else if strings.Contains(text, "http://") || strings.Contains(text, "https://") {
		config.Transport = TransportHTTP
		// Extract URL
		if urlMatch := regexp.MustCompile(`https?://[^\s]+`).FindString(text); urlMatch != "" {
			config.URL = urlMatch
		}
	} else {
		config.Transport = TransportSTDIO
		// Try to extract command from common patterns
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "python") || strings.HasPrefix(line, "node") || strings.HasPrefix(line, "npx") {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					config.Command = parts[0]
					if len(parts) > 1 {
						config.Args = parts[1:]
					}
				}
				break
			}
		}
	}

	return config
}

// Helper functions for detecting field types
func (p *ConfigParser) isAPIKeyField(key, value string) bool {
	keyLower := strings.ToLower(key)
	return strings.Contains(keyLower, "api") || strings.Contains(keyLower, "key") ||
		strings.Contains(keyLower, "secret") || strings.Contains(keyLower, "token") ||
		(value != "" && len(value) > 20 && regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(value))
}

func (p *ConfigParser) isPathField(key, value string) bool {
	keyLower := strings.ToLower(key)
	return strings.Contains(keyLower, "path") || strings.Contains(keyLower, "dir") ||
		strings.Contains(keyLower, "file") || strings.HasPrefix(value, "/") ||
		strings.Contains(value, "\\")
}

// GetTransportDescription returns a human-readable description of the transport type
func (p *ConfigParser) GetTransportDescription(transport MCPTransportType) string {
	switch transport {
	case TransportSTDIO:
		return "STDIO - Direct process communication (python, node, etc.)"
	case TransportDocker:
		return "Docker - Containerized MCP server with isolated environment"
	case TransportHTTP:
		return "HTTP - Web-based MCP server with REST API"
	case TransportSSE:
		return "SSE - Server-Sent Events for real-time communication"
	default:
		return "Unknown transport type"
	}
}
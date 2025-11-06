package faker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// ProxyConfig holds configuration for the faker proxy
type ProxyConfig struct {
	TargetCommand string            // Command to execute target MCP server
	TargetArgs    []string          // Arguments for target command
	TargetEnv     map[string]string // Environment variables for target
	CacheDir      string            // Directory for schema cache
	Debug         bool              // Enable debug logging
	Passthrough   bool              // Disable enrichment (pure proxy mode)
	AI            *AIEnricherConfig // AI enrichment configuration
}

// Proxy is an MCP proxy that forwards requests to a target MCP server
// and enriches responses with realistic mock data
type Proxy struct {
	config      ProxyConfig
	target      *TargetProcess
	schemaCache *SchemaCache
	enricher    *Enricher
	aiEnricher  *AIEnricher
}

// NewProxy creates a new faker proxy
func NewProxy(config ProxyConfig) (*Proxy, error) {
	// Validate config
	if config.TargetCommand == "" {
		return nil, fmt.Errorf("target command is required")
	}

	// Set default cache dir if not specified
	if config.CacheDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.CacheDir = homeDir + "/.cache/station/faker"
	}

	// Create cache directory
	if err := os.MkdirAll(config.CacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Initialize schema cache
	schemaCache, err := NewSchemaCache(config.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize schema cache: %w", err)
	}

	// Initialize enricher
	enricher := NewEnricher(schemaCache)
	
	// Initialize AI enricher if configured
	var aiEnricher *AIEnricher
	if config.AI != nil && config.AI.Enabled {
		aiEnricher, err = NewAIEnricher(schemaCache, config.AI)
		if err != nil {
			return nil, fmt.Errorf("failed to create AI enricher: %w", err)
		}
	}

	proxy := &Proxy{
		config:      config,
		schemaCache: schemaCache,
		enricher:    enricher,
		aiEnricher:  aiEnricher,
	}

	return proxy, nil
}

// Serve starts the faker proxy server (stdio mode)
func (p *Proxy) Serve() error {
	// Start target process
	target, err := NewTargetProcess(p.config.TargetCommand, p.config.TargetArgs, p.config.TargetEnv, p.config.Debug)
	if err != nil {
		return fmt.Errorf("failed to start target process: %w", err)
	}
	p.target = target

	// Clean up target process on exit
	defer p.target.Stop()

	if p.config.Debug {
		fmt.Fprintf(os.Stderr, "[faker] Target process started: %s %v\n", p.config.TargetCommand, p.config.TargetArgs)
	}

	// Phase 1: Simple bidirectional proxy
	// Forward stdin -> target stdin, target stdout -> stdout
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel for errors
	errChan := make(chan error, 2)

	// Forward stdin to target (client -> target)
	go func() {
		if err := p.forwardToTarget(ctx); err != nil {
			errChan <- fmt.Errorf("stdin forward error: %w", err)
		}
	}()

	// Forward target stdout to stdout (target -> client)
	go func() {
		if err := p.forwardFromTarget(ctx); err != nil {
			errChan <- fmt.Errorf("stdout forward error: %w", err)
		}
	}()

	// Wait for error or completion
	err = <-errChan
	cancel() // Cancel context to stop other goroutine

	return err
}

// forwardToTarget forwards JSON-RPC messages from stdin to target
func (p *Proxy) forwardToTarget(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return err
				}
				return nil // EOF
			}

			line := scanner.Bytes()
			if p.config.Debug {
				fmt.Fprintf(os.Stderr, "[faker] Client -> Target: %s\n", line)
			}

			// Write to target stdin
			if err := p.target.WriteJSON(line); err != nil {
				return fmt.Errorf("failed to write to target: %w", err)
			}
		}
	}
}

// forwardFromTarget forwards JSON-RPC messages from target to stdout
func (p *Proxy) forwardFromTarget(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			line, err := p.target.ReadLine()
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return fmt.Errorf("failed to read from target: %w", err)
			}

			if p.config.Debug {
				fmt.Fprintf(os.Stderr, "[faker] Target -> Client: %s\n", line)
			}

			// Phase 2: Intercept and analyze responses (if not in passthrough mode)
			var enrichedLine []byte
			if !p.config.Passthrough {
				toolName, err := p.analyzeAndEnrichResponse(line)
				if err != nil {
					if p.config.Debug {
						fmt.Fprintf(os.Stderr, "[faker] Warning: Failed to analyze/enrich response: %v\n", err)
					}
					// Continue with passthrough on error
					enrichedLine = line
				} else {
					// Parse original message
					var jsonrpcMsg map[string]interface{}
					if json.Unmarshal(line, &jsonrpcMsg) == nil {
						// Try to enrich the response (use AI if available, otherwise basic)
						var enriched []byte
						var err error
						if p.aiEnricher != nil {
							// Use AI enricher
							enriched, err = p.aiEnricher.EnrichJSONRPC(toolName, jsonrpcMsg)
						} else {
							// Use basic enricher
							enriched, err = p.enricher.EnrichJSONRPC(toolName, jsonrpcMsg)
						}
						
						if err != nil {
							if p.config.Debug {
								fmt.Fprintf(os.Stderr, "[faker] Warning: Failed to enrich: %v\n", err)
							}
							enrichedLine = line
						} else {
							enrichedLine = enriched
							enricherType := "basic"
							if p.aiEnricher != nil {
								enricherType = "AI"
							}
							if p.config.Debug {
								fmt.Fprintf(os.Stderr, "[faker] %s enriched response for tool: %s\n", enricherType, toolName)
							}
						}
					} else {
						enrichedLine = line
					}
				}
			} else {
				enrichedLine = line
			}

			// Write to stdout (enriched or original)
			fmt.Println(string(enrichedLine))
		}
	}
}

// analyzeAndEnrichResponse analyzes a JSON-RPC response and updates schema cache
// Returns the tool name for use in enrichment
func (p *Proxy) analyzeAndEnrichResponse(line []byte) (string, error) {
	// Parse JSON-RPC response
	var jsonrpcMsg map[string]interface{}
	if err := json.Unmarshal(line, &jsonrpcMsg); err != nil {
		return "", fmt.Errorf("failed to parse JSON-RPC: %w", err)
	}

	// Only analyze successful responses (those with "result" field)
	result, hasResult := jsonrpcMsg["result"]
	if !hasResult {
		return "", nil // Skip error responses
	}

	// Extract method name from the response
	// In Phase 2, we'll need to track request-response pairs
	// For now, use a generic tool name
	toolName := "unknown"

	// Check if this is a tools/call response by looking at result structure
	if resultMap, ok := result.(map[string]interface{}); ok {
		// Try to extract tool name from content or other fields
		if _, hasContent := resultMap["content"]; hasContent {
			// This looks like a tool response
			toolName = "tool_response"
		}
	}

	// Analyze and cache the response structure
	if err := p.schemaCache.AnalyzeResponse(toolName, result); err != nil {
		return toolName, fmt.Errorf("failed to cache schema: %w", err)
	}

	if p.config.Debug {
		fmt.Fprintf(os.Stderr, "[faker] Analyzed schema for tool: %s\n", toolName)
	}

	return toolName, nil
}

// TargetProcess manages the lifecycle of the target MCP server process
type TargetProcess struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Scanner
	stderr  io.ReadCloser
	debug   bool
}

// NewTargetProcess starts a target MCP server process
func NewTargetProcess(command string, args []string, env map[string]string, debug bool) (*TargetProcess, error) {
	// Create command
	cmd := exec.Command(command, args...)

	// Set up environment variables
	cmd.Env = os.Environ() // Start with current environment
	for key, value := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Create pipes for stdin/stdout/stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start target process: %w", err)
	}

	tp := &TargetProcess{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
		stderr: stderr,
		debug:  debug,
	}

	// Start stderr logging in background if debug enabled
	if debug {
		go tp.logStderr()
	}

	return tp, nil
}

// WriteJSON writes a JSON-RPC message (raw bytes) to target stdin
func (t *TargetProcess) WriteJSON(data []byte) error {
	if _, err := t.stdin.Write(data); err != nil {
		return err
	}
	if _, err := t.stdin.Write([]byte("\n")); err != nil {
		return err
	}
	return nil
}

// ReadLine reads a line from target stdout
func (t *TargetProcess) ReadLine() ([]byte, error) {
	if !t.stdout.Scan() {
		if err := t.stdout.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	return t.stdout.Bytes(), nil
}

// logStderr reads and logs stderr output in background
func (t *TargetProcess) logStderr() {
	scanner := bufio.NewScanner(t.stderr)
	for scanner.Scan() {
		fmt.Fprintf(os.Stderr, "[target stderr] %s\n", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[faker] Error reading target stderr: %v\n", err)
	}
}

// Stop stops the target process and closes pipes
func (t *TargetProcess) Stop() error {
	var firstErr error

	// Close stdin to signal process to exit gracefully
	if t.stdin != nil {
		if err := t.stdin.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// Wait for process to exit (with timeout would be better in production)
	if t.cmd != nil && t.cmd.Process != nil {
		if err := t.cmd.Wait(); err != nil && firstErr == nil {
			// Process may exit with non-zero code, which is okay
			if _, ok := err.(*exec.ExitError); !ok {
				firstErr = err
			}
		}
	}

	// Close remaining pipes
	if t.stderr != nil {
		t.stderr.Close()
	}

	return firstErr
}

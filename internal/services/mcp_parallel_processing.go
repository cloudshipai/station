package services

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"station/internal/db/repositories"
	"station/internal/logging"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/plugins/mcp"
)

// processFileConfigsParallel processes multiple file configs in parallel for faster tool discovery
func (mcm *MCPConnectionManager) processFileConfigsParallel(ctx context.Context, fileConfigs []*repositories.FileConfigRecord) ([]ai.Tool, []*mcp.GenkitMCPClient) {
	if len(fileConfigs) == 0 {
		return nil, nil
	}
	
	// Create worker pool - conservative limit for file config processing
	maxWorkers := getEnvIntOrDefault("STATION_MCP_CONFIG_WORKERS", 2) // Default: 2 workers
	if len(fileConfigs) < maxWorkers {
		maxWorkers = len(fileConfigs)
	}
	
	// Channel to send file configs to workers
	configChan := make(chan *repositories.FileConfigRecord, len(fileConfigs))
	
	// Channel to collect results
	type configResult struct {
		configName string
		tools      []ai.Tool
		clients    []*mcp.GenkitMCPClient
	}
	resultChan := make(chan configResult, len(fileConfigs))
	
	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for fileConfig := range configChan {
				logging.Debug("Worker %d processing file config: %s", workerID, fileConfig.ConfigName)
				tools, clients := mcm.processFileConfig(ctx, fileConfig)
				resultChan <- configResult{
					configName: fileConfig.ConfigName,
					tools:      tools,
					clients:    clients,
				}
			}
		}(i)
	}
	
	// Send all file configs to workers
	for _, fileConfig := range fileConfigs {
		configChan <- fileConfig
	}
	close(configChan)
	
	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results
	var allTools []ai.Tool
	var allClients []*mcp.GenkitMCPClient
	successCount := 0
	
	for result := range resultChan {
		fcResultMsg := fmt.Sprintf("File config %s returned %d tools and %d clients", result.configName, len(result.tools), len(result.clients))
		logging.Info("MCPCONNMGR: %s", fcResultMsg)
		debugLogToFile("MCPCONNMGR processFileConfigsParallel: " + fcResultMsg)
		
		if result.tools != nil {
			allTools = append(allTools, result.tools...)
			successCount++
		}
		if result.clients != nil {
			allClients = append(allClients, result.clients...)
		}
	}
	
	logging.Info("Parallel file config processing completed: %d configs, %d successful, %d total tools", 
		len(fileConfigs), successCount, len(allTools))
	
	return allTools, allClients
}

// processServersParallel processes multiple servers in parallel for faster connection setup
func (mcm *MCPConnectionManager) processServersParallel(ctx context.Context, serversData map[string]interface{}) ([]ai.Tool, []*mcp.GenkitMCPClient) {
	if len(serversData) == 0 {
		return nil, nil
	}
	
	// Create worker pool - limit concurrent connections to avoid overwhelming system
	maxWorkers := getEnvIntOrDefault("STATION_MCP_SERVER_WORKERS", 3) // Default: 3 workers
	if len(serversData) < maxWorkers {
		maxWorkers = len(serversData)
	}
	
	// Channel to send server configs to workers
	type serverJob struct {
		name   string
		config interface{}
	}
	jobChan := make(chan serverJob, len(serversData))
	
	// Channel to collect results
	type serverResult struct {
		name    string
		tools   []ai.Tool
		client  *mcp.GenkitMCPClient
	}
	resultChan := make(chan serverResult, len(serversData))
	
	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobChan {
				logging.Debug("Worker %d connecting to MCP server: %s", workerID, job.name)
				tools, client := mcm.connectToMCPServer(ctx, job.name, job.config)
				resultChan <- serverResult{
					name:   job.name,
					tools:  tools,
					client: client,
				}
			}
		}(i)
	}
	
	// Send all server jobs to workers
	for serverName, serverConfig := range serversData {
		jobChan <- serverJob{name: serverName, config: serverConfig}
	}
	close(jobChan)
	
	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results
	var allTools []ai.Tool
	var allClients []*mcp.GenkitMCPClient
	successCount := 0
	
	for result := range resultChan {
		if result.tools != nil {
			allTools = append(allTools, result.tools...)
			successCount++
		}
		if result.client != nil {
			allClients = append(allClients, result.client)
		}
	}
	
	logging.Info("Parallel server processing completed: %d servers, %d successful connections, %d total tools", 
		len(serversData), successCount, len(allTools))
	
	return allTools, allClients
}

// debugLogToFile writes debug messages to a file for investigation
func debugLogToFile(message string) {
	// Use user's home directory for cross-platform compatibility
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return // Silently fail if can't get home dir
	}
	logFile := fmt.Sprintf("%s/.config/station/debug-mcp-sync.log", homeDir)
	
	// Ensure directory exists
	logDir := fmt.Sprintf("%s/.config/station", homeDir)
	os.MkdirAll(logDir, 0755)
	
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return // Silently fail if can't write
	}
	defer f.Close()
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, message))
}
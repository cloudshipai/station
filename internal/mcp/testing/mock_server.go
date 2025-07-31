package testing

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MockMCPServer creates a mock MCP server for testing
type MockMCPServer struct {
	server *server.MCPServer
	tools  []mcp.Tool
}

// NewMockFileSystemServer creates a mock filesystem server
func NewMockFileSystemServer() *MockMCPServer {
	mcpServer := server.NewMCPServer(
		"Mock FileSystem Server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)
	
	// Create filesystem tools
	readFileTool := mcp.NewTool("read_file",
		mcp.WithDescription("Read contents of a file"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the file to read"),
		),
	)
	
	writeFileTool := mcp.NewTool("write_file",
		mcp.WithDescription("Write contents to a file"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("Path to the file to write"),
		),
		mcp.WithString("content",
			mcp.Required(),
			mcp.Description("Content to write to the file"),
		),
	)
	
	listFilesTool := mcp.NewTool("list_files",
		mcp.WithDescription("List files in a directory"),
		mcp.WithString("directory",
			mcp.Required(),
			mcp.Description("Directory path to list"),
		),
	)
	
	// Add tool handlers
	mcpServer.AddTool(readFileTool, handleReadFile)
	mcpServer.AddTool(writeFileTool, handleWriteFile)
	mcpServer.AddTool(listFilesTool, handleListFiles)
	
	return &MockMCPServer{
		server: mcpServer,
		tools:  []mcp.Tool{readFileTool, writeFileTool, listFilesTool},
	}
}

// NewMockDatabaseServer creates a mock database server
func NewMockDatabaseServer() *MockMCPServer {
	mcpServer := server.NewMCPServer(
		"Mock Database Server", 
		"1.0.0",
		server.WithToolCapabilities(true),
	)
	
	// Create database tools
	queryTool := mcp.NewTool("query_db",
		mcp.WithDescription("Execute a database query"),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("SQL query to execute"),
		),
	)
	
	insertTool := mcp.NewTool("insert_record",
		mcp.WithDescription("Insert a record into the database"),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Table name"),
		),
		mcp.WithObject("data",
			mcp.Required(),
			mcp.Description("Record data as JSON object"),
		),
	)
	
	// Add tool handlers
	mcpServer.AddTool(queryTool, handleQueryDB)
	mcpServer.AddTool(insertTool, handleInsertRecord)
	
	return &MockMCPServer{
		server: mcpServer,
		tools:  []mcp.Tool{queryTool, insertTool},
	}
}

// NewMockWebScraperServer creates a mock web scraper server
func NewMockWebScraperServer() *MockMCPServer {
	mcpServer := server.NewMCPServer(
		"Mock Web Scraper Server",
		"1.0.0", 
		server.WithToolCapabilities(true),
	)
	
	// Create web scraper tools
	fetchURLTool := mcp.NewTool("fetch_url",
		mcp.WithDescription("Fetch content from a URL"),
		mcp.WithString("url",
			mcp.Required(),
			mcp.Description("URL to fetch"),
		),
	)
	
	extractDataTool := mcp.NewTool("extract_data",
		mcp.WithDescription("Extract structured data from HTML"),
		mcp.WithString("html",
			mcp.Required(),
			mcp.Description("HTML content to parse"),
		),
		mcp.WithString("selector",
			mcp.Required(),
			mcp.Description("CSS selector for data extraction"),
		),
	)
	
	// Add tool handlers
	mcpServer.AddTool(fetchURLTool, handleFetchURL)
	mcpServer.AddTool(extractDataTool, handleExtractData)
	
	return &MockMCPServer{
		server: mcpServer,
		tools:  []mcp.Tool{fetchURLTool, extractDataTool},
	}
}

// GetServer returns the underlying MCP server
func (m *MockMCPServer) GetServer() *server.MCPServer {
	return m.server
}

// GetTools returns the tools provided by this mock server
func (m *MockMCPServer) GetTools() []mcp.Tool {
	return m.tools
}

// Mock tool handlers

func handleReadFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	// Mock file content
	content := fmt.Sprintf("Mock file content from %s", path)
	return mcp.NewToolResultText(content), nil
}

func handleWriteFile(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	content, err := request.RequireString("content")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	result := fmt.Sprintf("Mock: Wrote %d bytes to %s", len(content), path)
	return mcp.NewToolResultText(result), nil
}

func handleListFiles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	directory, err := request.RequireString("directory")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	// Mock file listing
	files := []string{"file1.txt", "file2.txt", "subdirectory/"}
	result := fmt.Sprintf("Mock: Files in %s: %v", directory, files)
	return mcp.NewToolResultText(result), nil
}

func handleQueryDB(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sql, err := request.RequireString("sql")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	// Mock query result
	result := fmt.Sprintf("Mock DB Result for query: %s\nRows: [{id: 1, name: 'John'}, {id: 2, name: 'Jane'}]", sql)
	return mcp.NewToolResultText(result), nil
}

func handleInsertRecord(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	table, err := request.RequireString("table")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	data := request.GetArguments()["data"]
	result := fmt.Sprintf("Mock: Inserted record into %s: %v", table, data)
	return mcp.NewToolResultText(result), nil
}

func handleFetchURL(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	url, err := request.RequireString("url")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	// Mock web content
	content := fmt.Sprintf("<html><body><h1>Mock content from %s</h1></body></html>", url)
	return mcp.NewToolResultText(content), nil
}

func handleExtractData(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	html, err := request.RequireString("html")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	selector, err := request.RequireString("selector")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	
	// Mock extraction result
	result := fmt.Sprintf("Mock: Extracted data using selector '%s' from %d bytes of HTML: ['item1', 'item2']", 
		selector, len(html))
	return mcp.NewToolResultText(result), nil
}
# Station Changelog

All notable changes to Station are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.10.9] - 2025-09-11

### 🚀 Major Performance Improvements

#### **Parallel MCP Server Validation**
- **Concurrent Template Processing**: MCP templates now processed in parallel during sync operations
- **Configurable Worker Pools**: Added `STATION_SYNC_TEMPLATE_WORKERS` environment variable (default: 3)
- **Significant Speed Improvements**: Sync operations now complete much faster for environments with multiple MCP configurations
- **Template Isolation**: Each worker processes templates independently to prevent conflicts

### 🔧 Core System Upgrades

#### **GenKit v1.0.1 Integration**
- **Official Plugin Migration**: Replaced Station's custom OpenAI plugin with official GenKit v1.0.1 plugin
- **API Compatibility**: Updated all breaking API changes (Plugin.Init, genkit.Init, DefineModel → LookupModel)
- **Enhanced Model Support**: Comprehensive OpenAI model configuration including o1, o3-mini, gpt-4o series
- **Simplified Architecture**: Removed complex tool call extraction in favor of GenKit's automatic execution
- **Cleaner UI**: Streamlined interface by removing tool call tracking components

#### **MCP Connection Architecture Refactoring**
- **Modular File Structure**: Split 1000-line `mcp_connection_manager.go` into three focused modules:
  - `mcp_connection_manager.go` - Core connection management
  - `mcp_connection_pool.go` - Connection pooling and server lifecycle
  - `mcp_parallel_processing.go` - Parallel processing patterns
- **Enhanced Connection Pooling**: Improved MCP connection management with parallel initialization
- **Environment Variable Controls**: 
  - `STATION_MCP_POOL_WORKERS` - Pool initialization workers (default: 5)
  - `STATION_MCP_CONFIG_WORKERS` - File config workers (default: 2)
  - `STATION_MCP_SERVER_WORKERS` - Server connection workers (default: 3)

### 🎯 Technical Improvements

#### **Parallel Processing Patterns**
- **Worker Pool Implementation**: Robust worker pools using `sync.WaitGroup` for controlled concurrency
- **Error Aggregation**: Comprehensive error collection and reporting across parallel operations  
- **Resource Management**: Proper goroutine lifecycle management and channel cleanup
- **Debug Logging**: Enhanced logging for parallel operation monitoring and troubleshooting

#### **Agent Execution Optimization**
- **Faster Startup**: Parallel MCP connection setup reduces agent initialization time
- **Connection Reuse**: Improved connection pooling for better resource utilization
- **Automatic Tool Execution**: Streamlined tool execution with GenKit v1.0.1's automatic handling

### 🧹 Code Cleanup

#### **Removed Legacy Components**
- **Custom GenKit Package**: Eliminated custom GenKit implementation (~2000 lines removed)
- **Tool Call UI Components**: Removed complex tool call tracking interface
- **Deprecated Test Files**: Cleaned up unused test infrastructure
- **Legacy Utilities**: Removed outdated GenKit compatibility layers

### 📈 Performance Metrics

- **Sync Operations**: Up to 3x faster for environments with multiple MCP configurations
- **Agent Startup**: Reduced agent initialization time through parallel MCP connections
- **Memory Usage**: Reduced memory footprint by eliminating custom GenKit codebase
- **Code Maintainability**: Improved with modular architecture and focused file responsibilities

### 🔄 Backwards Compatibility

- **Configuration**: All existing MCP configurations continue to work unchanged
- **Environment Variables**: New variables have sensible defaults, no migration required
- **API Compatibility**: All existing CLI commands and behaviors preserved
- **Database Schema**: No database migrations required

---

## [v0.8.7] - 2025-01-18

### 🎉 Major Features Added

#### **Embedded React UI with Production Build System**
- **Complete React UI Integration**: Full-featured web interface embedded directly in Station binary
- **Production Build Pipeline**: Integrated UI build into GoReleaser with GitHub Actions automation
- **Zero-Configuration UI**: No separate installation required - UI embedded in all releases
- **TypeScript Build System**: Fixed production build errors and integrated type checking
- **Vite Build Optimization**: Optimized build process for embedded web assets

#### **Comprehensive Agent Management Interface**
- **Agent Editor with Monaco**: Full VS Code-like editing experience for agent .prompt files
- **Live Agent Editing**: Direct editing of agent configurations with real-time save functionality
- **Agent Card Management**: Enhanced agent cards with edit icons and improved navigation
- **Bundle Management UI**: Complete bundle installation and management interface
- **Environment Graph Visualization**: ReactFlow-based environment and agent relationship graphs

#### **Advanced Bundle System**
- **Bundle Installation Modal**: Support for URL and file path installation
- **Automated Bundle Extraction**: Backend tar.gz extraction and environment creation
- **Bundle Directory Management**: Centralized bundle storage and listing
- **Bundle Creation API**: Complete backend support for bundle generation and sharing
- **Environment-Specific Bundles**: Full environment isolation for bundle installations

#### **Enhanced MCP Server Management**
- **Add Server Modal**: Comprehensive MCP server configuration with templates
- **Template-Based Configuration**: JSON template system with variable substitution
- **Server Status Monitoring**: Real-time MCP server health and error reporting
- **Interactive Configuration**: Template variable prompting and validation
- **Ship CLI Integration**: Comprehensive information page about Ship MCP Framework

### 🔧 Core System Improvements

#### **Agent Execution Engine Overhaul**
- **Complete Token Usage Tracking**: Full capture of input/output tokens from GenKit responses
- **Detailed Execution Steps**: Comprehensive tool call logging with parameters and results
- **Enhanced Metadata Capture**: Duration, model names, and execution timing
- **Run Details API**: Complete agent run information including token usage and steps
- **Execution Step Display**: Rich UI showing actual tool calls and execution flow

#### **Template Variable System**
- **Fixed Variable Prompting**: Resolved critical hanging issue with interactive template processing
- **Go Template Integration**: Proper `missingkey=error` configuration for reliable template rendering
- **Variable Validation**: Enhanced template variable detection and error reporting
- **Interactive Mode**: Fixed `stn sync` to properly prompt for missing variables

#### **Build System & Release Process**
- **Embedded UI Assets**: Complete integration of React UI into Go binary
- **Multi-Platform Releases**: GoReleaser configuration for all supported platforms
- **GitHub Actions Automation**: Automated build and release pipeline
- **Local Development**: Streamlined development workflow with embedded assets

### 🎨 User Interface Enhancements

#### **Tokyo Night Theme Integration**
- **Consistent Theming**: Complete Tokyo Night color scheme across all UI components
- **High-Contrast Design**: Improved text visibility and accessibility
- **Modal Styling**: Enhanced modal designs with proper theme integration
- **Component Consistency**: Unified styling across all interface elements

#### **Navigation & User Experience**
- **Breadcrumb Navigation**: Clear navigation paths in agent editor and other complex views
- **Sidebar Optimization**: Removed unused sections (Users) and streamlined navigation
- **Environment Switching**: Fixed environment graph rebuilding when switching contexts
- **Responsive Design**: Mobile-friendly interface design

### 🔧 Technical Improvements

#### **Database & Backend**
- **Enhanced Agent Runs Schema**: Complete metadata capture with proper database fields
- **SQLC Integration**: Automated code generation for database operations
- **Repository Pattern**: Clean separation of data access logic
- **Migration System**: Proper database versioning and upgrade path

#### **API & Integration**
- **MCP Server API**: Complete CRUD operations for MCP server management
- **Bundle API**: Full backend support for bundle operations
- **Environment API**: Enhanced environment management endpoints
- **WebSocket Integration**: Real-time updates for agent execution

#### **Documentation System**
- **Simplified Quickstart**: Removed complex `stn load` commands in favor of direct file copying
- **Updated Documentation**: Comprehensive updates across all documentation sites
- **MCP Integration Guide**: Streamlined setup process for Claude Desktop integration

### 🐛 Bug Fixes

#### **Critical Fixes**
- **Template Variable Hanging**: Fixed critical bug where `stn sync` would hang on missing template variables
- **TypeScript Build Errors**: Resolved all production build compilation issues
- **ReactFlow Integration**: Fixed environment graph not updating when switching environments
- **Token Usage Display**: Fixed "N/A" token usage showing actual captured data
- **Tool Assignment**: Resolved agent tools being lost after sync operations

#### **UI/UX Fixes**
- **Text Visibility**: Fixed poor text contrast in Ship CLI and other colored components
- **Modal Theming**: Proper Tokyo Night theme integration for all modals
- **JavaScript Errors**: Fixed ToolNode reference errors and runtime crashes
- **Bundle List Refresh**: Fixed bundle installation not refreshing the bundles list

#### **System Fixes**
- **Execution Steps**: Fixed execution steps showing actual content instead of just "Step X"
- **Environment Graph**: Fixed API parameter mismatch between frontend and backend
- **MCP Server Errors**: Added proper error status display for failed MCP connections
- **Build System**: Fixed UI embedding and release pipeline integration

### 📚 Documentation Updates

- **Updated README**: Streamlined quickstart with one-line curl install and 5-step setup
- **Documentation Site**: Comprehensive updates to remove deprecated commands
- **MCP Quick Start**: Enhanced 5-minute setup guide with proper configuration flow
- **Feature Documentation**: Comprehensive documentation of new features and capabilities

### 🔄 Development Workflow

- **Local Development**: Improved `make local-install` with UI embedding
- **Testing Integration**: Enhanced testing workflow with UI build verification
- **Version Control**: Proper gitignore and clean repository management
- **Command Caching**: Fixed shell command caching issues for local installations

### 🚀 Performance Improvements

- **UI Bundle Size**: Optimized React build for smaller embedded assets
- **Database Queries**: Enhanced query performance with proper indexing
- **Memory Usage**: Reduced memory footprint of embedded UI assets
- **Startup Time**: Faster Station initialization with optimized asset loading

---

## [v0.8.6] - 2025-01-17

### Previous Features
- File-based configuration system
- MCP server integration
- Agent execution engine
- Basic CLI functionality
- SSH/TUI interface

---

## Upgrade Notes

### From v0.8.6 to v0.8.7

1. **UI Now Embedded**: No separate UI installation required
2. **Updated Commands**: Replace `stn load` with direct file copying to environment directories
3. **New Web Interface**: Access full Station management at `http://localhost:8585`
4. **Enhanced Agent Editor**: Edit agents directly in the web interface
5. **Bundle System**: Install and manage bundles through the web interface

### Breaking Changes

- **Removed `stn load` Command**: Use direct file copying to environment directories instead
- **Updated Sync Behavior**: `stn sync` now works without environment parameter
- **UI Access Method**: Web interface now embedded in binary, no separate installation

### Migration Guide

**Old workflow:**
```bash
stn load config.json --env default
stn sync default
```

**New workflow:**
```bash
cp config.json ~/.config/station/environments/default/template.json
echo "VARIABLE: value" > ~/.config/station/environments/default/variables.yml
stn sync
```

---

## Development Highlights

This release represents a major milestone in Station's evolution from a CLI-only tool to a comprehensive agent management platform with a full web interface. Key achievements include:

- **35+ New Features** across UI, backend, and build systems
- **50+ Bug Fixes** improving stability and user experience
- **Complete Documentation Overhaul** with simplified setup process
- **Production-Ready Build System** with embedded UI and automated releases
- **Enhanced Agent Management** with visual editing and execution monitoring

Station now provides a complete solution for managing intelligent sub-agents with both CLI and web interfaces, making it accessible to users with different preferences and use cases.
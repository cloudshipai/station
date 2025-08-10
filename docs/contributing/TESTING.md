# Testing Guide for Station 🧪

This guide covers all aspects of testing Station, from unit tests to integration testing and manual CLI verification.

## 🚀 Quick Start

### Run All Tests
```bash
# Comprehensive test suite (recommended)
./test-all.sh

# Or step by step
make dev        # Build development version
make test       # Run unit tests
./stn init      # Initialize Station
./stn banner    # Display beautiful banner
```

## 📋 Testing Checklist

### ✅ **Pre-Testing Setup**
- [ ] Clean build environment: `make clean`
- [ ] Build Station: `make dev` or `make build`
- [ ] Clear any running processes: `make stop-station`

### ✅ **Unit Tests**
```bash
# Run all unit tests
make test

# Run with coverage
make test-coverage

# Run integration tests
make test-integration

# Run benchmarks
make test-bench
```

### ✅ **CLI Command Testing**

#### **Core Commands**
```bash
# Test initialization
./stn init

# Test configuration management
./stn config show
./stn config edit

# Test help system
./stn --help
./stn config --help
./stn mcp --help
```

#### **Key Management**
```bash
# Test key operations
./stn key generate
./stn key status
./stn key rotate
./stn key finish-rotation
```

#### **MCP Operations**
```bash
# Test MCP commands
./stn mcp list
./stn mcp tools
./stn mcp add --help

# Test GitHub discovery
./stn load https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem
```

#### **Hidden Commands** 
```bash
# Test banner display (great for screenshots!)
./stn banner

# Test blastoff animation
./stn blastoff
```

### ✅ **Server Testing**

#### **Start Server**
```bash
# Start in foreground
./stn serve

# Start in background for testing
./stn serve &
SERVER_PID=$!
```

#### **Test SSH Connection**
```bash
# Connect to admin interface
ssh admin@localhost -p 2222

# Test non-interactive connection
timeout 5 ssh -o ConnectTimeout=2 -o StrictHostKeyChecking=no admin@localhost -p 2222 "exit"
```

#### **Stop Server**
```bash
# Kill background server
kill $SERVER_PID

# Or use make target
make stop-station
```

## 🔧 Test Infrastructure

### **Test Files Location**
```
station/
├── pkg/crypto/crypto_test.go
├── pkg/crypto/keymanager_test.go
├── internal/config/config_test.go
├── internal/db/db_test.go
├── internal/db/repositories/environments_test.go
├── internal/services/genkit_basic_test.go
├── internal/services/mcp_config_service_test.go
└── test-all.sh (comprehensive test script)
```

### **Makefile Targets**
- `make test` - Run all unit tests
- `make test-coverage` - Generate coverage report
- `make test-integration` - Run integration tests  
- `make test-bench` - Run benchmarks
- `make lint` - Code quality checks

## 🧪 Test Categories

### **1. Unit Tests**
Focus on individual functions and components:
- Cryptographic operations
- Database repositories
- Configuration management
- Service layer logic

### **2. Integration Tests**
Test component interactions:
- CLI command execution
- Server startup/shutdown
- SSH connectivity
- MCP server communication

### **3. Manual Testing**
Human verification of:
- User interface experience
- Error messages and handling
- Performance under load
- Cross-platform compatibility

## 🔍 Testing Specific Features

### **MCP Server Discovery**
```bash
# Test with various GitHub URLs
./stn load https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem
./stn load https://github.com/awslabs/mcp/tree/main/src/cfn-mcp-server
./stn load https://github.com/kocierik/mcp-nomad

# Verify discovery worked
./stn mcp list
./stn mcp tools
```

### **Environment Management**
```bash
# Test multiple environments
./stn mcp list --environment dev
./stn mcp list --environment staging
./stn mcp list --environment prod
```

### **Interactive Features**
```bash
# Test interactive MCP add
./stn mcp add --interactive

# Test forms and navigation
# Use arrow keys, Enter, Ctrl+C to test
```

## 🚨 Common Issues & Debugging

### **Port Conflicts**
```bash
# Clear stuck processes
make kill-ports

# Check what's using ports
lsof -i :2222
lsof -i :3000
lsof -i :8080
```

### **Database Issues**
```bash
# Remove test database
rm -f station.db

# Clear configuration
rm -rf ~/.config/station
```

### **SSH Connection Problems**
```bash
# Generate new SSH host key
ssh-keygen -t rsa -f ssh_host_key -N ""

# Test SSH server manually
ssh -v admin@localhost -p 2222
```

## 📊 Coverage Goals

### **Current Coverage Targets**
- **Overall**: >80%
- **Critical paths**: >95%
- **CLI commands**: >90%
- **Database operations**: >85%
- **Cryptographic functions**: >95%

### **Generate Coverage Report**
```bash
make test-coverage
open coverage.html  # View in browser
```

## 🎯 Performance Testing

### **Benchmarks**
```bash
# Run performance benchmarks
make test-bench

# Profile specific operations
go test -bench=BenchmarkEncryption -cpuprofile=cpu.prof
go tool pprof cpu.prof
```

### **Load Testing**
```bash
# Test multiple concurrent connections
for i in {1..10}; do
  ssh admin@localhost -p 2222 "exit" &
done
wait
```

## 🚀 CI/CD Testing

### **Automated Pipeline**
The GitHub Actions workflow automatically runs:
1. Unit tests across multiple Go versions
2. Cross-platform compatibility tests
3. Security scanning
4. Code quality checks

### **Local CI Simulation**
```bash
# Simulate CI environment
make clean
make test
make lint
make build
```

## 📝 Writing New Tests

### **Unit Test Template**
```go
func TestMyFunction(t *testing.T) {
    // Arrange
    input := "test input"
    expected := "expected output"
    
    // Act
    result := MyFunction(input)
    
    // Assert
    assert.Equal(t, expected, result)
}
```

### **Integration Test Template**
```go
//go:build integration
// +build integration

func TestServerIntegration(t *testing.T) {
    // Setup test environment
    // Start server
    // Test functionality
    // Cleanup
}
```

## 🛠️ Test Utilities

### **Automated Test Script**
The `test-all.sh` script provides comprehensive testing:
- Builds Station
- Runs unit tests
- Tests CLI commands
- Starts/stops server
- Validates SSH connectivity
- Cleans up automatically

### **Manual Test Scenarios**

#### **New User Experience**
1. Fresh install: `./stn init`
2. First server start: `./stn serve`
3. First SSH connection: `ssh admin@localhost -p 2222`
4. First MCP server: `./stn load <github-url>`

#### **Power User Workflow**  
1. Multiple environments
2. Key rotation
3. Complex MCP configurations
4. Background automation

## 🎉 Test Results

### **Success Criteria**
- [ ] All unit tests pass
- [ ] CLI commands work correctly
- [ ] Server starts and accepts connections
- [ ] SSH interface is accessible
- [ ] MCP discovery functions properly
- [ ] Configuration persists correctly
- [ ] No memory leaks or deadlocks

### **Performance Benchmarks**
- Server startup: < 5 seconds
- SSH connection: < 2 seconds  
- MCP discovery: < 30 seconds
- Database operations: < 100ms

---

## 🤝 Contributing Tests

When contributing to Station:

1. **Write tests first** (TDD approach preferred)
2. **Test both happy and error paths**
3. **Include integration tests for new features**
4. **Update this guide** with new testing procedures
5. **Ensure >80% code coverage** for new code

## 📞 Getting Help

- **Issues**: Report testing problems in GitHub Issues
- **Discussions**: Ask questions in GitHub Discussions  
- **Documentation**: Check `/docs/` for additional guides

---

**Happy Testing!** 🧪✨

*Remember: Good tests are the foundation of reliable software. Test early, test often, and test with purpose.*
# Template Bundle System - Testing Strategy & Feedback Loops

## Testing Philosophy

**Test-Driven Development Approach**: Build comprehensive test coverage before and during implementation to ensure reliability and enable rapid iteration.

**Feedback Loop Strategy**: Create fast, automated feedback at every stage of development to catch issues early and maintain development velocity.

## Testing Pyramid

### Unit Tests (70% of test coverage)
**Fast feedback loop: <5 seconds**

#### Bundle Creation & Validation
```go
// pkg/bundle/creator_test.go
func TestBundleCreator_Create(t *testing.T) {
    creator := NewBundleCreator(afero.NewMemMapFs())
    
    err := creator.Create("test-bundle", CreateOptions{
        Author: "Test Author",
        Description: "Test bundle",
    })
    
    assert.NoError(t, err)
    
    // Verify structure created
    assert.FileExists(t, "test-bundle/manifest.json")
    assert.FileExists(t, "test-bundle/template.json")
    assert.FileExists(t, "test-bundle/variables.schema.json")
    
    // Verify content
    manifest := loadManifest(t, "test-bundle/manifest.json")
    assert.Equal(t, "test-bundle", manifest.Name)
    assert.Equal(t, "Test Author", manifest.Author)
}

func TestBundleValidator_Validate(t *testing.T) {
    validator := NewBundleValidator()
    
    tests := []struct {
        name        string
        bundleSetup func(fs afero.Fs)
        wantErr     bool
        wantIssues  []ValidationIssue
    }{
        {
            name: "valid bundle",
            bundleSetup: func(fs afero.Fs) {
                createValidBundle(fs, "valid-bundle")
            },
            wantErr: false,
        },
        {
            name: "missing manifest",
            bundleSetup: func(fs afero.Fs) {
                // Create bundle without manifest.json
            },
            wantErr: true,
            wantIssues: []ValidationIssue{
                {Type: "missing_file", File: "manifest.json"},
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            fs := afero.NewMemMapFs()
            tt.bundleSetup(fs)
            
            result, err := validator.Validate(fs, "bundle-path")
            
            if tt.wantErr {
                assert.Error(t, err)
                assert.Equal(t, tt.wantIssues, result.Issues)
            } else {
                assert.NoError(t, err)
                assert.True(t, result.Valid)
            }
        })
    }
}
```

#### Registry Implementations
```go
// pkg/bundle/registry/http_test.go
func TestHTTPRegistry_Download(t *testing.T) {
    // Setup mock HTTP server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        assert.Equal(t, "/bundles/aws-tools/1.0.0/aws-tools.zip", r.URL.Path)
        
        // Return mock zip file
        w.Header().Set("Content-Type", "application/zip")
        w.Write(createMockBundleZip(t))
    }))
    defer server.Close()
    
    registry := NewHTTPRegistry(server.URL, http.DefaultClient)
    
    zipBytes, err := registry.Download("aws-tools", "1.0.0")
    
    assert.NoError(t, err)
    assert.NotEmpty(t, zipBytes)
    
    // Verify zip content
    bundle := extractMockBundle(t, zipBytes)
    assert.Equal(t, "aws-tools", bundle.Name)
}

// pkg/bundle/registry/s3_test.go (using minio for testing)
func TestS3Registry_Download(t *testing.T) {
    // Setup local minio server for testing
    minioServer := startMockS3Server(t)
    defer minioServer.Close()
    
    registry := NewS3Registry(S3Config{
        Endpoint: minioServer.URL,
        Bucket:   "test-bundles",
        Region:   "us-east-1",
    })
    
    // Upload test bundle
    uploadTestBundle(t, minioServer, "test-bundle", "1.0.0")
    
    zipBytes, err := registry.Download("test-bundle", "1.0.0")
    
    assert.NoError(t, err)
    assert.NotEmpty(t, zipBytes)
}
```

#### Variable Resolution
```go
// pkg/bundle/variables_test.go
func TestVariableResolver_ResolveVariables(t *testing.T) {
    tests := []struct {
        name           string
        bundleSchema   *BundleSchema
        envFiles       map[string]string  // filename -> content
        systemEnv      map[string]string
        expectedVars   map[string]interface{}
        expectedPrompts []string
    }{
        {
            name: "bundle defaults used",
            bundleSchema: &BundleSchema{
                Variables: map[string]VariableSpec{
                    "REGION": {Type: "string", Default: "us-east-1"},
                },
            },
            expectedVars: map[string]interface{}{
                "REGION": "us-east-1",
            },
        },
        {
            name: "environment file overrides defaults",
            bundleSchema: &BundleSchema{
                Variables: map[string]VariableSpec{
                    "REGION": {Type: "string", Default: "us-east-1"},
                },
            },
            envFiles: map[string]string{
                "variables.yml": "REGION: eu-west-1",
            },
            expectedVars: map[string]interface{}{
                "REGION": "eu-west-1",
            },
        },
        {
            name: "system env overrides everything",
            bundleSchema: &BundleSchema{
                Variables: map[string]VariableSpec{
                    "REGION": {Type: "string", Default: "us-east-1"},
                },
            },
            envFiles: map[string]string{
                "variables.yml": "REGION: eu-west-1",
            },
            systemEnv: map[string]string{
                "REGION": "ap-southeast-1",
            },
            expectedVars: map[string]interface{}{
                "REGION": "ap-southeast-1",
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            fs := setupTestFS(t, tt.envFiles)
            resolver := NewVariableResolver(fs, "test-env", tt.bundleSchema)
            
            // Mock system environment
            for k, v := range tt.systemEnv {
                t.Setenv(k, v)
            }
            
            result, err := resolver.ResolveVariables([]string{"REGION"})
            
            assert.NoError(t, err)
            assert.Equal(t, tt.expectedVars, result.Resolved)
            assert.Equal(t, tt.expectedPrompts, result.PromptsRequired)
        })
    }
}
```

### Integration Tests (20% of test coverage)
**Medium feedback loop: <30 seconds**

#### End-to-End Bundle Workflow
```go
// test/integration/bundle_workflow_test.go
func TestBundleWorkflow_CreateToInstall(t *testing.T) {
    tempDir := t.TempDir()
    bundleManager := setupBundleManager(t, tempDir)
    
    // 1. Create bundle
    err := bundleManager.Create("test-bundle", CreateOptions{
        Author: "Test",
        Description: "Test bundle",
    })
    assert.NoError(t, err)
    
    // 2. Add some template content
    addTestTemplate(t, filepath.Join(tempDir, "test-bundle"))
    
    // 3. Validate
    result, err := bundleManager.Validate("test-bundle")
    assert.NoError(t, err)
    assert.True(t, result.Valid)
    
    // 4. Package
    zipPath, err := bundleManager.Package("test-bundle")
    assert.NoError(t, err)
    assert.FileExists(t, zipPath)
    
    // 5. Publish to local registry
    localRegistry := setupLocalRegistry(t, tempDir)
    err = bundleManager.Publish("test-bundle", "local")
    assert.NoError(t, err)
    
    // 6. Install from registry (clean environment)
    cleanManager := setupCleanBundleManager(t)
    err = cleanManager.Install("test-bundle")
    assert.NoError(t, err)
    
    // 7. Verify installation
    installed := cleanManager.ListInstalled()
    assert.Contains(t, installed, "test-bundle")
}
```

#### Multi-Registry Discovery
```go
func TestMultiRegistry_Discovery(t *testing.T) {
    // Setup multiple test registries
    httpRegistry := setupMockHTTPRegistry(t)
    s3Registry := setupMockS3Registry(t)
    localRegistry := setupLocalRegistry(t)
    
    bundleManager := NewBundleManager(BundleConfig{
        Registries: map[string]RegistryConfig{
            "http":  {Type: "http", URL: httpRegistry.URL},
            "s3":    {Type: "s3", Bucket: "test-bucket"},
            "local": {Type: "local", Path: localRegistry.Path},
        },
    })
    
    // Populate registries with test bundles
    seedRegistry(t, httpRegistry, "aws-tools", "github-tools")
    seedRegistry(t, s3Registry, "company-internal", "security-tools")  
    seedRegistry(t, localRegistry, "dev-tools")
    
    // Test discovery
    allBundles, err := bundleManager.List()
    assert.NoError(t, err)
    
    expectedBundles := []string{
        "aws-tools", "github-tools",    // from HTTP
        "company-internal", "security-tools",  // from S3
        "dev-tools",                    // from local
    }
    
    for _, expected := range expectedBundles {
        assert.Contains(t, bundleNames(allBundles), expected)
    }
    
    // Test registry-specific discovery
    httpBundles, err := bundleManager.List(WithRegistry("http"))
    assert.NoError(t, err)
    assert.Len(t, httpBundles, 2)
}
```

#### MCP Sync Integration
```go
// test/integration/mcp_sync_test.go  
func TestMCPSync_WithBundles(t *testing.T) {
    // Setup test environment
    testDir := setupTestStationConfig(t)
    
    // Install test bundle
    bundleManager := setupBundleManager(t, testDir)
    err := bundleManager.InstallFromZip("aws-tools", createTestAWSBundle(t))
    assert.NoError(t, err)
    
    // Create environment variables
    createVariablesFile(t, testDir, "test-env", map[string]interface{}{
        "AWS_REGION": "us-west-2",
        "AWS_ACCESS_KEY_ID": "test-key",
    })
    
    // Run MCP sync
    syncResult, err := runMCPSync(t, testDir, "test-env")
    assert.NoError(t, err)
    
    // Verify bundle was processed
    assert.Contains(t, syncResult.ProcessedConfigs, "aws-tools")
    
    // Verify tools were registered
    tools := getRegisteredTools(t, testDir, "test-env")
    expectedTools := []string{"aws_s3_list", "aws_cloudwatch_metrics"}
    for _, tool := range expectedTools {
        assert.Contains(t, tools, tool)
    }
}
```

### System Tests (10% of test coverage)
**Slow feedback loop: <2 minutes**

#### Docker-based GitOps Tests
```bash
# test/system/gitops_test.sh
#!/bin/bash
set -e

echo "Testing GitOps deployment with bundles..."

# Create test bundle registry
docker run -d --name bundle-registry -p 8080:80 nginx:alpine
populate_test_registry "http://localhost:8080"

# Build station image with bundle support
docker build -t station:bundle-test .

# Test deployment with bundle installation
docker run --rm \
  -e AWS_ACCESS_KEY_ID=test-key \
  -e AWS_REGION=us-east-1 \
  -v $(pwd)/test/fixtures/environments:/station/environments:ro \
  station:bundle-test \
  /bin/bash -c "
    stn template install aws-powertools && \
    stn mcp sync production --validate-only && \
    echo 'GitOps deployment test passed'
  "

echo "GitOps test completed successfully"
```

#### Performance & Load Tests
```go
// test/system/performance_test.go
func TestBundleSystem_Performance(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping performance test in short mode")
    }
    
    // Test large bundle catalog discovery
    t.Run("large_catalog_discovery", func(t *testing.T) {
        registry := setupRegistryWithBundles(t, 1000) // 1000 bundles
        
        start := time.Now()
        bundles, err := registry.List()
        elapsed := time.Since(start)
        
        assert.NoError(t, err)
        assert.Len(t, bundles, 1000)
        assert.Less(t, elapsed, 5*time.Second, "Discovery should complete within 5 seconds")
    })
    
    // Test concurrent bundle installations
    t.Run("concurrent_installations", func(t *testing.T) {
        manager := setupBundleManager(t, t.TempDir())
        
        var wg sync.WaitGroup
        errors := make(chan error, 10)
        
        // Install 10 bundles concurrently
        for i := 0; i < 10; i++ {
            wg.Add(1)
            go func(i int) {
                defer wg.Done()
                bundleName := fmt.Sprintf("test-bundle-%d", i)
                if err := manager.Install(bundleName); err != nil {
                    errors <- err
                }
            }(i)
        }
        
        wg.Wait()
        close(errors)
        
        // Check for any errors
        for err := range errors {
            t.Errorf("Installation error: %v", err)
        }
    })
}
```

## Development Feedback Loops

### 1. **Immediate Feedback Loop (< 5 seconds)**
```bash
# Fast unit test runner with watch mode
make test-watch
# Or using gotestsum for better output
gotestsum --watch -- ./pkg/bundle/... -v
```

### 2. **Quick Integration Feedback (< 30 seconds)**  
```bash
# Run integration tests for specific component
make test-integration COMPONENT=bundle
# Or run specific test patterns
go test -run TestBundleWorkflow ./test/integration/...
```

### 3. **Full System Validation (< 2 minutes)**
```bash
# Complete test suite with coverage
make test-all
# System tests with Docker
make test-system
```

### 4. **Live Development Server**
```bash
# Development server with hot reload for testing
make dev-server
# In another terminal, test live changes
make test-dev-workflow
```

## Test Data & Fixtures Strategy

### Mock Bundle Registry
```go
// test/fixtures/registry.go
func CreateMockRegistry(t *testing.T) *MockRegistry {
    registry := &MockRegistry{
        bundles: map[string]map[string]*Bundle{
            "aws-powertools": {
                "1.0.0": createAWSBundle("1.0.0"),
                "1.1.0": createAWSBundle("1.1.0"),
            },
            "github-automation": {
                "2.0.0": createGitHubBundle("2.0.0"),
            },
        },
    }
    
    return registry
}

func createAWSBundle(version string) *Bundle {
    return &Bundle{
        Manifest: BundleManifest{
            Name:    "aws-powertools",
            Version: version,
            RequiredVariables: map[string]VariableSpec{
                "AWS_ACCESS_KEY_ID": {
                    Type:        "string",
                    Description: "AWS Access Key",
                    Secret:      true,
                    Required:    true,
                },
                "AWS_REGION": {
                    Type:    "string", 
                    Default: "us-east-1",
                },
            },
        },
        Template: createMockMCPTemplate("aws-s3", "aws-cloudwatch"),
    }
}
```

### Test Environment Setup
```go
// test/helpers/environment.go
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
    tempDir := t.TempDir()
    
    env := &TestEnvironment{
        ConfigDir:   tempDir,
        DatabaseURL: filepath.Join(tempDir, "test.db"),
        FileSystem:  afero.NewMemMapFs(),
    }
    
    // Setup test configuration
    setupTestConfig(env)
    
    // Setup test database
    setupTestDatabase(env)
    
    // Register cleanup
    t.Cleanup(func() {
        env.Cleanup()
    })
    
    return env
}
```

## Automated Testing Pipeline

### GitHub Actions Workflow
```yaml
# .github/workflows/bundle-system.yml
name: Bundle System Tests

on:
  push:
    branches: [feature/template-bundle-system]
  pull_request:
    branches: [main]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run Unit Tests
        run: |
          go test -race -coverprofile=coverage.out ./pkg/bundle/...
          go tool cover -html=coverage.out -o coverage.html
      
      - name: Upload Coverage
        uses: codecov/codecov-action@v3
  
  integration-tests:
    runs-on: ubuntu-latest
    services:
      minio:
        image: minio/minio
        env:
          MINIO_ACCESS_KEY: minioadmin
          MINIO_SECRET_KEY: minioadmin
        ports:
          - 9000:9000
        options: --health-cmd "curl -f http://localhost:9000/minio/health/live"
    
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run Integration Tests
        env:
          MINIO_ENDPOINT: localhost:9000
          MINIO_ACCESS_KEY: minioadmin
          MINIO_SECRET_KEY: minioadmin
        run: go test -v ./test/integration/...
  
  system-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Build Docker Image
        run: docker build -t station:test .
      
      - name: Run System Tests
        run: |
          chmod +x test/system/*.sh
          ./test/system/bundle_workflow_test.sh
          ./test/system/gitops_test.sh
```

## Local Development Workflow

### Development Scripts
```bash
# scripts/dev-setup.sh
#!/bin/bash
echo "Setting up development environment for bundle system..."

# Install development dependencies
go install github.com/gotestsum@latest
go install github.com/vektra/mockery/v2@latest

# Generate mocks for interfaces  
go generate ./pkg/bundle/...

# Setup test data
mkdir -p test/fixtures/bundles
./scripts/create-test-bundles.sh

# Start local test registry
docker-compose -f test/docker-compose.test.yml up -d

echo "Development environment ready!"
echo "Run 'make test-watch' to start continuous testing"
```

### Makefile Targets
```makefile
# Makefile additions for bundle system
.PHONY: test-bundle test-bundle-watch test-integration-bundle

# Fast feedback for bundle development
test-bundle:
	@echo "Running bundle unit tests..."
	gotestsum --format pkgname -- ./pkg/bundle/... -race -cover

test-bundle-watch:
	@echo "Starting bundle test watcher..."  
	gotestsum --watch --format pkgname -- ./pkg/bundle/... -race

# Integration tests with external dependencies
test-integration-bundle:
	@echo "Running bundle integration tests..."
	docker-compose -f test/docker-compose.test.yml up -d
	gotestsum --format pkgname -- ./test/integration/bundle/... -v
	docker-compose -f test/docker-compose.test.yml down

# Full bundle system validation
test-bundle-system:
	@echo "Running complete bundle system tests..."
	$(MAKE) test-bundle
	$(MAKE) test-integration-bundle
	./test/system/bundle_system_test.sh
```

This comprehensive testing strategy provides multiple feedback loops and ensures high confidence in the bundle system implementation. Each test layer serves a specific purpose and provides different types of feedback at appropriate speeds for development velocity.
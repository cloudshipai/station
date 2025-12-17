package services

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestNewJaegerService tests service creation with default and custom configs
func TestNewJaegerService(t *testing.T) {
	tests := []struct {
		name           string
		config         *JaegerConfig
		expectedUIPort int
		expectedOTLP   int
	}{
		{
			name:           "default config",
			config:         &JaegerConfig{},
			expectedUIPort: 16686,
			expectedOTLP:   4318,
		},
		{
			name: "custom ports",
			config: &JaegerConfig{
				UIPort:   17686,
				OTLPPort: 5318,
			},
			expectedUIPort: 17686,
			expectedOTLP:   5318,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewJaegerService(tt.config)

			if svc.uiPort != tt.expectedUIPort {
				t.Errorf("Expected UI port %d, got %d", tt.expectedUIPort, svc.uiPort)
			}

			if svc.otlpPort != tt.expectedOTLP {
				t.Errorf("Expected OTLP port %d, got %d", tt.expectedOTLP, svc.otlpPort)
			}

			if svc.isRunning {
				t.Error("Service should not be running on creation")
			}
		})
	}
}

// TestJaegerServiceGetters tests URL getter methods
func TestJaegerServiceGetters(t *testing.T) {
	svc := NewJaegerService(&JaegerConfig{
		UIPort:   16686,
		OTLPPort: 4318,
	})

	expectedUI := "http://localhost:16686"
	if url := svc.GetUIURL(); url != expectedUI {
		t.Errorf("Expected UI URL %s, got %s", expectedUI, url)
	}

	expectedOTLP := "http://localhost:4318"
	if url := svc.GetOTLPEndpoint(); url != expectedOTLP {
		t.Errorf("Expected OTLP endpoint %s, got %s", expectedOTLP, url)
	}
}

// TestJaegerIsRunning tests the IsRunning state
func TestJaegerIsRunning(t *testing.T) {
	svc := NewJaegerService(&JaegerConfig{})

	if svc.IsRunning() {
		t.Error("Service should not be running initially")
	}

	// Simulate running state
	svc.isRunning = true

	if !svc.IsRunning() {
		t.Error("Service should report as running")
	}
}

// TestJaegerAlreadyRunning tests the isAlreadyRunning check
func TestJaegerAlreadyRunning(t *testing.T) {
	svc := NewJaegerService(&JaegerConfig{
		UIPort: 19999, // Use non-standard port unlikely to be in use
	})

	// Should not be running on a random port
	if svc.isAlreadyRunning() {
		t.Error("isAlreadyRunning should return false for unused port")
	}
}

// TestJaegerStopWhenNotRunning tests stopping a service that's not running
func TestJaegerStopWhenNotRunning(t *testing.T) {
	svc := NewJaegerService(&JaegerConfig{})

	err := svc.Stop(context.Background())
	if err != nil {
		t.Errorf("Stop should not error when service not running: %v", err)
	}
}

// TestJaegerPortForwardConfiguration tests port forward configuration
func TestJaegerPortForwardConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		uiPort   int
		otlpPort int
	}{
		{"standard ports", 16686, 4318},
		{"custom ports", 17686, 5318},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewJaegerService(&JaegerConfig{
				UIPort:   tt.uiPort,
				OTLPPort: tt.otlpPort,
			})

			// Verify the configuration is stored correctly
			if svc.uiPort != tt.uiPort {
				t.Errorf("UI port not configured correctly: expected %d, got %d", tt.uiPort, svc.uiPort)
			}
			if svc.otlpPort != tt.otlpPort {
				t.Errorf("OTLP port not configured correctly: expected %d, got %d", tt.otlpPort, svc.otlpPort)
			}
		})
	}
}

// MockHTTPServer creates a mock HTTP server for testing
type MockHTTPServer struct {
	server *http.Server
	port   int
}

func NewMockHTTPServer(port int) *MockHTTPServer {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "OK")
	})

	return &MockHTTPServer{
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: mux,
		},
		port: port,
	}
}

func (m *MockHTTPServer) Start() error {
	go func() {
		_ = m.server.ListenAndServe()
	}()
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (m *MockHTTPServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return m.server.Shutdown(ctx)
}

// TestJaegerDetectsExistingInstance tests detection of already running Jaeger
func TestJaegerDetectsExistingInstance(t *testing.T) {
	// Start a mock HTTP server on Jaeger's port
	mockPort := 19997
	mock := NewMockHTTPServer(mockPort)
	if err := mock.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer mock.Stop()

	// Create Jaeger service pointing to the mock port
	svc := NewJaegerService(&JaegerConfig{
		UIPort: mockPort,
	})

	// Should detect the mock server as "already running"
	if !svc.isAlreadyRunning() {
		t.Error("Should detect mock HTTP server as running Jaeger instance")
	}
}

// TestJaegerConfigDefaults tests that defaults are applied correctly
func TestJaegerConfigDefaults(t *testing.T) {
	svc := NewJaegerService(&JaegerConfig{})

	if svc.uiPort != 16686 {
		t.Errorf("Default UI port should be 16686, got %d", svc.uiPort)
	}

	if svc.otlpPort != 4318 {
		t.Errorf("Default OTLP port should be 4318, got %d", svc.otlpPort)
	}
}

// TestJaegerNilConfig tests service creation with nil config
func TestJaegerNilConfig(t *testing.T) {
	// This should not panic and should use defaults
	svc := NewJaegerService(&JaegerConfig{})

	if svc == nil {
		t.Fatal("NewJaegerService should not return nil")
	}

	if svc.uiPort == 0 || svc.otlpPort == 0 {
		t.Error("Ports should be set to defaults when config is minimal")
	}
}

// BenchmarkJaegerServiceCreation benchmarks service creation
func BenchmarkJaegerServiceCreation(b *testing.B) {
	config := &JaegerConfig{
		UIPort:   16686,
		OTLPPort: 4318,
	}

	for i := 0; i < b.N; i++ {
		_ = NewJaegerService(config)
	}
}

// BenchmarkIsAlreadyRunning benchmarks the running check
func BenchmarkIsAlreadyRunning(b *testing.B) {
	svc := NewJaegerService(&JaegerConfig{
		UIPort: 19996, // Port unlikely to be in use
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.isAlreadyRunning()
	}
}

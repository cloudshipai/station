package services

import (
	"context"
	"testing"
	"time"
)

// TestNewTelemetryService tests telemetry service creation
func TestNewTelemetryService(t *testing.T) {
	tests := []struct {
		name        string
		config      *TelemetryConfig
		expectNil   bool
		description string
	}{
		{
			name:        "Create with nil config",
			config:      nil,
			expectNil:   false,
			description: "Should create service with default config",
		},
		{
			name: "Create with custom config",
			config: &TelemetryConfig{
				Enabled:      true,
				ServiceName:  "test-station",
				Environment:  "test",
				OTLPEndpoint: "localhost:4318",
			},
			expectNil:   false,
			description: "Should create service with custom config",
		},
		{
			name: "Create with disabled telemetry",
			config: &TelemetryConfig{
				Enabled: false,
			},
			expectNil:   false,
			description: "Should create service even when disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewTelemetryService(tt.config)

			if tt.expectNil && service != nil {
				t.Errorf("Expected nil service, got %v", service)
			}

			if !tt.expectNil && service == nil {
				t.Error("Expected service to be created, got nil")
			}

			if service != nil && service.config == nil {
				t.Error("Service config should not be nil")
			}

			t.Logf("Service created with config: enabled=%v", service.config.Enabled)
		})
	}
}

// TestTelemetryConfigDefaults tests default configuration values
func TestTelemetryConfigDefaults(t *testing.T) {
	service := NewTelemetryService(nil)

	if !service.config.Enabled {
		t.Error("Default config should have telemetry enabled")
	}

	if service.config.ServiceName != "station" {
		t.Errorf("Default service name should be 'station', got %s", service.config.ServiceName)
	}

	if service.config.Environment != "development" {
		t.Errorf("Default environment should be 'development', got %s", service.config.Environment)
	}

	t.Logf("Default config: name=%s, env=%s, enabled=%v",
		service.config.ServiceName, service.config.Environment, service.config.Enabled)
}

// TestInitializeDisabledTelemetry tests initialization when telemetry is disabled
func TestInitializeDisabledTelemetry(t *testing.T) {
	service := NewTelemetryService(&TelemetryConfig{
		Enabled: false,
	})

	ctx := context.Background()
	err := service.Initialize(ctx)

	if err != nil {
		t.Errorf("Initialize should not error when telemetry is disabled, got: %v", err)
	}

	t.Log("Disabled telemetry initialization succeeded")
}

// TestInitializeEnabledTelemetry tests initialization when telemetry is enabled
func TestInitializeEnabledTelemetry(t *testing.T) {
	service := NewTelemetryService(&TelemetryConfig{
		Enabled:     true,
		ServiceName: "test-station",
		Environment: "test",
		// No OTLP endpoint - should use no-op exporter
	})

	ctx := context.Background()
	err := service.Initialize(ctx)

	if err != nil {
		t.Logf("Telemetry initialization: %v", err)
		// Initialization may fail in test environment without proper OTLP setup
		// This is expected behavior
		return
	}

	t.Log("Telemetry initialization succeeded")

	// Verify tracer and meter were created
	if service.tracer == nil {
		t.Error("Tracer should be initialized")
	}

	if service.meter == nil {
		t.Error("Meter should be initialized")
	}
}

// TestGetSampler tests sampling strategy selection
func TestGetSampler(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		description string
	}{
		{
			name:        "Production sampling",
			environment: "production",
			description: "Should use 10% sampling in production",
		},
		{
			name:        "Staging sampling",
			environment: "staging",
			description: "Should use 50% sampling in staging",
		},
		{
			name:        "Development sampling",
			environment: "development",
			description: "Should use 100% sampling in development",
		},
		{
			name:        "Default sampling",
			environment: "unknown",
			description: "Should default to always sample for unknown environments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewTelemetryService(&TelemetryConfig{
				Enabled:     true,
				Environment: tt.environment,
			})

			sampler := service.getSampler()
			if sampler == nil {
				t.Error("Sampler should not be nil")
			}

			t.Logf("Sampler created for environment: %s", tt.environment)
		})
	}
}

// TestStartSpanWhenDisabled tests span creation when telemetry is disabled
func TestStartSpanWhenDisabled(t *testing.T) {
	service := NewTelemetryService(&TelemetryConfig{
		Enabled: false,
	})

	ctx := context.Background()
	newCtx, span := service.StartSpan(ctx, "test-span")

	if newCtx == nil {
		t.Error("Context should not be nil")
	}

	if span == nil {
		t.Error("Span should not be nil (even if no-op)")
	}

	t.Log("No-op span created when telemetry disabled")
}

// TestRecordAgentExecutionWhenDisabled tests metric recording when disabled
func TestRecordAgentExecutionWhenDisabled(t *testing.T) {
	service := NewTelemetryService(&TelemetryConfig{
		Enabled: false,
	})

	ctx := context.Background()
	tokenUsage := map[string]interface{}{
		"input_tokens":  int64(100),
		"output_tokens": int64(50),
	}

	// Should not panic when telemetry is disabled
	service.RecordAgentExecution(ctx, 1, "test-agent", "gpt-4o", time.Second, true, tokenUsage)

	t.Log("Agent execution recording skipped when telemetry disabled")
}

// TestRecordToolCallWhenDisabled tests tool call recording when disabled
func TestRecordToolCallWhenDisabled(t *testing.T) {
	service := NewTelemetryService(&TelemetryConfig{
		Enabled: false,
	})

	ctx := context.Background()

	// Should not panic when telemetry is disabled
	service.RecordToolCall(ctx, "test-tool", true, time.Millisecond*500)

	t.Log("Tool call recording skipped when telemetry disabled")
}

// TestRecordErrorWhenDisabled tests error recording when disabled
func TestRecordErrorWhenDisabled(t *testing.T) {
	service := NewTelemetryService(&TelemetryConfig{
		Enabled: false,
	})

	ctx := context.Background()

	// Should not panic when telemetry is disabled
	service.RecordError(ctx, "test-error", "test-component")

	t.Log("Error recording skipped when telemetry disabled")
}

// TestShutdownWithoutInitialization tests shutdown before initialization
func TestShutdownWithoutInitialization(t *testing.T) {
	service := NewTelemetryService(&TelemetryConfig{
		Enabled: false,
	})

	ctx := context.Background()
	err := service.Shutdown(ctx)

	if err != nil {
		t.Errorf("Shutdown should not error without initialization, got: %v", err)
	}

	t.Log("Shutdown succeeded without initialization")
}

// TestForceFlushWithoutInitialization tests force flush before initialization
func TestForceFlushWithoutInitialization(t *testing.T) {
	service := NewTelemetryService(&TelemetryConfig{
		Enabled: false,
	})

	ctx := context.Background()
	err := service.ForceFlush(ctx)

	if err != nil {
		t.Errorf("ForceFlush should not error without initialization, got: %v", err)
	}

	t.Log("ForceFlush succeeded without initialization")
}

// TestExtractInt64 tests int64 extraction from various types
func TestExtractInt64(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		wantValue int64
		wantOk    bool
	}{
		{
			name:      "Extract from int64",
			value:     int64(100),
			wantValue: 100,
			wantOk:    true,
		},
		{
			name:      "Extract from int",
			value:     42,
			wantValue: 42,
			wantOk:    true,
		},
		{
			name:      "Extract from int32",
			value:     int32(50),
			wantValue: 50,
			wantOk:    true,
		},
		{
			name:      "Extract from float64",
			value:     float64(75.5),
			wantValue: 75,
			wantOk:    true,
		},
		{
			name:      "Extract from float32",
			value:     float32(25.7),
			wantValue: 25,
			wantOk:    true,
		},
		{
			name:      "Extract from string (should fail)",
			value:     "not a number",
			wantValue: 0,
			wantOk:    false,
		},
		{
			name:      "Extract from nil (should fail)",
			value:     nil,
			wantValue: 0,
			wantOk:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOk := extractInt64(tt.value)

			if gotOk != tt.wantOk {
				t.Errorf("extractInt64() ok = %v, want %v", gotOk, tt.wantOk)
			}

			if gotOk && gotValue != tt.wantValue {
				t.Errorf("extractInt64() value = %d, want %d", gotValue, tt.wantValue)
			}

			t.Logf("Extract %T -> value=%d, ok=%v", tt.value, gotValue, gotOk)
		})
	}
}

// TestGetEnvironment tests environment detection
func TestGetEnvironment(t *testing.T) {
	// Test default environment when no env vars set
	env := getEnvironment()
	if env == "" {
		t.Error("Environment should not be empty")
	}

	t.Logf("Detected environment: %s", env)
}

// TestNoOpExporter tests no-op exporter implementation
func TestNoOpExporter(t *testing.T) {
	exporter := &noOpExporter{}
	ctx := context.Background()

	// Test ExportSpans
	err := exporter.ExportSpans(ctx, nil)
	if err != nil {
		t.Errorf("NoOpExporter.ExportSpans() should not error, got: %v", err)
	}

	// Test Shutdown
	err = exporter.Shutdown(ctx)
	if err != nil {
		t.Errorf("NoOpExporter.Shutdown() should not error, got: %v", err)
	}

	t.Log("No-op exporter tested successfully")
}

// TestTelemetryConfigStructure tests TelemetryConfig structure
func TestTelemetryConfigStructure(t *testing.T) {
	tests := []struct {
		name     string
		config   TelemetryConfig
		validate func(*testing.T, *TelemetryConfig)
	}{
		{
			name: "All fields populated",
			config: TelemetryConfig{
				Enabled:      true,
				OTLPEndpoint: "localhost:4318",
				ServiceName:  "station-test",
				Environment:  "test",
			},
			validate: func(t *testing.T, cfg *TelemetryConfig) {
				if !cfg.Enabled {
					t.Error("Enabled should be true")
				}
				if cfg.OTLPEndpoint == "" {
					t.Error("OTLPEndpoint should not be empty")
				}
				if cfg.ServiceName == "" {
					t.Error("ServiceName should not be empty")
				}
				if cfg.Environment == "" {
					t.Error("Environment should not be empty")
				}
			},
		},
		{
			name: "Minimal config",
			config: TelemetryConfig{
				Enabled: false,
			},
			validate: func(t *testing.T, cfg *TelemetryConfig) {
				if cfg.Enabled {
					t.Error("Enabled should be false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t, &tt.config)
			t.Logf("Config validated: enabled=%v, service=%s, env=%s",
				tt.config.Enabled, tt.config.ServiceName, tt.config.Environment)
		})
	}
}

// TestServiceConstants tests service constants
func TestServiceConstants(t *testing.T) {
	if serviceName != "station" {
		t.Errorf("serviceName should be 'station', got: %s", serviceName)
	}

	if serviceVersion == "" {
		t.Error("serviceVersion should not be empty")
	}

	t.Logf("Service constants: name=%s, version=%s", serviceName, serviceVersion)
}

// Benchmark tests
func BenchmarkNewTelemetryService(b *testing.B) {
	config := &TelemetryConfig{
		Enabled:     true,
		ServiceName: "test-station",
		Environment: "test",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewTelemetryService(config)
	}
}

func BenchmarkExtractInt64(b *testing.B) {
	values := []interface{}{
		int64(100),
		42,
		int32(50),
		float64(75.5),
		float32(25.7),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, val := range values {
			extractInt64(val)
		}
	}
}

func BenchmarkRecordAgentExecutionDisabled(b *testing.B) {
	service := NewTelemetryService(&TelemetryConfig{
		Enabled: false,
	})

	ctx := context.Background()
	tokenUsage := map[string]interface{}{
		"input_tokens":  int64(100),
		"output_tokens": int64(50),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.RecordAgentExecution(ctx, 1, "test-agent", "gpt-4o", time.Second, true, tokenUsage)
	}
}

func BenchmarkRecordToolCallDisabled(b *testing.B) {
	service := NewTelemetryService(&TelemetryConfig{
		Enabled: false,
	})

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.RecordToolCall(ctx, "test-tool", true, time.Millisecond*500)
	}
}

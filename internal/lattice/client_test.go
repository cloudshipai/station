package lattice

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"station/internal/config"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.LatticeConfig
		wantID   bool
		wantName string
	}{
		{
			name: "generates station ID when empty",
			cfg: config.LatticeConfig{
				StationID:   "",
				StationName: "test-station",
			},
			wantID:   true,
			wantName: "test-station",
		},
		{
			name: "uses provided station ID",
			cfg: config.LatticeConfig{
				StationID:   "custom-id-123",
				StationName: "custom-station",
			},
			wantID:   true,
			wantName: "custom-station",
		},
		{
			name: "uses station ID as name when name is empty",
			cfg: config.LatticeConfig{
				StationID:   "id-only",
				StationName: "",
			},
			wantID:   true,
			wantName: "id-only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg)
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			if tt.wantID && client.StationID() == "" {
				t.Error("NewClient() StationID is empty, want non-empty")
			}

			if tt.cfg.StationID != "" && client.StationID() != tt.cfg.StationID {
				t.Errorf("NewClient() StationID = %v, want %v", client.StationID(), tt.cfg.StationID)
			}

			expectedName := tt.wantName
			if expectedName == "" {
				expectedName = client.StationID()
			}
			if client.StationName() != expectedName {
				t.Errorf("NewClient() StationName = %v, want %v", client.StationName(), expectedName)
			}
		})
	}
}

func TestClientConnectionOptions(t *testing.T) {
	cfg := config.LatticeConfig{
		StationID:   "test-id",
		StationName: "test-station",
		NATS: config.LatticeNATSConfig{
			URL:              "nats://localhost:4222",
			ReconnectWaitSec: 5,
			MaxReconnects:    10,
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.cfg.NATS.ReconnectWaitSec != 5 {
		t.Errorf("ReconnectWaitSec = %v, want 5", client.cfg.NATS.ReconnectWaitSec)
	}

	if client.cfg.NATS.MaxReconnects != 10 {
		t.Errorf("MaxReconnects = %v, want 10", client.cfg.NATS.MaxReconnects)
	}
}

func TestClientNotConnected(t *testing.T) {
	cfg := config.LatticeConfig{
		StationID: "test-id",
		NATS: config.LatticeNATSConfig{
			URL: "nats://localhost:4222",
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.IsConnected() {
		t.Error("IsConnected() = true before Connect(), want false")
	}

	if client.Conn() != nil {
		t.Error("Conn() != nil before Connect(), want nil")
	}

	if client.JetStream() != nil {
		t.Error("JetStream() != nil before Connect(), want nil")
	}
}

func TestClientPublishWithoutConnection(t *testing.T) {
	cfg := config.LatticeConfig{
		StationID: "test-id",
		NATS: config.LatticeNATSConfig{
			URL: "nats://localhost:4222",
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	err = client.Publish("test.subject", []byte("test data"))
	if err == nil {
		t.Error("Publish() without connection should return error")
	}
}

func TestClientSubscribeWithoutConnection(t *testing.T) {
	cfg := config.LatticeConfig{
		StationID: "test-id",
		NATS: config.LatticeNATSConfig{
			URL: "nats://localhost:4222",
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Subscribe("test.subject", func(msg *nats.Msg) {})
	if err == nil {
		t.Error("Subscribe() without connection should return error")
	}
}

func TestClientRequestWithoutConnection(t *testing.T) {
	cfg := config.LatticeConfig{
		StationID: "test-id",
		NATS: config.LatticeNATSConfig{
			URL: "nats://localhost:4222",
		},
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = client.Request("test.subject", []byte("test data"), time.Second)
	if err == nil {
		t.Error("Request() without connection should return error")
	}
}

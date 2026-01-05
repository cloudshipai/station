package lattice

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/nats-io/nats.go"

	"station/internal/config"
)

func getFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func TestEmbeddedServerStartStop(t *testing.T) {
	port := getFreePort(t)
	httpPort := getFreePort(t)

	cfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := NewEmbeddedServer(cfg)

	if server.IsRunning() {
		t.Error("IsRunning() = true before Start(), want false")
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !server.IsRunning() {
		t.Error("IsRunning() = false after Start(), want true")
	}

	clientURL := server.ClientURL()
	expectedClientURL := "nats://127.0.0.1:" + strconv.Itoa(port)
	if clientURL != expectedClientURL {
		t.Errorf("ClientURL() = %v, want %v", clientURL, expectedClientURL)
	}

	monitorURL := server.MonitoringURL()
	expectedMonitorURL := "http://127.0.0.1:" + strconv.Itoa(httpPort)
	if monitorURL != expectedMonitorURL {
		t.Errorf("MonitoringURL() = %v, want %v", monitorURL, expectedMonitorURL)
	}

	conn, err := nats.Connect(clientURL)
	if err != nil {
		t.Fatalf("Failed to connect to embedded NATS: %v", err)
	}
	conn.Close()

	server.Shutdown()

	if server.IsRunning() {
		t.Error("IsRunning() = true after Shutdown(), want false")
	}
}

func TestEmbeddedServerJetStream(t *testing.T) {
	port := getFreePort(t)
	httpPort := getFreePort(t)

	cfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := NewEmbeddedServer(cfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Shutdown()

	conn, err := nats.Connect(server.ClientURL())
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	js, err := conn.JetStream()
	if err != nil {
		t.Fatalf("JetStream() error = %v", err)
	}

	_, err = js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket: "test-bucket",
	})
	if err != nil {
		t.Fatalf("CreateKeyValue() error = %v", err)
	}

	kv, err := js.KeyValue("test-bucket")
	if err != nil {
		t.Fatalf("KeyValue() error = %v", err)
	}

	_, err = kv.Put("key1", []byte("value1"))
	if err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	entry, err := kv.Get("key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if string(entry.Value()) != "value1" {
		t.Errorf("Value = %v, want value1", string(entry.Value()))
	}
}

func TestEmbeddedServerDoubleShutdown(t *testing.T) {
	port := getFreePort(t)
	httpPort := getFreePort(t)

	cfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := NewEmbeddedServer(cfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	server.Shutdown()
	server.Shutdown()

	if server.IsRunning() {
		t.Error("IsRunning() = true after double Shutdown(), want false")
	}
}

func TestEmbeddedServerClientConnection(t *testing.T) {
	port := getFreePort(t)
	httpPort := getFreePort(t)

	cfg := config.LatticeEmbeddedNATSConfig{
		Port:     port,
		HTTPPort: httpPort,
		StoreDir: t.TempDir(),
	}

	server := NewEmbeddedServer(cfg)
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer server.Shutdown()

	latticeCfg := config.LatticeConfig{
		StationID: "test-station",
		NATS: config.LatticeNATSConfig{
			URL: server.ClientURL(),
		},
	}

	client, err := NewClient(latticeCfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if err := client.Connect(); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer client.Close()

	if !client.IsConnected() {
		t.Error("IsConnected() = false after Connect(), want true")
	}

	received := make(chan string, 1)
	_, err = client.Subscribe("test.topic", func(msg *nats.Msg) {
		received <- string(msg.Data)
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if err := client.Publish("test.topic", []byte("hello")); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	select {
	case msg := <-received:
		if msg != "hello" {
			t.Errorf("Received message = %v, want hello", msg)
		}
	case <-time.After(time.Second):
		t.Error("Timeout waiting for message")
	}
}

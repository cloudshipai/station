package lattice

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"

	"station/internal/config"
)

type EmbeddedServer struct {
	cfg    config.LatticeEmbeddedNATSConfig
	server *natsserver.Server
}

func NewEmbeddedServer(cfg config.LatticeEmbeddedNATSConfig) *EmbeddedServer {
	return &EmbeddedServer{cfg: cfg}
}

func (e *EmbeddedServer) Start() error {
	port := e.cfg.Port
	if port == 0 {
		port = 4222
	}

	httpPort := e.cfg.HTTPPort
	if httpPort == 0 {
		httpPort = 8222
	}

	storeDir := e.cfg.StoreDir
	if storeDir == "" {
		dataDir, err := defaultDataDir()
		if err != nil {
			return fmt.Errorf("failed to determine data directory: %w", err)
		}
		storeDir = filepath.Join(dataDir, "nats")
	}

	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return fmt.Errorf("failed to create NATS store directory %s: %w", storeDir, err)
	}

	opts := &natsserver.Options{
		Host:         "0.0.0.0",
		Port:         port,
		HTTPPort:     httpPort,
		JetStream:    true,
		StoreDir:     storeDir,
		MaxPayload:   8 * 1024 * 1024,
		ServerName:   "station-lattice-orchestrator",
		Debug:        false,
		Trace:        false,
		Logtime:      true,
		NoLog:        false,
		NoSigs:       true,
		PingInterval: 2 * time.Minute,
		MaxPingsOut:  2,
	}

	if e.cfg.Auth.Enabled {
		if e.cfg.Auth.Token != "" {
			opts.Authorization = e.cfg.Auth.Token
			fmt.Printf("[lattice] NATS auth enabled: token-based\n")
		} else if len(e.cfg.Auth.Users) > 0 {
			var users []*natsserver.User
			for _, u := range e.cfg.Auth.Users {
				users = append(users, &natsserver.User{
					Username: u.User,
					Password: u.Password,
				})
			}
			opts.Users = users
			fmt.Printf("[lattice] NATS auth enabled: %d user(s) configured\n", len(users))
		}
	}

	server, err := natsserver.NewServer(opts)
	if err != nil {
		return fmt.Errorf("failed to create embedded NATS server: %w", err)
	}

	server.ConfigureLogger()

	go server.Start()

	if !server.ReadyForConnections(10 * time.Second) {
		server.Shutdown()
		return fmt.Errorf("embedded NATS server failed to start within timeout")
	}

	e.server = server
	fmt.Printf("[lattice] Embedded NATS server started on port %d (HTTP monitoring: %d)\n", port, httpPort)
	fmt.Printf("[lattice] JetStream storage: %s\n", storeDir)

	return nil
}

func (e *EmbeddedServer) Shutdown() {
	if e.server != nil {
		e.server.Shutdown()
		e.server.WaitForShutdown()
		e.server = nil
		fmt.Println("[lattice] Embedded NATS server shutdown complete")
	}
}

func (e *EmbeddedServer) IsRunning() bool {
	return e.server != nil && e.server.Running()
}

func (e *EmbeddedServer) ClientURL() string {
	if e.server == nil {
		return ""
	}
	port := e.cfg.Port
	if port == 0 {
		port = 4222
	}
	return fmt.Sprintf("nats://127.0.0.1:%d", port)
}

func (e *EmbeddedServer) MonitoringURL() string {
	if e.server == nil {
		return ""
	}
	httpPort := e.cfg.HTTPPort
	if httpPort == 0 {
		httpPort = 8222
	}
	return fmt.Sprintf("http://127.0.0.1:%d", httpPort)
}

func (e *EmbeddedServer) Server() *natsserver.Server {
	return e.server
}

func defaultDataDir() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome != "" {
		return filepath.Join(dataHome, "station", "lattice"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".local", "share", "station", "lattice"), nil
}

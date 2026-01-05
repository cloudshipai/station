package lattice

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"station/internal/config"
)

type Client struct {
	cfg       config.LatticeConfig
	stationID string

	mu   sync.RWMutex
	conn *nats.Conn
	js   nats.JetStreamContext
}

func NewClient(cfg config.LatticeConfig) (*Client, error) {
	stationID := cfg.StationID
	if stationID == "" {
		stationID = uuid.New().String()
	}

	return &Client{
		cfg:       cfg,
		stationID: stationID,
	}, nil
}

func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && c.conn.IsConnected() {
		return nil
	}

	opts, err := c.buildConnectionOptions()
	if err != nil {
		return fmt.Errorf("failed to build connection options: %w", err)
	}

	conn, err := nats.Connect(c.cfg.NATS.URL, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to lattice NATS at %s: %w", c.cfg.NATS.URL, err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to initialize JetStream: %w", err)
	}

	c.conn = conn
	c.js = js

	return nil
}

func (c *Client) buildConnectionOptions() ([]nats.Option, error) {
	opts := []nats.Option{
		nats.Name(fmt.Sprintf("station-%s", c.stationID)),
	}

	reconnectWait := time.Duration(c.cfg.NATS.ReconnectWaitSec) * time.Second
	if reconnectWait == 0 {
		reconnectWait = 2 * time.Second
	}
	opts = append(opts, nats.ReconnectWait(reconnectWait))

	maxReconnects := c.cfg.NATS.MaxReconnects
	if maxReconnects == 0 {
		maxReconnects = -1
	}
	opts = append(opts, nats.MaxReconnects(maxReconnects))

	opts = append(opts, nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
		if err != nil {
			fmt.Printf("[lattice] Disconnected from NATS: %v\n", err)
		}
	}))

	opts = append(opts, nats.ReconnectHandler(func(nc *nats.Conn) {
		fmt.Printf("[lattice] Reconnected to NATS at %s\n", nc.ConnectedUrl())
	}))

	opts = append(opts, nats.ClosedHandler(func(nc *nats.Conn) {
		fmt.Printf("[lattice] NATS connection closed\n")
	}))

	authOpts, err := c.buildAuthOptions()
	if err != nil {
		return nil, err
	}
	opts = append(opts, authOpts...)

	tlsOpts, err := c.buildTLSOptions()
	if err != nil {
		return nil, err
	}
	opts = append(opts, tlsOpts...)

	return opts, nil
}

func (c *Client) buildAuthOptions() ([]nats.Option, error) {
	auth := c.cfg.NATS.Auth
	var opts []nats.Option

	if auth.CredsFile != "" {
		opts = append(opts, nats.UserCredentials(auth.CredsFile))
		return opts, nil
	}

	if auth.NKeyFile != "" {
		opt, err := nats.NkeyOptionFromSeed(auth.NKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load NKey from file %s: %w", auth.NKeyFile, err)
		}
		opts = append(opts, opt)
		return opts, nil
	}

	if auth.NKeySeed != "" {
		opt, err := nats.NkeyOptionFromSeed(auth.NKeySeed)
		if err != nil {
			return nil, fmt.Errorf("failed to parse NKey seed: %w", err)
		}
		opts = append(opts, opt)
		return opts, nil
	}

	if auth.Token != "" {
		opts = append(opts, nats.Token(auth.Token))
		return opts, nil
	}

	if auth.User != "" {
		opts = append(opts, nats.UserInfo(auth.User, auth.Password))
		return opts, nil
	}

	return opts, nil
}

func (c *Client) buildTLSOptions() ([]nats.Option, error) {
	tlsCfg := c.cfg.NATS.TLS
	if !tlsCfg.Enabled {
		return nil, nil
	}

	var opts []nats.Option

	config := &tls.Config{
		InsecureSkipVerify: tlsCfg.SkipVerify,
	}

	if tlsCfg.CertFile != "" && tlsCfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	}

	if tlsCfg.CAFile != "" {
		caCert, err := os.ReadFile(tlsCfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		config.RootCAs = caCertPool
	}

	opts = append(opts, nats.Secure(config))

	return opts, nil
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.js = nil
	}
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.conn != nil && c.conn.IsConnected()
}

func (c *Client) StationID() string {
	return c.stationID
}

func (c *Client) StationName() string {
	if c.cfg.StationName != "" {
		return c.cfg.StationName
	}
	return c.stationID
}

func (c *Client) Conn() *nats.Conn {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn
}

func (c *Client) JetStream() nats.JetStreamContext {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.js
}

func (c *Client) Publish(subject string, data []byte) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected to lattice")
	}
	return conn.Publish(subject, data)
}

func (c *Client) Subscribe(subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected to lattice")
	}
	return conn.Subscribe(subject, handler)
}

func (c *Client) QueueSubscribe(subject, queue string, handler nats.MsgHandler) (*nats.Subscription, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected to lattice")
	}
	return conn.QueueSubscribe(subject, queue, handler)
}

func (c *Client) Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return nil, fmt.Errorf("not connected to lattice")
	}
	return conn.Request(subject, data, timeout)
}

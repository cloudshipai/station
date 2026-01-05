package lattice

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	PresenceSubject   = "lattice.presence"
	PresenceHeartbeat = "lattice.presence.heartbeat"
	PresenceAnnounce  = "lattice.presence.announce"
	PresenceGoodbye   = "lattice.presence.goodbye"
)

type PresenceMessage struct {
	StationID   string           `json:"station_id"`
	StationName string           `json:"station_name"`
	Type        PresenceType     `json:"type"`
	Timestamp   time.Time        `json:"timestamp"`
	Manifest    *StationManifest `json:"manifest,omitempty"`
}

type PresenceType string

const (
	PresenceTypeHeartbeat PresenceType = "heartbeat"
	PresenceTypeAnnounce  PresenceType = "announce"
	PresenceTypeGoodbye   PresenceType = "goodbye"
)

type Presence struct {
	client   *Client
	registry *Registry
	manifest StationManifest

	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	running  bool
	interval time.Duration
}

func NewPresence(client *Client, registry *Registry, manifest StationManifest, intervalSec int) *Presence {
	interval := time.Duration(intervalSec) * time.Second
	if interval == 0 {
		interval = 10 * time.Second
	}

	return &Presence{
		client:   client,
		registry: registry,
		manifest: manifest,
		interval: interval,
	}
}

func (p *Presence) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return nil
	}

	p.ctx, p.cancel = context.WithCancel(ctx)
	p.running = true
	p.mu.Unlock()

	if err := p.announce(); err != nil {
		return fmt.Errorf("failed to announce presence: %w", err)
	}

	go p.heartbeatLoop()

	if err := p.subscribeToPresence(); err != nil {
		fmt.Printf("[presence] Warning: failed to subscribe to presence updates: %v\n", err)
	}

	return nil
}

func (p *Presence) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}

	p.goodbye()
	p.cancel()
	p.running = false
}

func (p *Presence) announce() error {
	msg := PresenceMessage{
		StationID:   p.manifest.StationID,
		StationName: p.manifest.StationName,
		Type:        PresenceTypeAnnounce,
		Timestamp:   time.Now(),
		Manifest:    &p.manifest,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return p.client.Publish(PresenceAnnounce, data)
}

func (p *Presence) goodbye() {
	msg := PresenceMessage{
		StationID:   p.manifest.StationID,
		StationName: p.manifest.StationName,
		Type:        PresenceTypeGoodbye,
		Timestamp:   time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	_ = p.client.Publish(PresenceGoodbye, data)
}

func (p *Presence) heartbeatLoop() {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			if err := p.sendHeartbeat(); err != nil {
				fmt.Printf("[presence] Warning: heartbeat failed: %v\n", err)
			}
		}
	}
}

func (p *Presence) sendHeartbeat() error {
	msg := PresenceMessage{
		StationID:   p.manifest.StationID,
		StationName: p.manifest.StationName,
		Type:        PresenceTypeHeartbeat,
		Timestamp:   time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return p.client.Publish(PresenceHeartbeat, data)
}

func (p *Presence) subscribeToPresence() error {
	_, err := p.client.Subscribe(PresenceAnnounce, func(msg *nats.Msg) {
		var presence PresenceMessage
		if err := json.Unmarshal(msg.Data, &presence); err != nil {
			return
		}

		if presence.StationID == p.manifest.StationID {
			return
		}

		if presence.Manifest != nil && p.registry != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := p.registry.RegisterStation(ctx, *presence.Manifest); err != nil {
				fmt.Printf("[presence] Warning: failed to register station %s: %v\n",
					presence.StationName, err)
			} else {
				fmt.Printf("[presence] Station joined: %s (%s)\n",
					presence.StationName, presence.StationID)
			}
		}
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to announcements: %w", err)
	}

	_, err = p.client.Subscribe(PresenceGoodbye, func(msg *nats.Msg) {
		var presence PresenceMessage
		if err := json.Unmarshal(msg.Data, &presence); err != nil {
			return
		}

		if presence.StationID == p.manifest.StationID {
			return
		}

		if p.registry != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := p.registry.UnregisterStation(ctx, presence.StationID); err != nil {
				fmt.Printf("[presence] Warning: failed to unregister station %s: %v\n",
					presence.StationName, err)
			} else {
				fmt.Printf("[presence] Station left: %s (%s)\n",
					presence.StationName, presence.StationID)
			}
		}
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to goodbyes: %w", err)
	}

	_, err = p.client.Subscribe(PresenceHeartbeat, func(msg *nats.Msg) {
		var presence PresenceMessage
		if err := json.Unmarshal(msg.Data, &presence); err != nil {
			return
		}

		if presence.StationID == p.manifest.StationID {
			return
		}

		if p.registry != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			_ = p.registry.UpdateStationStatus(ctx, presence.StationID, StatusOnline)
		}
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to heartbeats: %w", err)
	}

	return nil
}

func (p *Presence) UpdateManifest(manifest StationManifest) {
	p.mu.Lock()
	p.manifest = manifest
	p.mu.Unlock()

	if p.registry != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.registry.RegisterStation(ctx, manifest)
	}

	_ = p.announce()
}

package stream

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

type NATSPublisher struct {
	nc        *nats.Conn
	js        nats.JetStreamContext
	stationID string
	useJS     bool
}

type NATSPublisherConfig struct {
	StationID    string
	UseJetStream bool
}

func NewNATSPublisher(nc *nats.Conn, cfg NATSPublisherConfig) (*NATSPublisher, error) {
	p := &NATSPublisher{
		nc:        nc,
		stationID: cfg.StationID,
		useJS:     cfg.UseJetStream,
	}

	if cfg.UseJetStream {
		js, err := nc.JetStream()
		if err != nil {
			return nil, fmt.Errorf("failed to get JetStream context: %w", err)
		}
		p.js = js
	}

	return p, nil
}

func (p *NATSPublisher) subject(runID string) string {
	return fmt.Sprintf("station.%s.run.%s.stream", p.stationID, runID)
}

func (p *NATSPublisher) Publish(ctx context.Context, event *Event) error {
	data, err := event.JSON()
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	subject := p.subject(event.RunUUID)

	if p.useJS && p.js != nil {
		_, err = p.js.Publish(subject, data)
	} else {
		err = p.nc.Publish(subject, data)
	}

	if err != nil {
		return fmt.Errorf("failed to publish to %s: %w", subject, err)
	}

	return nil
}

func (p *NATSPublisher) Close() error {
	return nil
}

type NATSSubscriber struct {
	nc        *nats.Conn
	stationID string
}

func NewNATSSubscriber(nc *nats.Conn, stationID string) *NATSSubscriber {
	return &NATSSubscriber{
		nc:        nc,
		stationID: stationID,
	}
}

func (s *NATSSubscriber) SubscribeRun(ctx context.Context, runID string, handler func(*Event)) (*nats.Subscription, error) {
	subject := fmt.Sprintf("station.%s.run.%s.stream", s.stationID, runID)

	sub, err := s.nc.Subscribe(subject, func(msg *nats.Msg) {
		var event Event
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}
		handler(&event)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}

	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
	}()

	return sub, nil
}

func (s *NATSSubscriber) SubscribeStation(ctx context.Context, handler func(*Event)) (*nats.Subscription, error) {
	subject := fmt.Sprintf("station.%s.run.*.stream", s.stationID)

	sub, err := s.nc.Subscribe(subject, func(msg *nats.Msg) {
		var event Event
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			return
		}
		handler(&event)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}

	go func() {
		<-ctx.Done()
		sub.Unsubscribe()
	}()

	return sub, nil
}

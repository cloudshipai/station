package events

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// StreamConfig configures the JetStream event stream
type StreamConfig struct {
	// StreamName is the name of the JetStream stream
	StreamName string

	// Subjects are the NATS subjects the stream captures
	Subjects []string

	// MaxAge is how long to retain events (default: 7 days)
	MaxAge time.Duration

	// MaxBytes is the maximum size of the stream (default: 1GB)
	MaxBytes int64

	// MaxMsgs is the maximum number of messages (default: unlimited)
	MaxMsgs int64

	// Replicas for HA (default: 1 for single node)
	Replicas int

	// Storage type (file or memory)
	Storage nats.StorageType
}

// DefaultStreamConfig returns sensible defaults for the event stream
func DefaultStreamConfig() StreamConfig {
	return StreamConfig{
		StreamName: "LATTICE_EVENTS",
		Subjects: []string{
			"lattice.events.>", // All lattice events
		},
		MaxAge:   7 * 24 * time.Hour, // 7 days retention
		MaxBytes: 1 << 30,            // 1GB
		MaxMsgs:  -1,                 // Unlimited messages
		Replicas: 1,
		Storage:  nats.FileStorage,
	}
}

// Stream manages the JetStream event stream for lattice events
type Stream struct {
	js     nats.JetStreamContext
	config StreamConfig
	info   *nats.StreamInfo
}

// NewStream creates a new event stream manager
func NewStream(js nats.JetStreamContext, config StreamConfig) (*Stream, error) {
	if js == nil {
		return nil, fmt.Errorf("JetStream context is required")
	}

	if config.StreamName == "" {
		config = DefaultStreamConfig()
	}

	return &Stream{
		js:     js,
		config: config,
	}, nil
}

// EnsureStream creates or updates the stream configuration
func (s *Stream) EnsureStream(ctx context.Context) error {
	streamConfig := &nats.StreamConfig{
		Name:        s.config.StreamName,
		Description: "Station Lattice Events (CloudEvents format)",
		Subjects:    s.config.Subjects,
		MaxAge:      s.config.MaxAge,
		MaxBytes:    s.config.MaxBytes,
		MaxMsgs:     s.config.MaxMsgs,
		Replicas:    s.config.Replicas,
		Storage:     s.config.Storage,
		Retention:   nats.LimitsPolicy, // Delete old messages based on limits
		Discard:     nats.DiscardOld,   // Discard oldest when full
		Duplicates:  5 * time.Minute,   // Dedupe window
		AllowRollup: false,             // No rollup for audit trail
		DenyDelete:  true,              // Append-only for audit
		DenyPurge:   true,              // No purge for audit
	}

	// Try to get existing stream
	info, err := s.js.StreamInfo(s.config.StreamName)
	if err != nil {
		if err == nats.ErrStreamNotFound {
			// Create new stream
			info, err = s.js.AddStream(streamConfig)
			if err != nil {
				return fmt.Errorf("failed to create event stream: %w", err)
			}
			fmt.Printf("[events] Created stream %s\n", s.config.StreamName)
		} else {
			return fmt.Errorf("failed to get stream info: %w", err)
		}
	} else {
		// Update existing stream
		info, err = s.js.UpdateStream(streamConfig)
		if err != nil {
			return fmt.Errorf("failed to update event stream: %w", err)
		}
		fmt.Printf("[events] Updated stream %s (msgs: %d, bytes: %d)\n",
			s.config.StreamName, info.State.Msgs, info.State.Bytes)
	}

	s.info = info
	return nil
}

// Info returns the current stream info
func (s *Stream) Info() (*nats.StreamInfo, error) {
	return s.js.StreamInfo(s.config.StreamName)
}

// Stats returns stream statistics
func (s *Stream) Stats() (*StreamStats, error) {
	info, err := s.Info()
	if err != nil {
		return nil, err
	}

	return &StreamStats{
		StreamName:    info.Config.Name,
		Messages:      info.State.Msgs,
		Bytes:         info.State.Bytes,
		FirstSeq:      info.State.FirstSeq,
		LastSeq:       info.State.LastSeq,
		FirstTime:     info.State.FirstTime,
		LastTime:      info.State.LastTime,
		ConsumerCount: info.State.Consumers,
	}, nil
}

// StreamStats provides event stream statistics
type StreamStats struct {
	StreamName    string    `json:"stream_name"`
	Messages      uint64    `json:"messages"`
	Bytes         uint64    `json:"bytes"`
	FirstSeq      uint64    `json:"first_seq"`
	LastSeq       uint64    `json:"last_seq"`
	FirstTime     time.Time `json:"first_time"`
	LastTime      time.Time `json:"last_time"`
	ConsumerCount int       `json:"consumer_count"`
}

// ConsumerConfig configures a durable consumer for the event stream
type ConsumerConfig struct {
	Name          string
	Description   string
	FilterSubject string // Optional: filter to specific event types
	DeliverPolicy nats.DeliverPolicy
	AckPolicy     nats.AckPolicy
	MaxAckPending int
	MaxDeliver    int
}

// CreateConsumer creates a durable consumer for reading events
func (s *Stream) CreateConsumer(ctx context.Context, config ConsumerConfig) (*nats.ConsumerInfo, error) {
	consumerConfig := &nats.ConsumerConfig{
		Durable:       config.Name,
		Description:   config.Description,
		FilterSubject: config.FilterSubject,
		DeliverPolicy: config.DeliverPolicy,
		AckPolicy:     config.AckPolicy,
		MaxAckPending: config.MaxAckPending,
		MaxDeliver:    config.MaxDeliver,
	}

	if consumerConfig.DeliverPolicy == 0 {
		consumerConfig.DeliverPolicy = nats.DeliverAllPolicy
	}
	if consumerConfig.AckPolicy == 0 {
		consumerConfig.AckPolicy = nats.AckExplicitPolicy
	}
	if consumerConfig.MaxAckPending == 0 {
		consumerConfig.MaxAckPending = 1000
	}
	if consumerConfig.MaxDeliver == 0 {
		consumerConfig.MaxDeliver = 5
	}

	info, err := s.js.AddConsumer(s.config.StreamName, consumerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer %s: %w", config.Name, err)
	}

	return info, nil
}

// Subscribe creates a push-based subscription for events
func (s *Stream) Subscribe(
	ctx context.Context,
	subject string,
	handler func(*CloudEvent) error,
) (*nats.Subscription, error) {
	msgHandler := func(msg *nats.Msg) {
		var event CloudEvent
		if err := event.UnmarshalJSON(msg.Data); err != nil {
			fmt.Printf("[events] Failed to unmarshal event: %v\n", err)
			return
		}

		if err := handler(&event); err != nil {
			fmt.Printf("[events] Handler error for %s: %v\n", event.Type, err)
			// NAK for retry
			if err := msg.Nak(); err != nil {
				fmt.Printf("[events] Failed to NAK: %v\n", err)
			}
			return
		}

		// ACK on success
		if err := msg.Ack(); err != nil {
			fmt.Printf("[events] Failed to ACK: %v\n", err)
		}
	}

	sub, err := s.js.Subscribe(subject, msgHandler, nats.ManualAck())
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}

	return sub, nil
}

func (s *Stream) Query(ctx context.Context, opts QueryOptions) ([]*CloudEvent, error) {
	subOpts := []nats.SubOpt{nats.AckNone()}

	if opts.StartSeq > 0 {
		subOpts = append(subOpts, nats.StartSequence(opts.StartSeq))
	} else if !opts.StartTime.IsZero() {
		subOpts = append(subOpts, nats.StartTime(opts.StartTime))
	} else {
		subOpts = append(subOpts, nats.DeliverAll())
	}

	sub, err := s.js.SubscribeSync(opts.Subject, subOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create query subscription: %w", err)
	}
	defer sub.Unsubscribe()

	var events []*CloudEvent
	limit := opts.Limit
	if limit == 0 {
		limit = 100
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	deadline := time.Now().Add(timeout)
	for len(events) < limit && time.Now().Before(deadline) {
		msg, err := sub.NextMsg(100 * time.Millisecond)
		if err != nil {
			if err == nats.ErrTimeout {
				continue
			}
			break
		}

		var event CloudEvent
		if err := event.UnmarshalJSON(msg.Data); err != nil {
			continue
		}

		// Apply type filter
		if len(opts.Types) > 0 {
			matched := false
			for _, t := range opts.Types {
				if event.Type == t {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// Apply end time filter
		if !opts.EndTime.IsZero() && event.Time.After(opts.EndTime) {
			break
		}

		events = append(events, &event)
	}

	return events, nil
}

// QueryOptions configures event queries
type QueryOptions struct {
	Subject   string        // NATS subject pattern
	Types     []string      // Filter by event types
	StartSeq  uint64        // Start from sequence number
	StartTime time.Time     // Start from time
	EndTime   time.Time     // End at time
	Limit     int           // Max events to return
	Timeout   time.Duration // Query timeout
}

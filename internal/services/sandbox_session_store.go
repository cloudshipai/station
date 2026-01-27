package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	SessionStoreBucket = "sandbox-sessions"
	SessionStoreTTL    = 24 * time.Hour
)

type SessionRecord struct {
	SessionID  string            `json:"session_id"`
	Key        SessionKey        `json:"key"`
	MachineID  string            `json:"machine_id,omitempty"`
	AppName    string            `json:"app_name,omitempty"`
	Image      string            `json:"image"`
	Workdir    string            `json:"workdir"`
	Backend    string            `json:"backend"`
	CreatedAt  time.Time         `json:"created_at"`
	LastUsedAt time.Time         `json:"last_used_at"`
	Env        map[string]string `json:"env,omitempty"`
	Limits     ResourceLimits    `json:"limits"`
}

type SessionStore interface {
	Put(ctx context.Context, key SessionKey, record *SessionRecord) error
	Get(ctx context.Context, key SessionKey) (*SessionRecord, error)
	GetBySessionID(ctx context.Context, sessionID string) (*SessionRecord, error)
	Delete(ctx context.Context, key SessionKey) error
	DeleteByPrefix(ctx context.Context, prefix string) (int, error)
	List(ctx context.Context) ([]*SessionRecord, error)
	ListByPrefix(ctx context.Context, prefix string) ([]*SessionRecord, error)
	UpdateLastUsed(ctx context.Context, key SessionKey) error
}

type NATSSessionStore struct {
	kv nats.KeyValue
}

type SessionStoreConfig struct {
	Replicas int
	History  int
	TTL      time.Duration
}

func DefaultSessionStoreConfig() SessionStoreConfig {
	return SessionStoreConfig{
		Replicas: 1,
		History:  5,
		TTL:      SessionStoreTTL,
	}
}

func NewNATSSessionStore(js nats.JetStreamContext, config SessionStoreConfig) (*NATSSessionStore, error) {
	if js == nil {
		return nil, fmt.Errorf("JetStream context is required")
	}

	if config.Replicas <= 0 {
		config.Replicas = 1
	}
	if config.History <= 0 {
		config.History = 5
	}
	if config.TTL <= 0 {
		config.TTL = SessionStoreTTL
	}

	kv, err := js.KeyValue(SessionStoreBucket)
	if err == nats.ErrBucketNotFound {
		kv, err = js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket:   SessionStoreBucket,
			Replicas: config.Replicas,
			History:  uint8(config.History),
			TTL:      config.TTL,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("create/get session KV bucket: %w", err)
	}

	return &NATSSessionStore{kv: kv}, nil
}

func (s *NATSSessionStore) Put(ctx context.Context, key SessionKey, record *SessionRecord) error {
	record.Key = key
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	record.LastUsedAt = time.Now()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal session record: %w", err)
	}

	kvKey := s.keyFromSessionKey(key)
	_, err = s.kv.Put(kvKey, data)
	if err != nil {
		return fmt.Errorf("put session record: %w", err)
	}

	if record.SessionID != "" {
		_, _ = s.kv.Put(s.sessionIDIndexKey(record.SessionID), []byte(kvKey))
	}

	return nil
}

func (s *NATSSessionStore) Get(ctx context.Context, key SessionKey) (*SessionRecord, error) {
	kvKey := s.keyFromSessionKey(key)
	entry, err := s.kv.Get(kvKey)
	if err == nats.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session record: %w", err)
	}

	var record SessionRecord
	if err := json.Unmarshal(entry.Value(), &record); err != nil {
		return nil, fmt.Errorf("unmarshal session record: %w", err)
	}

	return &record, nil
}

func (s *NATSSessionStore) GetBySessionID(ctx context.Context, sessionID string) (*SessionRecord, error) {
	indexKey := s.sessionIDIndexKey(sessionID)
	entry, err := s.kv.Get(indexKey)
	if err == nats.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session index: %w", err)
	}

	kvKey := string(entry.Value())
	mainEntry, err := s.kv.Get(kvKey)
	if err == nats.ErrKeyNotFound {
		_ = s.kv.Delete(indexKey)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get session record by ID: %w", err)
	}

	var record SessionRecord
	if err := json.Unmarshal(mainEntry.Value(), &record); err != nil {
		return nil, fmt.Errorf("unmarshal session record: %w", err)
	}

	return &record, nil
}

func (s *NATSSessionStore) Delete(ctx context.Context, key SessionKey) error {
	kvKey := s.keyFromSessionKey(key)

	entry, err := s.kv.Get(kvKey)
	if err == nil {
		var record SessionRecord
		if json.Unmarshal(entry.Value(), &record) == nil && record.SessionID != "" {
			_ = s.kv.Delete(s.sessionIDIndexKey(record.SessionID))
		}
	}

	err = s.kv.Delete(kvKey)
	if err != nil && err != nats.ErrKeyNotFound {
		return fmt.Errorf("delete session record: %w", err)
	}

	return nil
}

func (s *NATSSessionStore) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
	keys, err := s.kv.Keys()
	if err == nats.ErrNoKeysFound {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("list keys: %w", err)
	}

	deleted := 0
	for _, key := range keys {
		if strings.HasPrefix(key, "session.") && strings.Contains(key, prefix) {
			entry, err := s.kv.Get(key)
			if err == nil {
				var record SessionRecord
				if json.Unmarshal(entry.Value(), &record) == nil && record.SessionID != "" {
					_ = s.kv.Delete(s.sessionIDIndexKey(record.SessionID))
				}
			}

			if err := s.kv.Delete(key); err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}

func (s *NATSSessionStore) List(ctx context.Context) ([]*SessionRecord, error) {
	return s.ListByPrefix(ctx, "")
}

func (s *NATSSessionStore) ListByPrefix(ctx context.Context, prefix string) ([]*SessionRecord, error) {
	keys, err := s.kv.Keys()
	if err == nats.ErrNoKeysFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list keys: %w", err)
	}

	var records []*SessionRecord
	for _, key := range keys {
		if !strings.HasPrefix(key, "session.") {
			continue
		}
		if prefix != "" && !strings.Contains(key, prefix) {
			continue
		}

		entry, err := s.kv.Get(key)
		if err != nil {
			continue
		}

		var record SessionRecord
		if err := json.Unmarshal(entry.Value(), &record); err != nil {
			continue
		}

		records = append(records, &record)
	}

	return records, nil
}

func (s *NATSSessionStore) UpdateLastUsed(ctx context.Context, key SessionKey) error {
	record, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	if record == nil {
		return nil
	}

	record.LastUsedAt = time.Now()
	return s.Put(ctx, key, record)
}

func (s *NATSSessionStore) keyFromSessionKey(key SessionKey) string {
	return fmt.Sprintf("session.%s.%s.%s", key.Namespace, key.ID, key.Key)
}

func (s *NATSSessionStore) sessionIDIndexKey(sessionID string) string {
	return fmt.Sprintf("idx.session_id.%s", sessionID)
}

type InMemorySessionStore struct {
	records map[string]*SessionRecord
}

func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		records: make(map[string]*SessionRecord),
	}
}

func (s *InMemorySessionStore) Put(ctx context.Context, key SessionKey, record *SessionRecord) error {
	record.Key = key
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	record.LastUsedAt = time.Now()
	s.records[key.String()] = record
	return nil
}

func (s *InMemorySessionStore) Get(ctx context.Context, key SessionKey) (*SessionRecord, error) {
	record, ok := s.records[key.String()]
	if !ok {
		return nil, nil
	}
	return record, nil
}

func (s *InMemorySessionStore) GetBySessionID(ctx context.Context, sessionID string) (*SessionRecord, error) {
	for _, record := range s.records {
		if record.SessionID == sessionID {
			return record, nil
		}
	}
	return nil, nil
}

func (s *InMemorySessionStore) Delete(ctx context.Context, key SessionKey) error {
	delete(s.records, key.String())
	return nil
}

func (s *InMemorySessionStore) DeleteByPrefix(ctx context.Context, prefix string) (int, error) {
	deleted := 0
	for k := range s.records {
		if strings.HasPrefix(k, prefix) {
			delete(s.records, k)
			deleted++
		}
	}
	return deleted, nil
}

func (s *InMemorySessionStore) List(ctx context.Context) ([]*SessionRecord, error) {
	records := make([]*SessionRecord, 0, len(s.records))
	for _, r := range s.records {
		records = append(records, r)
	}
	return records, nil
}

func (s *InMemorySessionStore) ListByPrefix(ctx context.Context, prefix string) ([]*SessionRecord, error) {
	var records []*SessionRecord
	for k, r := range s.records {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			records = append(records, r)
		}
	}
	return records, nil
}

func (s *InMemorySessionStore) UpdateLastUsed(ctx context.Context, key SessionKey) error {
	if record, ok := s.records[key.String()]; ok {
		record.LastUsedAt = time.Now()
	}
	return nil
}

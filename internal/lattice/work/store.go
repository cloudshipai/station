package work

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/nats-io/nats.go"
)

type WorkStoreConfig struct {
	Replicas int
	History  int
	TTL      time.Duration
}

func DefaultWorkStoreConfig() WorkStoreConfig {
	return WorkStoreConfig{
		Replicas: 1,
		History:  10,
		TTL:      7 * 24 * time.Hour,
	}
}

type WorkStore struct {
	kv     nats.KeyValue
	config WorkStoreConfig
}

func NewWorkStore(js nats.JetStreamContext, config WorkStoreConfig) (*WorkStore, error) {
	if js == nil {
		return nil, fmt.Errorf("JetStream context is required")
	}

	if config.Replicas <= 0 {
		config.Replicas = 1
	}
	if config.History <= 0 {
		config.History = 10
	}
	if config.TTL <= 0 {
		config.TTL = 7 * 24 * time.Hour
	}

	kv, err := js.KeyValue("lattice-work")
	if err == nats.ErrBucketNotFound {
		kv, err = js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket:   "lattice-work",
			Replicas: config.Replicas,
			History:  uint8(config.History),
			TTL:      config.TTL,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create/get KV bucket: %w", err)
	}

	return &WorkStore{
		kv:     kv,
		config: config,
	}, nil
}

func (s *WorkStore) Assign(ctx context.Context, record *WorkRecord) error {
	record.Status = StatusAssigned
	if record.AssignedAt.IsZero() {
		record.AssignedAt = time.Now()
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal work record: %w", err)
	}

	_, err = s.kv.Put(keyWork(record.WorkID), data)
	if err != nil {
		return fmt.Errorf("failed to put work record: %w", err)
	}

	if record.TargetStation != "" {
		_ = s.addToIndex(keyStationActive(record.TargetStation), record.WorkID)
	}

	if record.OrchestratorRunID != "" {
		_ = s.addToIndex(keyRun(record.OrchestratorRunID), record.WorkID)
	}

	return nil
}

func (s *WorkStore) UpdateStatus(ctx context.Context, workID, status string, result *WorkResult) error {
	key := keyWork(workID)

	entry, err := s.kv.Get(key)
	if err != nil {
		return fmt.Errorf("failed to get work record: %w", err)
	}

	var record WorkRecord
	if err := json.Unmarshal(entry.Value(), &record); err != nil {
		return fmt.Errorf("failed to unmarshal work record: %w", err)
	}

	record.Status = status
	record.UpdatedAt = time.Now()

	switch status {
	case StatusAccepted:
		record.AcceptedAt = time.Now()
	case StatusComplete, StatusFailed, StatusEscalated:
		record.CompletedAt = time.Now()
		if record.TargetStation != "" {
			_ = s.removeFromIndex(keyStationActive(record.TargetStation), workID)
		}
	}

	if result != nil {
		record.Result = result.Result
		record.Error = result.Error
		record.DurationMs = result.DurationMs
		record.ToolCalls = result.ToolCalls
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal updated record: %w", err)
	}

	_, err = s.kv.Update(key, data, entry.Revision())
	if err != nil {
		return fmt.Errorf("failed to update work record: %w", err)
	}

	return nil
}

func (s *WorkStore) Get(ctx context.Context, workID string) (*WorkRecord, error) {
	entry, err := s.kv.Get(keyWork(workID))
	if err != nil {
		if err == nats.ErrKeyNotFound {
			return nil, fmt.Errorf("work %s not found", workID)
		}
		return nil, fmt.Errorf("failed to get work record: %w", err)
	}

	var record WorkRecord
	if err := json.Unmarshal(entry.Value(), &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal work record: %w", err)
	}

	return &record, nil
}

func (s *WorkStore) GetHistory(ctx context.Context, workID string) ([]WorkRecord, error) {
	history, err := s.kv.History(keyWork(workID))
	if err != nil {
		return nil, fmt.Errorf("failed to get work history: %w", err)
	}

	var records []WorkRecord
	for _, entry := range history {
		if entry.Operation() == nats.KeyValueDelete {
			continue
		}
		var record WorkRecord
		if err := json.Unmarshal(entry.Value(), &record); err != nil {
			continue
		}
		records = append(records, record)
	}

	return records, nil
}

func (s *WorkStore) Watch(ctx context.Context, workID string) (<-chan *WorkRecord, error) {
	watcher, err := s.kv.Watch(keyWork(workID))
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	ch := make(chan *WorkRecord, 10)

	go func() {
		defer close(ch)
		defer watcher.Stop()

		for {
			select {
			case entry := <-watcher.Updates():
				if entry == nil {
					continue
				}
				if entry.Operation() == nats.KeyValueDelete {
					continue
				}
				var record WorkRecord
				if err := json.Unmarshal(entry.Value(), &record); err != nil {
					continue
				}
				select {
				case ch <- &record:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

func (s *WorkStore) WatchAll(ctx context.Context) (<-chan *WorkRecord, error) {
	watcher, err := s.kv.Watch("work.*")
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	ch := make(chan *WorkRecord, 100)

	go func() {
		defer close(ch)
		defer watcher.Stop()

		for {
			select {
			case entry := <-watcher.Updates():
				if entry == nil {
					continue
				}
				if entry.Operation() == nats.KeyValueDelete {
					continue
				}
				var record WorkRecord
				if err := json.Unmarshal(entry.Value(), &record); err != nil {
					continue
				}
				select {
				case ch <- &record:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, nil
}

func (s *WorkStore) GetByOrchestrator(ctx context.Context, runID string) ([]*WorkRecord, error) {
	entry, err := s.kv.Get(keyRun(runID))
	if err != nil {
		if err == nats.ErrKeyNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get run index: %w", err)
	}

	var workIDs []string
	if err := json.Unmarshal(entry.Value(), &workIDs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal run index: %w", err)
	}

	var records []*WorkRecord
	for _, workID := range workIDs {
		record, err := s.Get(ctx, workID)
		if err != nil {
			continue
		}
		records = append(records, record)
	}

	return records, nil
}

func (s *WorkStore) GetStationActive(ctx context.Context, stationID string) ([]*WorkRecord, error) {
	entry, err := s.kv.Get(keyStationActive(stationID))
	if err != nil {
		if err == nats.ErrKeyNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get station index: %w", err)
	}

	var workIDs []string
	if err := json.Unmarshal(entry.Value(), &workIDs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal station index: %w", err)
	}

	var records []*WorkRecord
	for _, workID := range workIDs {
		record, err := s.Get(ctx, workID)
		if err != nil {
			continue
		}
		records = append(records, record)
	}

	return records, nil
}

func (s *WorkStore) Delete(ctx context.Context, workID string) error {
	return s.kv.Delete(keyWork(workID))
}

func (s *WorkStore) addToIndex(key, workID string) error {
	var workIDs []string

	entry, err := s.kv.Get(key)
	if err == nil {
		_ = json.Unmarshal(entry.Value(), &workIDs)
	} else if err != nats.ErrKeyNotFound {
		return err
	}

	if slices.Contains(workIDs, workID) {
		return nil
	}

	workIDs = append(workIDs, workID)
	data, _ := json.Marshal(workIDs)

	if entry != nil {
		_, err = s.kv.Update(key, data, entry.Revision())
	} else {
		_, err = s.kv.Create(key, data)
	}
	return err
}

func (s *WorkStore) removeFromIndex(key, workID string) error {
	entry, err := s.kv.Get(key)
	if err != nil {
		return nil
	}

	var workIDs []string
	if err := json.Unmarshal(entry.Value(), &workIDs); err != nil {
		return nil
	}

	var newIDs []string
	for _, id := range workIDs {
		if id != workID {
			newIDs = append(newIDs, id)
		}
	}

	if len(newIDs) == 0 {
		return s.kv.Delete(key)
	}

	data, _ := json.Marshal(newIDs)
	_, err = s.kv.Update(key, data, entry.Revision())
	return err
}

func keyWork(workID string) string {
	return "work." + workID
}

func keyStationActive(stationID string) string {
	return "station." + stationID + ".active"
}

func keyRun(runID string) string {
	return "run." + runID
}

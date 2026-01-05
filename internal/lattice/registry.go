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
	StationsBucket = "lattice-stations"
	AgentsBucket   = "lattice-agents"
)

type AgentNameConflictError struct {
	AgentName        string
	ExistingStation  string
	AttemptedStation string
}

func (e *AgentNameConflictError) Error() string {
	return fmt.Sprintf(
		"agent name '%s' already registered by station '%s', cannot register from '%s'",
		e.AgentName, e.ExistingStation, e.AttemptedStation,
	)
}

type RegistrationResult struct {
	RegisteredAgents  []string
	ConflictingAgents []AgentNameConflictError
	Success           bool
}

type StationManifest struct {
	StationID   string         `json:"station_id"`
	StationName string         `json:"station_name"`
	Agents      []AgentInfo    `json:"agents"`
	Workflows   []WorkflowInfo `json:"workflows"`
	LastSeen    time.Time      `json:"last_seen"`
	Status      StationStatus  `json:"status"`
}

type AgentInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	InputSchema  string   `json:"input_schema,omitempty"`
	OutputSchema string   `json:"output_schema,omitempty"`
	Examples     []string `json:"examples,omitempty"`
}

type WorkflowInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type StationStatus string

const (
	StatusOnline  StationStatus = "online"
	StatusOffline StationStatus = "offline"
)

type Registry struct {
	client    *Client
	telemetry *Telemetry

	mu          sync.RWMutex
	stationsKV  nats.KeyValue
	agentsKV    nats.KeyValue
	initialized bool
}

func NewRegistry(client *Client) *Registry {
	return &Registry{
		client:    client,
		telemetry: NewTelemetry(),
	}
}

func (r *Registry) Initialize(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.initialized {
		return nil
	}

	js := r.client.JetStream()
	if js == nil {
		return fmt.Errorf("JetStream not available")
	}

	stationsKV, err := r.getOrCreateKV(js, StationsBucket)
	if err != nil {
		return fmt.Errorf("failed to initialize stations KV: %w", err)
	}
	r.stationsKV = stationsKV

	agentsKV, err := r.getOrCreateKV(js, AgentsBucket)
	if err != nil {
		return fmt.Errorf("failed to initialize agents KV: %w", err)
	}
	r.agentsKV = agentsKV

	r.initialized = true
	return nil
}

func (r *Registry) getOrCreateKV(js nats.JetStreamContext, bucket string) (nats.KeyValue, error) {
	kv, err := js.KeyValue(bucket)
	if err == nil {
		return kv, nil
	}

	if err != nats.ErrBucketNotFound {
		return nil, err
	}

	kv, err = js.CreateKeyValue(&nats.KeyValueConfig{
		Bucket:      bucket,
		Description: fmt.Sprintf("Station Lattice %s registry", bucket),
		Storage:     nats.FileStorage,
		History:     5,
		TTL:         0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create KV bucket %s: %w", bucket, err)
	}

	return kv, nil
}

func (r *Registry) RegisterStation(ctx context.Context, manifest StationManifest) error {
	_, span := r.telemetry.StartRegistrySpan(ctx, "register", manifest.StationID)
	var resultErr error
	defer func() { r.telemetry.EndSpan(span, resultErr) }()

	r.mu.RLock()
	if !r.initialized {
		r.mu.RUnlock()
		resultErr = fmt.Errorf("registry not initialized")
		return resultErr
	}
	stationsKV := r.stationsKV
	agentsKV := r.agentsKV
	r.mu.RUnlock()

	manifest.LastSeen = time.Now()
	manifest.Status = StatusOnline

	data, err := json.Marshal(manifest)
	if err != nil {
		resultErr = fmt.Errorf("failed to marshal station manifest: %w", err)
		return resultErr
	}

	_, err = stationsKV.Put(manifest.StationID, data)
	if err != nil {
		resultErr = fmt.Errorf("failed to store station manifest: %w", err)
		return resultErr
	}

	for _, agent := range manifest.Agents {
		agentKey := fmt.Sprintf("%s.%s", manifest.StationID, agent.ID)
		agentEntry := struct {
			AgentInfo
			StationID   string `json:"station_id"`
			StationName string `json:"station_name"`
		}{
			AgentInfo:   agent,
			StationID:   manifest.StationID,
			StationName: manifest.StationName,
		}

		agentData, err := json.Marshal(agentEntry)
		if err != nil {
			continue
		}

		_, err = agentsKV.Put(agentKey, agentData)
		if err != nil {
			fmt.Printf("[registry] Warning: failed to index agent %s: %v\n", agent.Name, err)
		}
	}

	return nil
}

func (r *Registry) RegisterStationWithConflictCheck(ctx context.Context, manifest StationManifest) (*RegistrationResult, error) {
	_, span := r.telemetry.StartRegistrySpan(ctx, "register_with_check", manifest.StationID)
	var resultErr error
	defer func() { r.telemetry.EndSpan(span, resultErr) }()

	r.mu.RLock()
	if !r.initialized {
		r.mu.RUnlock()
		resultErr = fmt.Errorf("registry not initialized")
		return nil, resultErr
	}
	stationsKV := r.stationsKV
	agentsKV := r.agentsKV
	r.mu.RUnlock()

	conflicts, err := r.findAgentNameConflicts(ctx, manifest)
	if err != nil {
		resultErr = err
		return nil, err
	}

	result := &RegistrationResult{
		RegisteredAgents:  []string{},
		ConflictingAgents: conflicts,
		Success:           true,
	}

	conflictNames := make(map[string]bool)
	for _, c := range conflicts {
		conflictNames[c.AgentName] = true
	}

	var agentsToRegister []AgentInfo
	for _, agent := range manifest.Agents {
		if conflictNames[agent.Name] {
			continue
		}
		agentsToRegister = append(agentsToRegister, agent)
		result.RegisteredAgents = append(result.RegisteredAgents, agent.Name)
	}

	manifestToStore := manifest
	manifestToStore.Agents = agentsToRegister
	manifestToStore.LastSeen = time.Now()
	manifestToStore.Status = StatusOnline

	data, err := json.Marshal(manifestToStore)
	if err != nil {
		resultErr = fmt.Errorf("failed to marshal station manifest: %w", err)
		return nil, resultErr
	}

	_, err = stationsKV.Put(manifest.StationID, data)
	if err != nil {
		resultErr = fmt.Errorf("failed to store station manifest: %w", err)
		return nil, resultErr
	}

	for _, agent := range agentsToRegister {
		agentKey := fmt.Sprintf("%s.%s", manifest.StationID, agent.ID)
		agentEntry := struct {
			AgentInfo
			StationID   string `json:"station_id"`
			StationName string `json:"station_name"`
		}{
			AgentInfo:   agent,
			StationID:   manifest.StationID,
			StationName: manifest.StationName,
		}

		agentData, err := json.Marshal(agentEntry)
		if err != nil {
			continue
		}

		_, err = agentsKV.Put(agentKey, agentData)
		if err != nil {
			fmt.Printf("[registry] Warning: failed to index agent %s: %v\n", agent.Name, err)
		}
	}

	return result, nil
}

func (r *Registry) findAgentNameConflicts(ctx context.Context, manifest StationManifest) ([]AgentNameConflictError, error) {
	stations, err := r.ListStations(ctx)
	if err != nil {
		return nil, err
	}

	var conflicts []AgentNameConflictError

	for _, agent := range manifest.Agents {
		for _, existingStation := range stations {
			if existingStation.StationID == manifest.StationID {
				continue
			}

			for _, existingAgent := range existingStation.Agents {
				if existingAgent.Name == agent.Name {
					conflicts = append(conflicts, AgentNameConflictError{
						AgentName:        agent.Name,
						ExistingStation:  existingStation.StationName,
						AttemptedStation: manifest.StationName,
					})
					break
				}
			}
		}
	}

	return conflicts, nil
}

func (r *Registry) CheckAgentNameAvailable(ctx context.Context, agentName, excludeStationID string) (bool, *StationManifest) {
	stations, err := r.ListStations(ctx)
	if err != nil {
		return true, nil
	}

	for _, station := range stations {
		if station.StationID == excludeStationID {
			continue
		}

		for _, agent := range station.Agents {
			if agent.Name == agentName {
				return false, &station
			}
		}
	}

	return true, nil
}

func (r *Registry) UnregisterStation(ctx context.Context, stationID string) error {
	_, span := r.telemetry.StartRegistrySpan(ctx, "unregister", stationID)
	var resultErr error
	defer func() { r.telemetry.EndSpan(span, resultErr) }()

	r.mu.RLock()
	if !r.initialized {
		r.mu.RUnlock()
		resultErr = fmt.Errorf("registry not initialized")
		return resultErr
	}
	stationsKV := r.stationsKV
	agentsKV := r.agentsKV
	r.mu.RUnlock()

	entry, err := stationsKV.Get(stationID)
	if err != nil && err != nats.ErrKeyNotFound {
		resultErr = fmt.Errorf("failed to get station: %w", err)
		return resultErr
	}

	if entry != nil {
		var manifest StationManifest
		if err := json.Unmarshal(entry.Value(), &manifest); err == nil {
			for _, agent := range manifest.Agents {
				agentKey := fmt.Sprintf("%s.%s", stationID, agent.ID)
				_ = agentsKV.Delete(agentKey)
			}
		}
	}

	if err := stationsKV.Delete(stationID); err != nil && err != nats.ErrKeyNotFound {
		resultErr = fmt.Errorf("failed to delete station: %w", err)
		return resultErr
	}

	return nil
}

func (r *Registry) GetStation(ctx context.Context, stationID string) (*StationManifest, error) {
	_, span := r.telemetry.StartRegistrySpan(ctx, "get", stationID)
	var resultErr error
	defer func() { r.telemetry.EndSpan(span, resultErr) }()

	r.mu.RLock()
	if !r.initialized {
		r.mu.RUnlock()
		resultErr = fmt.Errorf("registry not initialized")
		return nil, resultErr
	}
	stationsKV := r.stationsKV
	r.mu.RUnlock()

	entry, err := stationsKV.Get(stationID)
	if err != nil {
		if err == nats.ErrKeyNotFound {
			return nil, nil
		}
		resultErr = fmt.Errorf("failed to get station: %w", err)
		return nil, resultErr
	}

	var manifest StationManifest
	if err := json.Unmarshal(entry.Value(), &manifest); err != nil {
		resultErr = fmt.Errorf("failed to unmarshal station manifest: %w", err)
		return nil, resultErr
	}

	return &manifest, nil
}

func (r *Registry) ListStations(ctx context.Context) ([]StationManifest, error) {
	_, span := r.telemetry.StartRegistrySpan(ctx, "list", "")
	var resultErr error
	defer func() { r.telemetry.EndSpan(span, resultErr) }()

	r.mu.RLock()
	if !r.initialized {
		r.mu.RUnlock()
		resultErr = fmt.Errorf("registry not initialized")
		return nil, resultErr
	}
	stationsKV := r.stationsKV
	r.mu.RUnlock()

	keys, err := stationsKV.Keys()
	if err != nil {
		if err == nats.ErrNoKeysFound {
			return []StationManifest{}, nil
		}
		resultErr = fmt.Errorf("failed to list stations: %w", err)
		return nil, resultErr
	}

	var stations []StationManifest
	for _, key := range keys {
		entry, err := stationsKV.Get(key)
		if err != nil {
			continue
		}

		var manifest StationManifest
		if err := json.Unmarshal(entry.Value(), &manifest); err != nil {
			continue
		}

		stations = append(stations, manifest)
	}

	return stations, nil
}

func (r *Registry) UpdateStationStatus(ctx context.Context, stationID string, status StationStatus) error {
	r.mu.RLock()
	if !r.initialized {
		r.mu.RUnlock()
		return fmt.Errorf("registry not initialized")
	}
	stationsKV := r.stationsKV
	r.mu.RUnlock()

	entry, err := stationsKV.Get(stationID)
	if err != nil {
		return fmt.Errorf("failed to get station: %w", err)
	}

	var manifest StationManifest
	if err := json.Unmarshal(entry.Value(), &manifest); err != nil {
		return fmt.Errorf("failed to unmarshal station manifest: %w", err)
	}

	manifest.Status = status
	manifest.LastSeen = time.Now()

	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal station manifest: %w", err)
	}

	_, err = stationsKV.Put(stationID, data)
	if err != nil {
		return fmt.Errorf("failed to update station: %w", err)
	}

	return nil
}

func (r *Registry) FindAgentsByCapability(ctx context.Context, capability string) ([]AgentInfo, error) {
	r.mu.RLock()
	if !r.initialized {
		r.mu.RUnlock()
		return nil, fmt.Errorf("registry not initialized")
	}
	stationsKV := r.stationsKV
	r.mu.RUnlock()

	stations, err := r.ListStations(ctx)
	if err != nil {
		return nil, err
	}

	var matchingAgents []AgentInfo
	for _, station := range stations {
		if station.Status != StatusOnline {
			continue
		}

		for _, agent := range station.Agents {
			for _, cap := range agent.Capabilities {
				if cap == capability {
					matchingAgents = append(matchingAgents, agent)
					break
				}
			}
		}
	}

	_ = stationsKV

	return matchingAgents, nil
}

func (r *Registry) WatchStations(ctx context.Context) (<-chan StationManifest, error) {
	r.mu.RLock()
	if !r.initialized {
		r.mu.RUnlock()
		return nil, fmt.Errorf("registry not initialized")
	}
	stationsKV := r.stationsKV
	r.mu.RUnlock()

	watcher, err := stationsKV.WatchAll()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	ch := make(chan StationManifest, 100)

	go func() {
		defer close(ch)
		defer watcher.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case entry, ok := <-watcher.Updates():
				if !ok {
					return
				}
				if entry == nil {
					continue
				}
				if entry.Operation() == nats.KeyValueDelete {
					continue
				}

				var manifest StationManifest
				if err := json.Unmarshal(entry.Value(), &manifest); err != nil {
					continue
				}

				select {
				case ch <- manifest:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch, nil
}

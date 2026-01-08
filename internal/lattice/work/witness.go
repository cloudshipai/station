package work

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type WitnessConfig struct {
	CheckInterval  time.Duration
	StuckThreshold time.Duration
	MaxRetries     int
	Enabled        bool
}

func DefaultWitnessConfig() WitnessConfig {
	return WitnessConfig{
		CheckInterval:  30 * time.Second,
		StuckThreshold: 5 * time.Minute,
		MaxRetries:     3,
		Enabled:        true,
	}
}

type WitnessHandler interface {
	OnStuckWork(record *WorkRecord, retryCount int) WitnessAction
	OnWorkEscalated(record *WorkRecord)
}

type WitnessAction int

const (
	WitnessActionRetry WitnessAction = iota
	WitnessActionEscalate
	WitnessActionIgnore
)

type Witness struct {
	store   *WorkStore
	config  WitnessConfig
	handler WitnessHandler

	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
	retries map[string]int
	stuckAt map[string]time.Time
}

func NewWitness(store *WorkStore, config WitnessConfig, handler WitnessHandler) *Witness {
	if config.CheckInterval == 0 {
		config.CheckInterval = 30 * time.Second
	}
	if config.StuckThreshold == 0 {
		config.StuckThreshold = 5 * time.Minute
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}

	return &Witness{
		store:   store,
		config:  config,
		handler: handler,
		retries: make(map[string]int),
		stuckAt: make(map[string]time.Time),
	}
}

func (w *Witness) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	if !w.config.Enabled {
		w.mu.Unlock()
		return nil
	}

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.running = true
	w.mu.Unlock()

	go w.monitorLoop()
	return nil
}

func (w *Witness) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.cancel()
	w.running = false
}

func (w *Witness) monitorLoop() {
	ticker := time.NewTicker(w.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.checkAllWork()
		}
	}
}

func (w *Witness) checkAllWork() {
	records, err := w.findInProgressWork()
	if err != nil {
		fmt.Printf("[witness] Error finding in-progress work: %v\n", err)
		return
	}

	now := time.Now()
	for _, record := range records {
		if w.isStuck(record, now) {
			w.handleStuckWork(record)
		} else {
			w.clearStuckTracking(record.WorkID)
		}
	}
}

func (w *Witness) findInProgressWork() ([]*WorkRecord, error) {
	ctx, cancel := context.WithTimeout(w.ctx, 10*time.Second)
	defer cancel()

	workCh, err := w.store.WatchAll(ctx)
	if err != nil {
		return nil, err
	}

	var records []*WorkRecord
	timeout := time.After(5 * time.Second)

collectLoop:
	for {
		select {
		case record, ok := <-workCh:
			if !ok {
				break collectLoop
			}
			if record.Status == StatusAssigned || record.Status == StatusAccepted {
				records = append(records, record)
			}
		case <-timeout:
			break collectLoop
		case <-ctx.Done():
			break collectLoop
		}
	}

	return records, nil
}

func (w *Witness) isStuck(record *WorkRecord, now time.Time) bool {
	var lastActivity time.Time
	switch record.Status {
	case StatusAssigned:
		lastActivity = record.AssignedAt
	case StatusAccepted:
		lastActivity = record.AcceptedAt
		if lastActivity.IsZero() {
			lastActivity = record.AssignedAt
		}
	default:
		return false
	}

	if lastActivity.IsZero() {
		return false
	}

	return now.Sub(lastActivity) > w.config.StuckThreshold
}

func (w *Witness) handleStuckWork(record *WorkRecord) {
	w.mu.Lock()
	retryCount := w.retries[record.WorkID]
	if _, tracked := w.stuckAt[record.WorkID]; !tracked {
		w.stuckAt[record.WorkID] = time.Now()
	}
	w.mu.Unlock()

	fmt.Printf("[witness] Work %s appears stuck (status=%s, retries=%d)\n",
		record.WorkID, record.Status, retryCount)

	action := WitnessActionRetry
	if w.handler != nil {
		action = w.handler.OnStuckWork(record, retryCount)
	}

	switch action {
	case WitnessActionRetry:
		if retryCount >= w.config.MaxRetries {
			w.escalateWork(record)
		} else {
			w.requeueWork(record, retryCount)
		}
	case WitnessActionEscalate:
		w.escalateWork(record)
	case WitnessActionIgnore:
		fmt.Printf("[witness] Ignoring stuck work %s per handler\n", record.WorkID)
	}
}

func (w *Witness) requeueWork(record *WorkRecord, retryCount int) {
	w.mu.Lock()
	w.retries[record.WorkID] = retryCount + 1
	w.mu.Unlock()

	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	err := w.store.UpdateStatus(ctx, record.WorkID, StatusAssigned, nil)
	if err != nil {
		fmt.Printf("[witness] Failed to requeue work %s: %v\n", record.WorkID, err)
		return
	}

	fmt.Printf("[witness] Requeued work %s (retry %d/%d)\n",
		record.WorkID, retryCount+1, w.config.MaxRetries)
}

func (w *Witness) escalateWork(record *WorkRecord) {
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	result := &WorkResult{
		Error: fmt.Sprintf("work stuck after %d retries", w.config.MaxRetries),
	}
	err := w.store.UpdateStatus(ctx, record.WorkID, StatusEscalated, result)
	if err != nil {
		fmt.Printf("[witness] Failed to escalate work %s: %v\n", record.WorkID, err)
		return
	}

	w.clearStuckTracking(record.WorkID)

	if w.handler != nil {
		w.handler.OnWorkEscalated(record)
	}

	fmt.Printf("[witness] Escalated work %s after max retries\n", record.WorkID)
}

func (w *Witness) clearStuckTracking(workID string) {
	w.mu.Lock()
	delete(w.retries, workID)
	delete(w.stuckAt, workID)
	w.mu.Unlock()
}

func (w *Witness) GetStats() WitnessStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return WitnessStats{
		TrackedWorkItems: len(w.retries),
		StuckWorkItems:   len(w.stuckAt),
		Running:          w.running,
	}
}

type WitnessStats struct {
	TrackedWorkItems int
	StuckWorkItems   int
	Running          bool
}

type DefaultWitnessHandler struct {
	onEscalate func(*WorkRecord)
}

func NewDefaultWitnessHandler(onEscalate func(*WorkRecord)) *DefaultWitnessHandler {
	return &DefaultWitnessHandler{onEscalate: onEscalate}
}

func (h *DefaultWitnessHandler) OnStuckWork(record *WorkRecord, retryCount int) WitnessAction {
	return WitnessActionRetry
}

func (h *DefaultWitnessHandler) OnWorkEscalated(record *WorkRecord) {
	if h.onEscalate != nil {
		h.onEscalate(record)
	}
}

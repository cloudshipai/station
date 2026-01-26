package harness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
)

type DoomLoopDetector struct {
	mu              sync.Mutex
	threshold       int
	callHistory     []callRecord
	consecutiveRuns map[string]int
}

type callRecord struct {
	toolName string
	argHash  string
}

func NewDoomLoopDetector(threshold int) *DoomLoopDetector {
	if threshold <= 0 {
		threshold = 3
	}
	return &DoomLoopDetector{
		threshold:       threshold,
		callHistory:     make([]callRecord, 0),
		consecutiveRuns: make(map[string]int),
	}
}

func (d *DoomLoopDetector) IsInDoomLoop(toolName string, args interface{}) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	argHash := hashArgs(args)
	key := toolName + ":" + argHash

	if len(d.callHistory) > 0 {
		last := d.callHistory[len(d.callHistory)-1]
		lastKey := last.toolName + ":" + last.argHash

		if lastKey == key {
			d.consecutiveRuns[key]++
		} else {
			d.consecutiveRuns[key] = 1
		}
	} else {
		d.consecutiveRuns[key] = 1
	}

	return d.consecutiveRuns[key] >= d.threshold
}

func (d *DoomLoopDetector) Record(toolName string, args interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()

	argHash := hashArgs(args)
	d.callHistory = append(d.callHistory, callRecord{
		toolName: toolName,
		argHash:  argHash,
	})

	const maxHistory = 100
	if len(d.callHistory) > maxHistory {
		d.callHistory = d.callHistory[len(d.callHistory)-maxHistory:]
	}
}

func (d *DoomLoopDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.callHistory = make([]callRecord, 0)
	d.consecutiveRuns = make(map[string]int)
}

func (d *DoomLoopDetector) GetConsecutiveCount(toolName string, args interface{}) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	argHash := hashArgs(args)
	key := toolName + ":" + argHash
	return d.consecutiveRuns[key]
}

func hashArgs(args interface{}) string {
	if args == nil {
		return "nil"
	}

	data, err := json.Marshal(args)
	if err != nil {
		return "error"
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:8])
}

package lighthouse

import (
	"sync"
	"time"
)

// LighthouseStatus represents the current status of lighthouse integration
type LighthouseStatus struct {
	Connected         bool      `json:"connected"`
	Registered        bool      `json:"registered"`
	LastError         string    `json:"last_error,omitempty"`
	LastErrorTime     time.Time `json:"last_error_time,omitempty"`
	LastSuccess       time.Time `json:"last_success,omitempty"`
	RegistrationKey   string    `json:"registration_key,omitempty"`
	TelemetrySent     int64     `json:"telemetry_sent"`
	TelemetryFailed   int64     `json:"telemetry_failed"`
	ServerURL         string    `json:"server_url,omitempty"`
}

// Global lighthouse status tracking
var (
	globalStatus = &LighthouseStatus{}
	statusMutex  sync.RWMutex
)

// UpdateStatus updates the global lighthouse status
func UpdateStatus(update func(*LighthouseStatus)) {
	statusMutex.Lock()
	defer statusMutex.Unlock()
	update(globalStatus)
}

// GetStatus returns a copy of the current lighthouse status
func GetStatus() LighthouseStatus {
	statusMutex.RLock()
	defer statusMutex.RUnlock()
	return *globalStatus
}

// SetConnected updates the connection status
func SetConnected(connected bool, serverURL string) {
	UpdateStatus(func(s *LighthouseStatus) {
		s.Connected = connected
		s.ServerURL = serverURL
		if connected {
			s.LastSuccess = time.Now()
		}
	})
}

// SetRegistered updates the registration status
func SetRegistered(registered bool, registrationKey string) {
	UpdateStatus(func(s *LighthouseStatus) {
		s.Registered = registered
		s.RegistrationKey = registrationKey
		if registered {
			s.LastSuccess = time.Now()
		}
	})
}

// RecordError records a lighthouse error
func RecordError(err string) {
	UpdateStatus(func(s *LighthouseStatus) {
		s.LastError = err
		s.LastErrorTime = time.Now()
	})
}

// RecordSuccess records successful telemetry transmission
func RecordSuccess() {
	UpdateStatus(func(s *LighthouseStatus) {
		s.TelemetrySent++
		s.LastSuccess = time.Now()
		s.LastError = "" // Clear error on success
	})
}

// RecordFailure records failed telemetry transmission
func RecordFailure(err string) {
	UpdateStatus(func(s *LighthouseStatus) {
		s.TelemetryFailed++
		s.LastError = err
		s.LastErrorTime = time.Now()
	})
}
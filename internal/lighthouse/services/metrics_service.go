package services

import (
	"runtime"
	"station/pkg/types"
	"time"
)

// MetricsService handles collection of Station-specific metrics for CloudShip monitoring
// This service focuses on Station application metrics rather than host system metrics,
// making it suitable for containerized deployments (ECS, Kubernetes, Docker, etc.)
type MetricsService struct {
	startTime         time.Time
	activeConnections int
	activeRuns        int
}

// NewMetricsService creates a new metrics service
func NewMetricsService() *MetricsService {
	return &MetricsService{
		startTime: time.Now(),
	}
}

// GetCurrentMetrics collects and returns current Station application metrics
func (ms *MetricsService) GetCurrentMetrics() *types.SystemMetrics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return &types.SystemMetrics{
		// Application memory usage (works in containers)
		CPUUsagePercent:    0.0, // Skip CPU - complex in containers
		MemoryUsagePercent: ms.getGoMemoryUsagePercent(&memStats),
		DiskUsageMB:        0, // Skip disk - not relevant for Station

		// Station application metrics
		UptimeSeconds:     int64(time.Since(ms.startTime).Seconds()),
		ActiveConnections: ms.activeConnections,
		ActiveRuns:        ms.activeRuns,

		// Skip network metrics - not directly measurable
		NetworkInBytes:  0,
		NetworkOutBytes: 0,

		// Go runtime metrics (useful for debugging)
		AdditionalMetrics: map[string]string{
			"go_version":     runtime.Version(),
			"num_goroutines": string(rune(runtime.NumGoroutine())),
			"num_cpu":        string(rune(runtime.NumCPU())),
			"heap_alloc_mb":  string(rune(int(memStats.HeapAlloc / 1024 / 1024))),
		},
	}
}

// UpdateActiveConnections updates the count of active connections (SSH, MCP, API)
func (ms *MetricsService) UpdateActiveConnections(count int) {
	ms.activeConnections = count
}

// UpdateActiveRuns updates the count of active agent runs
func (ms *MetricsService) UpdateActiveRuns(count int) {
	ms.activeRuns = count
}

// Private helper methods

func (ms *MetricsService) getGoMemoryUsagePercent(memStats *runtime.MemStats) float64 {
	// Calculate Go heap usage as percentage of heap in use vs heap system
	if memStats.HeapSys == 0 {
		return 0.0
	}
	return float64(memStats.HeapInuse) / float64(memStats.HeapSys) * 100.0
}

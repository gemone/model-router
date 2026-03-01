package metrics

import (
	"sync"
	"time"
)

// CompressionMetrics tracks compression-related metrics
type CompressionMetrics struct {
	mu sync.RWMutex

	// Selection latency tracking
	selectionLatencies map[string][]time.Duration // groupName -> latencies

	// Usage tracking
	usageCount map[string]int // groupName:profileID -> count

	// Fallback tracking
	fallbackReasons map[string]int // groupName:reason -> count
}

var (
	globalCompressionMetrics *CompressionMetrics
	compressionMetricsOnce   sync.Once
)

// GetCompressionMetrics returns the global compression metrics instance
func GetCompressionMetrics() *CompressionMetrics {
	compressionMetricsOnce.Do(func() {
		globalCompressionMetrics = &CompressionMetrics{
			selectionLatencies: make(map[string][]time.Duration),
			usageCount:         make(map[string]int),
			fallbackReasons:    make(map[string]int),
		}
	})
	return globalCompressionMetrics
}

// RecordSelectionLatency records a selector latency measurement
func (m *CompressionMetrics) RecordSelectionLatency(groupName, profileID string, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := groupName + ":" + profileID
	// Keep last 100 measurements
	m.selectionLatencies[key] = append(m.selectionLatencies[key], latency)
	if len(m.selectionLatencies[key]) > 100 {
		m.selectionLatencies[key] = m.selectionLatencies[key][1:]
	}
}

// RecordUsage records that a compression group was used
func (m *CompressionMetrics) RecordUsage(groupName, modelName, profileID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := groupName + ":" + profileID + ":" + modelName
	m.usageCount[key]++
}

// RecordFallback records a fallback event
func RecordCompressionGroupFallback(groupName, reason, profileID string) {
	m := GetCompressionMetrics()
	m.mu.Lock()
	defer m.mu.Unlock()

	key := groupName + ":" + reason + ":" + profileID
	m.fallbackReasons[key]++
}

// GetAverageSelectionLatency returns the average selection latency for a group
func (m *CompressionMetrics) GetAverageSelectionLatency(groupName, profileID string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := groupName + ":" + profileID
	latencies, ok := m.selectionLatencies[key]
	if !ok || len(latencies) == 0 {
		return 0
	}

	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	return sum / time.Duration(len(latencies))
}

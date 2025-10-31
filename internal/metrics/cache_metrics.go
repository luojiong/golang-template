package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// CacheMetrics tracks cache performance statistics
type CacheMetrics struct {
	mu sync.RWMutex

	// Hit/miss counters
	hits   uint64
	misses uint64

	// Operation counters
	sets   uint64
	gets   uint64
	deletes uint64

	// Performance tracking
	totalGetDuration  int64 // in nanoseconds
	totalSetDuration  int64 // in nanoseconds
	totalDeleteDuration int64 // in nanoseconds

	// Additional metrics
	evictions uint64
	errors    uint64

	// Operation history for recent performance analysis
	operationHistory []CacheOperation
	maxHistorySize   int
}

// CacheOperation represents a single cache operation
type CacheOperation struct {
	Type      OperationType `json:"type"`
	Key       string        `json:"key,omitempty"`
	Duration  time.Duration `json:"duration"`
	Success   bool          `json:"success"`
	Timestamp time.Time     `json:"timestamp"`
}

// OperationType represents the type of cache operation
type OperationType int

const (
	OperationTypeGet OperationType = iota
	OperationTypeSet
	OperationTypeDelete
)

// CacheStats represents aggregated cache statistics
type CacheStats struct {
	TotalRequests     uint64        `json:"total_requests"`
	CacheHits         uint64        `json:"cache_hits"`
	CacheMisses       uint64        `json:"cache_misses"`
	HitRate           float64       `json:"hit_rate"`
	MissRate          float64       `json:"miss_rate"`
	Sets              uint64        `json:"sets"`
	Gets              uint64        `json:"gets"`
	Deletes           uint64        `json:"deletes"`
	Evictions         uint64        `json:"evictions"`
	Errors            uint64        `json:"errors"`
	AvgGetDuration    time.Duration `json:"avg_get_duration"`
	AvgSetDuration    time.Duration `json:"avg_set_duration"`
	AvgDeleteDuration time.Duration `json:"avg_delete_duration"`
	RecentOperations  []CacheOperation `json:"recent_operations,omitempty"`
}

// NewCacheMetrics creates a new cache metrics instance
func NewCacheMetrics() *CacheMetrics {
	return &CacheMetrics{
		maxHistorySize:   1000, // Keep last 1000 operations
		operationHistory: make([]CacheOperation, 0),
	}
}

// RecordHit records a cache hit
func (cm *CacheMetrics) RecordHit() {
	atomic.AddUint64(&cm.hits, 1)
}

// RecordMiss records a cache miss
func (cm *CacheMetrics) RecordMiss() {
	atomic.AddUint64(&cm.misses, 1)
}

// RecordSet records a cache set operation
func (cm *CacheMetrics) RecordSet(key string, duration time.Duration, success bool) {
	atomic.AddUint64(&cm.sets, 1)
	atomic.AddInt64(&cm.totalSetDuration, int64(duration))

	if !success {
		atomic.AddUint64(&cm.errors, 1)
	}

	cm.recordOperation(CacheOperation{
		Type:      OperationTypeSet,
		Key:       key,
		Duration:  duration,
		Success:   success,
		Timestamp: time.Now(),
	})
}

// RecordGet records a cache get operation with hit/miss result
func (cm *CacheMetrics) RecordGet(key string, duration time.Duration, hit bool, success bool) {
	atomic.AddUint64(&cm.gets, 1)
	atomic.AddInt64(&cm.totalGetDuration, int64(duration))

	if hit {
		cm.RecordHit()
	} else {
		cm.RecordMiss()
	}

	if !success {
		atomic.AddUint64(&cm.errors, 1)
	}

	cm.recordOperation(CacheOperation{
		Type:      OperationTypeGet,
		Key:       key,
		Duration:  duration,
		Success:   success,
		Timestamp: time.Now(),
	})
}

// RecordDelete records a cache delete operation
func (cm *CacheMetrics) RecordDelete(key string, duration time.Duration, success bool) {
	atomic.AddUint64(&cm.deletes, 1)
	atomic.AddInt64(&cm.totalDeleteDuration, int64(duration))

	if !success {
		atomic.AddUint64(&cm.errors, 1)
	}

	cm.recordOperation(CacheOperation{
		Type:      OperationTypeDelete,
		Key:       key,
		Duration:  duration,
		Success:   success,
		Timestamp: time.Now(),
	})
}

// RecordEviction records a cache eviction
func (cm *CacheMetrics) RecordEviction() {
	atomic.AddUint64(&cm.evictions, 1)
}

// GetStats returns current cache statistics
func (cm *CacheMetrics) GetStats() CacheStats {
	hits := atomic.LoadUint64(&cm.hits)
	misses := atomic.LoadUint64(&cm.misses)
	sets := atomic.LoadUint64(&cm.sets)
	gets := atomic.LoadUint64(&cm.gets)
	deletes := atomic.LoadUint64(&cm.deletes)
	evictions := atomic.LoadUint64(&cm.evictions)
	errors := atomic.LoadUint64(&cm.errors)
	totalGetDuration := atomic.LoadInt64(&cm.totalGetDuration)
	totalSetDuration := atomic.LoadInt64(&cm.totalSetDuration)
	totalDeleteDuration := atomic.LoadInt64(&cm.totalDeleteDuration)

	totalRequests := hits + misses
	hitRate := float64(0)
	missRate := float64(0)

	if totalRequests > 0 {
		hitRate = float64(hits) / float64(totalRequests) * 100
		missRate = float64(misses) / float64(totalRequests) * 100
	}

	avgGetDuration := time.Duration(0)
	avgSetDuration := time.Duration(0)
	avgDeleteDuration := time.Duration(0)

	if gets > 0 {
		avgGetDuration = time.Duration(totalGetDuration / int64(gets))
	}
	if sets > 0 {
		avgSetDuration = time.Duration(totalSetDuration / int64(sets))
	}
	if deletes > 0 {
		avgDeleteDuration = time.Duration(totalDeleteDuration / int64(deletes))
	}

	cm.mu.RLock()
	recentOps := make([]CacheOperation, len(cm.operationHistory))
	copy(recentOps, cm.operationHistory)
	cm.mu.RUnlock()

	return CacheStats{
		TotalRequests:     totalRequests,
		CacheHits:         hits,
		CacheMisses:       misses,
		HitRate:           hitRate,
		MissRate:          missRate,
		Sets:              sets,
		Gets:              gets,
		Deletes:           deletes,
		Evictions:         evictions,
		Errors:            errors,
		AvgGetDuration:    avgGetDuration,
		AvgSetDuration:    avgSetDuration,
		AvgDeleteDuration: avgDeleteDuration,
		RecentOperations:  recentOps,
	}
}

// Reset resets all metrics
func (cm *CacheMetrics) Reset() {
	atomic.StoreUint64(&cm.hits, 0)
	atomic.StoreUint64(&cm.misses, 0)
	atomic.StoreUint64(&cm.sets, 0)
	atomic.StoreUint64(&cm.gets, 0)
	atomic.StoreUint64(&cm.deletes, 0)
	atomic.StoreUint64(&cm.evictions, 0)
	atomic.StoreUint64(&cm.errors, 0)
	atomic.StoreInt64(&cm.totalGetDuration, 0)
	atomic.StoreInt64(&cm.totalSetDuration, 0)
	atomic.StoreInt64(&cm.totalDeleteDuration, 0)

	cm.mu.Lock()
	cm.operationHistory = make([]CacheOperation, 0)
	cm.mu.Unlock()
}

// GetHitRate returns the current hit rate as a percentage
func (cm *CacheMetrics) GetHitRate() float64 {
	hits := atomic.LoadUint64(&cm.hits)
	misses := atomic.LoadUint64(&cm.misses)
	total := hits + misses

	if total == 0 {
		return 0
	}

	return float64(hits) / float64(total) * 100
}

// GetMissRate returns the current miss rate as a percentage
func (cm *CacheMetrics) GetMissRate() float64 {
	hits := atomic.LoadUint64(&cm.hits)
	misses := atomic.LoadUint64(&cm.misses)
	total := hits + misses

	if total == 0 {
		return 0
	}

	return float64(misses) / float64(total) * 100
}

// GetTotalRequests returns the total number of requests
func (cm *CacheMetrics) GetTotalRequests() uint64 {
	hits := atomic.LoadUint64(&cm.hits)
	misses := atomic.LoadUint64(&cm.misses)
	return hits + misses
}

// recordOperation records an operation in the history
func (cm *CacheMetrics) recordOperation(op CacheOperation) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Add new operation
	cm.operationHistory = append(cm.operationHistory, op)

	// Trim history if it exceeds max size
	if len(cm.operationHistory) > cm.maxHistorySize {
		// Remove oldest operations (keep only the last maxHistorySize)
		cm.operationHistory = cm.operationHistory[cm.maxHistorySize/2:]
	}
}

// GetRecentOperations returns the most recent operations
func (cm *CacheMetrics) GetRecentOperations(limit int) []CacheOperation {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if limit <= 0 || limit > len(cm.operationHistory) {
		limit = len(cm.operationHistory)
	}

	// Return the last 'limit' operations
	start := len(cm.operationHistory) - limit
	if start < 0 {
		start = 0
	}

	recentOps := make([]CacheOperation, limit)
	copy(recentOps, cm.operationHistory[start:])
	return recentOps
}

// GetPerformanceMetrics returns detailed performance metrics
type PerformanceMetrics struct {
	TotalOperations     uint64        `json:"total_operations"`
	AvgResponseTime     time.Duration `json:"avg_response_time"`
	P95ResponseTime     time.Duration `json:"p95_response_time"`
	P99ResponseTime     time.Duration `json:"p99_response_time"`
	ErrorRate           float64       `json:"error_rate"`
	OperationsPerSecond float64       `json:"ops_per_second"`
}

// GetPerformanceMetrics calculates detailed performance metrics
func (cm *CacheMetrics) GetPerformanceMetrics(timeWindow time.Duration) PerformanceMetrics {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-timeWindow)

	// Filter operations within time window
	var recentOps []CacheOperation
	for _, op := range cm.operationHistory {
		if op.Timestamp.After(cutoff) {
			recentOps = append(recentOps, op)
		}
	}

	if len(recentOps) == 0 {
		return PerformanceMetrics{}
	}

	// Calculate response time percentiles
	durations := make([]time.Duration, 0, len(recentOps))
	errorCount := 0

	for _, op := range recentOps {
		durations = append(durations, op.Duration)
		if !op.Success {
			errorCount++
		}
	}

	// Sort durations for percentile calculation
	// Simple insertion sort for small slices
	for i := 1; i < len(durations); i++ {
		key := durations[i]
		j := i - 1
		for j >= 0 && durations[j] > key {
			durations[j+1] = durations[j]
			j--
		}
		durations[j+1] = key
	}

	p95Index := int(float64(len(durations)) * 0.95)
	p99Index := int(float64(len(durations)) * 0.99)
	if p95Index >= len(durations) {
		p95Index = len(durations) - 1
	}
	if p99Index >= len(durations) {
		p99Index = len(durations) - 1
	}

	// Calculate average response time
	var totalDuration time.Duration
	for _, d := range durations {
		totalDuration += d
	}
	avgResponseTime := totalDuration / time.Duration(len(durations))

	// Calculate operations per second
	opsPerSecond := float64(len(recentOps)) / timeWindow.Seconds()

	return PerformanceMetrics{
		TotalOperations:     uint64(len(recentOps)),
		AvgResponseTime:     avgResponseTime,
		P95ResponseTime:     durations[p95Index],
		P99ResponseTime:     durations[p99Index],
		ErrorRate:           float64(errorCount) / float64(len(recentOps)) * 100,
		OperationsPerSecond: opsPerSecond,
	}
}

// SetMaxHistorySize sets the maximum number of operations to keep in history
func (cm *CacheMetrics) SetMaxHistorySize(size int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.maxHistorySize = size
	if len(cm.operationHistory) > size {
		// Trim history to new size
		cm.operationHistory = cm.operationHistory[len(cm.operationHistory)-size:]
	}
}
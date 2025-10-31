package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestNewCacheMetrics(t *testing.T) {
	cm := NewCacheMetrics()
	if cm == nil {
		t.Fatal("NewCacheMetrics() returned nil")
	}

	if cm.maxHistorySize != 1000 {
		t.Errorf("Expected maxHistorySize to be 1000, got %d", cm.maxHistorySize)
	}

	if len(cm.operationHistory) != 0 {
		t.Errorf("Expected operationHistory to be empty, got %d items", len(cm.operationHistory))
	}
}

func TestRecordHitMiss(t *testing.T) {
	cm := NewCacheMetrics()

	// Test initial state
	if cm.GetHitRate() != 0 {
		t.Errorf("Expected initial hit rate to be 0, got %f", cm.GetHitRate())
	}

	// Record some hits and misses
	cm.RecordHit()
	cm.RecordHit()
	cm.RecordMiss()
	cm.RecordHit()

	// Check hit rate calculation
	expectedHitRate := float64(3) / float64(4) * 100
	if cm.GetHitRate() != expectedHitRate {
		t.Errorf("Expected hit rate to be %f, got %f", expectedHitRate, cm.GetHitRate())
	}

	// Check miss rate calculation
	expectedMissRate := float64(1) / float64(4) * 100
	if cm.GetMissRate() != expectedMissRate {
		t.Errorf("Expected miss rate to be %f, got %f", expectedMissRate, cm.GetMissRate())
	}

	// Check total requests
	if cm.GetTotalRequests() != 4 {
		t.Errorf("Expected total requests to be 4, got %d", cm.GetTotalRequests())
	}
}

func TestRecordSet(t *testing.T) {
	cm := NewCacheMetrics()
	key := "test_key"
	duration := 5 * time.Millisecond

	// Record successful set
	cm.RecordSet(key, duration, true)

	stats := cm.GetStats()
	if stats.Sets != 1 {
		t.Errorf("Expected sets to be 1, got %d", stats.Sets)
	}
	if stats.Errors != 0 {
		t.Errorf("Expected errors to be 0, got %d", stats.Errors)
	}
	if stats.AvgSetDuration != duration {
		t.Errorf("Expected avg set duration to be %v, got %v", duration, stats.AvgSetDuration)
	}

	// Record failed set
	cm.RecordSet(key+"2", duration, false)

	stats = cm.GetStats()
	if stats.Sets != 2 {
		t.Errorf("Expected sets to be 2, got %d", stats.Sets)
	}
	if stats.Errors != 1 {
		t.Errorf("Expected errors to be 1, got %d", stats.Errors)
	}
}

func TestRecordGet(t *testing.T) {
	cm := NewCacheMetrics()
	key := "test_key"
	duration := 2 * time.Millisecond

	// Record cache hit
	cm.RecordGet(key, duration, true, true)

	stats := cm.GetStats()
	if stats.CacheHits != 1 {
		t.Errorf("Expected cache hits to be 1, got %d", stats.CacheHits)
	}
	if stats.CacheMisses != 0 {
		t.Errorf("Expected cache misses to be 0, got %d", stats.CacheMisses)
	}
	if stats.Gets != 1 {
		t.Errorf("Expected gets to be 1, got %d", stats.Gets)
	}

	// Record cache miss
	cm.RecordGet(key+"2", duration, false, true)

	stats = cm.GetStats()
	if stats.CacheHits != 1 {
		t.Errorf("Expected cache hits to be 1, got %d", stats.CacheHits)
	}
	if stats.CacheMisses != 1 {
		t.Errorf("Expected cache misses to be 1, got %d", stats.CacheMisses)
	}
	if stats.Gets != 2 {
		t.Errorf("Expected gets to be 2, got %d", stats.Gets)
	}

	// Record failed get
	cm.RecordGet(key+"3", duration, true, false)

	stats = cm.GetStats()
	if stats.Errors != 1 {
		t.Errorf("Expected errors to be 1, got %d", stats.Errors)
	}
}

func TestRecordDelete(t *testing.T) {
	cm := NewCacheMetrics()
	key := "test_key"
	duration := 1 * time.Millisecond

	// Record successful delete
	cm.RecordDelete(key, duration, true)

	stats := cm.GetStats()
	if stats.Deletes != 1 {
		t.Errorf("Expected deletes to be 1, got %d", stats.Deletes)
	}
	if stats.Errors != 0 {
		t.Errorf("Expected errors to be 0, got %d", stats.Errors)
	}

	// Record failed delete
	cm.RecordDelete(key+"2", duration, false)

	stats = cm.GetStats()
	if stats.Deletes != 2 {
		t.Errorf("Expected deletes to be 2, got %d", stats.Deletes)
	}
	if stats.Errors != 1 {
		t.Errorf("Expected errors to be 1, got %d", stats.Errors)
	}
}

func TestRecordEviction(t *testing.T) {
	cm := NewCacheMetrics()

	cm.RecordEviction()
	cm.RecordEviction()
	cm.RecordEviction()

	stats := cm.GetStats()
	if stats.Evictions != 3 {
		t.Errorf("Expected evictions to be 3, got %d", stats.Evictions)
	}
}

func TestGetStats(t *testing.T) {
	cm := NewCacheMetrics()

	// Record some operations
	cm.RecordSet("key1", 5*time.Millisecond, true)
	cm.RecordGet("key1", 2*time.Millisecond, true, true)
	cm.RecordGet("key2", 2*time.Millisecond, false, true)
	cm.RecordDelete("key1", 1*time.Millisecond, true)
	cm.RecordEviction()

	stats := cm.GetStats()

	// Verify stats
	if stats.TotalRequests != 2 {
		t.Errorf("Expected total requests to be 2, got %d", stats.TotalRequests)
	}
	if stats.CacheHits != 1 {
		t.Errorf("Expected cache hits to be 1, got %d", stats.CacheHits)
	}
	if stats.CacheMisses != 1 {
		t.Errorf("Expected cache misses to be 1, got %d", stats.CacheMisses)
	}
	if stats.Sets != 1 {
		t.Errorf("Expected sets to be 1, got %d", stats.Sets)
	}
	if stats.Gets != 2 {
		t.Errorf("Expected gets to be 2, got %d", stats.Gets)
	}
	if stats.Deletes != 1 {
		t.Errorf("Expected deletes to be 1, got %d", stats.Deletes)
	}
	if stats.Evictions != 1 {
		t.Errorf("Expected evictions to be 1, got %d", stats.Evictions)
	}

	// Verify percentages
	expectedHitRate := float64(1) / float64(2) * 100
	if stats.HitRate != expectedHitRate {
		t.Errorf("Expected hit rate to be %f, got %f", expectedHitRate, stats.HitRate)
	}

	expectedMissRate := float64(1) / float64(2) * 100
	if stats.MissRate != expectedMissRate {
		t.Errorf("Expected miss rate to be %f, got %f", expectedMissRate, stats.MissRate)
	}

	// Verify average durations
	if stats.AvgSetDuration != 5*time.Millisecond {
		t.Errorf("Expected avg set duration to be %v, got %v", 5*time.Millisecond, stats.AvgSetDuration)
	}
	if stats.AvgGetDuration != 2*time.Millisecond {
		t.Errorf("Expected avg get duration to be %v, got %v", 2*time.Millisecond, stats.AvgGetDuration)
	}
	if stats.AvgDeleteDuration != 1*time.Millisecond {
		t.Errorf("Expected avg delete duration to be %v, got %v", 1*time.Millisecond, stats.AvgDeleteDuration)
	}
}

func TestReset(t *testing.T) {
	cm := NewCacheMetrics()

	// Record some operations
	cm.RecordHit()
	cm.RecordMiss()
	cm.RecordSet("key1", 5*time.Millisecond, true)

	// Reset metrics
	cm.Reset()

	stats := cm.GetStats()

	// Verify everything is reset to zero
	if stats.TotalRequests != 0 {
		t.Errorf("Expected total requests to be 0 after reset, got %d", stats.TotalRequests)
	}
	if stats.CacheHits != 0 {
		t.Errorf("Expected cache hits to be 0 after reset, got %d", stats.CacheHits)
	}
	if stats.CacheMisses != 0 {
		t.Errorf("Expected cache misses to be 0 after reset, got %d", stats.CacheMisses)
	}
	if stats.Sets != 0 {
		t.Errorf("Expected sets to be 0 after reset, got %d", stats.Sets)
	}
	if stats.Gets != 0 {
		t.Errorf("Expected gets to be 0 after reset, got %d", stats.Gets)
	}
	if stats.Deletes != 0 {
		t.Errorf("Expected deletes to be 0 after reset, got %d", stats.Deletes)
	}
	if stats.Evictions != 0 {
		t.Errorf("Expected evictions to be 0 after reset, got %d", stats.Evictions)
	}
	if stats.Errors != 0 {
		t.Errorf("Expected errors to be 0 after reset, got %d", stats.Errors)
	}

	if len(stats.RecentOperations) != 0 {
		t.Errorf("Expected recent operations to be empty after reset, got %d", len(stats.RecentOperations))
	}
}

func TestOperationHistory(t *testing.T) {
	cm := NewCacheMetrics()

	// Record some operations
	cm.RecordSet("key1", 1*time.Millisecond, true)
	cm.RecordGet("key1", 2*time.Millisecond, true, true)
	cm.RecordDelete("key1", 3*time.Millisecond, true)

	// Test GetRecentOperations
	recentOps := cm.GetRecentOperations(3)
	if len(recentOps) != 3 {
		t.Errorf("Expected 3 recent operations, got %d", len(recentOps))
	}

	// Verify order (should be most recent first)
	// Delete should be last (most recent), Get should be middle, Set should be first (oldest)
	if recentOps[2].Type != OperationTypeDelete {
		t.Errorf("Expected first operation to be delete, got %v", recentOps[2].Type)
	}
	if recentOps[1].Type != OperationTypeGet {
		t.Errorf("Expected second operation to be get, got %v", recentOps[1].Type)
	}
	if recentOps[0].Type != OperationTypeSet {
		t.Errorf("Expected third operation to be set, got %v", recentOps[0].Type)
	}

	// Test with limit larger than history
	allOps := cm.GetRecentOperations(10)
	if len(allOps) != 3 {
		t.Errorf("Expected 3 operations, got %d", len(allOps))
	}

	// Test with limit 0
	allOpsZero := cm.GetRecentOperations(0)
	if len(allOpsZero) != 3 {
		t.Errorf("Expected 3 operations with limit 0, got %d", len(allOpsZero))
	}
}

func TestSetMaxHistorySize(t *testing.T) {
	cm := NewCacheMetrics()

	// Set a small history size
	cm.SetMaxHistorySize(3)

	// Record more operations than the limit
	for i := 0; i < 10; i++ {
		cm.RecordSet("key", 1*time.Millisecond, true)
	}

	recentOps := cm.GetRecentOperations(10)
	if len(recentOps) != 3 {
		t.Errorf("Expected 3 operations after setting max history size, got %d", len(recentOps))
	}
}

func TestGetPerformanceMetrics(t *testing.T) {
	cm := NewCacheMetrics()

	// Record some operations with different durations

	// Record operations with known durations
	operations := []struct {
		duration time.Duration
		success  bool
	}{
		{1 * time.Millisecond, true},
		{2 * time.Millisecond, true},
		{3 * time.Millisecond, true},
		{4 * time.Millisecond, true},
		{5 * time.Millisecond, true},
		{10 * time.Millisecond, false}, // This should be an error
	}

	for _, op := range operations {
		cm.RecordSet("key", op.duration, op.success)
	}

	// Get performance metrics for a large time window
	metrics := cm.GetPerformanceMetrics(time.Hour)

	if metrics.TotalOperations != 6 {
		t.Errorf("Expected 6 total operations, got %d", metrics.TotalOperations)
	}

	// Check error rate
	expectedErrorRate := float64(1) / float64(6) * 100
	if metrics.ErrorRate != expectedErrorRate {
		t.Errorf("Expected error rate to be %f, got %f", expectedErrorRate, metrics.ErrorRate)
	}

	// Check average response time (should be sum of all durations / count)
	// Total duration: 1+2+3+4+5+10 = 25ms, average = 25/6 = 4.166666ms
	expectedAvg := 4166666 * time.Nanosecond // approximately 4.166666ms
	// Allow for small calculation differences
	if metrics.AvgResponseTime < expectedAvg-10*time.Microsecond || metrics.AvgResponseTime > expectedAvg+10*time.Microsecond {
		t.Errorf("Expected avg response time to be approximately %v, got %v", expectedAvg, metrics.AvgResponseTime)
	}

	// Test with empty metrics
	emptyCm := NewCacheMetrics()
	emptyMetrics := emptyCm.GetPerformanceMetrics(time.Hour)

	if emptyMetrics.TotalOperations != 0 {
		t.Errorf("Expected 0 operations for empty metrics, got %d", emptyMetrics.TotalOperations)
	}
}

func TestConcurrentOperations(t *testing.T) {
	cm := NewCacheMetrics()

	// Test concurrent access to ensure thread safety
	var wg sync.WaitGroup
	numGoroutines := 100
	operationsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				cm.RecordSet("key", 1*time.Millisecond, true)
				cm.RecordGet("key", 1*time.Millisecond, true, true)
				cm.RecordDelete("key", 1*time.Millisecond, true)
			}
		}(i)
	}

	wg.Wait()

	stats := cm.GetStats()

	expectedHits := uint64(numGoroutines * operationsPerGoroutine) // Each RecordGet with hit=true
	expectedSets := uint64(numGoroutines * operationsPerGoroutine)
	expectedGets := uint64(numGoroutines * operationsPerGoroutine)
	expectedDeletes := uint64(numGoroutines * operationsPerGoroutine)

	if stats.CacheHits != expectedHits {
		t.Errorf("Expected %d hits, got %d", expectedHits, stats.CacheHits)
	}
	if stats.CacheMisses != 0 {
		t.Errorf("Expected 0 misses, got %d", stats.CacheMisses)
	}
	if stats.Sets != expectedSets {
		t.Errorf("Expected %d sets, got %d", expectedSets, stats.Sets)
	}
	if stats.Gets != expectedGets {
		t.Errorf("Expected %d gets, got %d", expectedGets, stats.Gets)
	}
	if stats.Deletes != expectedDeletes {
		t.Errorf("Expected %d deletes, got %d", expectedDeletes, stats.Deletes)
	}
}

func TestEdgeCases(t *testing.T) {
	cm := NewCacheMetrics()

	// Test hit rate with no operations
	if cm.GetHitRate() != 0 {
		t.Errorf("Expected hit rate to be 0 with no operations, got %f", cm.GetHitRate())
	}

	// Test miss rate with no operations
	if cm.GetMissRate() != 0 {
		t.Errorf("Expected miss rate to be 0 with no operations, got %f", cm.GetMissRate())
	}

	// Test total requests with no operations
	if cm.GetTotalRequests() != 0 {
		t.Errorf("Expected total requests to be 0 with no operations, got %d", cm.GetTotalRequests())
	}

	// Record only hits and check rates
	cm.RecordHit()
	cm.RecordHit()

	if cm.GetHitRate() != 100 {
		t.Errorf("Expected hit rate to be 100 with only hits, got %f", cm.GetHitRate())
	}
	if cm.GetMissRate() != 0 {
		t.Errorf("Expected miss rate to be 0 with only hits, got %f", cm.GetMissRate())
	}

	// Reset and record only misses
	cm.Reset()
	cm.RecordMiss()
	cm.RecordMiss()

	if cm.GetHitRate() != 0 {
		t.Errorf("Expected hit rate to be 0 with only misses, got %f", cm.GetHitRate())
	}
	if cm.GetMissRate() != 100 {
		t.Errorf("Expected miss rate to be 100 with only misses, got %f", cm.GetMissRate())
	}
}

// Benchmark tests
func BenchmarkRecordHit(b *testing.B) {
	cm := NewCacheMetrics()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cm.RecordHit()
	}
}

func BenchmarkRecordGet(b *testing.B) {
	cm := NewCacheMetrics()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cm.RecordGet("key", 1*time.Microsecond, true, true)
	}
}

func BenchmarkGetStats(b *testing.B) {
	cm := NewCacheMetrics()

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		cm.RecordHit()
		cm.RecordMiss()
		cm.RecordSet("key", 1*time.Millisecond, true)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cm.GetStats()
	}
}
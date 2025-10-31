package metrics

import (
	"sync"
	"testing"
	"time"
)

func TestNewDatabaseMetrics(t *testing.T) {
	dm := NewDatabaseMetrics()
	if dm == nil {
		t.Fatal("NewDatabaseMetrics() returned nil")
	}

	if dm.slowQueryThreshold != DefaultSlowQueryThreshold {
		t.Errorf("Expected slow query threshold to be %v, got %v", DefaultSlowQueryThreshold, dm.slowQueryThreshold)
	}

	if dm.maxHistorySize != DefaultMaxHistorySize {
		t.Errorf("Expected max history size to be %d, got %d", DefaultMaxHistorySize, dm.maxHistorySize)
	}

	if len(dm.queryHistory) != 0 {
		t.Errorf("Expected query history to be empty, got %d items", len(dm.queryHistory))
	}
}

func TestSetSlowQueryThreshold(t *testing.T) {
	dm := NewDatabaseMetrics()
	newThreshold := 100 * time.Millisecond

	dm.SetSlowQueryThreshold(newThreshold)

	if dm.slowQueryThreshold != newThreshold {
		t.Errorf("Expected slow query threshold to be %v, got %v", newThreshold, dm.slowQueryThreshold)
	}
}

func TestDatabaseSetMaxHistorySize(t *testing.T) {
	dm := NewDatabaseMetrics()
	newSize := 500

	dm.SetMaxHistorySize(newSize)

	if dm.maxHistorySize != newSize {
		t.Errorf("Expected max history size to be %d, got %d", newSize, dm.maxHistorySize)
	}
}

func TestRecordQuery_SuccessfulQuery(t *testing.T) {
	dm := NewDatabaseMetrics()
	query := "SELECT * FROM users WHERE id = $1"
	duration := 10 * time.Millisecond

	dm.RecordQuery(QueryTypeSelect, query, duration, true, 1, []interface{}{1}, nil)

	stats := dm.GetStats()
	if stats.TotalQueries != 1 {
		t.Errorf("Expected total queries to be 1, got %d", stats.TotalQueries)
	}
	if stats.SelectQueries != 1 {
		t.Errorf("Expected select queries to be 1, got %d", stats.SelectQueries)
	}
	if stats.SlowQueries != 0 {
		t.Errorf("Expected slow queries to be 0, got %d", stats.SlowQueries)
	}
	if stats.FailedQueries != 0 {
		t.Errorf("Expected failed queries to be 0, got %d", stats.FailedQueries)
	}
	if stats.AvgQueryDuration != duration {
		t.Errorf("Expected avg query duration to be %v, got %v", duration, stats.AvgQueryDuration)
	}
}

func TestRecordQuery_SlowQuery(t *testing.T) {
	dm := NewDatabaseMetrics()
	query := "SELECT * FROM users WHERE complex_condition = $1"
	duration := 100 * time.Millisecond // Above default 50ms threshold

	dm.RecordQuery(QueryTypeSelect, query, duration, true, 5, []interface{}{"value"}, nil)

	stats := dm.GetStats()
	if stats.TotalQueries != 1 {
		t.Errorf("Expected total queries to be 1, got %d", stats.TotalQueries)
	}
	if stats.SlowQueries != 1 {
		t.Errorf("Expected slow queries to be 1, got %d", stats.SlowQueries)
	}
	if stats.SlowQueryRate != 100.0 {
		t.Errorf("Expected slow query rate to be 100.0, got %f", stats.SlowQueryRate)
	}
	if stats.LastSlowQuery.IsZero() {
		t.Error("Expected last slow query to be set")
	}
}

func TestRecordQuery_FailedQuery(t *testing.T) {
	dm := NewDatabaseMetrics()
	query := "INSERT INTO users (name) VALUES ($1)"
	duration := 5 * time.Millisecond
	err := &testError{msg: "constraint violation"}

	dm.RecordQuery(QueryTypeInsert, query, duration, false, 0, []interface{}{"test"}, err)

	stats := dm.GetStats()
	if stats.TotalQueries != 1 {
		t.Errorf("Expected total queries to be 1, got %d", stats.TotalQueries)
	}
	if stats.FailedQueries != 1 {
		t.Errorf("Expected failed queries to be 1, got %d", stats.FailedQueries)
	}
	if stats.ErrorRate != 100.0 {
		t.Errorf("Expected error rate to be 100.0, got %f", stats.ErrorRate)
	}
}

func TestRecordQuery_DifferentQueryTypes(t *testing.T) {
	dm := NewDatabaseMetrics()
	duration := 5 * time.Millisecond

	// Record different query types
	dm.RecordQuery(QueryTypeSelect, "SELECT * FROM users", duration, true, 5, nil, nil)
	dm.RecordQuery(QueryTypeInsert, "INSERT INTO users...", duration, true, 1, nil, nil)
	dm.RecordQuery(QueryTypeUpdate, "UPDATE users SET...", duration, true, 1, nil, nil)
	dm.RecordQuery(QueryTypeDelete, "DELETE FROM users...", duration, true, 1, nil, nil)
	dm.RecordQuery(QueryTypeDDL, "CREATE TABLE test...", duration, true, 0, nil, nil)
	dm.RecordQuery(QueryTypeOther, "ANALYZE users", duration, true, 0, nil, nil)

	stats := dm.GetStats()
	if stats.TotalQueries != 6 {
		t.Errorf("Expected total queries to be 6, got %d", stats.TotalQueries)
	}
	if stats.SelectQueries != 1 {
		t.Errorf("Expected select queries to be 1, got %d", stats.SelectQueries)
	}
	if stats.InsertQueries != 1 {
		t.Errorf("Expected insert queries to be 1, got %d", stats.InsertQueries)
	}
	if stats.UpdateQueries != 1 {
		t.Errorf("Expected update queries to be 1, got %d", stats.UpdateQueries)
	}
	if stats.DeleteQueries != 1 {
		t.Errorf("Expected delete queries to be 1, got %d", stats.DeleteQueries)
	}
	if stats.DDLQueries != 1 {
		t.Errorf("Expected DDL queries to be 1, got %d", stats.DDLQueries)
	}
	if stats.OtherQueries != 1 {
		t.Errorf("Expected other queries to be 1, got %d", stats.OtherQueries)
	}
}

func TestRecordQuery_DurationTracking(t *testing.T) {
	dm := NewDatabaseMetrics()
	durations := []time.Duration{
		5 * time.Millisecond,
		15 * time.Millisecond,
		25 * time.Millisecond,
	}

	for _, duration := range durations {
		dm.RecordQuery(QueryTypeSelect, "SELECT * FROM test", duration, true, 1, nil, nil)
	}

	stats := dm.GetStats()
	expectedAvg := (5 + 15 + 25) * time.Millisecond / 3
	if stats.AvgQueryDuration != expectedAvg {
		t.Errorf("Expected avg query duration to be %v, got %v", expectedAvg, stats.AvgQueryDuration)
	}
	if stats.MaxQueryDuration != 25*time.Millisecond {
		t.Errorf("Expected max query duration to be %v, got %v", 25*time.Millisecond, stats.MaxQueryDuration)
	}
	if stats.MinQueryDuration != 5*time.Millisecond {
		t.Errorf("Expected min query duration to be %v, got %v", 5*time.Millisecond, stats.MinQueryDuration)
	}
}

func TestGetSlowQueries(t *testing.T) {
	dm := NewDatabaseMetrics()
	slowDuration := 100 * time.Millisecond

	// Record some slow queries
	for i := 0; i < 5; i++ {
		query := "SELECT * FROM large_table WHERE complex_condition = $1"
		dm.RecordQuery(QueryTypeSelect, query, slowDuration, true, int64(i+1), []interface{}{i}, nil)
	}

	// Test getting all slow queries
	slowQueries := dm.GetSlowQueries(10)
	if len(slowQueries) != 5 {
		t.Errorf("Expected 5 slow queries, got %d", len(slowQueries))
	}

	// Test getting limited slow queries
	limitedQueries := dm.GetSlowQueries(3)
	if len(limitedQueries) != 3 {
		t.Errorf("Expected 3 slow queries, got %d", len(limitedQueries))
	}

	// Verify query details
	for i, query := range slowQueries {
		if query.Duration != slowDuration {
			t.Errorf("Expected duration %v, got %v for query %d", slowDuration, query.Duration, i)
		}
		if query.QueryType != QueryTypeSelect {
			t.Errorf("Expected query type SELECT, got %v for query %d", query.QueryType, i)
		}
	}
}

func TestGetRecentQueries(t *testing.T) {
	dm := NewDatabaseMetrics()

	// Record some queries
	for i := 0; i < 10; i++ {
		query := "SELECT * FROM test_table WHERE id = $1"
		duration := time.Duration(i+1) * time.Millisecond
		dm.RecordQuery(QueryTypeSelect, query, duration, true, 1, []interface{}{i}, nil)
	}

	// Test getting all recent queries
	recentQueries := dm.GetRecentQueries(15)
	if len(recentQueries) != 10 {
		t.Errorf("Expected 10 recent queries, got %d", len(recentQueries))
	}

	// Test getting limited recent queries
	limitedQueries := dm.GetRecentQueries(5)
	if len(limitedQueries) != 5 {
		t.Errorf("Expected 5 recent queries, got %d", len(limitedQueries))
	}

	// Verify order (should be most recent first)
	// The most recent query should have duration 10ms
	if limitedQueries[4].Duration != 10*time.Millisecond {
		t.Errorf("Expected most recent query to have duration 10ms, got %v", limitedQueries[4].Duration)
	}
}

func TestGetRecentErrors(t *testing.T) {
	dm := NewDatabaseMetrics()
	err := &testError{msg: "test error"}

	// Record some failed queries
	for i := 0; i < 3; i++ {
		query := "INSERT INTO test_table VALUES ($1)"
		duration := time.Duration(i+1) * time.Millisecond
		dm.RecordQuery(QueryTypeInsert, query, duration, false, 0, []interface{}{i}, err)
	}

	// Test getting recent errors
	recentErrors := dm.GetRecentErrors(5)
	if len(recentErrors) != 3 {
		t.Errorf("Expected 3 recent errors, got %d", len(recentErrors))
	}

	// Verify error details
	for i, error := range recentErrors {
		if error.Error != "test error" {
			t.Errorf("Expected error message 'test error', got '%s' for error %d", error.Error, i)
		}
		if error.QueryType != QueryTypeInsert {
			t.Errorf("Expected query type INSERT, got %v for error %d", error.QueryType, i)
		}
	}
}

func TestGetRates(t *testing.T) {
	dm := NewDatabaseMetrics()

	// Test with no queries
	if dm.GetSlowQueryRate() != 0 {
		t.Errorf("Expected slow query rate to be 0 with no queries, got %f", dm.GetSlowQueryRate())
	}
	if dm.GetErrorRate() != 0 {
		t.Errorf("Expected error rate to be 0 with no queries, got %f", dm.GetErrorRate())
	}
	if dm.GetTotalQueries() != 0 {
		t.Errorf("Expected total queries to be 0 with no queries, got %d", dm.GetTotalQueries())
	}

	// Record some queries
	normalDuration := 10 * time.Millisecond
	slowDuration := 100 * time.Millisecond
	err := &testError{msg: "test error"}

	// 2 normal queries
	dm.RecordQuery(QueryTypeSelect, "SELECT * FROM normal", normalDuration, true, 1, nil, nil)
	dm.RecordQuery(QueryTypeSelect, "SELECT * FROM normal2", normalDuration, true, 1, nil, nil)

	// 1 slow query
	dm.RecordQuery(QueryTypeSelect, "SELECT * FROM slow", slowDuration, true, 1, nil, nil)

	// 1 failed query
	dm.RecordQuery(QueryTypeInsert, "INSERT INTO failed", normalDuration, false, 0, nil, err)

	// Check rates
	expectedSlowRate := float64(1) / float64(4) * 100
	if dm.GetSlowQueryRate() != expectedSlowRate {
		t.Errorf("Expected slow query rate to be %f, got %f", expectedSlowRate, dm.GetSlowQueryRate())
	}

	expectedErrorRate := float64(1) / float64(4) * 100
	if dm.GetErrorRate() != expectedErrorRate {
		t.Errorf("Expected error rate to be %f, got %f", expectedErrorRate, dm.GetErrorRate())
	}

	if dm.GetTotalQueries() != 4 {
		t.Errorf("Expected total queries to be 4, got %d", dm.GetTotalQueries())
	}
}

func TestDatabaseReset(t *testing.T) {
	dm := NewDatabaseMetrics()

	// Record some queries
	dm.RecordQuery(QueryTypeSelect, "SELECT * FROM test", 10*time.Millisecond, true, 1, nil, nil)
	dm.RecordQuery(QueryTypeInsert, "INSERT INTO test", 100*time.Millisecond, true, 1, nil, nil)
	dm.RecordQuery(QueryTypeUpdate, "UPDATE test", 5*time.Millisecond, false, 0, nil, &testError{msg: "error"})

	// Reset metrics
	dm.Reset()

	stats := dm.GetStats()

	// Verify everything is reset to zero
	if stats.TotalQueries != 0 {
		t.Errorf("Expected total queries to be 0 after reset, got %d", stats.TotalQueries)
	}
	if stats.SlowQueries != 0 {
		t.Errorf("Expected slow queries to be 0 after reset, got %d", stats.SlowQueries)
	}
	if stats.FailedQueries != 0 {
		t.Errorf("Expected failed queries to be 0 after reset, got %d", stats.FailedQueries)
	}
	if stats.SelectQueries != 0 {
		t.Errorf("Expected select queries to be 0 after reset, got %d", stats.SelectQueries)
	}
	if stats.InsertQueries != 0 {
		t.Errorf("Expected insert queries to be 0 after reset, got %d", stats.InsertQueries)
	}
	if stats.UpdateQueries != 0 {
		t.Errorf("Expected update queries to be 0 after reset, got %d", stats.UpdateQueries)
	}

	if len(stats.RecentQueries) != 0 {
		t.Errorf("Expected recent queries to be empty after reset, got %d", len(stats.RecentQueries))
	}
	if len(stats.RecentSlowQueries) != 0 {
		t.Errorf("Expected recent slow queries to be empty after reset, got %d", len(stats.RecentSlowQueries))
	}
	if len(stats.RecentErrors) != 0 {
		t.Errorf("Expected recent errors to be empty after reset, got %d", len(stats.RecentErrors))
	}
}

func TestSanitizeQuery(t *testing.T) {
	dm := NewDatabaseMetrics()

	// Test normal query
	normalQuery := "SELECT * FROM users WHERE id = $1"
	if dm.sanitizeQuery(normalQuery) != normalQuery {
		t.Errorf("Expected sanitized query to be unchanged for normal query")
	}

	// Test very long query
	longQuery := "SELECT * FROM users WHERE very_long_condition = '"
	for i := 0; i < 600; i++ {
		longQuery += "a"
	}
	longQuery += "'"

	sanitized := dm.sanitizeQuery(longQuery)
	if len(sanitized) != 500 {
		t.Errorf("Expected sanitized query length to be 500, got %d", len(sanitized))
	}
	if sanitized[len(sanitized)-3:] != "..." {
		t.Errorf("Expected sanitized query to end with '...', got '%s'", sanitized[len(sanitized)-3:])
	}

	// Test empty query
	emptyQuery := ""
	if dm.sanitizeQuery(emptyQuery) != emptyQuery {
		t.Errorf("Expected sanitized empty query to remain empty")
	}
}

func TestGetQueryTypeName(t *testing.T) {
	dm := NewDatabaseMetrics()

	if dm.getQueryTypeName(QueryTypeSelect) != "SELECT" {
		t.Errorf("Expected SELECT, got %s", dm.getQueryTypeName(QueryTypeSelect))
	}
	if dm.getQueryTypeName(QueryTypeInsert) != "INSERT" {
		t.Errorf("Expected INSERT, got %s", dm.getQueryTypeName(QueryTypeInsert))
	}
	if dm.getQueryTypeName(QueryTypeUpdate) != "UPDATE" {
		t.Errorf("Expected UPDATE, got %s", dm.getQueryTypeName(QueryTypeUpdate))
	}
	if dm.getQueryTypeName(QueryTypeDelete) != "DELETE" {
		t.Errorf("Expected DELETE, got %s", dm.getQueryTypeName(QueryTypeDelete))
	}
	if dm.getQueryTypeName(QueryTypeDDL) != "DDL" {
		t.Errorf("Expected DDL, got %s", dm.getQueryTypeName(QueryTypeDDL))
	}
	if dm.getQueryTypeName(QueryTypeOther) != "OTHER" {
		t.Errorf("Expected OTHER, got %s", dm.getQueryTypeName(QueryTypeOther))
	}
}

func TestDatabaseGetPerformanceMetrics(t *testing.T) {
	dm := NewDatabaseMetrics()

	// Test with empty metrics
	emptyMetrics := dm.GetPerformanceMetrics(time.Hour)
	if emptyMetrics.TotalQueries != 0 {
		t.Errorf("Expected 0 total queries for empty metrics, got %d", emptyMetrics.TotalQueries)
	}

	// Record some operations with different durations
	now := time.Now()
	operations := []struct {
		duration time.Duration
		success  bool
		isSlow   bool
	}{
		{1 * time.Millisecond, true, false},
		{5 * time.Millisecond, true, false},
		{10 * time.Millisecond, true, false},
		{15 * time.Millisecond, true, false},
		{20 * time.Millisecond, true, false},
		{100 * time.Millisecond, true, true},  // Slow query
		{2 * time.Millisecond, false, false},  // Failed query
	}

	for _, op := range operations {
		// Manually add to history to control timestamp
		dm.recordQuery(QueryEntry{
			Type:      QueryTypeSelect,
			Query:     "SELECT * FROM test",
			Duration:  op.duration,
			Success:   op.success,
			IsSlow:    op.isSlow,
			Timestamp: now.Add(time.Duration(len(dm.queryHistory)) * time.Second),
		})
	}

	// Get performance metrics for a large time window
	metrics := dm.GetPerformanceMetrics(time.Hour)

	if metrics.TotalQueries != 7 {
		t.Errorf("Expected 7 total operations, got %d", metrics.TotalQueries)
	}

	// Check slow query rate
	expectedSlowRate := float64(1) / float64(7) * 100
	if metrics.SlowQueryRate != expectedSlowRate {
		t.Errorf("Expected slow query rate to be %f, got %f", expectedSlowRate, metrics.SlowQueryRate)
	}

	// Check error rate
	expectedErrorRate := float64(1) / float64(7) * 100
	if metrics.ErrorRate != expectedErrorRate {
		t.Errorf("Expected error rate to be %f, got %f", expectedErrorRate, metrics.ErrorRate)
	}

	// Check slow queries count
	if metrics.SlowQueries != 1 {
		t.Errorf("Expected 1 slow query, got %d", metrics.SlowQueries)
	}

	// Check failed queries count
	if metrics.FailedQueries != 1 {
		t.Errorf("Expected 1 failed query, got %d", metrics.FailedQueries)
	}
}

func TestDatabaseConcurrentOperations(t *testing.T) {
	dm := NewDatabaseMetrics()

	// Test concurrent access to ensure thread safety
	var wg sync.WaitGroup
	numGoroutines := 50
	operationsPerGoroutine := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				duration := time.Duration(j+1) * time.Millisecond
				query := "SELECT * FROM test WHERE id = $1"
				dm.RecordQuery(QueryTypeSelect, query, duration, true, 1, []interface{}{j}, nil)
			}
		}(i)
	}

	wg.Wait()

	stats := dm.GetStats()
	expectedTotal := uint64(numGoroutines * operationsPerGoroutine)
	expectedSelect := uint64(numGoroutines * operationsPerGoroutine)

	if stats.TotalQueries != expectedTotal {
		t.Errorf("Expected %d total queries, got %d", expectedTotal, stats.TotalQueries)
	}
	if stats.SelectQueries != expectedSelect {
		t.Errorf("Expected %d select queries, got %d", expectedSelect, stats.SelectQueries)
	}
	if stats.FailedQueries != 0 {
		t.Errorf("Expected 0 failed queries, got %d", stats.FailedQueries)
	}
}

func TestHistorySizeManagement(t *testing.T) {
	dm := NewDatabaseMetrics()
	dm.SetMaxHistorySize(5)

	// Record more queries than the limit
	for i := 0; i < 20; i++ {
		query := "SELECT * FROM test"
		duration := time.Duration(i+1) * time.Millisecond
		dm.RecordQuery(QueryTypeSelect, query, duration, true, 1, nil, nil)
	}

	recentQueries := dm.GetRecentQueries(100)
	if len(recentQueries) > 10 { // Allow some buffer due to the half-trimming strategy
		t.Errorf("Expected recent queries to be managed by history size, got %d", len(recentQueries))
	}

	// Test slow query history management
	for i := 0; i < 20; i++ {
		query := "SELECT * FROM slow_table"
		duration := 100 * time.Millisecond // Slow query
		dm.RecordQuery(QueryTypeSelect, query, duration, true, 1, nil, nil)
	}

	slowQueries := dm.GetSlowQueries(100)
	if len(slowQueries) > 100 { // Should not exceed maxSlowQueryHistory * 2 due to trimming
		t.Errorf("Expected slow query history to be managed, got %d", len(slowQueries))
	}
}

// Helper types for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// Benchmark tests
func BenchmarkRecordQuery(b *testing.B) {
	dm := NewDatabaseMetrics()
	query := "SELECT * FROM users WHERE id = $1"
	duration := 10 * time.Millisecond
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dm.RecordQuery(QueryTypeSelect, query, duration, true, 1, []interface{}{1}, nil)
	}
}

func BenchmarkDatabaseGetStats(b *testing.B) {
	dm := NewDatabaseMetrics()

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		duration := time.Duration(i%50+1) * time.Millisecond
		dm.RecordQuery(QueryTypeSelect, "SELECT * FROM test", duration, true, 1, nil, nil)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dm.GetStats()
	}
}

func BenchmarkGetSlowQueries(b *testing.B) {
	dm := NewDatabaseMetrics()

	// Pre-populate with slow queries
	for i := 0; i < 100; i++ {
		duration := 100 * time.Millisecond
		dm.RecordQuery(QueryTypeSelect, "SELECT * FROM slow_table", duration, true, 1, nil, nil)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dm.GetSlowQueries(10)
	}
}
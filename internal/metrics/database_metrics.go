package metrics

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// DatabaseMetrics tracks database query performance statistics
type DatabaseMetrics struct {
	mu sync.RWMutex

	// Query counters
	totalQueries uint64
	slowQueries  uint64
	failedQueries uint64

	// Query type counters
	selectQueries uint64
	insertQueries uint64
	updateQueries uint64
	deleteQueries uint64
	ddlQueries    uint64
	otherQueries  uint64

	// Performance tracking
	totalQueryDuration int64 // in nanoseconds
	maxQueryDuration    int64 // in nanoseconds
	minQueryDuration    int64 // in nanoseconds

	// Slow query tracking
	slowQueryThreshold time.Duration
	lastSlowQuery      time.Time
	slowQueryHistory   []SlowQueryEntry
	maxSlowQueryHistory int

	// Query history for recent performance analysis
	queryHistory []QueryEntry
	maxHistorySize int

	// Error tracking
	errors []QueryError
	maxErrorHistory int
}

// QueryEntry represents a single database query operation
type QueryEntry struct {
	Type         QueryType     `json:"type"`
	Query        string        `json:"query,omitempty"`
	Duration     time.Duration `json:"duration"`
	Success      bool          `json:"success"`
	RowsAffected int64         `json:"rows_affected"`
	Timestamp    time.Time     `json:"timestamp"`
	IsSlow       bool          `json:"is_slow"`
}

// SlowQueryEntry represents a slow query entry for optimization review
type SlowQueryEntry struct {
	Query         string        `json:"query"`
	Parameters    interface{}   `json:"parameters,omitempty"`
	Duration      time.Duration `json:"duration"`
	RowsAffected  int64         `json:"rows_affected"`
	Timestamp     time.Time     `json:"timestamp"`
	QueryType     QueryType     `json:"query_type"`
	Error         string        `json:"error,omitempty"`
}

// QueryError represents a database query error
type QueryError struct {
	Query     string        `json:"query"`
	Error     string        `json:"error"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	QueryType QueryType     `json:"query_type"`
}

// QueryType represents the type of database query
type QueryType int

const (
	QueryTypeSelect QueryType = iota
	QueryTypeInsert
	QueryTypeUpdate
	QueryTypeDelete
	QueryTypeDDL
	QueryTypeOther
)

// DatabaseStats represents aggregated database statistics
type DatabaseStats struct {
	TotalQueries         uint64        `json:"total_queries"`
	SlowQueries          uint64        `json:"slow_queries"`
	FailedQueries        uint64        `json:"failed_queries"`
	SelectQueries        uint64        `json:"select_queries"`
	InsertQueries        uint64        `json:"insert_queries"`
	UpdateQueries        uint64        `json:"update_queries"`
	DeleteQueries        uint64        `json:"delete_queries"`
	DDLQueries           uint64        `json:"ddl_queries"`
	OtherQueries         uint64        `json:"other_queries"`
	AvgQueryDuration     time.Duration `json:"avg_query_duration"`
	MaxQueryDuration     time.Duration `json:"max_query_duration"`
	MinQueryDuration     time.Duration `json:"min_query_duration"`
	SlowQueryThreshold   time.Duration `json:"slow_query_threshold"`
	SlowQueryRate        float64       `json:"slow_query_rate"`
	ErrorRate            float64       `json:"error_rate"`
	LastSlowQuery        time.Time     `json:"last_slow_query,omitempty"`
	RecentQueries        []QueryEntry  `json:"recent_queries,omitempty"`
	RecentSlowQueries    []SlowQueryEntry `json:"recent_slow_queries,omitempty"`
	RecentErrors         []QueryError  `json:"recent_errors,omitempty"`
}

// Constants for database performance monitoring
const (
	DefaultSlowQueryThreshold = 50 * time.Millisecond // REQ-DB-003: 50ms threshold
	DefaultMaxHistorySize     = 1000
	DefaultMaxSlowQueryHistory = 100
	DefaultMaxErrorHistory    = 50
)

// NewDatabaseMetrics creates a new database metrics instance
func NewDatabaseMetrics() *DatabaseMetrics {
	return &DatabaseMetrics{
		slowQueryThreshold:   DefaultSlowQueryThreshold,
		maxHistorySize:       DefaultMaxHistorySize,
		maxSlowQueryHistory:  DefaultMaxSlowQueryHistory,
		maxErrorHistory:      DefaultMaxErrorHistory,
		queryHistory:         make([]QueryEntry, 0),
		slowQueryHistory:     make([]SlowQueryEntry, 0),
		errors:               make([]QueryError, 0),
	}
}

// SetSlowQueryThreshold sets the threshold for considering a query as slow
func (dm *DatabaseMetrics) SetSlowQueryThreshold(threshold time.Duration) {
	dm.slowQueryThreshold = threshold
}

// SetMaxHistorySize sets the maximum number of queries to keep in history
func (dm *DatabaseMetrics) SetMaxHistorySize(size int) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.maxHistorySize = size
	if len(dm.queryHistory) > size {
		// Trim history to new size
		dm.queryHistory = dm.queryHistory[len(dm.queryHistory)-size:]
	}
}

// RecordQuery records a database query operation
func (dm *DatabaseMetrics) RecordQuery(queryType QueryType, query string, duration time.Duration, success bool, rowsAffected int64, parameters interface{}, err error) {
	// Update atomic counters
	atomic.AddUint64(&dm.totalQueries, 1)
	atomic.AddInt64(&dm.totalQueryDuration, int64(duration))

	// Update query type counters
	switch queryType {
	case QueryTypeSelect:
		atomic.AddUint64(&dm.selectQueries, 1)
	case QueryTypeInsert:
		atomic.AddUint64(&dm.insertQueries, 1)
	case QueryTypeUpdate:
		atomic.AddUint64(&dm.updateQueries, 1)
	case QueryTypeDelete:
		atomic.AddUint64(&dm.deleteQueries, 1)
	case QueryTypeDDL:
		atomic.AddUint64(&dm.ddlQueries, 1)
	default:
		atomic.AddUint64(&dm.otherQueries, 1)
	}

	// Update min/max durations
	maxDuration := atomic.LoadInt64(&dm.maxQueryDuration)
	if duration > time.Duration(maxDuration) {
		atomic.CompareAndSwapInt64(&dm.maxQueryDuration, maxDuration, int64(duration))
	}

	minDuration := atomic.LoadInt64(&dm.minQueryDuration)
	if minDuration == 0 || duration < time.Duration(minDuration) {
		atomic.CompareAndSwapInt64(&dm.minQueryDuration, minDuration, int64(duration))
	}

	// Check if this is a slow query
	isSlowQuery := duration > dm.slowQueryThreshold
	if isSlowQuery {
		atomic.AddUint64(&dm.slowQueries, 1)
		
		// Update last slow query timestamp
		dm.mu.Lock()
		dm.lastSlowQuery = time.Now()
		dm.mu.Unlock()
		
		// Log slow query for optimization review (REQ-DB-003)
		dm.logSlowQuery(queryType, query, duration, parameters, err, rowsAffected)
		dm.recordSlowQuery(queryType, query, duration, parameters, err, rowsAffected)
	}

	// Handle failed queries
	if !success || err != nil {
		atomic.AddUint64(&dm.failedQueries, 1)
		dm.recordQueryError(queryType, query, duration, err)
	}

	// Record query in history
	dm.recordQuery(QueryEntry{
		Type:         queryType,
		Query:        dm.sanitizeQuery(query),
		Duration:     duration,
		Success:      success,
		RowsAffected: rowsAffected,
		Timestamp:    time.Now(),
		IsSlow:       isSlowQuery,
	})
}

// logSlowQuery logs slow queries for optimization review as per REQ-DB-003
func (dm *DatabaseMetrics) logSlowQuery(queryType QueryType, query string, duration time.Duration, parameters interface{}, err error, rowsAffected int64) {
	// Clean up the query string for logging
	cleanQuery := dm.sanitizeQuery(query)
	if cleanQuery == "" {
		cleanQuery = "Unknown query"
	}

	// Build the log message
	logMessage := fmt.Sprintf(
		"[SLOW QUERY - OPTIMIZATION REVIEW] Duration: %v | Type: %s | Query: %s | Rows: %d",
		duration,
		dm.getQueryTypeName(queryType),
		cleanQuery,
		rowsAffected,
	)

	// Add error information if present
	if err != nil {
		logMessage += fmt.Sprintf(" | Error: %v", err)
	}

	// Log the slow query for optimization review
	log.Printf("DATABASE PERFORMANCE WARNING: %s", logMessage)
}

// sanitizeQuery cleans up the query string for logging and storage
func (dm *DatabaseMetrics) sanitizeQuery(query string) string {
	// Truncate very long queries to avoid excessive storage
	if len(query) > 500 {
		return query[:497] + "..."
	}
	return query
}

// getQueryTypeName returns the string representation of query type
func (dm *DatabaseMetrics) getQueryTypeName(queryType QueryType) string {
	switch queryType {
	case QueryTypeSelect:
		return "SELECT"
	case QueryTypeInsert:
		return "INSERT"
	case QueryTypeUpdate:
		return "UPDATE"
	case QueryTypeDelete:
		return "DELETE"
	case QueryTypeDDL:
		return "DDL"
	default:
		return "OTHER"
	}
}

// recordSlowQuery records a slow query in the slow query history
func (dm *DatabaseMetrics) recordSlowQuery(queryType QueryType, query string, duration time.Duration, parameters interface{}, err error, rowsAffected int64) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	entry := SlowQueryEntry{
		Query:        dm.sanitizeQuery(query),
		Parameters:   parameters,
		Duration:     duration,
		RowsAffected: rowsAffected,
		Timestamp:    time.Now(),
		QueryType:    queryType,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	// Add to slow query history
	dm.slowQueryHistory = append(dm.slowQueryHistory, entry)

	// Trim history if it exceeds max size
	if len(dm.slowQueryHistory) > dm.maxSlowQueryHistory {
		// Remove oldest slow queries
		dm.slowQueryHistory = dm.slowQueryHistory[dm.maxSlowQueryHistory/2:]
	}
}

// recordQuery records a query in the query history
func (dm *DatabaseMetrics) recordQuery(entry QueryEntry) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// Add new query
	dm.queryHistory = append(dm.queryHistory, entry)

	// Trim history if it exceeds max size
	if len(dm.queryHistory) > dm.maxHistorySize {
		// Remove oldest queries (keep only the last maxHistorySize)
		dm.queryHistory = dm.queryHistory[dm.maxHistorySize/2:]
	}
}

// recordQueryError records a query error in the error history
func (dm *DatabaseMetrics) recordQueryError(queryType QueryType, query string, duration time.Duration, err error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	errorEntry := QueryError{
		Query:     dm.sanitizeQuery(query),
		Error:     err.Error(),
		Duration:  duration,
		Timestamp: time.Now(),
		QueryType: queryType,
	}

	// Add to error history
	dm.errors = append(dm.errors, errorEntry)

	// Trim error history if it exceeds max size
	if len(dm.errors) > dm.maxErrorHistory {
		// Remove oldest errors
		dm.errors = dm.errors[dm.maxErrorHistory/2:]
	}
}

// GetStats returns current database statistics
func (dm *DatabaseMetrics) GetStats() DatabaseStats {
	totalQueries := atomic.LoadUint64(&dm.totalQueries)
	slowQueries := atomic.LoadUint64(&dm.slowQueries)
	failedQueries := atomic.LoadUint64(&dm.failedQueries)
	selectQueries := atomic.LoadUint64(&dm.selectQueries)
	insertQueries := atomic.LoadUint64(&dm.insertQueries)
	updateQueries := atomic.LoadUint64(&dm.updateQueries)
	deleteQueries := atomic.LoadUint64(&dm.deleteQueries)
	ddlQueries := atomic.LoadUint64(&dm.ddlQueries)
	otherQueries := atomic.LoadUint64(&dm.otherQueries)
	totalQueryDuration := atomic.LoadInt64(&dm.totalQueryDuration)
	maxQueryDuration := atomic.LoadInt64(&dm.maxQueryDuration)
	minQueryDuration := atomic.LoadInt64(&dm.minQueryDuration)
	// Get last slow query timestamp
	dm.mu.RLock()
	lastSlowQuery := dm.lastSlowQuery
	dm.mu.RUnlock()

	// Calculate rates
	slowQueryRate := float64(0)
	errorRate := float64(0)

	if totalQueries > 0 {
		slowQueryRate = float64(slowQueries) / float64(totalQueries) * 100
		errorRate = float64(failedQueries) / float64(totalQueries) * 100
	}

	// Calculate average duration
	avgQueryDuration := time.Duration(0)
	if totalQueries > 0 {
		avgQueryDuration = time.Duration(totalQueryDuration / int64(totalQueries))
	}

	// Get recent data
	dm.mu.RLock()
	recentQueries := make([]QueryEntry, len(dm.queryHistory))
	copy(recentQueries, dm.queryHistory)

	recentSlowQueries := make([]SlowQueryEntry, len(dm.slowQueryHistory))
	copy(recentSlowQueries, dm.slowQueryHistory)

	recentErrors := make([]QueryError, len(dm.errors))
	copy(recentErrors, dm.errors)
	dm.mu.RUnlock()

	lastSlowQueryCopy := lastSlowQuery // Create a copy

	return DatabaseStats{
		TotalQueries:      totalQueries,
		SlowQueries:       slowQueries,
		FailedQueries:     failedQueries,
		SelectQueries:     selectQueries,
		InsertQueries:     insertQueries,
		UpdateQueries:     updateQueries,
		DeleteQueries:     deleteQueries,
		DDLQueries:        ddlQueries,
		OtherQueries:      otherQueries,
		AvgQueryDuration:  avgQueryDuration,
		MaxQueryDuration:  time.Duration(maxQueryDuration),
		MinQueryDuration:  time.Duration(minQueryDuration),
		SlowQueryThreshold: dm.slowQueryThreshold,
		SlowQueryRate:     slowQueryRate,
		ErrorRate:         errorRate,
		LastSlowQuery:     lastSlowQueryCopy,
		RecentQueries:     recentQueries,
		RecentSlowQueries: recentSlowQueries,
		RecentErrors:      recentErrors,
	}
}

// GetSlowQueries returns the most recent slow queries
func (dm *DatabaseMetrics) GetSlowQueries(limit int) []SlowQueryEntry {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if limit <= 0 || limit > len(dm.slowQueryHistory) {
		limit = len(dm.slowQueryHistory)
	}

	// Return the last 'limit' slow queries
	start := len(dm.slowQueryHistory) - limit
	if start < 0 {
		start = 0
	}

	recentQueries := make([]SlowQueryEntry, limit)
	copy(recentQueries, dm.slowQueryHistory[start:])
	return recentQueries
}

// GetRecentQueries returns the most recent queries
func (dm *DatabaseMetrics) GetRecentQueries(limit int) []QueryEntry {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if limit <= 0 || limit > len(dm.queryHistory) {
		limit = len(dm.queryHistory)
	}

	// Return the last 'limit' queries
	start := len(dm.queryHistory) - limit
	if start < 0 {
		start = 0
	}

	recentQueries := make([]QueryEntry, limit)
	copy(recentQueries, dm.queryHistory[start:])
	return recentQueries
}

// GetRecentErrors returns the most recent query errors
func (dm *DatabaseMetrics) GetRecentErrors(limit int) []QueryError {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if limit <= 0 || limit > len(dm.errors) {
		limit = len(dm.errors)
	}

	// Return the last 'limit' errors
	start := len(dm.errors) - limit
	if start < 0 {
		start = 0
	}

	recentErrors := make([]QueryError, limit)
	copy(recentErrors, dm.errors[start:])
	return recentErrors
}

// GetSlowQueryRate returns the current slow query rate as a percentage
func (dm *DatabaseMetrics) GetSlowQueryRate() float64 {
	totalQueries := atomic.LoadUint64(&dm.totalQueries)
	slowQueries := atomic.LoadUint64(&dm.slowQueries)

	if totalQueries == 0 {
		return 0
	}

	return float64(slowQueries) / float64(totalQueries) * 100
}

// GetErrorRate returns the current error rate as a percentage
func (dm *DatabaseMetrics) GetErrorRate() float64 {
	totalQueries := atomic.LoadUint64(&dm.totalQueries)
	failedQueries := atomic.LoadUint64(&dm.failedQueries)

	if totalQueries == 0 {
		return 0
	}

	return float64(failedQueries) / float64(totalQueries) * 100
}

// GetTotalQueries returns the total number of queries
func (dm *DatabaseMetrics) GetTotalQueries() uint64 {
	return atomic.LoadUint64(&dm.totalQueries)
}

// Reset resets all metrics
func (dm *DatabaseMetrics) Reset() {
	atomic.StoreUint64(&dm.totalQueries, 0)
	atomic.StoreUint64(&dm.slowQueries, 0)
	atomic.StoreUint64(&dm.failedQueries, 0)
	atomic.StoreUint64(&dm.selectQueries, 0)
	atomic.StoreUint64(&dm.insertQueries, 0)
	atomic.StoreUint64(&dm.updateQueries, 0)
	atomic.StoreUint64(&dm.deleteQueries, 0)
	atomic.StoreUint64(&dm.ddlQueries, 0)
	atomic.StoreUint64(&dm.otherQueries, 0)
	atomic.StoreInt64(&dm.totalQueryDuration, 0)
	atomic.StoreInt64(&dm.maxQueryDuration, 0)
	atomic.StoreInt64(&dm.minQueryDuration, 0)
	// Clear last slow query timestamp
	dm.mu.Lock()
	dm.lastSlowQuery = time.Time{}
	dm.mu.Unlock()

	dm.mu.Lock()
	dm.queryHistory = make([]QueryEntry, 0)
	dm.slowQueryHistory = make([]SlowQueryEntry, 0)
	dm.errors = make([]QueryError, 0)
	dm.mu.Unlock()

	log.Println("Database metrics reset")
}

// GetPerformanceMetrics returns detailed performance metrics for a time window
type DatabasePerformanceMetrics struct {
	TotalQueries       uint64        `json:"total_queries"`
	AvgResponseTime    time.Duration `json:"avg_response_time"`
	P95ResponseTime    time.Duration `json:"p95_response_time"`
	P99ResponseTime    time.Duration `json:"p99_response_time"`
	SlowQueryRate      float64       `json:"slow_query_rate"`
	ErrorRate          float64       `json:"error_rate"`
	QueriesPerSecond   float64       `json:"queries_per_second"`
	SlowQueries        uint64        `json:"slow_queries"`
	FailedQueries      uint64        `json:"failed_queries"`
}

// GetPerformanceMetrics calculates detailed performance metrics for a time window
func (dm *DatabaseMetrics) GetPerformanceMetrics(timeWindow time.Duration) DatabasePerformanceMetrics {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-timeWindow)

	// Filter queries within time window
	var recentQueries []QueryEntry
	for _, query := range dm.queryHistory {
		if query.Timestamp.After(cutoff) {
			recentQueries = append(recentQueries, query)
		}
	}

	if len(recentQueries) == 0 {
		return DatabasePerformanceMetrics{}
	}

	// Calculate response time percentiles
	durations := make([]time.Duration, 0, len(recentQueries))
	slowQueryCount := 0
	errorCount := 0

	for _, query := range recentQueries {
		durations = append(durations, query.Duration)
		if query.IsSlow {
			slowQueryCount++
		}
		if !query.Success {
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

	// Calculate queries per second
	queriesPerSecond := float64(len(recentQueries)) / timeWindow.Seconds()

	return DatabasePerformanceMetrics{
		TotalQueries:     uint64(len(recentQueries)),
		AvgResponseTime:  avgResponseTime,
		P95ResponseTime:  durations[p95Index],
		P99ResponseTime:  durations[p99Index],
		SlowQueryRate:    float64(slowQueryCount) / float64(len(recentQueries)) * 100,
		ErrorRate:        float64(errorCount) / float64(len(recentQueries)) * 100,
		QueriesPerSecond: queriesPerSecond,
		SlowQueries:      uint64(slowQueryCount),
		FailedQueries:    uint64(errorCount),
	}
}
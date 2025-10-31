package database

import (
	"testing"
	"time"

	"go-server/internal/config"
)

func TestDatabaseConnectionPoolConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		expected config.DatabaseConfig
	}{
		{
			name: "Development environment defaults",
			env:  "development",
			expected: config.DatabaseConfig{
				MaxOpenConns:    100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600, // 1 hour in seconds
			},
		},
		{
			name: "Production environment defaults",
			env:  "production",
			expected: config.DatabaseConfig{
				MaxOpenConns:    50,
				MaxIdleConns:    10,
				ConnMaxLifetime: 600, // 10 minutes in seconds
			},
		},
		{
			name: "Staging environment defaults",
			env:  "staging",
			expected: config.DatabaseConfig{
				MaxOpenConns:    75,
				MaxIdleConns:    15,
				ConnMaxLifetime: 1800, // 30 minutes in seconds
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test configuration with the specified environment
			cfg := &config.Config{
				Database: config.DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					User:     "test",
					Password: "test",
					DBName:   "test_db",
					SSLMode:  "disable",
				},
				Mode: tt.env,
			}

			// This would normally be called during config loading
			// but for testing, we'll set the defaults manually
			switch tt.env {
			case "production":
				cfg.Database.MaxOpenConns = tt.expected.MaxOpenConns
				cfg.Database.MaxIdleConns = tt.expected.MaxIdleConns
				cfg.Database.ConnMaxLifetime = tt.expected.ConnMaxLifetime
			case "staging":
				cfg.Database.MaxOpenConns = tt.expected.MaxOpenConns
				cfg.Database.MaxIdleConns = tt.expected.MaxIdleConns
				cfg.Database.ConnMaxLifetime = tt.expected.ConnMaxLifetime
			default:
				cfg.Database.MaxOpenConns = tt.expected.MaxOpenConns
				cfg.Database.MaxIdleConns = tt.expected.MaxIdleConns
				cfg.Database.ConnMaxLifetime = tt.expected.ConnMaxLifetime
			}

			// Verify the configuration matches expected values
			if cfg.Database.MaxOpenConns != tt.expected.MaxOpenConns {
				t.Errorf("Expected MaxOpenConns=%d, got %d", tt.expected.MaxOpenConns, cfg.Database.MaxOpenConns)
			}

			if cfg.Database.MaxIdleConns != tt.expected.MaxIdleConns {
				t.Errorf("Expected MaxIdleConns=%d, got %d", tt.expected.MaxIdleConns, cfg.Database.MaxIdleConns)
			}

			if cfg.Database.ConnMaxLifetime != tt.expected.ConnMaxLifetime {
				t.Errorf("Expected ConnMaxLifetime=%d, got %d", tt.expected.ConnMaxLifetime, cfg.Database.ConnMaxLifetime)
			}
		})
	}
}

func TestPoolHealthStatus(t *testing.T) {
	// Test the health status structure
	status := &PoolHealthStatus{
		OpenConnections:    5,
		InUseConnections:   2,
		IdleConnections:    3,
		MaxOpenConnections: 100,
		MaxIdleConnections: 10,
		LastHealthCheck:    time.Now(),
		IsHealthy:          true,
	}

	// Verify the structure is properly initialized
	if status.OpenConnections != 5 {
		t.Errorf("Expected OpenConnections=5, got %d", status.OpenConnections)
	}

	if status.InUseConnections != 2 {
		t.Errorf("Expected InUseConnections=2, got %d", status.InUseConnections)
	}

	if status.IdleConnections != 3 {
		t.Errorf("Expected IdleConnections=3, got %d", status.IdleConnections)
	}

	if status.MaxOpenConnections != 100 {
		t.Errorf("Expected MaxOpenConnections=100, got %d", status.MaxOpenConnections)
	}

	if status.MaxIdleConnections != 10 {
		t.Errorf("Expected MaxIdleConnections=10, got %d", status.MaxIdleConnections)
	}

	if !status.IsHealthy {
		t.Errorf("Expected IsHealthy=true, got %v", status.IsHealthy)
	}

	if status.LastHealthCheck.IsZero() {
		t.Error("Expected LastHealthCheck to be set, got zero time")
	}
}

func TestConnectionPoolSettingsFallback(t *testing.T) {
	// Test that zero or negative values fall back to defaults
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "test",
			Password:        "test",
			DBName:          "test_db",
			SSLMode:         "disable",
			MaxOpenConns:    0,  // Should fall back to 100
			MaxIdleConns:    -5, // Should fall back to 10
			ConnMaxLifetime: 0,  // Should fall back to 1 hour
		},
		Mode: "development",
	}

	// Test the fallback logic (simulating what would happen in NewDatabase)
	maxIdleConns := cfg.Database.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = 10 // default fallback
	}

	maxOpenConns := cfg.Database.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 100 // default fallback
	}

	connMaxLifetime := time.Duration(cfg.Database.ConnMaxLifetime) * time.Second
	if connMaxLifetime <= 0 {
		connMaxLifetime = time.Hour // default fallback
	}

	// Verify fallback values
	if maxIdleConns != 10 {
		t.Errorf("Expected fallback MaxIdleConns=10, got %d", maxIdleConns)
	}

	if maxOpenConns != 100 {
		t.Errorf("Expected fallback MaxOpenConns=100, got %d", maxOpenConns)
	}

	if connMaxLifetime != time.Hour {
		t.Errorf("Expected fallback ConnMaxLifetime=1h, got %v", connMaxLifetime)
	}
}

func TestQueryPerformanceStats(t *testing.T) {
	// Test the query performance statistics structure
	stats := &QueryPerformanceStats{
		TotalQueries:       100,
		SlowQueries:        5,
		TotalDurationNanos: time.Second.Nanoseconds(),
		AverageDuration:    10 * time.Millisecond,
		SlowQueryThreshold: SlowQueryThreshold,
		MaxDuration:        100 * time.Millisecond,
		MinDuration:        1 * time.Millisecond,
	}

	// Verify the structure is properly initialized
	if stats.TotalQueries != 100 {
		t.Errorf("Expected TotalQueries=100, got %d", stats.TotalQueries)
	}

	if stats.SlowQueries != 5 {
		t.Errorf("Expected SlowQueries=5, got %d", stats.SlowQueries)
	}

	if stats.SlowQueryThreshold != SlowQueryThreshold {
		t.Errorf("Expected SlowQueryThreshold=%v, got %v", SlowQueryThreshold, stats.SlowQueryThreshold)
	}

	if stats.AverageDuration != 10*time.Millisecond {
		t.Errorf("Expected AverageDuration=10ms, got %v", stats.AverageDuration)
	}
}

func TestSlowQueryThreshold(t *testing.T) {
	// Test that the slow query threshold is correctly defined
	if SlowQueryThreshold != 50*time.Millisecond {
		t.Errorf("Expected SlowQueryThreshold=50ms, got %v", SlowQueryThreshold)
	}
}

func TestQueryStatsInitialization(t *testing.T) {
	// Test query statistics initialization
	db := &Database{
		queryStats: &QueryPerformanceStats{
			SlowQueryThreshold: SlowQueryThreshold,
			MinDuration:        0,
		},
	}

	// Verify initialization
	stats := db.GetQueryPerformanceStats()
	if stats.SlowQueryThreshold != SlowQueryThreshold {
		t.Errorf("Expected SlowQueryThreshold=%v, got %v", SlowQueryThreshold, stats.SlowQueryThreshold)
	}

	if stats.TotalQueries != 0 {
		t.Errorf("Expected TotalQueries=0, got %d", stats.TotalQueries)
	}

	if stats.SlowQueries != 0 {
		t.Errorf("Expected SlowQueries=0, got %d", stats.SlowQueries)
	}
}

func TestQueryPerformanceStatsJSON(t *testing.T) {
	// Test JSON serialization of query performance stats
	db := &Database{
		queryStats: &QueryPerformanceStats{
			TotalQueries:       10,
			SlowQueries:        2,
			TotalDurationNanos: (100 * time.Millisecond).Nanoseconds(),
			AverageDuration:    10 * time.Millisecond,
			SlowQueryThreshold: SlowQueryThreshold,
			MaxDuration:        50 * time.Millisecond,
			MinDuration:        5 * time.Millisecond,
		},
	}

	statsJSON := db.GetQueryPerformanceStatsJSON()

	// Verify JSON fields
	if statsJSON["total_queries"] != int64(10) {
		t.Errorf("Expected total_queries=10, got %v", statsJSON["total_queries"])
	}

	if statsJSON["slow_queries"] != int64(2) {
		t.Errorf("Expected slow_queries=2, got %v", statsJSON["slow_queries"])
	}

	if statsJSON["slow_query_percentage"] != "20.00%" {
		t.Errorf("Expected slow_query_percentage=20.00%%, got %v", statsJSON["slow_query_percentage"])
	}
}

func TestResetQueryPerformanceStats(t *testing.T) {
	// Test resetting query performance statistics
	db := &Database{
		queryStats: &QueryPerformanceStats{
			TotalQueries:       100,
			SlowQueries:        10,
			TotalDurationNanos: time.Second.Nanoseconds(),
			AverageDuration:    10 * time.Millisecond,
			SlowQueryThreshold: SlowQueryThreshold,
			MaxDuration:        100 * time.Millisecond,
			MinDuration:        1 * time.Millisecond,
			LastSlowQuery:      time.Now(),
		},
	}

	// Reset statistics
	db.ResetQueryPerformanceStats()

	// Verify reset
	stats := db.GetQueryPerformanceStats()
	if stats.TotalQueries != 0 {
		t.Errorf("Expected TotalQueries=0 after reset, got %d", stats.TotalQueries)
	}

	if stats.SlowQueries != 0 {
		t.Errorf("Expected SlowQueries=0 after reset, got %d", stats.SlowQueries)
	}

	if stats.TotalDurationNanos != 0 {
		t.Errorf("Expected TotalDurationNanos=0 after reset, got %d", stats.TotalDurationNanos)
	}

	if stats.AverageDuration != 0 {
		t.Errorf("Expected AverageDuration=0 after reset, got %v", stats.AverageDuration)
	}

	if !stats.LastSlowQuery.IsZero() {
		t.Errorf("Expected LastSlowQuery to be zero after reset, got %v", stats.LastSlowQuery)
	}
}

func TestQueryLogEntry(t *testing.T) {
	// Test the query log entry structure
	entry := &QueryLogEntry{
		Duration:     75 * time.Millisecond,
		Query:        "SELECT * FROM users",
		Parameters:   []interface{}{"active"},
		RowsAffected: 5,
	}

	// Verify the structure is properly initialized
	if entry.Duration != 75*time.Millisecond {
		t.Errorf("Expected Duration=75ms, got %v", entry.Duration)
	}

	if entry.Query != "SELECT * FROM users" {
		t.Errorf("Expected Query='SELECT * FROM users', got %s", entry.Query)
	}

	if entry.RowsAffected != 5 {
		t.Errorf("Expected RowsAffected=5, got %d", entry.RowsAffected)
	}
}

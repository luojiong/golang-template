package database

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"go-server/internal/config"
	"go-server/internal/models"

	"github.com/google/uuid"
)

// setupBenchmarkDB creates a test database connection for benchmarks
func setupBenchmarkDB(b *testing.B) *Database {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "postgres",
			DBName:          "golang_template_bench",
			SSLMode:         "disable",
			MaxOpenConns:    100,
			MaxIdleConns:    10,
			ConnMaxLifetime: 3600, // 1 hour in seconds
		},
		Mode: "test",
	}

	loggerManager := logger.NewManager(config.LoggingConfig{}, "test")
	db, err := NewDatabase(cfg, loggerManager)
	if err != nil {
		b.Skipf("Skipping benchmark tests: database not available: %v", err)
		return nil
	}

	// Auto-migrate for benchmark tables
	if err := db.AutoMigrate(); err != nil {
		b.Skipf("Skipping benchmark tests: migration failed: %v", err)
	}

	// Clean up any existing test data
	db.DB.Exec("DELETE FROM users WHERE email LIKE 'bench-%'")

	return db
}

// cleanupBenchmarkDB cleans up test data after benchmarks
func cleanupBenchmarkDB(b *testing.B, db *Database) {
	if db != nil {
		// Clean up test data
		db.DB.Exec("DELETE FROM users WHERE email LIKE 'bench-%'")
		db.Close()
	}
}

// createTestUser creates a test user for benchmarks
func createTestUser(index int) models.User {
	return models.User{
		ID:        uuid.New().String(),
		Username:  fmt.Sprintf("benchuser_%d", index),
		Email:     fmt.Sprintf("bench-%d@example.com", index),
		Password:  "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi",
		FirstName: fmt.Sprintf("Bench_%d", index),
		LastName:  "User",
		IsActive:  true,
		IsAdmin:   false,
	}
}

// BenchmarkDatabaseConnection benchmarks database connection establishment
func BenchmarkDatabaseConnection(b *testing.B) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "postgres",
			DBName:          "golang_template_bench",
			SSLMode:         "disable",
			MaxOpenConns:    100,
			MaxIdleConns:    10,
			ConnMaxLifetime: 3600,
		},
		Mode: "test",
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		loggerManager := logger.NewManager(config.LoggingConfig{}, "test")
	db, err := NewDatabase(cfg, loggerManager)
		if err != nil {
			b.Fatalf("Failed to create database connection: %v", err)
		}
		db.Close()
	}
}

// BenchmarkConnectionPoolAcquisition benchmarks connection pool acquisition speed
func BenchmarkConnectionPoolAcquisition(b *testing.B) {
	db := setupBenchmarkDB(b)
	if db == nil {
		return
	}
	defer cleanupBenchmarkDB(b, db)

	sqlDB, err := db.DB.DB()
	if err != nil {
		b.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		conn, err := sqlDB.Conn(context.Background())
		if err != nil {
			b.Fatalf("Failed to acquire connection: %v", err)
		}
		conn.Close()
	}
}

// BenchmarkConnectionPoolConcurrency benchmarks connection pool under concurrent load
func BenchmarkConnectionPoolConcurrency(b *testing.B) {
	db := setupBenchmarkDB(b)
	if db == nil {
		return
	}
	defer cleanupBenchmarkDB(b, db)

	sqlDB, err := db.DB.DB()
	if err != nil {
		b.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	// Test with different numbers of goroutines
	concurrencyLevels := []int{10, 50, 100, 500, 1000}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Goroutines-%d", concurrency), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			var wg sync.WaitGroup
			semaphore := make(chan struct{}, concurrency)

			for i := 0; i < b.N; i++ {
				wg.Add(1)
				semaphore <- struct{}{}

				go func() {
					defer wg.Done()
					defer func() { <-semaphore }()

					conn, err := sqlDB.Conn(context.Background())
					if err != nil {
						b.Errorf("Failed to acquire connection: %v", err)
						return
					}

					// Perform a simple query
					var result int
					conn.QueryRowContext(context.Background(), "SELECT 1").Scan(&result)
					conn.Close()
				}()
			}

			wg.Wait()
		})
	}
}

// BenchmarkSimpleQuery benchmarks simple SELECT query performance
func BenchmarkSimpleQuery(b *testing.B) {
	db := setupBenchmarkDB(b)
	if db == nil {
		return
	}
	defer cleanupBenchmarkDB(b, db)

	// Reset query statistics before benchmark
	db.ResetQueryPerformanceStats()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var count int64
		if err := db.DB.Raw("SELECT COUNT(*) FROM users").Scan(&count).Error; err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}

	// Report query performance statistics
	stats := db.GetQueryPerformanceStats()
	b.ReportMetric(float64(stats.AverageDuration.Nanoseconds())/1e6, "avg_ms/op")
	b.ReportMetric(float64(stats.TotalQueries), "queries_total")
}

// BenchmarkInsertOperations benchmarks INSERT operation performance
func BenchmarkInsertOperations(b *testing.B) {
	db := setupBenchmarkDB(b)
	if db == nil {
		return
	}
	defer cleanupBenchmarkDB(b, db)

	// Reset query statistics before benchmark
	db.ResetQueryPerformanceStats()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		user := createTestUser(i)
		if err := db.DB.Create(&user).Error; err != nil {
			b.Fatalf("Insert failed: %v", err)
		}
	}

	// Report query performance statistics
	stats := db.GetQueryPerformanceStats()
	b.ReportMetric(float64(stats.AverageDuration.Nanoseconds())/1e6, "avg_ms/op")
	b.ReportMetric(float64(stats.TotalQueries), "queries_total")
}

// BenchmarkBatchInsert benchmarks batch INSERT operation performance
func BenchmarkBatchInsert(b *testing.B) {
	db := setupBenchmarkDB(b)
	if db == nil {
		return
	}
	defer cleanupBenchmarkDB(b, db)

	batchSizes := []int{10, 50, 100, 500}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("BatchSize-%d", batchSize), func(b *testing.B) {
			// Reset query statistics before benchmark
			db.ResetQueryPerformanceStats()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				var users []models.User
				for j := 0; j < batchSize; j++ {
					users = append(users, createTestUser(i*batchSize+j))
				}

				if err := db.DB.CreateInBatches(users, batchSize).Error; err != nil {
					b.Fatalf("Batch insert failed: %v", err)
				}
			}

			// Report query performance statistics
			stats := db.GetQueryPerformanceStats()
			b.ReportMetric(float64(stats.AverageDuration.Nanoseconds())/1e6, "avg_ms/op")
			b.ReportMetric(float64(stats.TotalQueries), "queries_total")
		})
	}
}

// BenchmarkUpdateOperations benchmarks UPDATE operation performance
func BenchmarkUpdateOperations(b *testing.B) {
	db := setupBenchmarkDB(b)
	if db == nil {
		return
	}
	defer cleanupBenchmarkDB(b, db)

	// Create some test users first
	var testUsers []models.User
	for i := 0; i < 100; i++ {
		user := createTestUser(i)
		if err := db.DB.Create(&user).Error; err != nil {
			b.Fatalf("Failed to create test user: %v", err)
		}
		testUsers = append(testUsers, user)
	}

	// Reset query statistics before benchmark
	db.ResetQueryPerformanceStats()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		userIndex := i % len(testUsers)
		newFirstName := fmt.Sprintf("Updated_%d", i)
		if err := db.DB.Model(&testUsers[userIndex]).Update("first_name", newFirstName).Error; err != nil {
			b.Fatalf("Update failed: %v", err)
		}
	}

	// Report query performance statistics
	stats := db.GetQueryPerformanceStats()
	b.ReportMetric(float64(stats.AverageDuration.Nanoseconds())/1e6, "avg_ms/op")
	b.ReportMetric(float64(stats.TotalQueries), "queries_total")
}

// BenchmarkDeleteOperations benchmarks DELETE operation performance
func BenchmarkDeleteOperations(b *testing.B) {
	db := setupBenchmarkDB(b)
	if db == nil {
		return
	}
	defer cleanupBenchmarkDB(b, db)

	// Create test users in batches for deletion testing
	createTestUsers := func(count int) []models.User {
		var users []models.User
		for i := 0; i < count; i++ {
			user := createTestUser(i)
			if err := db.DB.Create(&user).Error; err != nil {
				b.Fatalf("Failed to create test user: %v", err)
			}
			users = append(users, user)
		}
		return users
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create fresh users for each iteration
		testUsers := createTestUsers(10)

		// Delete the users
		for _, user := range testUsers {
			if err := db.DB.Delete(&user).Error; err != nil {
				b.Fatalf("Delete failed: %v", err)
			}
		}
	}
}

// BenchmarkComplexQueries benchmarks complex query performance
func BenchmarkComplexQueries(b *testing.B) {
	db := setupBenchmarkDB(b)
	if db == nil {
		return
	}
	defer cleanupBenchmarkDB(b, db)

	// Create test data
	var testUsers []models.User
	for i := 0; i < 1000; i++ {
		user := createTestUser(i)
		user.IsActive = i%2 == 0 // Mix of active/inactive users
		user.IsAdmin = i%10 == 0 // 10% admin users
		if err := db.DB.Create(&user).Error; err != nil {
			b.Fatalf("Failed to create test user: %v", err)
		}
		testUsers = append(testUsers, user)
	}

	// Reset query statistics before benchmark
	db.ResetQueryPerformanceStats()

	b.Run("ComplexSelectWithJoins", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			var users []models.User
			if err := db.DB.Where("is_active = ? AND is_admin = ?", true, false).
				Order("created_at DESC").
				Limit(100).
				Find(&users).Error; err != nil {
				b.Fatalf("Complex query failed: %v", err)
			}
		}
	})

	b.Run("AggregationQuery", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			var result struct {
				TotalUsers   int64 `json:"total_users"`
				ActiveUsers  int64 `json:"active_users"`
				AdminUsers   int64 `json:"admin_users"`
				ActiveAdmins int64 `json:"active_admins"`
			}

			if err := db.DB.Model(&models.User{}).
				Select("COUNT(*) as total_users, COUNT(CASE WHEN is_active = true THEN 1 END) as active_users, COUNT(CASE WHEN is_admin = true THEN 1 END) as admin_users, COUNT(CASE WHEN is_active = true AND is_admin = true THEN 1 END) as active_admins").
				Scan(&result).Error; err != nil {
				b.Fatalf("Aggregation query failed: %v", err)
			}
		}
	})

	// Report query performance statistics
	stats := db.GetQueryPerformanceStats()
	b.ReportMetric(float64(stats.AverageDuration.Nanoseconds())/1e6, "avg_ms/op")
	b.ReportMetric(float64(stats.TotalQueries), "queries_total")
}

// BenchmarkConnectionPoolConfiguration benchmarks different connection pool configurations
func BenchmarkConnectionPoolConfiguration(b *testing.B) {
	configs := []struct {
		name         string
		maxOpenConns int
		maxIdleConns int
	}{
		{"Small", 10, 2},
		{"Medium", 50, 10},
		{"Large", 100, 20},
		{"ExtraLarge", 200, 50},
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			testCfg := &config.Config{
				Database: config.DatabaseConfig{
					Host:            "localhost",
					Port:            5432,
					User:            "postgres",
					Password:        "postgres",
					DBName:          "golang_template_bench",
					SSLMode:         "disable",
					MaxOpenConns:    cfg.maxOpenConns,
					MaxIdleConns:    cfg.maxIdleConns,
					ConnMaxLifetime: 3600,
				},
				Mode: "test",
			}

			db, err := NewDatabase(testCfg)
			if err != nil {
				b.Skipf("Skipping config %s: %v", cfg.name, err)
				return
			}
			defer db.Close()

			// Reset query statistics before benchmark
			db.ResetQueryPerformanceStats()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				var count int64
				if err := db.DB.Raw("SELECT COUNT(*) FROM users").Scan(&count).Error; err != nil {
					b.Fatalf("Query failed: %v", err)
				}
			}

			// Report connection pool statistics
			poolStats, err := db.GetConnectionPoolStats()
			if err == nil {
				b.ReportMetric(poolStats["utilization_percent"].(float64), "pool_utilization_%")
			}
		})
	}
}

// BenchmarkHealthCheck benchmarks database health check performance
func BenchmarkHealthCheck(b *testing.B) {
	db := setupBenchmarkDB(b)
	if db == nil {
		return
	}
	defer cleanupBenchmarkDB(b, db)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := db.Health(); err != nil {
			b.Fatalf("Health check failed: %v", err)
		}
	}
}

// BenchmarkConcurrentQueries benchmarks concurrent query execution
func BenchmarkConcurrentQueries(b *testing.B) {
	db := setupBenchmarkDB(b)
	if db == nil {
		return
	}
	defer cleanupBenchmarkDB(b, db)

	concurrencyLevels := []int{10, 50, 100}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency-%d", concurrency), func(b *testing.B) {
			// Reset query statistics before benchmark
			db.ResetQueryPerformanceStats()

			b.ResetTimer()
			b.ReportAllocs()

			var wg sync.WaitGroup
			semaphore := make(chan struct{}, concurrency)

			for i := 0; i < b.N; i++ {
				wg.Add(1)
				semaphore <- struct{}{}

				go func(index int) {
					defer wg.Done()
					defer func() { <-semaphore }()

					var users []models.User
					if err := db.DB.Where("is_active = ?", true).
						Limit(10).
						Find(&users).Error; err != nil {
						b.Errorf("Concurrent query failed: %v", err)
					}
				}(i)
			}

			wg.Wait()

			// Report query performance statistics
			stats := db.GetQueryPerformanceStats()
			b.ReportMetric(float64(stats.AverageDuration.Nanoseconds())/1e6, "avg_ms/op")
			b.ReportMetric(float64(stats.TotalQueries), "queries_total")
		})
	}
}

// BenchmarkQueryPerformanceMonitoring benchmarks query performance monitoring overhead
func BenchmarkQueryPerformanceMonitoring(b *testing.B) {
	db := setupBenchmarkDB(b)
	if db == nil {
		return
	}
	defer cleanupBenchmarkDB(b, db)

	b.Run("WithMonitoring", func(b *testing.B) {
		db.ResetQueryPerformanceStats()
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			var count int64
			if err := db.DB.Raw("SELECT COUNT(*) FROM users").Scan(&count).Error; err != nil {
				b.Fatalf("Query failed: %v", err)
			}
		}
	})
}

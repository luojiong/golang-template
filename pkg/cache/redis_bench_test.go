package cache

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// setupBenchmarkCache creates a test Redis cache for benchmarks
func setupBenchmarkCache(b *testing.B) Cache {
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.DB = 2 // Use different DB for benchmarks
	config.Prefix = "bench:"
	config.PoolSize = 50
	config.MinIdleConns = 10

	cache, err := NewRedisCache(config)
	if err != nil {
		b.Skipf("Skipping benchmark tests: Redis not available: %v", err)
		return nil
	}

	// Clean up any existing benchmark data
	ctx := context.Background()
	cache.Clear(ctx)

	return cache
}

// cleanupBenchmarkCache cleans up test data after benchmarks
func cleanupBenchmarkCache(b *testing.B, cache Cache) {
	if cache != nil {
		ctx := context.Background()
		cache.Clear(ctx)
		cache.Close()
	}
}

// generateBenchmarkValue creates a test value of specified size
func generateBenchmarkValue(size int) []byte {
	value := make([]byte, size)
	for i := range value {
		value[i] = byte(rand.Intn(256))
	}
	return value
}

// generateBenchmarkData creates test data for benchmarks
func generateBenchmarkData(count int, valueSize int) map[string]interface{} {
	data := make(map[string]interface{})
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("bench_key_%d", i)
		value := generateBenchmarkValue(valueSize)
		data[key] = value
	}
	return data
}

// BenchmarkCacheSet benchmarks cache Set operations
func BenchmarkCacheSet(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_set_%d", i)
		value := generateBenchmarkValue(256) // 256 bytes
		err := cache.Set(ctx, key, value, time.Hour)
		if err != nil {
			b.Fatalf("Failed to set cache value: %v", err)
		}
	}
}

// BenchmarkCacheSetSmall benchmarks cache Set operations with small values (64 bytes)
func BenchmarkCacheSetSmall(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_set_small_%d", i)
		value := generateBenchmarkValue(64) // 64 bytes
		err := cache.Set(ctx, key, value, time.Hour)
		if err != nil {
			b.Fatalf("Failed to set cache value: %v", err)
		}
	}
}

// BenchmarkCacheSetMedium benchmarks cache Set operations with medium values (1KB)
func BenchmarkCacheSetMedium(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_set_medium_%d", i)
		value := generateBenchmarkValue(1024) // 1KB
		err := cache.Set(ctx, key, value, time.Hour)
		if err != nil {
			b.Fatalf("Failed to set cache value: %v", err)
		}
	}
}

// BenchmarkCacheSetLarge benchmarks cache Set operations with large values (64KB)
func BenchmarkCacheSetLarge(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_set_large_%d", i)
		value := generateBenchmarkValue(64 * 1024) // 64KB
		err := cache.Set(ctx, key, value, time.Hour)
		if err != nil {
			b.Fatalf("Failed to set cache value: %v", err)
		}
	}
}

// BenchmarkCacheGet benchmarks cache Get operations
func BenchmarkCacheGet(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	// Pre-populate cache with test data
	preWarmCount := 1000
	for i := 0; i < preWarmCount; i++ {
		key := fmt.Sprintf("bench_get_%d", i)
		value := generateBenchmarkValue(256)
		err := cache.Set(ctx, key, value, time.Hour)
		if err != nil {
			b.Fatalf("Failed to pre-warm cache: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_get_%d", i%preWarmCount)
		_, found := cache.Get(ctx, key)
		if !found {
			b.Fatalf("Expected to find key %s in cache", key)
		}
	}
}

// BenchmarkCacheGetMiss benchmarks cache Get operations with misses
func BenchmarkCacheGetMiss(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("nonexistent_key_%d", i)
		_, found := cache.Get(ctx, key)
		if found {
			b.Fatalf("Expected not to find key %s in cache", key)
		}
	}
}

// BenchmarkCacheSetMultiple benchmarks batch Set operations
func BenchmarkCacheSetMultiple(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		batchSize := 100
		items := make(map[string]interface{})
		for j := 0; j < batchSize; j++ {
			key := fmt.Sprintf("bench_batch_%d_%d", i, j)
			value := generateBenchmarkValue(256)
			items[key] = value
		}

		err := cache.SetMultiple(ctx, items, time.Hour)
		if err != nil {
			b.Fatalf("Failed to set multiple cache values: %v", err)
		}
	}
}

// BenchmarkCacheGetMultiple benchmarks batch Get operations
func BenchmarkCacheGetMultiple(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	// Pre-populate cache with test data
	batchCount := 100
	itemsPerBatch := 50
	for i := 0; i < batchCount; i++ {
		items := make(map[string]interface{})
		for j := 0; j < itemsPerBatch; j++ {
			key := fmt.Sprintf("bench_multiget_%d_%d", i, j)
			value := generateBenchmarkValue(256)
			items[key] = value
		}
		err := cache.SetMultiple(ctx, items, time.Hour)
		if err != nil {
			b.Fatalf("Failed to pre-warm cache: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		keys := make([]string, itemsPerBatch)
		for j := 0; j < itemsPerBatch; j++ {
			keys[j] = fmt.Sprintf("bench_multiget_%d_%d", i%batchCount, j)
		}

		results, err := cache.GetMultiple(ctx, keys)
		if err != nil {
			b.Fatalf("Failed to get multiple cache values: %v", err)
		}
		if len(results) != itemsPerBatch {
			b.Fatalf("Expected %d results, got %d", itemsPerBatch, len(results))
		}
	}
}

// BenchmarkCacheDelete benchmarks cache Delete operations
func BenchmarkCacheDelete(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	// Pre-populate cache with test data
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_delete_%d", i)
		value := generateBenchmarkValue(256)
		err := cache.Set(ctx, key, value, time.Hour)
		if err != nil {
			b.Fatalf("Failed to pre-warm cache: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_delete_%d", i)
		err := cache.Delete(ctx, key)
		if err != nil {
			b.Fatalf("Failed to delete cache value: %v", err)
		}
	}
}

// BenchmarkCacheExists benchmarks cache Exists operations
func BenchmarkCacheExists(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	// Pre-populate cache with test data
	existingKeys := 500
	for i := 0; i < existingKeys; i++ {
		key := fmt.Sprintf("bench_exists_%d", i)
		value := generateBenchmarkValue(256)
		err := cache.Set(ctx, key, value, time.Hour)
		if err != nil {
			b.Fatalf("Failed to pre-warm cache: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_exists_%d", i%existingKeys)
		_, err := cache.Exists(ctx, key)
		if err != nil {
			b.Fatalf("Failed to check if key exists: %v", err)
		}
	}
}

// BenchmarkCacheIncrement benchmarks cache Increment operations
func BenchmarkCacheIncrement(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	// Pre-populate cache with initial values
	initialCount := 100
	for i := 0; i < initialCount; i++ {
		key := fmt.Sprintf("bench_incr_%d", i)
		err := cache.Set(ctx, key, int64(0), time.Hour)
		if err != nil {
			b.Fatalf("Failed to pre-warm cache: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_incr_%d", i%initialCount)
		_, err := cache.Increment(ctx, key, 1)
		if err != nil {
			b.Fatalf("Failed to increment cache value: %v", err)
		}
	}
}

// BenchmarkCacheSetIfNotExists benchmarks SetIfNotExists operations
func BenchmarkCacheSetIfNotExists(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_setnx_%d", i)
		value := generateBenchmarkValue(256)
		_, err := cache.SetIfNotExists(ctx, key, value, time.Hour)
		if err != nil {
			b.Fatalf("Failed to set if not exists: %v", err)
		}
	}
}

// BenchmarkCacheHitRate benchmarks cache hit rate performance
func BenchmarkCacheHitRate(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	// Simulate realistic hit rate scenarios
	hitRates := []struct {
		name      string
		hitRate   float64
		totalKeys int
		hotKeys   int
	}{
		{"HighHitRate_90%", 0.9, 1000, 100},
		{"MediumHitRate_70%", 0.7, 1000, 300},
		{"LowHitRate_30%", 0.3, 1000, 700},
	}

	for _, scenario := range hitRates {
		b.Run(scenario.name, func(b *testing.B) {
			// Clear and pre-populate cache with hot keys
			cache.Clear(ctx)

			for i := 0; i < scenario.hotKeys; i++ {
				key := fmt.Sprintf("hot_key_%d", i)
				value := generateBenchmarkValue(512)
				err := cache.Set(ctx, key, value, time.Hour)
				if err != nil {
					b.Fatalf("Failed to pre-warm cache: %v", err)
				}
			}

			b.ResetTimer()
			b.ReportAllocs()
			b.StopTimer()

			hits := 0
			misses := 0

			b.StartTimer()
			for i := 0; i < b.N; i++ {
				var key string
				if rand.Float64() < scenario.hitRate {
					// Access hot key (should be in cache)
					key = fmt.Sprintf("hot_key_%d", rand.Intn(scenario.hotKeys))
				} else {
					// Access cold key (should not be in cache)
					key = fmt.Sprintf("cold_key_%d", rand.Intn(scenario.totalKeys-scenario.hotKeys))
				}

				_, found := cache.Get(ctx, key)
				if found {
					hits++
				} else {
					misses++
				}
			}
			b.StopTimer()

			actualHitRate := float64(hits) / float64(hits+misses)
			b.ReportMetric(actualHitRate, "hit_rate")
		})
	}
}

// BenchmarkCacheConcurrency benchmarks concurrent cache operations
func BenchmarkCacheConcurrency(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("conc_key_%d", i)
		value := generateBenchmarkValue(256)
		err := cache.Set(ctx, key, value, time.Hour)
		if err != nil {
			b.Fatalf("Failed to pre-warm cache: %v", err)
		}
	}

	concurrencyLevels := []int{1, 2, 4, 8, 16, 32, 64}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Goroutines_%d", concurrency), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			var wg sync.WaitGroup
			b.SetParallelism(concurrency)

			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					// Mix of read and write operations
					keyIndex := rand.Intn(1000)
					key := fmt.Sprintf("conc_key_%d", keyIndex)

					if rand.Float64() < 0.8 {
						// 80% read operations
						cache.Get(ctx, key)
					} else {
						// 20% write operations
						value := generateBenchmarkValue(256)
						cache.Set(ctx, key, value, time.Hour)
					}
				}
			})
			wg.Wait()
		})
	}
}

// BenchmarkCacheTTL benchmarks cache operations with TTL
func BenchmarkCacheTTL(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_ttl_%d", i)
		value := generateBenchmarkValue(256)
		ttl := time.Duration(rand.Intn(3600)) * time.Second // Random TTL up to 1 hour
		err := cache.Set(ctx, key, value, ttl)
		if err != nil {
			b.Fatalf("Failed to set cache value with TTL: %v", err)
		}
	}
}

// BenchmarkCacheGetWithTTL benchmarks GetWithTTL operations
func BenchmarkCacheGetWithTTL(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	// Pre-populate cache with TTL values
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("bench_getttl_%d", i)
		value := generateBenchmarkValue(256)
		ttl := time.Duration(rand.Intn(3600)) * time.Second
		err := cache.Set(ctx, key, value, ttl)
		if err != nil {
			b.Fatalf("Failed to pre-warm cache: %v", err)
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_getttl_%d", i%1000)
		_, _, found := cache.GetWithTTL(ctx, key)
		if !found {
			b.Fatalf("Expected to find key %s in cache", key)
		}
	}
}

// BenchmarkCacheComplexTypes benchmarks cache operations with complex data types
func BenchmarkCacheComplexTypes(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	// Define test complex types
	type UserProfile struct {
		ID        string                 `json:"id"`
		Username  string                 `json:"username"`
		Email     string                 `json:"email"`
		CreatedAt time.Time              `json:"created_at"`
		Metadata  map[string]interface{} `json:"metadata"`
	}

	complexTypes := []struct {
		name  string
		value interface{}
	}{
		{
			name: "UserProfile",
			value: UserProfile{
				ID:        uuid.New().String(),
				Username:  "testuser",
				Email:     "test@example.com",
				CreatedAt: time.Now(),
				Metadata:  map[string]interface{}{"role": "user", "active": true},
			},
		},
		{
			name:  "StringSlice",
			value: []string{"item1", "item2", "item3", "item4", "item5"},
		},
		{
			name: "Map",
			value: map[string]interface{}{
				"string":  "value",
				"number":  42,
				"boolean": true,
				"array":   []int{1, 2, 3, 4, 5},
			},
		},
	}

	for _, complexType := range complexTypes {
		b.Run(complexType.name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("complex_%s_%d", complexType.name, i)

				// Test Set operation
				err := cache.Set(ctx, key, complexType.value, time.Hour)
				if err != nil {
					b.Fatalf("Failed to set complex type: %v", err)
				}

				// Test Get operation
				_, found := cache.Get(ctx, key)
				if !found {
					b.Fatalf("Expected to find key %s in cache", key)
				}
			}
		})
	}
}

// BenchmarkCacheMemoryUsage benchmarks memory usage patterns
func BenchmarkCacheMemoryUsage(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	valueSizes := []int{
		64,    // 64 bytes
		256,   // 256 bytes
		1024,  // 1KB
		4096,  // 4KB
		16384, // 16KB
		65536, // 64KB
	}

	for _, size := range valueSizes {
		b.Run(fmt.Sprintf("ValueSize_%dB", size), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				key := fmt.Sprintf("mem_test_%d_%d", size, i)
				value := generateBenchmarkValue(size)
				err := cache.Set(ctx, key, value, time.Hour)
				if err != nil {
					b.Fatalf("Failed to set value: %v", err)
				}

				// Retrieve to simulate realistic usage
				_, found := cache.Get(ctx, key)
				if !found {
					b.Fatalf("Expected to find key %s in cache", key)
				}
			}
		})
	}
}

// BenchmarkCacheRealWorldScenario simulates real-world caching scenarios
func BenchmarkCacheRealWorldScenario(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	// Simulate different caching patterns
	scenarios := []struct {
		name     string
		setup    func() Cache
		testFunc func(b *testing.B, cache Cache)
	}{
		{
			name: "UserProfileCaching",
			setup: func() Cache {
				// Pre-populate with user profiles
				for i := 0; i < 100; i++ {
					profile := map[string]interface{}{
						"id":       uuid.New().String(),
						"username": fmt.Sprintf("user_%d", i),
						"email":    fmt.Sprintf("user_%d@example.com", i),
						"role":     "user",
						"active":   true,
					}
					key := fmt.Sprintf("user_profile:%d", i)
					cache.Set(ctx, key, profile, 30*time.Minute)
				}
				return cache
			},
			testFunc: func(b *testing.B, cache Cache) {
				for i := 0; i < b.N; i++ {
					userId := rand.Intn(100)
					key := fmt.Sprintf("user_profile:%d", userId)
					cache.Get(ctx, key)
				}
			},
		},
		{
			name: "SessionCaching",
			setup: func() Cache {
				// Pre-populate with session data
				for i := 0; i < 200; i++ {
					session := map[string]interface{}{
						"user_id":    i,
						"created_at": time.Now().Unix(),
						"last_seen":  time.Now().Unix(),
						"ip_address": fmt.Sprintf("192.168.1.%d", i%255),
					}
					key := fmt.Sprintf("session:%d", i)
					cache.Set(ctx, key, session, 15*time.Minute)
				}
				return cache
			},
			testFunc: func(b *testing.B, cache Cache) {
				for i := 0; i < b.N; i++ {
					sessionId := rand.Intn(200)
					key := fmt.Sprintf("session:%d", sessionId)
					cache.Get(ctx, key)
				}
			},
		},
		{
			name: "ConfigurationCaching",
			setup: func() Cache {
				// Pre-populate with configuration
				config := map[string]interface{}{
					"app_name":        "go-server",
					"version":         "1.0.0",
					"debug_mode":      false,
					"max_connections": 100,
					"timeout":         30,
					"features":        []string{"auth", "cache", "logging"},
				}
				cache.Set(ctx, "app:config", config, 24*time.Hour)
				return cache
			},
			testFunc: func(b *testing.B, cache Cache) {
				for i := 0; i < b.N; i++ {
					cache.Get(ctx, "app:config")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			scenario.setup()
			b.ResetTimer()
			b.ReportAllocs()
			scenario.testFunc(b, cache)
		})
	}
}

// BenchmarkCacheEviction benchmarks cache eviction scenarios
func BenchmarkCacheEviction(b *testing.B) {
	cache := setupBenchmarkCache(b)
	defer cleanupBenchmarkCache(b, cache)

	ctx := context.Background()

	// Test with short TTL to trigger evictions
	shortTTL := 100 * time.Millisecond

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("evict_key_%d", i)
		value := generateBenchmarkValue(1024)
		err := cache.Set(ctx, key, value, shortTTL)
		if err != nil {
			b.Fatalf("Failed to set value for eviction test: %v", err)
		}

		// Occasionally trigger expiration by waiting
		if i%100 == 0 {
			time.Sleep(shortTTL + 10*time.Millisecond)
		}
	}
}

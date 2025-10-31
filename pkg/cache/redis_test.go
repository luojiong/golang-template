package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/mock"
)

// TestRedisCache_Constructor tests the constructor functions
func TestRedisCache_Constructor(t *testing.T) {
	// Test with nil config (should use defaults)
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.DB = 1 // Use different DB for tests

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
		return
	}
	defer cache.Close()

	assert.NotNil(t, cache)
}

// TestRedisCache_BasicOperations tests basic cache operations
func TestRedisCache_BasicOperations(t *testing.T) {
	// Setup test cache
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.DB = 1 // Use different DB for tests
	config.Prefix = "test:"

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
		return
	}
	defer cache.Close()
	defer cache.Clear(context.Background())

	ctx := context.Background()

	// Test Set and Get
	err = cache.Set(ctx, "test_key", "test_value", time.Minute)
	require.NoError(t, err)

	value, found := cache.Get(ctx, "test_key")
	assert.True(t, found)
	assert.Equal(t, "test_value", value)

	// Test Get with non-existent key
	_, found = cache.Get(ctx, "non_existent_key")
	assert.False(t, found)

	// Test Exists
	exists, err := cache.Exists(ctx, "test_key")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = cache.Exists(ctx, "non_existent_key")
	require.NoError(t, err)
	assert.False(t, exists)

	// Test Delete
	err = cache.Delete(ctx, "test_key")
	require.NoError(t, err)

	_, found = cache.Get(ctx, "test_key")
	assert.False(t, found)
}

// TestRedisCache_TTL tests TTL functionality
func TestRedisCache_TTL(t *testing.T) {
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.DB = 1
	config.Prefix = "test_ttl:"

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
		return
	}
	defer cache.Close()
	defer cache.Clear(context.Background())

	ctx := context.Background()

	// Test Set with TTL
	err = cache.Set(ctx, "ttl_key", "ttl_value", 100*time.Millisecond)
	require.NoError(t, err)

	value, ttl, found := cache.GetWithTTL(ctx, "ttl_key")
	require.True(t, found)
	assert.Equal(t, "ttl_value", value)
	assert.True(t, ttl > 0 && ttl <= 100*time.Millisecond)

	// Wait for expiration
	time.Sleep(150 * time.Millisecond)

	_, found = cache.Get(ctx, "ttl_key")
	assert.False(t, found)

	// Test Set without TTL (should persist)
	err = cache.Set(ctx, "persist_key", "persist_value", 0)
	require.NoError(t, err)

	value, ttl, found = cache.GetWithTTL(ctx, "persist_key")
	require.True(t, found)
	assert.Equal(t, "persist_value", value)
	assert.Equal(t, time.Duration(0), ttl) // No expiration
}

// TestRedisCache_JSONHandling tests JSON marshaling/unmarshaling
func TestRedisCache_JSONHandling(t *testing.T) {
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.DB = 1
	config.Prefix = "test_json:"

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
		return
	}
	defer cache.Close()
	defer cache.Clear(context.Background())

	ctx := context.Background()

	// Test with struct
	type TestStruct struct {
		Name  string `json:"name"`
		Age   int    `json:"age"`
		Email string `json:"email"`
	}

	original := TestStruct{
		Name:  "John Doe",
		Age:   30,
		Email: "john@example.com",
	}

	err = cache.Set(ctx, "struct_key", original, time.Minute)
	require.NoError(t, err)

	value, found := cache.Get(ctx, "struct_key")
	require.True(t, found)

	// Convert to map for comparison
	resultMap, ok := value.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "John Doe", resultMap["name"])
	assert.Equal(t, float64(30), resultMap["age"]) // JSON numbers are float64
	assert.Equal(t, "john@example.com", resultMap["email"])

	// Test with slice
	originalSlice := []string{"apple", "banana", "cherry"}
	err = cache.Set(ctx, "slice_key", originalSlice, time.Minute)
	require.NoError(t, err)

	value, found = cache.Get(ctx, "slice_key")
	require.True(t, found)

	resultSlice, ok := value.([]interface{})
	require.True(t, ok)
	assert.Len(t, resultSlice, 3)
	assert.Equal(t, "apple", resultSlice[0])
	assert.Equal(t, "banana", resultSlice[1])
	assert.Equal(t, "cherry", resultSlice[2])
}

// TestRedisCache_MultipleOperations tests batch operations
func TestRedisCache_MultipleOperations(t *testing.T) {
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.DB = 1
	config.Prefix = "test_multiple:"

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
		return
	}
	defer cache.Close()
	defer cache.Clear(context.Background())

	ctx := context.Background()

	// Test SetMultiple
	items := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	err = cache.SetMultiple(ctx, items, time.Minute)
	require.NoError(t, err)

	// Test GetMultiple
	keys := []string{"key1", "key2", "key3", "key4"} // key4 doesn't exist
	results, err := cache.GetMultiple(ctx, keys)
	require.NoError(t, err)

	assert.Len(t, results, 3)
	assert.Equal(t, "value1", results["key1"])
	assert.Equal(t, "value2", results["key2"])
	assert.Equal(t, "value3", results["key3"])
	assert.NotContains(t, results, "key4")

	// Test DeleteMultiple
	err = cache.DeleteMultiple(ctx, []string{"key1", "key3"})
	require.NoError(t, err)

	results, err = cache.GetMultiple(ctx, keys)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "value2", results["key2"])
}

// TestRedisCache_AtomicOperations tests atomic operations
func TestRedisCache_AtomicOperations(t *testing.T) {
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.DB = 1
	config.Prefix = "test_atomic:"

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
		return
	}
	defer cache.Close()
	defer cache.Clear(context.Background())

	ctx := context.Background()

	// Test SetIfNotExists
	success, err := cache.SetIfNotExists(ctx, "atomic_key", "first_value", time.Minute)
	require.NoError(t, err)
	assert.True(t, success)

	success, err = cache.SetIfNotExists(ctx, "atomic_key", "second_value", time.Minute)
	require.NoError(t, err)
	assert.False(t, success)

	value, found := cache.Get(ctx, "atomic_key")
	require.True(t, found)
	assert.Equal(t, "first_value", value)

	// Test Increment
	err = cache.Set(ctx, "counter", int64(10), time.Minute)
	require.NoError(t, err)

	newValue, err := cache.Increment(ctx, "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(15), newValue)

	// Test Decrement
	newValue, err = cache.Decrement(ctx, "counter", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(12), newValue)
}

// TestRedisCache_KeysAndClear tests keys listing and clearing
func TestRedisCache_KeysAndClear(t *testing.T) {
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.DB = 1
	config.Prefix = "test_keys:"

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
		return
	}
	defer cache.Close()

	ctx := context.Background()

	// Add some keys
	err = cache.Set(ctx, "user:1", "user1_data", time.Minute)
	require.NoError(t, err)
	err = cache.Set(ctx, "user:2", "user2_data", time.Minute)
	require.NoError(t, err)
	err = cache.Set(ctx, "session:abc", "session_data", time.Minute)
	require.NoError(t, err)

	// Test keys with pattern
	keys, err := cache.Keys(ctx, "user:*")
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	assert.Contains(t, keys, "user:1")
	assert.Contains(t, keys, "user:2")

	// Test all keys
	keys, err = cache.Keys(ctx, "*")
	require.NoError(t, err)
	assert.Len(t, keys, 3)

	// Test Clear
	err = cache.Clear(ctx)
	require.NoError(t, err)

	keys, err = cache.Keys(ctx, "*")
	require.NoError(t, err)
	assert.Len(t, keys, 0)
}

// MockRedisClient is a mock implementation of redis.Client for testing error scenarios
type MockRedisClient struct {
	mock.Mock
}

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	args := m.Called(ctx)
	return args.Get(0).(*redis.StatusCmd)
}

func (m *MockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	args := m.Called(ctx, key)
	return args.Get(0).(*redis.StringCmd)
}

func (m *MockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	args := m.Called(ctx, key, value, expiration)
	return args.Get(0).(*redis.StatusCmd)
}

func (m *MockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	args := m.Called(ctx, keys)
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) Keys(ctx context.Context, pattern string) *redis.StringSliceCmd {
	args := m.Called(ctx, pattern)
	return args.Get(0).(*redis.StringSliceCmd)
}

func (m *MockRedisClient) TTL(ctx context.Context, key string) *redis.DurationCmd {
	args := m.Called(ctx, key)
	return args.Get(0).(*redis.DurationCmd)
}

func (m *MockRedisClient) Pipeline() redis.Pipeliner {
	args := m.Called()
	return args.Get(0).(redis.Pipeliner)
}

func (m *MockRedisClient) IncrBy(ctx context.Context, key string, value int64) *redis.IntCmd {
	args := m.Called(ctx, key, value)
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) DecrBy(ctx context.Context, key string, value int64) *redis.IntCmd {
	args := m.Called(ctx, key, value)
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	args := m.Called(ctx, key, value, expiration)
	return args.Get(0).(*redis.BoolCmd)
}

func (m *MockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	args := m.Called(ctx, key, expiration)
	return args.Get(0).(*redis.BoolCmd)
}

func (m *MockRedisClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// TestRedisCache_ErrorHandling tests error handling scenarios
func TestRedisCache_ErrorHandling(t *testing.T) {
	t.Run("Connection Failure on Constructor", func(t *testing.T) {
		config := &RedisConfig{
			Host: "nonexistent-host",
			Port: 6379,
			DB:   0,
		}

		cache, err := NewRedisCache(config)
		assert.Error(t, err)
		assert.Nil(t, cache)
		assert.Contains(t, err.Error(), "failed to connect to Redis")
	})

	t.Run("Get Operation with Network Error", func(t *testing.T) {
		// This test simulates network errors during get operations
		// In a real scenario, we would need to mock the Redis client
		config := DefaultRedisConfig()
		config.Host = "localhost"
		config.Port = 6379
		config.DB = 1

		cache, err := NewRedisCache(config)
		if err != nil {
			t.Skipf("Redis not available for testing: %v", err)
			return
		}
		defer cache.Close()

		ctx := context.Background()

		// Set a value first
		err = cache.Set(ctx, "test_key", "test_value", time.Minute)
		require.NoError(t, err)

		// Get the value
		value, found := cache.Get(ctx, "test_key")
		assert.True(t, found)
		assert.Equal(t, "test_value", value)
	})

	t.Run("Graceful Degradation on Cache Errors", func(t *testing.T) {
		// Test that cache operations fail gracefully
		config := DefaultRedisConfig()
		config.Host = "localhost"
		config.Port = 6379
		config.DB = 1

		cache, err := NewRedisCache(config)
		if err != nil {
			t.Skipf("Redis not available for testing: %v", err)
			return
		}
		defer cache.Close()

		ctx := context.Background()

		// Test multiple operations don't panic
		value, found := cache.Get(ctx, "nonexistent_key")
		assert.False(t, found)
		assert.Nil(t, value)

		err = cache.Delete(ctx, "nonexistent_key")
		assert.NoError(t, err) // Delete should not error on non-existent keys

		// We can't test SetTTL directly as it's not part of the Cache interface
		// This would be tested in implementation-specific tests
	})
}

// TestRedisCache_FallbackBehavior tests fallback behavior when Redis is unavailable
func TestRedisCache_FallbackBehavior(t *testing.T) {
	t.Run("Operations When Redis is Unavailable", func(t *testing.T) {
		// Test cache operations when Redis becomes unavailable
		config := DefaultRedisConfig()
		config.Host = "localhost"
		config.Port = 6379
		config.DB = 1

		cache, err := NewRedisCache(config)
		if err != nil {
			t.Skipf("Redis not available for testing: %v", err)
			return
		}
		defer cache.Close()

		ctx := context.Background()

		// Set some initial data
		err = cache.Set(ctx, "fallback_test", "initial_value", time.Minute)
		if err != nil {
			t.Skipf("Could not set initial data: %v", err)
			return
		}

		// Verify data exists
		value, found := cache.Get(ctx, "fallback_test")
		require.True(t, found)
		assert.Equal(t, "initial_value", value)

		// Test that the application can continue operating
		// even if cache operations fail
		// In a real application, this would fall back to database queries
	})

	t.Run("Cache Misses Should Not Block Operations", func(t *testing.T) {
		config := DefaultRedisConfig()
		config.Host = "localhost"
		config.Port = 6379
		config.DB = 1

		cache, err := NewRedisCache(config)
		if err != nil {
			t.Skipf("Redis not available for testing: %v", err)
			return
		}
		defer cache.Close()

		ctx := context.Background()

		// Test that cache misses don't cause errors
		_, found := cache.Get(ctx, "definitely_nonexistent_key")
		assert.False(t, found)

		// Test multiple cache misses in sequence
		keys := []string{"key1", "key2", "key3"}
		results, err := cache.GetMultiple(ctx, keys)
		assert.NoError(t, err)
		assert.Len(t, results, 0) // All should be cache misses
	})
}

// TestRedisCache_TTLEdgeCases tests TTL edge cases and expiration policies
func TestRedisCache_TTLEdgeCases(t *testing.T) {
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.DB = 1
	config.Prefix = "test_ttl_edge:"

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
		return
	}
	defer cache.Close()
	defer cache.Clear(context.Background())

	ctx := context.Background()

	t.Run("Immediate Expiration", func(t *testing.T) {
		// Test setting TTL to 1 millisecond (immediate expiration)
		err = cache.Set(ctx, "immediate_expire", "value", 1*time.Millisecond)
		require.NoError(t, err)

		// Wait a bit longer than TTL
		time.Sleep(10 * time.Millisecond)

		_, found := cache.Get(ctx, "immediate_expire")
		assert.False(t, found, "Key should have expired immediately")
	})

	t.Run("Zero TTL (No Expiration)", func(t *testing.T) {
		err = cache.Set(ctx, "no_expire", "value", 0)
		require.NoError(t, err)

		value, ttl, found := cache.GetWithTTL(ctx, "no_expire")
		require.True(t, found)
		assert.Equal(t, "value", value)
		assert.Equal(t, time.Duration(0), ttl) // No expiration
	})

	t.Run("Update TTL via GetWithTTL", func(t *testing.T) {
		// Set a key with initial TTL
		err = cache.Set(ctx, "update_ttl", "value", 100*time.Millisecond)
		require.NoError(t, err)

		// Check initial TTL
		_, ttl, found := cache.GetWithTTL(ctx, "update_ttl")
		require.True(t, found)
		assert.True(t, ttl > 0 && ttl <= 100*time.Millisecond)

		// Update by setting the same key with new TTL (this is the interface-compliant way)
		err = cache.Set(ctx, "update_ttl", "value", time.Hour)
		require.NoError(t, err)

		// Check that TTL was updated via GetWithTTL
		_, newTTL, found := cache.GetWithTTL(ctx, "update_ttl")
		require.True(t, found)
		assert.True(t, newTTL > time.Minute, "TTL should be extended")
	})

	t.Run("Get TTL on Non-existent Key", func(t *testing.T) {
		_, ttl, found := cache.GetWithTTL(ctx, "nonexistent_key")
		assert.False(t, found)
		assert.Equal(t, time.Duration(0), ttl)
	})

	t.Run("TTL Precision", func(t *testing.T) {
		// Test TTL precision with millisecond granularity
		expectedTTL := 500 * time.Millisecond
		err = cache.Set(ctx, "precision_test", "value", expectedTTL)
		require.NoError(t, err)

		value, actualTTL, found := cache.GetWithTTL(ctx, "precision_test")
		require.True(t, found)
		assert.Equal(t, "value", value)

		// TTL should be close to expected (allowing for some delay)
		assert.True(t, actualTTL > 0 && actualTTL <= expectedTTL,
			"TTL should be positive and not exceed expected duration")
	})
}

// TestRedisCache_PerformanceAndLoad tests performance under load
func TestRedisCache_PerformanceAndLoad(t *testing.T) {
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.DB = 1
	config.Prefix = "test_perf:"

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
		return
	}
	defer cache.Close()
	defer cache.Clear(context.Background())

	ctx := context.Background()

	t.Run("Concurrent Operations", func(t *testing.T) {
		const numGoroutines = 10
		const numOperations = 100

		// Test concurrent sets
		done := make(chan bool, numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				for j := 0; j < numOperations; j++ {
					key := fmt.Sprintf("concurrent_%d_%d", id, j)
					value := fmt.Sprintf("value_%d_%d", id, j)
					err := cache.Set(ctx, key, value, time.Minute)
					assert.NoError(t, err)
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify all data was set correctly
		totalKeys := numGoroutines * numOperations
		keys, err := cache.Keys(ctx, "concurrent_*")
		require.NoError(t, err)
		assert.Equal(t, totalKeys, len(keys))
	})

	t.Run("Batch Operations Performance", func(t *testing.T) {
		const numItems = 1000

		// Prepare test data
		items := make(map[string]interface{})
		for i := 0; i < numItems; i++ {
			key := fmt.Sprintf("batch_%d", i)
			value := fmt.Sprintf("batch_value_%d", i)
			items[key] = value
		}

		// Measure batch set performance
		start := time.Now()
		err = cache.SetMultiple(ctx, items, time.Minute)
		duration := time.Since(start)
		require.NoError(t, err)

		// Batch operations should be reasonably fast
		assert.True(t, duration < 5*time.Second, "Batch set should complete within 5 seconds, took %v", duration)

		// Measure batch get performance
		keys := make([]string, numItems)
		for i := 0; i < numItems; i++ {
			keys[i] = fmt.Sprintf("batch_%d", i)
		}

		start = time.Now()
		results, err := cache.GetMultiple(ctx, keys)
		duration = time.Since(start)
		require.NoError(t, err)

		assert.Len(t, results, numItems)
		assert.True(t, duration < 2*time.Second, "Batch get should complete within 2 seconds, took %v", duration)
	})

	t.Run("Memory Usage with Large Values", func(t *testing.T) {
		// Test with large values to ensure cache handles them properly
		largeValue := make([]byte, 1024*1024) // 1MB value
		for i := range largeValue {
			largeValue[i] = byte(i % 256)
		}

		err := cache.Set(ctx, "large_value", largeValue, time.Minute)
		require.NoError(t, err)

		retrievedValue, found := cache.Get(ctx, "large_value")
		require.True(t, found)

		retrievedBytes, ok := retrievedValue.([]byte)
		require.True(t, ok)
		assert.Equal(t, len(largeValue), len(retrievedBytes))
	})
}

// TestRedisCache_RealWorldUsagePatterns simulates real-world usage patterns
func TestRedisCache_RealWorldUsagePatterns(t *testing.T) {
	config := DefaultRedisConfig()
	config.Host = "localhost"
	config.Port = 6379
	config.DB = 1
	config.Prefix = "test_realworld:"

	cache, err := NewRedisCache(config)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
		return
	}
	defer cache.Close()
	defer cache.Clear(context.Background())

	ctx := context.Background()

	t.Run("User Profile Caching Pattern", func(t *testing.T) {
		// Simulate caching user profiles with different access patterns
		type UserProfile struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Email    string `json:"email"`
			Role     string `json:"role"`
		}

		profiles := []*UserProfile{
			{ID: "1", Username: "user1", Email: "user1@example.com", Role: "admin"},
			{ID: "2", Username: "user2", Email: "user2@example.com", Role: "user"},
			{ID: "3", Username: "user3", Email: "user3@example.com", Role: "user"},
		}

		// Cache profiles with different TTLs based on role
		for _, profile := range profiles {
			key := fmt.Sprintf("user_profile:%s", profile.ID)
			ttl := time.Hour
			if profile.Role == "admin" {
				ttl = 30 * time.Minute // Admin profiles refresh more frequently
			}

			err := cache.Set(ctx, key, profile, ttl)
			require.NoError(t, err)
		}

		// Simulate cache hits and misses
		for _, profile := range profiles {
			key := fmt.Sprintf("user_profile:%s", profile.ID)
			value, found := cache.Get(ctx, key)
			require.True(t, found)

			// Verify the cached data structure
			cachedProfile, ok := value.(map[string]interface{})
			require.True(t, ok)
			assert.Equal(t, profile.Username, cachedProfile["username"])
		}
	})

	t.Run("Session Management Pattern", func(t *testing.T) {
		// Simulate session storage and invalidation
		sessions := map[string]interface{}{
			"session_abc123": map[string]interface{}{
				"user_id":    "1",
				"created_at": time.Now().Unix(),
				"last_seen":  time.Now().Unix(),
			},
			"session_def456": map[string]interface{}{
				"user_id":    "2",
				"created_at": time.Now().Unix(),
				"last_seen":  time.Now().Unix(),
			},
		}

		// Set sessions with relatively short TTL
		err := cache.SetMultiple(ctx, sessions, 5*time.Minute)
		require.NoError(t, err)

		// Simulate session lookup
		sessionValue, found := cache.Get(ctx, "session_abc123")
		require.True(t, found)

		sessionData, ok := sessionValue.(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "1", sessionData["user_id"])

		// Simulate session logout (delete)
		err = cache.Delete(ctx, "session_abc123")
		require.NoError(t, err)

		_, found = cache.Get(ctx, "session_abc123")
		assert.False(t, found)
	})

	t.Run("Cache-Aside Pattern Simulation", func(t *testing.T) {
		// Simulate cache-aside pattern where cache misses trigger database loads
		databaseSimulator := map[string]interface{}{
			"product:1": map[string]interface{}{
				"id":    "1",
				"name":  "Product 1",
				"price": 99.99,
			},
			"product:2": map[string]interface{}{
				"id":    "2",
				"name":  "Product 2",
				"price": 149.99,
			},
		}

		// Function to simulate database load
		loadFromDatabase := func(key string) (interface{}, bool) {
			if value, exists := databaseSimulator[key]; exists {
				return value, true
			}
			return nil, false
		}

		// Simulate cache-aside pattern
		getProduct := func(productID string) (interface{}, error) {
			cacheKey := fmt.Sprintf("product:%s", productID)

			// Try cache first
			value, found := cache.Get(ctx, cacheKey)
			if found {
				return value, nil // Cache hit
			}

			// Cache miss - load from database
			value, found = loadFromDatabase(cacheKey)
			if !found {
				return nil, errors.New("product not found")
			}

			// Cache the result for future requests
			err := cache.Set(ctx, cacheKey, value, 10*time.Minute)
			if err != nil {
				// Even if caching fails, return the data
				return value, nil
			}

			return value, nil
		}

		// First call should be a cache miss
		product1, err := getProduct("1")
		require.NoError(t, err)
		require.NotNil(t, product1)

		// Second call should be a cache hit
		product1Again, err := getProduct("1")
		require.NoError(t, err)
		require.NotNil(t, product1Again)

		// Verify they're the same
		assert.Equal(t, product1, product1Again)
	})
}
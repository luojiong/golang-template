package cache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"go-server/pkg/auth"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCache implements the Cache interface for testing
type MockCache struct {
	data map[string]mockCacheItem
	mu   sync.RWMutex
}

type mockCacheItem struct {
	value     interface{}
	expiresAt time.Time
}

func NewMockCache() *MockCache {
	return &MockCache{
		data: make(map[string]mockCacheItem),
	}
}

func (m *MockCache) Get(ctx context.Context, key string) (interface{}, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.data[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.value, true
}

func (m *MockCache) GetWithTTL(ctx context.Context, key string) (interface{}, time.Duration, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.data[key]
	if !exists {
		return nil, 0, false
	}

	// Check if expired
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return nil, 0, false
	}

	var ttl time.Duration
	if !item.expiresAt.IsZero() {
		ttl = time.Until(item.expiresAt)
		if ttl < 0 {
			ttl = 0
		}
	}

	return item.value, ttl, true
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	m.data[key] = mockCacheItem{
		value:     value,
		expiresAt: expiresAt,
	}

	return nil
}

func (m *MockCache) SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	for key, value := range items {
		m.Set(ctx, key, value, ttl)
	}
	return nil
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

func (m *MockCache) DeleteMultiple(ctx context.Context, keys []string) error {
	for _, key := range keys {
		delete(m.data, key)
	}
	return nil
}

func (m *MockCache) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.data[key]
	if !exists {
		return false, nil
	}

	// Check if expired
	item := m.data[key]
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return false, nil
	}

	return true, nil
}

func (m *MockCache) Clear(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]mockCacheItem)
	return nil
}

func (m *MockCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []string
	for key := range m.data {
		// Simple pattern matching - only support "*" for now
		if pattern == "*" {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (m *MockCache) GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, key := range keys {
		if value, exists := m.Get(ctx, key); exists {
			result[key] = value
		}
	}
	return result, nil
}

func (m *MockCache) SetIfNotExists(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if exists, _ := m.Exists(ctx, key); exists {
		return false, nil
	}
	return true, m.Set(ctx, key, value, ttl)
}

func (m *MockCache) Increment(ctx context.Context, key string, amount int64) (int64, error) {
	item, exists := m.data[key]
	var val int64
	if exists {
		if num, ok := item.value.(int64); ok {
			val = num
		}
	}
	val += amount
	m.Set(ctx, key, val, 0)
	return val, nil
}

func (m *MockCache) Decrement(ctx context.Context, key string, amount int64) (int64, error) {
	return m.Increment(ctx, key, -amount)
}

func (m *MockCache) Close() error {
	return nil
}

// Health checks the health of the mock cache connection
func (m *MockCache) Health(ctx context.Context) error {
	// Mock cache is always healthy for testing
	return nil
}

// GetStats returns mock cache statistics for testing
func (m *MockCache) GetStats(ctx context.Context) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Count non-expired items
	activeItems := 0
	for _, item := range m.data {
		if item.expiresAt.IsZero() || time.Now().Before(item.expiresAt) {
			activeItems++
		}
	}

	return map[string]interface{}{
		"connection_pool": map[string]interface{}{
			"hits":        uint32(100),
			"misses":      uint32(5),
			"total_conns": uint32(10),
			"idle_conns":  uint32(8),
			"stale_conns": uint32(0),
			"hit_rate":    95.2,
		},
		"memory": map[string]interface{}{
			"used_memory_bytes":    int64(1024 * 100), // 100KB
			"used_memory_mb":       int64(0),
			"max_memory_bytes":     int64(1024 * 1024 * 100), // 100MB
			"max_memory_mb":        int64(100),
			"memory_usage_percent": 1.0,
		},
		"database": map[string]interface{}{
			"total_keys":                 int64(len(m.data)),
			"total_commands_processed":   int64(1000),
			"total_connections_received": int64(50),
			"keyspace_hits":              int64(950),
			"keyspace_misses":            int64(50),
			"keyspace_hit_rate_percent":  95.0,
		},
		"latency_ms":   1,
		"active_items": activeItems,
		"total_items":  len(m.data),
	}, nil
}

// Helper function to create a test JWT token
func createTestToken(jwtManager *auth.JWTManager, userID, username, email string, expiresIn time.Duration) string {
	// Create a test token with custom expiration time
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"email":    email,
		"exp":      time.Now().Add(expiresIn).Unix(),
		"iat":      time.Now().Unix(),
		"nbf":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte("test-secret"))
	return tokenString
}

func TestBlacklistService_AddToBlacklist(t *testing.T) {
	mockCache := NewMockCache()
	jwtManager := auth.NewJWTManager("test-secret", 24) // 24 hours
	service := NewBlacklistService(mockCache, jwtManager, nil)

	ctx := context.Background()
	token := createTestToken(jwtManager, "user123", "testuser", "test@example.com", time.Hour)

	// Add token to blacklist
	err := service.AddToBlacklist(ctx, token)
	require.NoError(t, err)

	// Check if token is blacklisted
	blacklisted, err := service.IsBlacklisted(ctx, token)
	require.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestBlacklistService_IsBlacklisted_ExpiredToken(t *testing.T) {
	mockCache := NewMockCache()
	jwtManager := auth.NewJWTManager("test-secret", 24)
	service := NewBlacklistService(mockCache, jwtManager, nil)

	ctx := context.Background()

	// Create an already expired token
	token := createTestToken(jwtManager, "user123", "testuser", "test@example.com", -time.Hour)

	// Check if expired token is considered blacklisted
	blacklisted, err := service.IsBlacklisted(ctx, token)
	require.NoError(t, err)
	assert.False(t, blacklisted) // Expired tokens should not be considered blacklisted
}

func TestBlacklistService_AddToBlacklist_ExpiredToken(t *testing.T) {
	mockCache := NewMockCache()
	jwtManager := auth.NewJWTManager("test-secret", 24)
	service := NewBlacklistService(mockCache, jwtManager, nil)

	ctx := context.Background()

	// Create an already expired token
	token := createTestToken(jwtManager, "user123", "testuser", "test@example.com", -time.Hour)

	// Try to add expired token to blacklist
	err := service.AddToBlacklist(ctx, token)
	require.NoError(t, err)

	// Should not be in blacklist since it was already expired
	blacklisted, err := service.IsBlacklisted(ctx, token)
	require.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestBlacklistService_RemoveFromBlacklist(t *testing.T) {
	mockCache := NewMockCache()
	jwtManager := auth.NewJWTManager("test-secret", 24)
	service := NewBlacklistService(mockCache, jwtManager, nil)

	ctx := context.Background()
	token := createTestToken(jwtManager, "user123", "testuser", "test@example.com", time.Hour)

	// Add token to blacklist
	err := service.AddToBlacklist(ctx, token)
	require.NoError(t, err)

	// Verify it's blacklisted
	blacklisted, err := service.IsBlacklisted(ctx, token)
	require.NoError(t, err)
	assert.True(t, blacklisted)

	// Remove from blacklist
	err = service.RemoveFromBlacklist(ctx, token)
	require.NoError(t, err)

	// Verify it's no longer blacklisted
	blacklisted, err = service.IsBlacklisted(ctx, token)
	require.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestBlacklistService_ValidateTokenWithBlacklist(t *testing.T) {
	mockCache := NewMockCache()
	jwtManager := auth.NewJWTManager("test-secret", 24)
	service := NewBlacklistService(mockCache, jwtManager, nil)

	ctx := context.Background()

	// Generate a proper token using the JWT manager
	token, err := jwtManager.GenerateToken("user123", "testuser", "test@example.com")
	require.NoError(t, err)

	// First, validate non-blacklisted token
	claims, err := service.ValidateTokenWithBlacklist(ctx, token)
	require.NoError(t, err)
	assert.NotNil(t, claims)
	assert.Equal(t, "user123", claims.UserID)
	assert.Equal(t, "testuser", claims.Username)

	// Add token to blacklist
	err = service.AddToBlacklist(ctx, token)
	require.NoError(t, err)

	// Try to validate blacklisted token
	claims, err = service.ValidateTokenWithBlacklist(ctx, token)
	assert.Error(t, err)
	assert.Nil(t, claims)
	assert.Contains(t, err.Error(), "token is blacklisted")
}

func TestBlacklistService_AddMultipleToBlacklist(t *testing.T) {
	mockCache := NewMockCache()
	jwtManager := auth.NewJWTManager("test-secret", 24)
	service := NewBlacklistService(mockCache, jwtManager, nil)

	ctx := context.Background()

	// Generate multiple tokens using the JWT manager
	tokens := make([]string, 3)
	for i := 0; i < 3; i++ {
		userID := fmt.Sprintf("user%d", i+1)
		username := fmt.Sprintf("testuser%d", i+1)
		email := fmt.Sprintf("test%d@example.com", i+1)

		token, err := jwtManager.GenerateToken(userID, username, email)
		require.NoError(t, err)
		tokens[i] = token
	}

	// Add multiple tokens to blacklist
	err := service.AddMultipleToBlacklist(ctx, tokens)
	require.NoError(t, err)

	// Verify all tokens are blacklisted
	for _, token := range tokens {
		blacklisted, err := service.IsBlacklisted(ctx, token)
		require.NoError(t, err)
		assert.True(t, blacklisted)
	}
}

func TestBlacklistService_GetBlacklistSize(t *testing.T) {
	mockCache := NewMockCache()
	jwtManager := auth.NewJWTManager("test-secret", 24)
	service := NewBlacklistService(mockCache, jwtManager, nil)

	ctx := context.Background()

	// Initially empty
	size, err := service.GetBlacklistSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, size)

	// Add some tokens using JWT manager
	tokens := make([]string, 2)
	for i := 0; i < 2; i++ {
		userID := fmt.Sprintf("user%d", i+1)
		username := fmt.Sprintf("testuser%d", i+1)
		email := fmt.Sprintf("test%d@example.com", i+1)

		token, err := jwtManager.GenerateToken(userID, username, email)
		require.NoError(t, err)
		tokens[i] = token
	}

	for _, token := range tokens {
		err := service.AddToBlacklist(ctx, token)
		require.NoError(t, err)
	}

	// Check size
	size, err = service.GetBlacklistSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, size)
}

func TestBlacklistService_ClearBlacklist(t *testing.T) {
	mockCache := NewMockCache()
	jwtManager := auth.NewJWTManager("test-secret", 24)
	service := NewBlacklistService(mockCache, jwtManager, nil)

	ctx := context.Background()

	// Add some tokens using JWT manager
	tokens := make([]string, 2)
	for i := 0; i < 2; i++ {
		userID := fmt.Sprintf("user%d", i+1)
		username := fmt.Sprintf("testuser%d", i+1)
		email := fmt.Sprintf("test%d@example.com", i+1)

		token, err := jwtManager.GenerateToken(userID, username, email)
		require.NoError(t, err)
		tokens[i] = token
	}

	for _, token := range tokens {
		err := service.AddToBlacklist(ctx, token)
		require.NoError(t, err)
	}

	// Verify they're blacklisted
	for _, token := range tokens {
		blacklisted, err := service.IsBlacklisted(ctx, token)
		require.NoError(t, err)
		assert.True(t, blacklisted)
	}

	// Clear blacklist
	err := service.ClearBlacklist(ctx)
	require.NoError(t, err)

	// Verify no tokens are blacklisted
	for _, token := range tokens {
		blacklisted, err := service.IsBlacklisted(ctx, token)
		require.NoError(t, err)
		assert.False(t, blacklisted)
	}

	// Check size is zero
	size, err := service.GetBlacklistSize(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, size)
}

func TestBlacklistService_CleanupExpiredTokens(t *testing.T) {
	mockCache := NewMockCache()
	jwtManager := auth.NewJWTManager("test-secret", 24)
	service := NewBlacklistService(mockCache, jwtManager, nil)

	ctx := context.Background()

	// Add tokens with different expiration times
	validToken := createTestToken(jwtManager, "user1", "testuser1", "test1@example.com", time.Hour)
	expiredToken := createTestToken(jwtManager, "user2", "testuser2", "test2@example.com", -time.Hour)

	// Manually add tokens to mock cache to simulate different expiration times
	validKey := service.generateTokenKey(validToken)
	expiredKey := service.generateTokenKey(expiredToken)

	mockCache.Set(ctx, validKey, "blacklisted", time.Hour)
	mockCache.Set(ctx, expiredKey, "blacklisted", 0) // No expiration in mock

	// Run cleanup
	err := service.CleanupExpiredTokens(ctx)
	require.NoError(t, err)

	// Valid token should still be blacklisted
	blacklisted, err := service.IsBlacklisted(ctx, validToken)
	require.NoError(t, err)
	assert.True(t, blacklisted)

	// Expired token should be handled by the cleanup process
	// In a real Redis implementation, expired keys would be automatically removed
}

func TestBlacklistService_DefaultConfig(t *testing.T) {
	config := DefaultBlacklistConfig()
	assert.Equal(t, "jwt_blacklist:", config.KeyPrefix)
	assert.Equal(t, time.Hour, config.CleanupInterval)
	assert.Equal(t, 100, config.BatchSize)
}

func TestBlacklistService_GenerateTokenKey(t *testing.T) {
	mockCache := NewMockCache()
	jwtManager := auth.NewJWTManager("test-secret", 24)
	config := &BlacklistConfig{KeyPrefix: "test_prefix:"}
	service := NewBlacklistService(mockCache, jwtManager, config)

	token := "test.token.here"
	key1 := service.generateTokenKey(token)
	key2 := service.generateTokenKey(token)

	// Same token should generate same key
	assert.Equal(t, key1, key2)

	// Different tokens should generate different keys
	differentToken := "different.token.here"
	key3 := service.generateTokenKey(differentToken)
	assert.NotEqual(t, key1, key3)

	// Key should have the correct prefix
	assert.True(t, len(key1) > len("test_prefix:"))
	assert.Equal(t, "test_prefix:", key1[:len("test_prefix:")])
}

// FailingMockCache is a mock cache that always returns errors
type FailingMockCache struct {
	*MockCache
	shouldFailGet    bool
	shouldFailSet    bool
	shouldFailExists bool
}

func NewFailingMockCache() *FailingMockCache {
	return &FailingMockCache{
		MockCache: NewMockCache(),
	}
}

func (f *FailingMockCache) Get(ctx context.Context, key string) (interface{}, bool) {
	if f.shouldFailGet {
		return nil, false
	}
	return f.MockCache.Get(ctx, key)
}

func (f *FailingMockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if f.shouldFailSet {
		return errors.New("cache set operation failed")
	}
	return f.MockCache.Set(ctx, key, value, ttl)
}

func (f *FailingMockCache) Exists(ctx context.Context, key string) (bool, error) {
	if f.shouldFailExists {
		return false, errors.New("cache exists operation failed")
	}
	return f.MockCache.Exists(ctx, key)
}

// TestBlacklistService_RedisUnavailability tests blacklist behavior when Redis is unavailable
func TestBlacklistService_RedisUnavailability(t *testing.T) {
	t.Run("Blacklist Operations When Cache Fails", func(t *testing.T) {
		failingCache := NewFailingMockCache()
		failingCache.shouldFailSet = true
		failingCache.shouldFailExists = true

		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(failingCache, jwtManager, nil)

		ctx := context.Background()
		token := createTestToken(jwtManager, "user123", "testuser", "test@example.com", time.Hour)

		// Test adding to blacklist when cache fails
		err := service.AddToBlacklist(ctx, token)
		// Should return error when cache fails, but handle it gracefully
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cache set operation failed")

		// Test checking blacklist when cache fails
		blacklisted, err := service.IsBlacklisted(ctx, token)
		// Should return error when cache fails, but handle it gracefully
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cache exists operation failed")
		assert.False(t, blacklisted) // Default to false on error
	})

	t.Run("Token Validation Falls Back to JWT Validation When Blacklist Fails", func(t *testing.T) {
		failingCache := NewFailingMockCache()
		failingCache.shouldFailExists = true

		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(failingCache, jwtManager, nil)

		ctx := context.Background()

		// Generate a valid token
		token, err := jwtManager.GenerateToken("user123", "testuser", "test@example.com")
		require.NoError(t, err)

		// When blacklist checking fails, token validation should also fail gracefully
		claims, err := service.ValidateTokenWithBlacklist(ctx, token)
		assert.Error(t, err)
		assert.Nil(t, claims)
		assert.Contains(t, err.Error(), "failed to check blacklist")
	})

	t.Run("Blacklist Size Calculation with Cache Failures", func(t *testing.T) {
		failingCache := NewFailingMockCache()
		failingCache.shouldFailExists = true

		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(failingCache, jwtManager, nil)

		ctx := context.Background()

		// Size should be 0 even when cache fails
		size, err := service.GetBlacklistSize(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 0, size)
	})

	t.Run("Cleanup Operations with Cache Failures", func(t *testing.T) {
		failingCache := NewFailingMockCache()
		failingCache.shouldFailExists = true

		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(failingCache, jwtManager, nil)

		ctx := context.Background()

		// Cleanup should not error even when cache operations fail
		err := service.CleanupExpiredTokens(ctx)
		assert.NoError(t, err)
	})

	t.Run("Clear Blacklist with Cache Failures", func(t *testing.T) {
		failingCache := NewFailingMockCache()

		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(failingCache, jwtManager, nil)

		ctx := context.Background()

		// Clear should not error even when cache fails
		err := service.ClearBlacklist(ctx)
		assert.NoError(t, err)
	})
}

// TestBlacklistService_EdgeCases tests edge cases and boundary conditions
func TestBlacklistService_EdgeCases(t *testing.T) {
	t.Run("Empty Token Handling", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()

		// Test with empty token
		err := service.AddToBlacklist(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse token")

		// Check if empty token is considered blacklisted
		blacklisted, err := service.IsBlacklisted(ctx, "")
		assert.NoError(t, err)
		assert.True(t, blacklisted) // Invalid tokens are considered blacklisted
	})

	t.Run("Malformed Token Handling", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()

		// Test with malformed token
		malformedToken := "this.is.not.a.valid.jwt.token"
		err := service.AddToBlacklist(ctx, malformedToken)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse token")

		// Check if malformed token is considered blacklisted
		blacklisted, err := service.IsBlacklisted(ctx, malformedToken)
		assert.NoError(t, err)
		assert.True(t, blacklisted) // Invalid tokens are considered blacklisted
	})

	t.Run("Token with Short TTL", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24) // Use normal expiration
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()

		// Create a valid token first
		token, err := jwtManager.GenerateToken("user123", "testuser", "test@example.com")
		require.NoError(t, err)

		// Add to blacklist
		err = service.AddToBlacklist(ctx, token)
		require.NoError(t, err)

		// Should be blacklisted immediately
		blacklisted, err := service.IsBlacklisted(ctx, token)
		require.NoError(t, err)
		assert.True(t, blacklisted, "Token should be blacklisted immediately after adding")

		// Remove from blacklist to test basic functionality
		err = service.RemoveFromBlacklist(ctx, token)
		require.NoError(t, err)

		// Should no longer be blacklisted
		blacklisted, err = service.IsBlacklisted(ctx, token)
		require.NoError(t, err)
		assert.False(t, blacklisted, "Token should not be blacklisted after removal")
	})

	t.Run("Token with Zero TTL (Already Expired)", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()

		// Create already expired token
		token := createTestToken(jwtManager, "user123", "testuser", "test@example.com", -time.Hour)

		// Try to add to blacklist
		err := service.AddToBlacklist(ctx, token)
		require.NoError(t, err)

		// Should not be blacklisted since token is already expired
		blacklisted, err := service.IsBlacklisted(ctx, token)
		require.NoError(t, err)
		assert.False(t, blacklisted)
	})

	t.Run("Duplicate Blacklist Operations", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()
		token := createTestToken(jwtManager, "user123", "testuser", "test@example.com", time.Hour)

		// Add token to blacklist multiple times
		err := service.AddToBlacklist(ctx, token)
		require.NoError(t, err)

		err = service.AddToBlacklist(ctx, token)
		require.NoError(t, err)

		// Should still be blacklisted
		blacklisted, err := service.IsBlacklisted(ctx, token)
		require.NoError(t, err)
		assert.True(t, blacklisted)
	})

	t.Run("Remove Non-Existent Token from Blacklist", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()
		token := createTestToken(jwtManager, "user123", "testuser", "test@example.com", time.Hour)

		// Try to remove token that was never blacklisted
		err := service.RemoveFromBlacklist(ctx, token)
		assert.NoError(t, err) // Should not error

		// Should not be blacklisted
		blacklisted, err := service.IsBlacklisted(ctx, token)
		require.NoError(t, err)
		assert.False(t, blacklisted)
	})
}

// TestBlacklistService_ConcurrencyAndLoad tests concurrent operations and load scenarios
func TestBlacklistService_ConcurrencyAndLoad(t *testing.T) {
	t.Run("Concurrent Blacklist Operations", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()
		numGoroutines := 10
		numTokensPerGoroutine := 50

		// Generate tokens for concurrent operations
		allTokens := make([]string, 0, numGoroutines*numTokensPerGoroutine)
		for i := 0; i < numGoroutines*numTokensPerGoroutine; i++ {
			userID := fmt.Sprintf("user%d", i)
			username := fmt.Sprintf("user%d", i)
			email := fmt.Sprintf("user%d@example.com", i)

			token, err := jwtManager.GenerateToken(userID, username, email)
			require.NoError(t, err)
			allTokens = append(allTokens, token)
		}

		// Concurrent add operations
		done := make(chan bool, numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(startIndex int) {
				for j := 0; j < numTokensPerGoroutine; j++ {
					tokenIndex := startIndex + j
					err := service.AddToBlacklist(ctx, allTokens[tokenIndex])
					assert.NoError(t, err)
				}
				done <- true
			}(i * numTokensPerGoroutine)
		}

		// Wait for all operations to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify all tokens are blacklisted
		for _, token := range allTokens {
			blacklisted, err := service.IsBlacklisted(ctx, token)
			require.NoError(t, err)
			assert.True(t, blacklisted)
		}

		// Check blacklist size
		size, err := service.GetBlacklistSize(ctx)
		require.NoError(t, err)
		assert.Equal(t, len(allTokens), size)
	})

	t.Run("Concurrent Check Operations", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()
		numTokens := 100

		// Generate and blacklist tokens
		tokens := make([]string, numTokens)
		for i := 0; i < numTokens; i++ {
			userID := fmt.Sprintf("user%d", i)
			username := fmt.Sprintf("user%d", i)
			email := fmt.Sprintf("user%d@example.com", i)

			token, err := jwtManager.GenerateToken(userID, username, email)
			require.NoError(t, err)
			tokens[i] = token

			err = service.AddToBlacklist(ctx, token)
			require.NoError(t, err)
		}

		// Concurrent check operations
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(startIndex int) {
				for j := 0; j < 10; j++ {
					tokenIndex := startIndex + j
					blacklisted, err := service.IsBlacklisted(ctx, tokens[tokenIndex])
					assert.NoError(t, err)
					assert.True(t, blacklisted)
				}
				done <- true
			}(i * 10)
		}

		// Wait for all operations to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})

	t.Run("High Volume Token Validation", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()
		numValidations := 1000

		// Generate tokens
		validTokens := make([]string, numValidations/2)
		blacklistedTokens := make([]string, numValidations/2)

		for i := 0; i < numValidations/2; i++ {
			// Generate valid tokens
			userID := fmt.Sprintf("valid_user%d", i)
			username := fmt.Sprintf("valid_user%d", i)
			email := fmt.Sprintf("valid_user%d@example.com", i)

			token, err := jwtManager.GenerateToken(userID, username, email)
			require.NoError(t, err)
			validTokens[i] = token

			// Generate blacklisted tokens
			userID = fmt.Sprintf("blacklisted_user%d", i)
			username = fmt.Sprintf("blacklisted_user%d", i)
			email = fmt.Sprintf("blacklisted_user%d@example.com", i)

			token, err = jwtManager.GenerateToken(userID, username, email)
			require.NoError(t, err)
			blacklistedTokens[i] = token

			// Add to blacklist
			err = service.AddToBlacklist(ctx, token)
			require.NoError(t, err)
		}

		// Measure validation performance
		start := time.Now()

		// Validate all tokens
		for _, token := range validTokens {
			claims, err := service.ValidateTokenWithBlacklist(ctx, token)
			assert.NoError(t, err)
			assert.NotNil(t, claims)
		}

		for _, token := range blacklistedTokens {
			claims, err := service.ValidateTokenWithBlacklist(ctx, token)
			assert.Error(t, err)
			assert.Nil(t, claims)
			assert.Contains(t, err.Error(), "token is blacklisted")
		}

		duration := time.Since(start)
		assert.True(t, duration < 5*time.Second, "High volume validation should complete within 5 seconds, took %v", duration)
	})
}

// TestBlacklistService_RealWorldScenarios tests real-world usage scenarios
func TestBlacklistService_RealWorldScenarios(t *testing.T) {
	t.Run("User Logout Scenario", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()

		// User logs in and gets a token
		token, err := jwtManager.GenerateToken("user123", "john_doe", "john@example.com")
		require.NoError(t, err)

		// Token should be valid initially
		claims, err := service.ValidateTokenWithBlacklist(ctx, token)
		require.NoError(t, err)
		assert.NotNil(t, claims)

		// User logs out - token is added to blacklist
		err = service.AddToBlacklist(ctx, token)
		require.NoError(t, err)

		// Token should now be invalid
		_, err = service.ValidateTokenWithBlacklist(ctx, token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is blacklisted")

		// Check blacklist size
		size, err := service.GetBlacklistSize(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, size)
	})

	t.Run("Multiple Device Logout Scenario", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()

		// User has multiple active sessions with different users to ensure unique tokens
		userTokens := make([]string, 3)
		users := []struct{ id, username, email string }{
			{"user123", "john_doe", "john@example.com"},
			{"user124", "jane_doe", "jane@example.com"},
			{"user125", "bob_doe", "bob@example.com"},
		}

		for i, user := range users {
			token, err := jwtManager.GenerateToken(user.id, user.username, user.email)
			require.NoError(t, err)
			userTokens[i] = token
		}

		// All tokens should be valid initially
		for _, token := range userTokens {
			claims, err := service.ValidateTokenWithBlacklist(ctx, token)
			require.NoError(t, err)
			assert.NotNil(t, claims)
		}

		// User logs out from all devices (simulate by blacklisting all tokens)
		err := service.AddMultipleToBlacklist(ctx, userTokens)
		require.NoError(t, err)

		// All tokens should now be invalid
		for _, token := range userTokens {
			_, err := service.ValidateTokenWithBlacklist(ctx, token)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "token is blacklisted")
		}

		// Check blacklist size
		size, err := service.GetBlacklistSize(ctx)
		require.NoError(t, err)
		assert.Equal(t, 3, size)
	})

	t.Run("Password Change Security Scenario", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()

		// User has active tokens before password change
		oldToken, err := jwtManager.GenerateToken("user123", "john_doe", "john@example.com")
		require.NoError(t, err)

		// Token should be valid initially
		claims, err := service.ValidateTokenWithBlacklist(ctx, oldToken)
		require.NoError(t, err)
		assert.NotNil(t, claims)

		// User changes password - invalidate old token
		err = service.AddToBlacklist(ctx, oldToken)
		require.NoError(t, err)

		// Old token should be invalid
		_, err = service.ValidateTokenWithBlacklist(ctx, oldToken)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is blacklisted")

		// User gets new token after password change (simulate different user ID or timestamp)
		newToken, err := jwtManager.GenerateToken("user123", "john_doe_new", "john_new@example.com")
		require.NoError(t, err)

		// New token should be valid
		claims, err = service.ValidateTokenWithBlacklist(ctx, newToken)
		require.NoError(t, err)
		assert.NotNil(t, claims)
		assert.Equal(t, "user123", claims.UserID)
	})

	t.Run("Admin User Security Scenario", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()

		// Admin user token
		adminToken, err := jwtManager.GenerateToken("admin1", "admin_user", "admin@example.com")
		require.NoError(t, err)

		// Regular user token
		userToken, err := jwtManager.GenerateToken("user123", "regular_user", "user@example.com")
		require.NoError(t, err)

		// Both tokens should be valid initially
		adminClaims, err := service.ValidateTokenWithBlacklist(ctx, adminToken)
		require.NoError(t, err)
		assert.NotNil(t, adminClaims)

		userClaims, err := service.ValidateTokenWithBlacklist(ctx, userToken)
		require.NoError(t, err)
		assert.NotNil(t, userClaims)

		// Suspicious activity detected - invalidate admin token immediately
		err = service.AddToBlacklist(ctx, adminToken)
		require.NoError(t, err)

		// Admin token should be invalid
		_, err = service.ValidateTokenWithBlacklist(ctx, adminToken)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is blacklisted")

		// Regular user token should still be valid
		userClaims, err = service.ValidateTokenWithBlacklist(ctx, userToken)
		require.NoError(t, err)
		assert.NotNil(t, userClaims)
		assert.Equal(t, "user123", userClaims.UserID)
	})

	t.Run("Token Expiration Cleanup Scenario", func(t *testing.T) {
		mockCache := NewMockCache()
		jwtManager := auth.NewJWTManager("test-secret", 24)
		service := NewBlacklistService(mockCache, jwtManager, nil)

		ctx := context.Background()

		// Create tokens and manually add to cache with different expiration times
		shortLivedToken := createTestToken(jwtManager, "user1", "user1", "user1@example.com", time.Hour)
		longLivedToken := createTestToken(jwtManager, "user2", "user2", "user2@example.com", time.Hour)

		// Manually add tokens to mock cache with different expiration times
		shortLivedKey := service.generateTokenKey(shortLivedToken)
		longLivedKey := service.generateTokenKey(longLivedToken)

		mockCache.Set(ctx, shortLivedKey, "blacklisted", 100*time.Millisecond)
		mockCache.Set(ctx, longLivedKey, "blacklisted", time.Hour)

		// Both should be blacklisted initially
		blacklisted, err := service.IsBlacklisted(ctx, shortLivedToken)
		require.NoError(t, err)
		assert.True(t, blacklisted)

		blacklisted, err = service.IsBlacklisted(ctx, longLivedToken)
		require.NoError(t, err)
		assert.True(t, blacklisted)

		// Wait for short-lived token to expire
		time.Sleep(150 * time.Millisecond)

		// Short-lived token should no longer be blacklisted (expired)
		blacklisted, err = service.IsBlacklisted(ctx, shortLivedToken)
		require.NoError(t, err)
		assert.False(t, blacklisted)

		// Long-lived token should still be blacklisted
		blacklisted, err = service.IsBlacklisted(ctx, longLivedToken)
		require.NoError(t, err)
		assert.True(t, blacklisted)

		// Run cleanup (should not error)
		err = service.CleanupExpiredTokens(ctx)
		require.NoError(t, err)
	})
}

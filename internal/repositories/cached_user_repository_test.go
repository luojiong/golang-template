package repositories

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-server/internal/config"
	"go-server/internal/database"
	"go-server/internal/logger"
	"go-server/internal/models"
	"go-server/pkg/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

// CachedUserRepositoryIntegrationTestSuite defines the test suite for cached user repository
type CachedUserRepositoryIntegrationTestSuite struct {
	suite.Suite
	db         *gorm.DB
	cache      cache.Cache
	baseRepo   UserRepository
	cachedRepo UserRepository
	cleanup    func()
}

// SetupSuite is called once before all tests in the suite
func (suite *CachedUserRepositoryIntegrationTestSuite) SetupSuite() {
	// Setup test database
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:            "localhost",
			Port:            5432,
			User:            "postgres",
			Password:        "postgres",
			DBName:          "golang_template_test",
			SSLMode:         "disable",
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: 300,
		},
	}

	loggerManager, err := logger.NewManager(config.LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		})
		if err != nil {
			suite.T().Skipf("Logger not available for testing: %v", err)
			return
		}
	db, err := database.NewDatabase(cfg, loggerManager)
	if err != nil {
		suite.T().Skipf("Database not available for testing: %v", err)
		return
	}
	suite.db = db.DB

	// Setup test Redis cache
	redisConfig := cache.DefaultRedisConfig()
	redisConfig.Host = "localhost"
	redisConfig.Port = 6379
	redisConfig.DB = 2 // Use different DB for tests
	redisConfig.Prefix = "test_cached_user:"

	testCache, err := cache.NewRedisCache(redisConfig)
	if err != nil {
		suite.T().Skipf("Redis not available for testing: %v", err)
		return
	}
	suite.cache = testCache

	// Create repositories
	suite.baseRepo = NewUserRepository(suite.db)
	suite.cachedRepo = NewCachedUserRepository(suite.baseRepo, suite.cache)

	// Setup cleanup function
	suite.cleanup = func() {
		// Clean up cache
		if suite.cache != nil {
			ctx := context.Background()
			suite.cache.Clear(ctx)
			suite.cache.Close()
		}
		// Clean up database
		if suite.db != nil {
			suite.db.Exec("DELETE FROM users")
			sqlDB, _ := suite.db.DB()
			sqlDB.Close()
		}
	}

	// Run auto-migration
	err = suite.db.AutoMigrate(&models.User{})
	require.NoError(suite.T(), err)
}

// TearDownSuite is called once after all tests in the suite
func (suite *CachedUserRepositoryIntegrationTestSuite) TearDownSuite() {
	if suite.cleanup != nil {
		suite.cleanup()
	}
}

// SetupTest is called before each test in the suite
func (suite *CachedUserRepositoryIntegrationTestSuite) SetupTest() {
	// Clear cache before each test
	if suite.cache != nil {
		ctx := context.Background()
		suite.cache.Clear(ctx)
	}

	// Clear database before each test
	if suite.db != nil {
		suite.db.Exec("DELETE FROM users")
	}
}

// createTestUser creates a test user for testing purposes
func (suite *CachedUserRepositoryIntegrationTestSuite) createTestUser(email, username string) *models.User {
	user := &models.User{
		Username:  username,
		Email:     email,
		FirstName: "Test",
		LastName:  "User",
		Password:  "hashedpassword",
		IsActive:  true,
		IsAdmin:   false,
	}
	return user
}

// TestCachedUserRepository_GetByID_CacheHitMiss tests cache hit and miss scenarios
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_GetByID_CacheHitMiss() {
	// Create a test user
	user := suite.createTestUser("cache@test.com", "cachetestuser")
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)
	require.NotEmpty(suite.T(), user.ID)

	ctx := context.Background()

	// First call should be a cache miss (database query)
	start := time.Now()
	userFromDB, err := suite.cachedRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), userFromDB)
	dbCallDuration := time.Since(start)

	// Verify the user data
	assert.Equal(suite.T(), user.ID, userFromDB.ID)
	assert.Equal(suite.T(), user.Username, userFromDB.Username)
	assert.Equal(suite.T(), user.Email, userFromDB.Email)

	// Verify user is cached
	cacheKey := fmt.Sprintf("user:id:%s", user.ID)
	cachedValue, found := suite.cache.Get(ctx, cacheKey)
	require.True(suite.T(), found, "User should be cached after first retrieval")
	require.NotNil(suite.T(), cachedValue)

	// Second call should be a cache hit (no database query)
	start = time.Now()
	userFromCache, err := suite.cachedRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), userFromCache)
	cacheCallDuration := time.Since(start)

	// Verify same data is returned
	assert.Equal(suite.T(), userFromDB.ID, userFromCache.ID)
	assert.Equal(suite.T(), userFromDB.Username, userFromCache.Username)
	assert.Equal(suite.T(), userFromDB.Email, userFromCache.Email)

	// Cache hit should be significantly faster than database call
	assert.Less(suite.T(), cacheCallDuration, dbCallDuration,
		"Cache hit should be faster than database call")
}

// TestCachedUserRepository_GetByEmail_CacheHitMiss tests email-based caching
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_GetByEmail_CacheHitMiss() {
	user := suite.createTestUser("emailcache@test.com", "emailcacheuser")
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)

	ctx := context.Background()

	// First call - cache miss
	user1, err := suite.cachedRepo.GetByEmail(user.Email)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), user1)

	// Verify cached
	cacheKey := fmt.Sprintf("user:email:%s", user.Email)
	cachedValue, found := suite.cache.Get(ctx, cacheKey)
	require.True(suite.T(), found)
	require.NotNil(suite.T(), cachedValue)

	// Second call - cache hit
	user2, err := suite.cachedRepo.GetByEmail(user.Email)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), user2)

	// Verify same data
	assert.Equal(suite.T(), user1.ID, user2.ID)
	assert.Equal(suite.T(), user1.Email, user2.Email)
}

// TestCachedUserRepository_GetByUsername_CacheHitMiss tests username-based caching
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_GetByUsername_CacheHitMiss() {
	user := suite.createTestUser("usercache@test.com", "usercacheusername")
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)

	ctx := context.Background()

	// First call - cache miss
	user1, err := suite.cachedRepo.GetByUsername(user.Username)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), user1)

	// Verify cached
	cacheKey := fmt.Sprintf("user:username:%s", user.Username)
	cachedValue, found := suite.cache.Get(ctx, cacheKey)
	require.True(suite.T(), found)
	require.NotNil(suite.T(), cachedValue)

	// Second call - cache hit
	user2, err := suite.cachedRepo.GetByUsername(user.Username)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), user2)

	// Verify same data
	assert.Equal(suite.T(), user1.ID, user2.ID)
	assert.Equal(suite.T(), user1.Username, user2.Username)
}

// TestCachedUserRepository_GetAll_PaginatedCaching tests paginated list caching
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_GetAll_PaginatedCaching() {
	// Create multiple test users
	users := make([]*models.User, 5)
	for i := 0; i < 5; i++ {
		user := suite.createTestUser(
			fmt.Sprintf("listuser%d@test.com", i),
			fmt.Sprintf("listuser%d", i),
		)
		err := suite.cachedRepo.Create(user)
		require.NoError(suite.T(), err)
		users[i] = user
	}

	ctx := context.Background()

	// Test first page
	offset, limit := 0, 3
	cacheKey := fmt.Sprintf("users:all:%d:%d", offset, limit)

	// First call - cache miss
	users1, total1, err := suite.cachedRepo.GetAll(offset, limit)
	require.NoError(suite.T(), err)
	assert.Len(suite.T(), users1, 3)
	assert.Equal(suite.T(), int64(5), total1)

	// Verify cached
	cachedValue, found := suite.cache.Get(ctx, cacheKey)
	require.True(suite.T(), found)
	require.NotNil(suite.T(), cachedValue)

	// Second call - cache hit
	users2, total2, err := suite.cachedRepo.GetAll(offset, limit)
	require.NoError(suite.T(), err)
	assert.Len(suite.T(), users2, 3)
	assert.Equal(suite.T(), total1, total2)

	// Verify same data (preserve order)
	for i := 0; i < 3; i++ {
		assert.Equal(suite.T(), users1[i].ID, users2[i].ID)
		assert.Equal(suite.T(), users1[i].Email, users2[i].Email)
	}

	// Test different page (should be cached separately)
	offset2 := 3
	cacheKey2 := fmt.Sprintf("users:all:%d:%d", offset2, limit)

	// First call for second page - cache miss
	users3, total3, err := suite.cachedRepo.GetAll(offset2, limit)
	require.NoError(suite.T(), err)
	assert.Len(suite.T(), users3, 2)
	assert.Equal(suite.T(), int64(5), total3)

	// Verify second page is cached separately
	cachedValue2, found := suite.cache.Get(ctx, cacheKey2)
	require.True(suite.T(), found)
	require.NotNil(suite.T(), cachedValue2)
}

// TestCachedUserRepository_ExistsByEmail_Caching tests existence check caching
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_ExistsByEmail_Caching() {
	email := "exists@test.com"
	user := suite.createTestUser(email, "existsuser")
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)

	ctx := context.Background()

	// First call - cache miss
	exists1, err := suite.cachedRepo.ExistsByEmail(email)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), exists1)

	// Verify cached
	cacheKey := fmt.Sprintf("user:exists:email:%s", email)
	cachedValue, found := suite.cache.Get(ctx, cacheKey)
	require.True(suite.T(), found)
	assert.Equal(suite.T(), true, cachedValue)

	// Second call - cache hit
	exists2, err := suite.cachedRepo.ExistsByEmail(email)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), exists1, exists2)

	// Test non-existent email
	nonExistentEmail := "nonexistent@test.com"
	exists3, err := suite.cachedRepo.ExistsByEmail(nonExistentEmail)
	require.NoError(suite.T(), err)
	assert.False(suite.T(), exists3)

	// Verify non-existence is also cached
	cacheKey3 := fmt.Sprintf("user:exists:email:%s", nonExistentEmail)
	cachedValue3, found := suite.cache.Get(ctx, cacheKey3)
	require.True(suite.T(), found)
	assert.Equal(suite.T(), false, cachedValue3)
}

// TestCachedUserRepository_ExistsByUsername_Caching tests username existence caching
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_ExistsByUsername_Caching() {
	username := "existsusername"
	user := suite.createTestUser("exists2@test.com", username)
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)

	ctx := context.Background()

	// First call - cache miss
	exists1, err := suite.cachedRepo.ExistsByUsername(username)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), exists1)

	// Verify cached
	cacheKey := fmt.Sprintf("user:exists:username:%s", username)
	cachedValue, found := suite.cache.Get(ctx, cacheKey)
	require.True(suite.T(), found)
	assert.Equal(suite.T(), true, cachedValue)

	// Second call - cache hit
	exists2, err := suite.cachedRepo.ExistsByUsername(username)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), exists1, exists2)
}

// TestCachedUserRepository_Count_Caching tests count caching
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_Count_Caching() {
	// Create test users
	for i := 0; i < 3; i++ {
		user := suite.createTestUser(
			fmt.Sprintf("countuser%d@test.com", i),
			fmt.Sprintf("countuser%d", i),
		)
		err := suite.cachedRepo.Create(user)
		require.NoError(suite.T(), err)
	}

	ctx := context.Background()

	// First call - cache miss
	count1, err := suite.cachedRepo.Count()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(3), count1)

	// Verify cached
	cachedValue, found := suite.cache.Get(ctx, "users:count")
	require.True(suite.T(), found)
	assert.Equal(suite.T(), int64(3), cachedValue)

	// Second call - cache hit
	count2, err := suite.cachedRepo.Count()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), count1, count2)
}

// TestCachedUserRepository_Create_CacheInvalidation tests cache invalidation on create
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_Create_CacheInvalidation() {
	ctx := context.Background()

	// Create a user and cache the count
	user1 := suite.createTestUser("invalidate1@test.com", "invalidateuser1")
	err := suite.cachedRepo.Create(user1)
	require.NoError(suite.T(), err)

	// Get initial count and verify it's cached
	count1, err := suite.cachedRepo.Count()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count1)

	_, found := suite.cache.Get(ctx, "users:count")
	require.True(suite.T(), found)

	// Create another user
	user2 := suite.createTestUser("invalidate2@test.com", "invalidateuser2")
	err = suite.cachedRepo.Create(user2)
	require.NoError(suite.T(), err)

	// Verify count cache was invalidated
	_, found = suite.cache.Get(ctx, "users:count")
	require.False(suite.T(), found, "Count cache should be invalidated after create")

	// Verify existence caches for new user are invalidated
	emailCacheKey := fmt.Sprintf("user:exists:email:%s", user2.Email)
	usernameCacheKey := fmt.Sprintf("user:exists:username:%s", user2.Username)

	_, found = suite.cache.Get(ctx, emailCacheKey)
	require.False(suite.T(), found, "Email existence cache should be invalidated")
	_, found = suite.cache.Get(ctx, usernameCacheKey)
	require.False(suite.T(), found, "Username existence cache should be invalidated")

	// Get updated count
	count2, err := suite.cachedRepo.Count()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(2), count2)
}

// TestCachedUserRepository_Update_CacheInvalidation tests cache invalidation on update
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_Update_CacheInvalidation() {
	ctx := context.Background()

	user := suite.createTestUser("update@test.com", "updateuser")
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)

	// Cache the user
	user1, err := suite.cachedRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), user1)

	// Verify user is cached by different keys
	idCacheKey := fmt.Sprintf("user:id:%s", user.ID)
	emailCacheKey := fmt.Sprintf("user:email:%s", user.Email)
	usernameCacheKey := fmt.Sprintf("user:username:%s", user.Username)

	_, found := suite.cache.Get(ctx, idCacheKey)
	require.True(suite.T(), found)
	_, found = suite.cache.Get(ctx, emailCacheKey)
	require.True(suite.T(), found)
	_, found = suite.cache.Get(ctx, usernameCacheKey)
	require.True(suite.T(), found)

	// Update user
	user.FirstName = "Updated"
	user.LastName = "Name"
	err = suite.cachedRepo.Update(user)
	require.NoError(suite.T(), err)

	// Verify all user-related caches are invalidated
	_, found = suite.cache.Get(ctx, idCacheKey)
	require.False(suite.T(), found, "ID cache should be invalidated after update")
	_, found = suite.cache.Get(ctx, emailCacheKey)
	require.False(suite.T(), found, "Email cache should be invalidated after update")
	_, found = suite.cache.Get(ctx, usernameCacheKey)
	require.False(suite.T(), found, "Username cache should be invalidated after update")

	// Verify list caches are also invalidated
	keys, err := suite.cache.Keys(ctx, "users:all:*")
	require.NoError(suite.T(), err)
	assert.Empty(suite.T(), keys, "List caches should be invalidated after update")

	// Get updated user (should be re-cached)
	user2, err := suite.cachedRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "Updated", user2.FirstName)
	assert.Equal(suite.T(), "Name", user2.LastName)

	// Verify user is re-cached
	_, found = suite.cache.Get(ctx, idCacheKey)
	require.True(suite.T(), found)
}

// TestCachedUserRepository_Delete_CacheInvalidation tests cache invalidation on delete
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_Delete_CacheInvalidation() {
	ctx := context.Background()

	user := suite.createTestUser("delete@test.com", "deleteuser")
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)

	// Cache the user and count
	_, err = suite.cachedRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)
	count1, err := suite.cachedRepo.Count()
	require.NoError(suite.T(), err)

	// Verify caches exist
	idCacheKey := fmt.Sprintf("user:id:%s", user.ID)
	emailCacheKey := fmt.Sprintf("user:email:%s", user.Email)
	usernameCacheKey := fmt.Sprintf("user:username:%s", user.Username)

	_, found := suite.cache.Get(ctx, idCacheKey)
	require.True(suite.T(), found)
	_, found = suite.cache.Get(ctx, "users:count")
	require.True(suite.T(), found)

	// Delete user
	err = suite.cachedRepo.Delete(user.ID)
	require.NoError(suite.T(), err)

	// Verify all user-related caches are invalidated
	_, found = suite.cache.Get(ctx, idCacheKey)
	require.False(suite.T(), found, "ID cache should be invalidated after delete")
	_, found = suite.cache.Get(ctx, emailCacheKey)
	require.False(suite.T(), found, "Email cache should be invalidated after delete")
	_, found = suite.cache.Get(ctx, usernameCacheKey)
	require.False(suite.T(), found, "Username cache should be invalidated after delete")
	_, found = suite.cache.Get(ctx, "users:count")
	require.False(suite.T(), found, "Count cache should be invalidated after delete")

	// Verify user is deleted
	user2, err := suite.cachedRepo.GetByID(user.ID)
	require.Error(suite.T(), err)
	require.Nil(suite.T(), user2)

	// Verify count is updated
	count2, err := suite.cachedRepo.Count()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), count1-1, count2)
}

// TestCachedUserRepository_UpdateLastLogin_CacheInvalidation tests cache invalidation on last login update
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_UpdateLastLogin_CacheInvalidation() {
	ctx := context.Background()

	user := suite.createTestUser("login@test.com", "loginuser")
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)

	// Cache the user
	user1, err := suite.cachedRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)
	assert.Nil(suite.T(), user1.LastLogin)

	// Verify user is cached
	idCacheKey := fmt.Sprintf("user:id:%s", user.ID)
	_, found := suite.cache.Get(ctx, idCacheKey)
	require.True(suite.T(), found)

	// Update last login
	err = suite.cachedRepo.UpdateLastLogin(user.ID)
	require.NoError(suite.T(), err)

	// Verify cache is invalidated
	_, found = suite.cache.Get(ctx, idCacheKey)
	require.False(suite.T(), found, "Cache should be invalidated after last login update")

	// Get updated user
	user2, err := suite.cachedRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), user2.LastLogin)

	// Verify user is re-cached with updated login time
	_, found = suite.cache.Get(ctx, idCacheKey)
	require.True(suite.T(), found)
}

// TestCachedUserRepository_TTL_Expiration tests TTL expiration behavior
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_TTL_Expiration() {
	user := suite.createTestUser("ttl@test.com", "ttluser")
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)

	ctx := context.Background()

	// Create a cached repository with short TTL for testing
	shortTTLRepo := &CachedUserRepository{
		repo:  suite.baseRepo,
		cache: suite.cache,
		ttl:   100 * time.Millisecond, // Very short TTL
	}

	// Cache the user
	user1, err := shortTTLRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), user1)

	// Verify cached
	cacheKey := fmt.Sprintf("user:id:%s", user.ID)
	cachedValue, found := suite.cache.Get(ctx, cacheKey)
	require.True(suite.T(), found)
	require.NotNil(suite.T(), cachedValue)

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Verify cache has expired
	cachedValue, found = suite.cache.Get(ctx, cacheKey)
	require.False(suite.T(), found, "Cache should have expired")

	// Next call should be cache miss
	user2, err := shortTTLRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), user2)

	// Verify same data is returned
	assert.Equal(suite.T(), user1.ID, user2.ID)
	assert.Equal(suite.T(), user1.Email, user2.Email)

	// Verify data is re-cached
	cachedValue, found = suite.cache.Get(ctx, cacheKey)
	require.True(suite.T(), found)
	require.NotNil(suite.T(), cachedValue)
}

// TestCachedUserRepository_ConcurrentAccess tests concurrent access to cached repository
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_ConcurrentAccess() {
	user := suite.createTestUser("concurrent@test.com", "concurrentuser")
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)

	const numGoroutines = 10
	const numOperations = 5

	done := make(chan bool, numGoroutines)

	// Launch multiple goroutines accessing the same user concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < numOperations; j++ {
				// Test different operations
				switch j % 3 {
				case 0:
					_, err := suite.cachedRepo.GetByID(user.ID)
					assert.NoError(suite.T(), err)
				case 1:
					_, err := suite.cachedRepo.GetByEmail(user.Email)
					assert.NoError(suite.T(), err)
				case 2:
					_, err := suite.cachedRepo.GetByUsername(user.Username)
					assert.NoError(suite.T(), err)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify user is properly cached
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:id:%s", user.ID)
	cachedValue, found := suite.cache.Get(ctx, cacheKey)
	require.True(suite.T(), found)
	require.NotNil(suite.T(), cachedValue)
}

// TestCachedUserRepository_CacheFallbackBehavior tests fallback behavior when cache operations fail
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_CacheFallbackBehavior() {
	user := suite.createTestUser("fallback@test.com", "fallbackuser")
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)

	// First retrieval should work and cache the user
	user1, err := suite.cachedRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), user1)

	// Close cache connection to simulate cache failure
	err = suite.cache.Close()
	require.NoError(suite.T(), err)

	// Operations should still work by falling back to database
	user2, err := suite.cachedRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), user2)

	// Verify same data is returned
	assert.Equal(suite.T(), user1.ID, user2.ID)
	assert.Equal(suite.T(), user1.Email, user2.Email)

	// Other operations should also work
	exists, err := suite.cachedRepo.ExistsByEmail(user.Email)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), exists)

	count, err := suite.cachedRepo.Count()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), count)
}

// TestCachedUserRepository_RealWorldScenario tests a real-world caching scenario
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_RealWorldScenario() {
	ctx := context.Background()

	// Create multiple users
	users := make([]*models.User, 3)
	for i := 0; i < 3; i++ {
		user := suite.createTestUser(
			fmt.Sprintf("realuser%d@test.com", i),
			fmt.Sprintf("realuser%d", i),
		)
		user.FirstName = fmt.Sprintf("User%d", i)
		user.LastName = "Test"
		err := suite.cachedRepo.Create(user)
		require.NoError(suite.T(), err)
		users[i] = user
	}

	// Simulate typical application access patterns

	// 1. User login (cache miss)
	start := time.Now()
	loginUser, err := suite.cachedRepo.GetByEmail(users[0].Email)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), loginUser)
	loginDuration := time.Since(start)

	// 2. Subsequent user data access (cache hit)
	start = time.Now()
	userProfile, err := suite.cachedRepo.GetByID(loginUser.ID)
	require.NoError(suite.T(), err)
	require.NotNil(suite.T(), userProfile)
	profileDuration := time.Since(start)

	// 3. Admin user list (cache miss)
	start = time.Now()
	allUsers, total, err := suite.cachedRepo.GetAll(0, 10)
	require.NoError(suite.T(), err)
	assert.Len(suite.T(), allUsers, 3)
	assert.Equal(suite.T(), int64(3), total)
	listDuration := time.Since(start)

	// 4. Check user existence (cache miss)
	start = time.Now()
	exists, err := suite.cachedRepo.ExistsByEmail(users[1].Email)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), exists)
	_ = time.Since(start) // existsDuration unused but time measured for performance context

	// 5. Another admin user list (cache hit)
	start = time.Now()
	allUsers2, total2, err := suite.cachedRepo.GetAll(0, 10)
	require.NoError(suite.T(), err)
	assert.Len(suite.T(), allUsers2, 3)
	assert.Equal(suite.T(), int64(3), total2)
	listDuration2 := time.Since(start)

	// Verify cache benefits
	assert.Less(suite.T(), profileDuration, loginDuration,
		"Profile access should be faster than initial login (cache hit vs miss)")
	assert.Less(suite.T(), listDuration2, listDuration,
		"Second user list should be faster than first (cache hit vs miss)")

	// 6. User update (should invalidate caches)
	loginUser.FirstName = "UpdatedName"
	err = suite.cachedRepo.Update(loginUser)
	require.NoError(suite.T(), err)

	// 7. Get updated user (cache miss due to invalidation)
	updatedUser, err := suite.cachedRepo.GetByID(loginUser.ID)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), "UpdatedName", updatedUser.FirstName)

	// 8. Verify list caches were invalidated
	keys, err := suite.cache.Keys(ctx, "users:all:*")
	require.NoError(suite.T(), err)
	assert.Empty(suite.T(), keys, "List caches should be invalidated after user update")

	// 9. User count (should work correctly after updates)
	count, err := suite.cachedRepo.Count()
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(3), count)
}

// TestCachedUserRepository_CacheConsistency tests cache consistency across different operations
func (suite *CachedUserRepositoryIntegrationTestSuite) TestCachedUserRepository_CacheConsistency() {
	ctx := context.Background()

	user := suite.createTestUser("consistency@test.com", "consistencyuser")
	err := suite.cachedRepo.Create(user)
	require.NoError(suite.T(), err)

	// Access user through different methods to populate various cache keys
	userByID, err := suite.cachedRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)

	userByEmail, err := suite.cachedRepo.GetByEmail(user.Email)
	require.NoError(suite.T(), err)

	userByUsername, err := suite.cachedRepo.GetByUsername(user.Username)
	require.NoError(suite.T(), err)

	// Verify all methods return the same data
	assert.Equal(suite.T(), userByID.ID, userByEmail.ID)
	assert.Equal(suite.T(), userByEmail.ID, userByUsername.ID)
	assert.Equal(suite.T(), userByID.Email, userByEmail.Email)
	assert.Equal(suite.T(), userByEmail.Email, userByUsername.Email)
	assert.Equal(suite.T(), userByID.Username, userByUsername.Username)

	// Verify all cache keys exist
	idCacheKey := fmt.Sprintf("user:id:%s", user.ID)
	emailCacheKey := fmt.Sprintf("user:email:%s", user.Email)
	usernameCacheKey := fmt.Sprintf("user:username:%s", user.Username)

	_, found := suite.cache.Get(ctx, idCacheKey)
	require.True(suite.T(), found)
	_, found = suite.cache.Get(ctx, emailCacheKey)
	require.True(suite.T(), found)
	_, found = suite.cache.Get(ctx, usernameCacheKey)
	require.True(suite.T(), found)

	// Update user
	user.FirstName = "Consistency Test"
	err = suite.cachedRepo.Update(user)
	require.NoError(suite.T(), err)

	// Verify all cache keys are invalidated
	_, found = suite.cache.Get(ctx, idCacheKey)
	require.False(suite.T(), found)
	_, found = suite.cache.Get(ctx, emailCacheKey)
	require.False(suite.T(), found)
	_, found = suite.cache.Get(ctx, usernameCacheKey)
	require.False(suite.T(), found)

	// Re-access user through different methods
	updatedUserByID, err := suite.cachedRepo.GetByID(user.ID)
	require.NoError(suite.T(), err)

	updatedUserByEmail, err := suite.cachedRepo.GetByEmail(user.Email)
	require.NoError(suite.T(), err)

	updatedUserByUsername, err := suite.cachedRepo.GetByUsername(user.Username)
	require.NoError(suite.T(), err)

	// Verify consistency after update
	assert.Equal(suite.T(), updatedUserByID.ID, updatedUserByEmail.ID)
	assert.Equal(suite.T(), updatedUserByEmail.ID, updatedUserByUsername.ID)
	assert.Equal(suite.T(), updatedUserByID.FirstName, "Consistency Test")
	assert.Equal(suite.T(), updatedUserByEmail.FirstName, "Consistency Test")
	assert.Equal(suite.T(), updatedUserByUsername.FirstName, "Consistency Test")
}

// TestCachedUserRepository_Integration runs the integration test suite
func TestCachedUserRepository_Integration(t *testing.T) {
	suite.Run(t, new(CachedUserRepositoryIntegrationTestSuite))
}

// TestCachedUserRepository_Unit_Tests provides some unit-level tests for specific edge cases
func TestCachedUserRepository_Unit_Tests(t *testing.T) {
	t.Run("NewCachedUserRepository_Creation", func(t *testing.T) {
		// Create a mock base repository
		mockRepo := &MockUserRepository{}
		mockCache := &MockCache{}

		// Create cached repository
		cachedRepo := NewCachedUserRepository(mockRepo, mockCache)

		// Verify type
		assert.NotNil(t, cachedRepo)
		_, ok := cachedRepo.(*CachedUserRepository)
		assert.True(t, ok)
	})

	t.Run("CachedUserRepository_DefaultTTL", func(t *testing.T) {
		mockRepo := &MockUserRepository{}
		mockCache := &MockCache{}

		cachedRepo := NewCachedUserRepository(mockRepo, mockCache)
		repo := cachedRepo.(*CachedUserRepository)

		// Verify default TTL is 5 minutes
		assert.Equal(t, 5*time.Minute, repo.ttl)
	})
}

// MockUserRepository is a mock implementation for unit testing
type MockUserRepository struct {
	users map[string]*models.User
}

func (m *MockUserRepository) Create(user *models.User) error {
	if m.users == nil {
		m.users = make(map[string]*models.User)
	}
	m.users[user.ID] = user
	return nil
}

func (m *MockUserRepository) GetByID(id string) (*models.User, error) {
	if user, exists := m.users[id]; exists {
		return user, nil
	}
	return nil, fmt.Errorf("user not found")
}

func (m *MockUserRepository) GetByEmail(email string) (*models.User, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (m *MockUserRepository) GetByUsername(username string) (*models.User, error) {
	for _, user := range m.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (m *MockUserRepository) GetAll(offset, limit int) ([]*models.User, int64, error) {
	users := make([]*models.User, 0, len(m.users))
	for _, user := range m.users {
		users = append(users, user)
	}
	return users, int64(len(users)), nil
}

func (m *MockUserRepository) Update(user *models.User) error {
	if _, exists := m.users[user.ID]; exists {
		m.users[user.ID] = user
		return nil
	}
	return fmt.Errorf("user not found")
}

func (m *MockUserRepository) Delete(id string) error {
	if _, exists := m.users[id]; exists {
		delete(m.users, id)
		return nil
	}
	return fmt.Errorf("user not found")
}

func (m *MockUserRepository) UpdateLastLogin(id string) error {
	if user, exists := m.users[id]; exists {
		now := time.Now()
		user.LastLogin = &now
		return nil
	}
	return fmt.Errorf("user not found")
}

func (m *MockUserRepository) ExistsByEmail(email string) (bool, error) {
	for _, user := range m.users {
		if user.Email == email {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockUserRepository) ExistsByUsername(username string) (bool, error) {
	for _, user := range m.users {
		if user.Username == username {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockUserRepository) Count() (int64, error) {
	return int64(len(m.users)), nil
}

// MockCache is a mock implementation for unit testing
type MockCache struct {
	data map[string]interface{}
}

func (m *MockCache) Get(ctx context.Context, key string) (interface{}, bool) {
	if m.data == nil {
		m.data = make(map[string]interface{})
	}
	value, exists := m.data[key]
	return value, exists
}

func (m *MockCache) GetWithTTL(ctx context.Context, key string) (interface{}, time.Duration, bool) {
	value, exists := m.Get(ctx, key)
	return value, 0, exists
}

func (m *MockCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if m.data == nil {
		m.data = make(map[string]interface{})
	}
	m.data[key] = value
	return nil
}

func (m *MockCache) SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if m.data == nil {
		m.data = make(map[string]interface{})
	}
	for key, value := range items {
		m.data[key] = value
	}
	return nil
}

func (m *MockCache) Delete(ctx context.Context, key string) error {
	if m.data != nil {
		delete(m.data, key)
	}
	return nil
}

func (m *MockCache) DeleteMultiple(ctx context.Context, keys []string) error {
	if m.data != nil {
		for _, key := range keys {
			delete(m.data, key)
		}
	}
	return nil
}

func (m *MockCache) Exists(ctx context.Context, key string) (bool, error) {
	_, exists := m.Get(ctx, key)
	return exists, nil
}

func (m *MockCache) Clear(ctx context.Context) error {
	m.data = make(map[string]interface{})
	return nil
}

func (m *MockCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	keys := make([]string, 0, len(m.data))
	for key := range m.data {
		keys = append(keys, key)
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
	if _, exists := m.Get(ctx, key); !exists {
		return m.Set(ctx, key, value, ttl) == nil, nil
	}
	return false, nil
}

func (m *MockCache) Increment(ctx context.Context, key string, amount int64) (int64, error) {
	if m.data == nil {
		m.data = make(map[string]interface{})
	}
	var current int64
	if val, exists := m.data[key]; exists {
		if num, ok := val.(int64); ok {
			current = num
		}
	}
	current += amount
	m.data[key] = current
	return current, nil
}

func (m *MockCache) Decrement(ctx context.Context, key string, amount int64) (int64, error) {
	return m.Increment(ctx, key, -amount)
}

func (m *MockCache) Close() error {
	m.data = nil
	return nil
}

// Health checks the health of the mock cache connection
func (m *MockCache) Health(ctx context.Context) error {
	// Mock cache is always healthy for testing
	return nil
}

// GetStats returns mock cache statistics for testing
func (m *MockCache) GetStats(ctx context.Context) (map[string]interface{}, error) {
	itemCount := 0
	if m.data != nil {
		itemCount = len(m.data)
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
			"total_keys":                 int64(itemCount),
			"total_commands_processed":   int64(1000),
			"total_connections_received": int64(50),
			"keyspace_hits":              int64(950),
			"keyspace_misses":            int64(50),
			"keyspace_hit_rate_percent":  95.0,
		},
		"latency_ms":  1,
		"total_items": itemCount,
	}, nil
}

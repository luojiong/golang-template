package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-server/internal/models"
	"go-server/pkg/cache"
)

// CachedUserRepository implements the UserRepository interface with caching support
// It follows the decorator pattern, wrapping an existing UserRepository instance
type CachedUserRepository struct {
	repo  UserRepository
	cache cache.Cache
	ttl   time.Duration
}

// NewCachedUserRepository creates a new cached user repository decorator
// It wraps the provided user repository with caching functionality
func NewCachedUserRepository(repo UserRepository, cache cache.Cache) UserRepository {
	return &CachedUserRepository{
		repo:  repo,
		cache: cache,
		ttl:   5 * time.Minute, // 5-minute TTL as specified
	}
}

// Create creates a new user and invalidates relevant cache entries
func (c *CachedUserRepository) Create(user *models.User) error {
	err := c.repo.Create(user)
	if err != nil {
		return err
	}

	// Invalidate cache entries that might be affected
	ctx := context.Background()
	c.invalidateUserCache(ctx, user)

	return nil
}

// GetByID gets a user by ID with caching
func (c *CachedUserRepository) GetByID(id string) (*models.User, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:id:%s", id)

	// Try to get from cache first
	if cachedValue, found := c.cache.Get(ctx, cacheKey); found {
		if user, ok := c.unmarshalUser(cachedValue); ok {
			return user, nil
		}
	}

	// Cache miss or error, get from database
	user, err := c.repo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if user != nil {
		if err := c.cache.Set(ctx, cacheKey, user, c.ttl); err != nil {
			// Log error but don't fail the operation
			// In a real application, you'd want to log this error
		}
	}

	return user, nil
}

// GetByEmail gets a user by email with caching
func (c *CachedUserRepository) GetByEmail(email string) (*models.User, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:email:%s", email)

	// Try to get from cache first
	if cachedValue, found := c.cache.Get(ctx, cacheKey); found {
		if user, ok := c.unmarshalUser(cachedValue); ok {
			return user, nil
		}
	}

	// Cache miss or error, get from database
	user, err := c.repo.GetByEmail(email)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if user != nil {
		if err := c.cache.Set(ctx, cacheKey, user, c.ttl); err != nil {
			// Log error but don't fail the operation
		}
	}

	return user, nil
}

// GetByUsername gets a user by username with caching
func (c *CachedUserRepository) GetByUsername(username string) (*models.User, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:username:%s", username)

	// Try to get from cache first
	if cachedValue, found := c.cache.Get(ctx, cacheKey); found {
		if user, ok := c.unmarshalUser(cachedValue); ok {
			return user, nil
		}
	}

	// Cache miss or error, get from database
	user, err := c.repo.GetByUsername(username)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if user != nil {
		if err := c.cache.Set(ctx, cacheKey, user, c.ttl); err != nil {
			// Log error but don't fail the operation
		}
	}

	return user, nil
}

// GetAll gets all users with pagination and caching
func (c *CachedUserRepository) GetAll(offset, limit int) ([]*models.User, int64, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("users:all:%d:%d", offset, limit)

	// Try to get from cache first
	if cachedValue, found := c.cache.Get(ctx, cacheKey); found {
		if result, ok := c.unmarshalUserListResult(cachedValue); ok {
			return result.Users, result.Total, nil
		}
	}

	// Cache miss or error, get from database
	users, total, err := c.repo.GetAll(offset, limit)
	if err != nil {
		return nil, 0, err
	}

	// Cache the result
	result := UserListResult{
		Users: users,
		Total: total,
	}
	if err := c.cache.Set(ctx, cacheKey, result, c.ttl); err != nil {
		// Log error but don't fail the operation
	}

	return users, total, nil
}

// Update updates a user and invalidates relevant cache entries
func (c *CachedUserRepository) Update(user *models.User) error {
	err := c.repo.Update(user)
	if err != nil {
		return err
	}

	// Invalidate cache entries that might be affected
	ctx := context.Background()
	c.invalidateUserCache(ctx, user)

	return nil
}

// Delete soft deletes a user and invalidates relevant cache entries
func (c *CachedUserRepository) Delete(id string) error {
	// Get the user before deletion to invalidate proper cache keys
	user, err := c.repo.GetByID(id)
	if err != nil {
		// Continue with deletion even if we can't get the user
		user = &models.User{ID: id}
	}

	err = c.repo.Delete(id)
	if err != nil {
		return err
	}

	// Invalidate cache entries that might be affected
	ctx := context.Background()
	c.invalidateUserCache(ctx, user)

	return nil
}

// UpdateLastLogin updates the last login time for a user and invalidates cache
func (c *CachedUserRepository) UpdateLastLogin(id string) error {
	err := c.repo.UpdateLastLogin(id)
	if err != nil {
		return err
	}

	// Invalidate cache entries that might be affected
	ctx := context.Background()
	c.invalidateUserCacheByID(ctx, id)

	return nil
}

// ExistsByEmail checks if a user exists by email with caching
func (c *CachedUserRepository) ExistsByEmail(email string) (bool, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:exists:email:%s", email)

	// Try to get from cache first
	if cachedValue, found := c.cache.Get(ctx, cacheKey); found {
		if exists, ok := cachedValue.(bool); ok {
			return exists, nil
		}
	}

	// Cache miss or error, get from database
	exists, err := c.repo.ExistsByEmail(email)
	if err != nil {
		return false, err
	}

	// Cache the result
	if err := c.cache.Set(ctx, cacheKey, exists, c.ttl); err != nil {
		// Log error but don't fail the operation
	}

	return exists, nil
}

// ExistsByUsername checks if a user exists by username with caching
func (c *CachedUserRepository) ExistsByUsername(username string) (bool, error) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:exists:username:%s", username)

	// Try to get from cache first
	if cachedValue, found := c.cache.Get(ctx, cacheKey); found {
		if exists, ok := cachedValue.(bool); ok {
			return exists, nil
		}
	}

	// Cache miss or error, get from database
	exists, err := c.repo.ExistsByUsername(username)
	if err != nil {
		return false, err
	}

	// Cache the result
	if err := c.cache.Set(ctx, cacheKey, exists, c.ttl); err != nil {
		// Log error but don't fail the operation
	}

	return exists, nil
}

// Count returns the total number of active users with caching
func (c *CachedUserRepository) Count() (int64, error) {
	ctx := context.Background()
	cacheKey := "users:count"

	// Try to get from cache first
	if cachedValue, found := c.cache.Get(ctx, cacheKey); found {
		if count, ok := cachedValue.(int64); ok {
			return count, nil
		}
	}

	// Cache miss or error, get from database
	count, err := c.repo.Count()
	if err != nil {
		return 0, err
	}

	// Cache the result
	if err := c.cache.Set(ctx, cacheKey, count, c.ttl); err != nil {
		// Log error but don't fail the operation
	}

	return count, nil
}

// invalidateUserCache invalidates all cache entries related to a user
func (c *CachedUserRepository) invalidateUserCache(ctx context.Context, user *models.User) {
	if user == nil {
		return
	}

	// Invalidate user-specific caches
	keys := []string{
		fmt.Sprintf("user:id:%s", user.ID),
		fmt.Sprintf("user:email:%s", user.Email),
		fmt.Sprintf("user:username:%s", user.Username),
		fmt.Sprintf("user:exists:email:%s", user.Email),
		fmt.Sprintf("user:exists:username:%s", user.Username),
	}

	// Delete keys in batch
	if err := c.cache.DeleteMultiple(ctx, keys); err != nil {
		// Log error but don't fail the operation
	}

	// Invalidate list caches (they might contain this user)
	c.invalidateUserListCaches(ctx)
}

// invalidateUserCacheByID invalidates cache entries by user ID
func (c *CachedUserRepository) invalidateUserCacheByID(ctx context.Context, id string) {
	// Invalidate by ID
	if err := c.cache.Delete(ctx, fmt.Sprintf("user:id:%s", id)); err != nil {
		// Log error but don't fail the operation
	}

	// Invalidate list caches as they might be affected
	c.invalidateUserListCaches(ctx)
}

// invalidateUserListCaches invalidates all user list caches
func (c *CachedUserRepository) invalidateUserListCaches(ctx context.Context) {
	// Get all keys matching user list patterns
	patterns := []string{"users:all:*", "users:count"}

	for _, pattern := range patterns {
		keys, err := c.cache.Keys(ctx, pattern)
		if err != nil {
			// Log error but continue
			continue
		}

		if len(keys) > 0 {
			if err := c.cache.DeleteMultiple(ctx, keys); err != nil {
				// Log error but continue
			}
		}
	}
}

// unmarshalUser attempts to unmarshal a cached value to a User model
func (c *CachedUserRepository) unmarshalUser(value interface{}) (*models.User, bool) {
	if value == nil {
		return nil, false
	}

	// Try to unmarshal as JSON
	switch v := value.(type) {
	case []byte:
		var user models.User
		if err := json.Unmarshal(v, &user); err == nil {
			return &user, true
		}
	case string:
		var user models.User
		if err := json.Unmarshal([]byte(v), &user); err == nil {
			return &user, true
		}
	}

	return nil, false
}

// unmarshalUserListResult attempts to unmarshal a cached value to a UserListResult
func (c *CachedUserRepository) unmarshalUserListResult(value interface{}) (UserListResult, bool) {
	var result UserListResult

	if value == nil {
		return result, false
	}

	// Try to unmarshal as JSON
	switch v := value.(type) {
	case []byte:
		if err := json.Unmarshal(v, &result); err == nil {
			return result, true
		}
	case string:
		if err := json.Unmarshal([]byte(v), &result); err == nil {
			return result, true
		}
	}

	return result, false
}

// UserListResult represents the result of GetAll operation for caching
type UserListResult struct {
	Users []*models.User `json:"users"`
	Total int64          `json:"total"`
}

package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go-server/internal/models"
	"go-server/pkg/cache"

	"gorm.io/gorm"
)

// UserRepository defines the interface for user database operations
type UserRepository interface {
	Create(user *models.User) error
	GetByID(id string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	GetAll(offset, limit int) ([]*models.User, int64, error)
	Update(user *models.User) error
	Delete(id string) error
	UpdateLastLogin(id string) error
	ExistsByEmail(email string) (bool, error)
	ExistsByUsername(username string) (bool, error)
	Count() (int64, error)
}

type userRepository struct {
	db    *gorm.DB
	cache cache.Cache
	ttl   time.Duration
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{
		db:    db,
		cache: nil,
		ttl:   0,
	}
}

// NewUserRepositoryWithCache creates a new user repository with cache support
func NewUserRepositoryWithCache(db *gorm.DB, cache cache.Cache, ttl time.Duration) UserRepository {
	return &userRepository{
		db:    db,
		cache: cache,
		ttl:   ttl,
	}
}

// Create creates a new user
func (r *userRepository) Create(user *models.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Invalidate cache entries that might be affected by user creation
	if r.cache != nil {
		// Invalidate list caches and existence checks
		ctx := context.Background()
		r.invalidateUserListCaches(ctx)

		// Invalidate existence checks for this user's email and username
		keys := []string{
			fmt.Sprintf("user:exists:email:%s", user.Email),
			fmt.Sprintf("user:exists:username:%s", user.Username),
		}
		if err := r.cache.DeleteMultiple(ctx, keys); err != nil {
			// Log error but don't fail the operation
		}
	}

	return nil
}

// GetByID gets a user by ID
func (r *userRepository) GetByID(id string) (*models.User, error) {
	// Try cache first if available
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:id:%s", id)
		if user, found := r.getUserFromCache(cacheKey); found {
			return user, nil
		}
	}

	// Cache miss or no cache available, get from database
	var user models.User
	err := r.db.Where("id = ? AND is_active = ?", id, true).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Cache the result if cache is available
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:id:%s", id)
		r.setUserCache(cacheKey, &user)
	}

	return &user, nil
}

// GetByEmail gets a user by email
func (r *userRepository) GetByEmail(email string) (*models.User, error) {
	// Try cache first if available
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:email:%s", email)
		if user, found := r.getUserFromCache(cacheKey); found {
			return user, nil
		}
	}

	// Cache miss or no cache available, get from database
	var user models.User
	err := r.db.Where("email = ? AND is_active = ?", email, true).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Cache the result if cache is available
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:email:%s", email)
		r.setUserCache(cacheKey, &user)
	}

	return &user, nil
}

// GetByUsername gets a user by username
func (r *userRepository) GetByUsername(username string) (*models.User, error) {
	// Try cache first if available
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:username:%s", username)
		if user, found := r.getUserFromCache(cacheKey); found {
			return user, nil
		}
	}

	// Cache miss or no cache available, get from database
	var user models.User
	err := r.db.Where("username = ? AND is_active = ?", username, true).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Cache the result if cache is available
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:username:%s", username)
		r.setUserCache(cacheKey, &user)
	}

	return &user, nil
}

// GetAll gets all users with pagination
func (r *userRepository) GetAll(offset, limit int) ([]*models.User, int64, error) {
	// Try cache first if available
	if r.cache != nil {
		ctx := context.Background()
		cacheKey := fmt.Sprintf("users:all:%d:%d", offset, limit)

		if cachedValue, found := r.cache.Get(ctx, cacheKey); found {
			if result, ok := r.unmarshalUserListResult(cachedValue); ok {
				return result.Users, result.Total, nil
			}
		}
	}

	// Cache miss or no cache available, get from database
	var users []*models.User
	var total int64

	// Get total count
	if err := r.db.Model(&models.User{}).Where("is_active = ?", true).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// Get users with pagination
	err := r.db.Where("is_active = ?", true).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&users).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get users: %w", err)
	}

	// Cache the result if cache is available
	if r.cache != nil {
		ctx := context.Background()
		cacheKey := fmt.Sprintf("users:all:%d:%d", offset, limit)
		result := UserListResult{
			Users: users,
			Total: total,
		}
		if err := r.cache.Set(ctx, cacheKey, result, r.ttl); err != nil {
			// Log error but don't fail the operation
		}
	}

	return users, total, nil
}

// Update updates a user
func (r *userRepository) Update(user *models.User) error {
	result := r.db.Where("id = ?", user.ID).Updates(user)
	if result.Error != nil {
		return fmt.Errorf("failed to update user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	// Invalidate cache entries that might be affected by user update
	if r.cache != nil {
		r.invalidateUserCache(user)
	}

	return nil
}

// Delete soft deletes a user
func (r *userRepository) Delete(id string) error {
	// Get the user before deletion to invalidate proper cache keys
	var user models.User
	if err := r.db.Where("id = ?", id).First(&user).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to get user for cache invalidation: %w", err)
		}
		// User doesn't exist, but we still need to try deletion
	}

	result := r.db.Where("id = ?", id).Delete(&models.User{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	// Invalidate cache entries that might be affected by user deletion
	if r.cache != nil {
		if user.ID != "" {
			// We have the user data, invalidate all related caches
			r.invalidateUserCache(&user)
		} else {
			// We don't have user data, invalidate by ID at minimum
			r.invalidateUserCacheByID(id)
		}
	}

	return nil
}

// UpdateLastLogin updates the last login time for a user
func (r *userRepository) UpdateLastLogin(id string) error {
	result := r.db.Model(&models.User{}).Where("id = ?", id).Update("last_login", "NOW()")
	if result.Error != nil {
		return fmt.Errorf("failed to update last login: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	// Invalidate cache entries that might be affected by last login update
	if r.cache != nil {
		r.invalidateUserCacheByID(id)
	}

	return nil
}

// ExistsByEmail checks if a user exists by email
func (r *userRepository) ExistsByEmail(email string) (bool, error) {
	// Try cache first if available
	if r.cache != nil {
		ctx := context.Background()
		cacheKey := fmt.Sprintf("user:exists:email:%s", email)

		if cachedValue, found := r.cache.Get(ctx, cacheKey); found {
			if exists, ok := cachedValue.(bool); ok {
				return exists, nil
			}
		}
	}

	// Cache miss or no cache available, get from database
	var count int64
	err := r.db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if user exists by email: %w", err)
	}
	exists := count > 0

	// Cache the result if cache is available
	if r.cache != nil {
		ctx := context.Background()
		cacheKey := fmt.Sprintf("user:exists:email:%s", email)
		if err := r.cache.Set(ctx, cacheKey, exists, r.ttl); err != nil {
			// Log error but don't fail the operation
		}
	}

	return exists, nil
}

// ExistsByUsername checks if a user exists by username
func (r *userRepository) ExistsByUsername(username string) (bool, error) {
	// Try cache first if available
	if r.cache != nil {
		ctx := context.Background()
		cacheKey := fmt.Sprintf("user:exists:username:%s", username)

		if cachedValue, found := r.cache.Get(ctx, cacheKey); found {
			if exists, ok := cachedValue.(bool); ok {
				return exists, nil
			}
		}
	}

	// Cache miss or no cache available, get from database
	var count int64
	err := r.db.Model(&models.User{}).Where("username = ?", username).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if user exists by username: %w", err)
	}
	exists := count > 0

	// Cache the result if cache is available
	if r.cache != nil {
		ctx := context.Background()
		cacheKey := fmt.Sprintf("user:exists:username:%s", username)
		if err := r.cache.Set(ctx, cacheKey, exists, r.ttl); err != nil {
			// Log error but don't fail the operation
		}
	}

	return exists, nil
}

// Count returns the total number of active users
func (r *userRepository) Count() (int64, error) {
	var count int64

	// Try cache first if available
	if r.cache != nil {
		ctx := context.Background()
		cacheKey := "users:count"

		if cachedValue, found := r.cache.Get(ctx, cacheKey); found {
			if cachedCount, ok := cachedValue.(int64); ok {
				return cachedCount, nil
			}
		}

		// Cache miss, get from database
		err := r.db.Model(&models.User{}).Where("is_active = ?", true).Count(&count).Error
		if err != nil {
			return 0, fmt.Errorf("failed to count users: %w", err)
		}

		// Cache the result
		if err := r.cache.Set(ctx, cacheKey, count, r.ttl); err != nil {
			// Log error but don't fail the operation
		}

		return count, nil
	}

	// No cache available, get from database directly
	err := r.db.Model(&models.User{}).Where("is_active = ?", true).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

// Helper methods for cache operations

// generateUserCacheKeys generates all possible cache keys for a user
func (r *userRepository) generateUserCacheKeys(user *models.User) []string {
	if user == nil {
		return []string{}
	}

	return []string{
		fmt.Sprintf("user:id:%s", user.ID),
		fmt.Sprintf("user:email:%s", user.Email),
		fmt.Sprintf("user:username:%s", user.Username),
		fmt.Sprintf("user:exists:email:%s", user.Email),
		fmt.Sprintf("user:exists:username:%s", user.Username),
	}
}

// invalidateUserCache invalidates all cache entries related to a user
func (r *userRepository) invalidateUserCache(user *models.User) {
	if r.cache == nil || user == nil {
		return
	}

	ctx := context.Background()
	keys := r.generateUserCacheKeys(user)

	// Delete user-specific cache keys
	if err := r.cache.DeleteMultiple(ctx, keys); err != nil {
		// Log error but don't fail the operation
	}

	// Invalidate list caches
	r.invalidateUserListCaches(ctx)
}

// invalidateUserCacheByID invalidates cache entries by user ID
func (r *userRepository) invalidateUserCacheByID(id string) {
	if r.cache == nil || id == "" {
		return
	}

	ctx := context.Background()

	// Delete user-specific cache by ID
	if err := r.cache.Delete(ctx, fmt.Sprintf("user:id:%s", id)); err != nil {
		// Log error but don't fail the operation
	}

	// Invalidate list caches
	r.invalidateUserListCaches(ctx)
}

// invalidateUserListCaches invalidates all user list caches
func (r *userRepository) invalidateUserListCaches(ctx context.Context) {
	if r.cache == nil {
		return
	}

	// Invalidate list caches with common patterns
	patterns := []string{"users:all:*", "users:count"}

	for _, pattern := range patterns {
		keys, err := r.cache.Keys(ctx, pattern)
		if err != nil {
			// Log error but continue
			continue
		}

		if len(keys) > 0 {
			if err := r.cache.DeleteMultiple(ctx, keys); err != nil {
				// Log error but continue
			}
		}
	}
}

// getUserFromCache attempts to get a user from cache
func (r *userRepository) getUserFromCache(cacheKey string) (*models.User, bool) {
	if r.cache == nil {
		return nil, false
	}

	ctx := context.Background()
	cachedValue, found := r.cache.Get(ctx, cacheKey)
	if !found {
		return nil, false
	}

	return r.unmarshalUser(cachedValue)
}

// setUserCache sets a user in cache
func (r *userRepository) setUserCache(cacheKey string, user *models.User) {
	if r.cache == nil || user == nil {
		return
	}

	ctx := context.Background()
	if err := r.cache.Set(ctx, cacheKey, user, r.ttl); err != nil {
		// Log error but don't fail the operation
	}
}

// unmarshalUser attempts to unmarshal a cached value to a User model
func (r *userRepository) unmarshalUser(value interface{}) (*models.User, bool) {
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
	case *models.User:
		return v, true
	}

	return nil, false
}

// unmarshalUserListResult attempts to unmarshal a cached value to a UserListResult
func (r *userRepository) unmarshalUserListResult(value interface{}) (UserListResult, bool) {
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
	case UserListResult:
		return v, true
	}

	return result, false
}

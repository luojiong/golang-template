package cache_manager

import (
	"context"
	"fmt"
	"time"

	"go-server/internal/models"
	"go-server/pkg/cache"
)

// Manager 统一的缓存管理器接口
type Manager interface {
	// 用户相关缓存操作
	InvalidateUserCache(userID string) error
	InvalidateUserListCache() error
	GetUserFromCache(key string) (*models.User, bool)
	SetUserCache(key string, user *models.User, ttl time.Duration) error
	GetUserListFromCache(key string) ([]*models.User, int64, bool)
	SetUserListCache(key string, users []*models.User, total int64, ttl time.Duration) error

	// 通用缓存操作
	InvalidateByPattern(pattern string) error
	InvalidateMultiple(keys []string) error
	Get(ctx context.Context, key string) (interface{}, bool)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// manager 缓存管理器实现
type manager struct {
	cache cache.Cache
	ttl   time.Duration
}

// NewManager 创建新的缓存管理器
func NewManager(cache cache.Cache, defaultTTL time.Duration) Manager {
	return &manager{
		cache: cache,
		ttl:   defaultTTL,
	}
}

// InvalidateUserCache 失效用户相关的所有缓存
func (m *manager) InvalidateUserCache(userID string) error {
	if m.cache == nil {
		return nil
	}

	// ctx := context.Background() // 预留给后续使用

	// 这里需要先获取用户信息来构建所有相关的缓存键
	// 在实际使用中，可以通过服务层传入用户信息来避免额外查询
	patterns := []string{
		fmt.Sprintf("user:id:%s", userID),
		"user:*", // 失效所有用户相关缓存
	}

	for _, pattern := range patterns {
		if err := m.InvalidateByPattern(pattern); err != nil {
			return fmt.Errorf("failed to invalidate cache pattern %s: %w", pattern, err)
		}
	}

	return nil
}

// InvalidateUserListCache 失效用户列表缓存
func (m *manager) InvalidateUserListCache() error {
	if m.cache == nil {
		return nil
	}

	patterns := []string{
		"users:all:*",
		"users:count",
	}

	for _, pattern := range patterns {
		if err := m.InvalidateByPattern(pattern); err != nil {
			return fmt.Errorf("failed to invalidate user list cache pattern %s: %w", pattern, err)
		}
	}

	return nil
}

// GetUserFromCache 从缓存获取用户信息
func (m *manager) GetUserFromCache(key string) (*models.User, bool) {
	if m.cache == nil {
		return nil, false
	}

	ctx := context.Background()
	cachedValue, found := m.cache.Get(ctx, key)
	if !found {
		return nil, false
	}

	return m.unmarshalUser(cachedValue)
}

// SetUserCache 设置用户信息到缓存
func (m *manager) SetUserCache(key string, user *models.User, ttl time.Duration) error {
	if m.cache == nil || user == nil {
		return nil
	}

	if ttl <= 0 {
		ttl = m.ttl
	}

	ctx := context.Background()
	return m.cache.Set(ctx, key, user, ttl)
}

// GetUserListFromCache 从缓存获取用户列表
func (m *manager) GetUserListFromCache(key string) ([]*models.User, int64, bool) {
	if m.cache == nil {
		return nil, 0, false
	}

	ctx := context.Background()
	cachedValue, found := m.cache.Get(ctx, key)
	if !found {
		return nil, 0, false
	}

	return m.unmarshalUserListResult(cachedValue)
}

// SetUserListCache 设置用户列表到缓存
func (m *manager) SetUserListCache(key string, users []*models.User, total int64, ttl time.Duration) error {
	if m.cache == nil {
		return nil
	}

	if ttl <= 0 {
		ttl = m.ttl
	}

	ctx := context.Background()
	result := UserListResult{
		Users: users,
		Total: total,
	}
	return m.cache.Set(ctx, key, result, ttl)
}

// InvalidateByPattern 按模式失效缓存
func (m *manager) InvalidateByPattern(pattern string) error {
	if m.cache == nil {
		return nil
	}

	ctx := context.Background()
	keys, err := m.cache.Keys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to get cache keys for pattern %s: %w", pattern, err)
	}

	if len(keys) > 0 {
		return m.InvalidateMultiple(keys)
	}

	return nil
}

// InvalidateMultiple 批量失效缓存
func (m *manager) InvalidateMultiple(keys []string) error {
	if m.cache == nil || len(keys) == 0 {
		return nil
	}

	ctx := context.Background()
	return m.cache.DeleteMultiple(ctx, keys)
}

// Get 获取缓存值
func (m *manager) Get(ctx context.Context, key string) (interface{}, bool) {
	if m.cache == nil {
		return nil, false
	}
	return m.cache.Get(ctx, key)
}

// Set 设置缓存值
func (m *manager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if m.cache == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = m.ttl
	}
	return m.cache.Set(ctx, key, value, ttl)
}

// Delete 删除缓存
func (m *manager) Delete(ctx context.Context, key string) error {
	if m.cache == nil {
		return nil
	}
	return m.cache.Delete(ctx, key)
}

// Helper methods

// UserListResult 用户列表缓存结果
type UserListResult struct {
	Users []*models.User `json:"users"`
	Total int64          `json:"total"`
}

// unmarshalUser 反序列化用户对象
func (m *manager) unmarshalUser(value interface{}) (*models.User, bool) {
	if value == nil {
		return nil, false
	}

	switch v := value.(type) {
	case *models.User:
		return v, true
	default:
		return nil, false
	}
}

// unmarshalUserListResult 反序列化用户列表结果
func (m *manager) unmarshalUserListResult(value interface{}) ([]*models.User, int64, bool) {
	if value == nil {
		return nil, 0, false
	}

	switch v := value.(type) {
	case UserListResult:
		return v.Users, v.Total, true
	default:
		return nil, 0, false
	}
}

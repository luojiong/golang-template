package cache

import (
	"context"
	"time"
)

// Cache 定义缓存操作接口
type Cache interface {
	// Get 从缓存中检索值
	// 返回值和一个布尔值，指示是否找到键
	Get(ctx context.Context, key string) (interface{}, bool)

	// GetWithTTL 从缓存中检索值及其剩余TTL
	// 返回值、剩余生存时间以及一个布尔值，指示是否找到键
	GetWithTTL(ctx context.Context, key string) (interface{}, time.Duration, bool)

	// 在缓存中存储值，可选TTL
	// 如果ttl为0，项目将不会过期
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// SetMultiple 在缓存中存储多个键值对
	// 如果指定，所有项目将具有相同的TTL
	SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error

	// Delete 从缓存中删除键
	Delete(ctx context.Context, key string) error

	// DeleteMultiple 从缓存中删除多个键
	DeleteMultiple(ctx context.Context, keys []string) error

	// Exists 检查键是否存在于缓存中
	Exists(ctx context.Context, key string) (bool, error)

	// Clear 从缓存中删除所有键
	Clear(ctx context.Context) error

	// Keys 返回匹配模式的所有键
	// 使用"*"表示所有键
	Keys(ctx context.Context, pattern string) ([]string, error)

	// GetMultiple 从缓存中检索多个值
	// 返回找到的键的键值对映射
	GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error)

	// SetIfNotExists 仅在键不存在时设置值
	// 如果设置了值返回true，如果键已存在返回false
	SetIfNotExists(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)

	// Increment 按给定数量递增键的数值
	// 返回递增后的新值
	Increment(ctx context.Context, key string, amount int64) (int64, error)

	// Decrement 按给定数量递减键的数值
	// 返回递减后的新值
	Decrement(ctx context.Context, key string, amount int64) (int64, error)

	// Close 关闭缓存连接并清理资源
	Close() error

	// Health 检查缓存连接的健康状况
	// 如果缓存不健康，返回错误
	Health(ctx context.Context) error

	// GetStats 返回缓存统计信息和健康指标
	// 返回有关缓存性能和使用的详细信息
	GetStats(ctx context.Context) (map[string]interface{}, error)
}
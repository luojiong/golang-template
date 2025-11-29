package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

// RedisCache 使用 Redis 实现 Cache 接口
type RedisCache struct {
	client *redis.Client
	prefix string
}

// RedisConfig 保存 Redis 连接的配置
type RedisConfig struct {
	Host         string
	Port         int
	Password     string
	DB           int
	Prefix       string
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolTimeout  time.Duration
}

// DefaultRedisConfig 返回 Redis 的默认配置
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Host:         "localhost",
		Port:         6379,
		Password:     "",
		DB:           0,
		Prefix:       "cache:",
		PoolSize:     10,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	}
}

// NewRedisCache 创建新的 Redis 缓存实例
func NewRedisCache(config *RedisConfig) (Cache, error) {
	if config == nil {
		config = DefaultRedisConfig()
	}

    rdb := redis.NewClient(&redis.Options{
        Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
        Password:     config.Password,
        DB:           config.DB,
        PoolSize:     config.PoolSize,
        MinIdleConns: config.MinIdleConns,
        DialTimeout:  config.DialTimeout,
        ReadTimeout:  config.ReadTimeout,
        WriteTimeout: config.WriteTimeout,
        PoolTimeout:  config.PoolTimeout,
        MaintNotificationsConfig: &maintnotifications.Config{ // disable unsupported Redis Cloud feature
            Mode: maintnotifications.ModeDisabled,
        },
    })

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: rdb,
		prefix: config.Prefix,
	}, nil
}

// NewRedisCacheWithClient 使用现有客户端创建新的 Redis 缓存实例
func NewRedisCacheWithClient(client *redis.Client, prefix string) Cache {
	if prefix == "" {
		prefix = "cache:"
	}
	return &RedisCache{
		client: client,
		prefix: prefix,
	}
}

// getKey 返回带前缀的完整键名
func (r *RedisCache) getKey(key string) string {
	return r.prefix + key
}

// Get 从缓存中检索值
func (r *RedisCache) Get(ctx context.Context, key string) (interface{}, bool) {
	result, err := r.client.Get(ctx, r.getKey(key)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, false
		}
		// 对于其他错误，我们返回 false 但可以记录错误日志
		return nil, false
	}

	// 首先尝试将结果解析为 JSON
	var value interface{}
	if err := json.Unmarshal([]byte(result), &value); err != nil {
		// 如果不是 JSON，则作为字符串返回
		value = result
	}

	return value, true
}

// GetWithTTL 从缓存中检索值及其剩余 TTL
func (r *RedisCache) GetWithTTL(ctx context.Context, key string) (interface{}, time.Duration, bool) {
	// 使用管道同时获取值和 TTL
	pipe := r.client.Pipeline()
	valueCmd := pipe.Get(ctx, r.getKey(key))
	ttlCmd := pipe.TTL(ctx, r.getKey(key))

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, 0, false
	}

	result, err := valueCmd.Result()
	if err != nil {
		if err == redis.Nil {
			return nil, 0, false
		}
		return nil, 0, false
	}

	ttl, _ := ttlCmd.Result()
	if ttl == -1 {
		ttl = 0 // 无过期时间
	}

	// 首先尝试将结果解析为 JSON
	var value interface{}
	if err := json.Unmarshal([]byte(result), &value); err != nil {
		// 如果不是 JSON，则作为字符串返回
		value = result
	}

	return value, ttl, true
}

// Set 在缓存中存储值，可选 TTL
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var data []byte
	var err error

	// 尝试将值序列化为 JSON
	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		data, err = json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal value: %w", err)
		}
	}

	if ttl > 0 {
		return r.client.Set(ctx, r.getKey(key), data, ttl).Err()
	}
	return r.client.Set(ctx, r.getKey(key), data, 0).Err()
}

// SetMultiple 在缓存中存储多个键值对
func (r *RedisCache) SetMultiple(ctx context.Context, items map[string]interface{}, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	pipe := r.client.Pipeline()
	for key, value := range items {
		var data []byte
		var err error

		switch v := value.(type) {
		case string:
			data = []byte(v)
		case []byte:
			data = v
		default:
			data, err = json.Marshal(value)
			if err != nil {
				return fmt.Errorf("failed to marshal value for key %s: %w", key, err)
			}
		}

		if ttl > 0 {
			pipe.Set(ctx, r.getKey(key), data, ttl)
		} else {
			pipe.Set(ctx, r.getKey(key), data, 0)
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

// Delete 从缓存中删除键
func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, r.getKey(key)).Err()
}

// DeleteMultiple 从缓存中删除多个键
func (r *RedisCache) DeleteMultiple(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	redisKeys := make([]string, len(keys))
	for i, key := range keys {
		redisKeys[i] = r.getKey(key)
	}

	return r.client.Del(ctx, redisKeys...).Err()
}

// Exists 检查键是否存在于缓存中
func (r *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := r.client.Exists(ctx, r.getKey(key)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check if key exists: %w", err)
	}
	return result > 0, nil
}

// Clear 从缓存中删除所有带配置前缀的键
func (r *RedisCache) Clear(ctx context.Context) error {
	pattern := r.prefix + "*"
	keys, err := r.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys for clearing: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	return r.client.Del(ctx, keys...).Err()
}

// Keys 返回匹配模式的所有键
func (r *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	redisPattern := r.prefix + pattern
	keys, err := r.client.Keys(ctx, redisPattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys matching pattern %s: %w", pattern, err)
	}

	// 从返回的键中移除前缀
	result := make([]string, len(keys))
	for i, key := range keys {
		result[i] = key[len(r.prefix):]
	}

	return result, nil
}

// GetMultiple 从缓存中检索多个值
func (r *RedisCache) GetMultiple(ctx context.Context, keys []string) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	redisKeys := make([]string, len(keys))
	for i, key := range keys {
		redisKeys[i] = r.getKey(key)
	}

	results := make(map[string]interface{})

	// 使用管道以获得更好的性能
	pipe := r.client.Pipeline()
	cmds := make(map[string]*redis.StringCmd)

	for i, key := range keys {
		cmds[key] = pipe.Get(ctx, redisKeys[i])
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	for key, cmd := range cmds {
		result, err := cmd.Result()
		if err != nil {
			if err != redis.Nil {
				// 记录错误但继续处理其他键
				continue
			}
			// 键不存在，跳过它
			continue
		}

		// 首先尝试将结果解析为 JSON
		var value interface{}
		if err := json.Unmarshal([]byte(result), &value); err != nil {
			// 如果不是 JSON，则作为字符串返回
			value = result
		}

		results[key] = value
	}

	return results, nil
}

// SetIfNotExists 仅在键不存在时设置值
func (r *RedisCache) SetIfNotExists(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	var data []byte
	var err error

	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		data, err = json.Marshal(value)
		if err != nil {
			return false, fmt.Errorf("failed to marshal value: %w", err)
		}
	}

	var success bool
	if ttl > 0 {
		success, err = r.client.SetNX(ctx, r.getKey(key), data, ttl).Result()
	} else {
		success, err = r.client.SetNX(ctx, r.getKey(key), data, 0).Result()
	}

	if err != nil {
		return false, fmt.Errorf("failed to set if not exists: %w", err)
	}

	return success, nil
}

// Increment 按给定数量递增键的数值
func (r *RedisCache) Increment(ctx context.Context, key string, amount int64) (int64, error) {
	result, err := r.client.IncrBy(ctx, r.getKey(key), amount).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment key %s: %w", key, err)
	}
	return result, nil
}

// Decrement 按给定数量递减键的数值
func (r *RedisCache) Decrement(ctx context.Context, key string, amount int64) (int64, error) {
	result, err := r.client.DecrBy(ctx, r.getKey(key), amount).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to decrement key %s: %w", key, err)
	}
	return result, nil
}

// Close 关闭缓存连接并清理资源
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// Health 检查 Redis 缓存连接的健康状况
func (r *RedisCache) Health(ctx context.Context) error {
	// 使用 ping 测试连接
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	// 测试读写能力
	testKey := "health_check_test"
	testValue := "test_value"

	if err := r.client.Set(ctx, testKey, testValue, 5*time.Second).Err(); err != nil {
		return fmt.Errorf("redis write test failed: %w", err)
	}

	if result, err := r.client.Get(ctx, testKey).Result(); err != nil {
		return fmt.Errorf("redis read test failed: %w", err)
	} else if result != testValue {
		return fmt.Errorf("redis data integrity test failed: expected %s, got %s", testValue, result)
	}

	// 清理测试键
	r.client.Del(ctx, testKey)

	return nil
}

// GetStats 返回 Redis 缓存统计信息和健康指标
func (r *RedisCache) GetStats(ctx context.Context) (map[string]interface{}, error) {
	// 获取 Redis 信息
	info, err := r.client.Info(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get redis info: %w", err)
	}

	// 解析 Redis 信息以获取关键指标
	stats := map[string]interface{}{
		"server_info": parseRedisInfo(info),
	}

	// 获取连接池统计信息
	poolStats := r.client.PoolStats()
	stats["connection_pool"] = map[string]interface{}{
		"hits":         poolStats.Hits,
		"misses":       poolStats.Misses,
		"total_conns":  poolStats.TotalConns,
		"idle_conns":   poolStats.IdleConns,
		"stale_conns":  poolStats.StaleConns,
		"hit_rate":     calculateHitRate(poolStats.Hits, poolStats.Misses),
	}

	// 获取内存使用情况
	memoryInfo, err := r.client.Info(ctx, "memory").Result()
	if err == nil {
		stats["memory"] = parseRedisMemoryInfo(memoryInfo)
	}

	// 获取数据库统计信息
	dbStats, err := r.client.Info(ctx, "keyspace", "stats").Result()
	if err == nil {
		stats["database"] = parseRedisDBInfo(dbStats)
	}

	// 测试延迟
	start := time.Now()
	if err := r.client.Ping(ctx).Err(); err == nil {
		stats["latency_ms"] = time.Since(start).Milliseconds()
	}

	return stats, nil
}

// parseRedisInfo 解析 Redis INFO 命令的输出
func parseRedisInfo(info string) map[string]interface{} {
	result := make(map[string]interface{})
	lines := strings.Split(info, "\r\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// 尝试解析数值
			if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
				result[key] = intVal
			} else if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
				result[key] = floatVal
			} else {
				result[key] = value
			}
		}
	}

	return result
}

// parseRedisMemoryInfo 解析 Redis INFO 中的内存特定信息
func parseRedisMemoryInfo(info string) map[string]interface{} {
	result := make(map[string]interface{})
	lines := strings.Split(info, "\r\n")

	var usedMemory, maxMemory int64

	for _, line := range lines {
		if strings.HasPrefix(line, "used_memory:") {
			if val, err := strconv.ParseInt(strings.Split(line, ":")[1], 10, 64); err == nil {
				usedMemory = val
			}
		} else if strings.HasPrefix(line, "maxmemory:") {
			if val, err := strconv.ParseInt(strings.Split(line, ":")[1], 10, 64); err == nil {
				maxMemory = val
			}
		}
	}

	result["used_memory_bytes"] = usedMemory
	result["used_memory_mb"] = usedMemory / (1024 * 1024)
	result["max_memory_bytes"] = maxMemory
	result["max_memory_mb"] = maxMemory / (1024 * 1024)

	if maxMemory > 0 {
		result["memory_usage_percent"] = float64(usedMemory) / float64(maxMemory) * 100
	} else {
		result["memory_usage_percent"] = 0
	}

	return result
}

// parseRedisDBInfo 解析 Redis INFO 中的数据库特定信息
func parseRedisDBInfo(info string) map[string]interface{} {
	result := make(map[string]interface{})
	lines := strings.Split(info, "\r\n")

	var totalKeys, totalCommands, totalConnections int64
	var hitRate float64

	for _, line := range lines {
		if strings.Contains(line, "keys=") {
			for _, part := range strings.Split(line, ":") {
				if strings.Contains(part, "keys=") {
					if val, err := strconv.ParseInt(strings.Split(part, "=")[1], 10, 64); err == nil {
						totalKeys += val
					}
				}
			}
		} else if strings.HasPrefix(line, "total_commands_processed:") {
			if val, err := strconv.ParseInt(strings.Split(line, ":")[1], 10, 64); err == nil {
				totalCommands = val
			}
		} else if strings.HasPrefix(line, "total_connections_received:") {
			if val, err := strconv.ParseInt(strings.Split(line, ":")[1], 10, 64); err == nil {
				totalConnections = val
			}
		} else if strings.HasPrefix(line, "keyspace_hits:") || strings.HasPrefix(line, "keyspace_misses:") {
			// 从键空间统计信息计算命中率
			if strings.HasPrefix(line, "keyspace_hits:") {
				if hits, err := strconv.ParseInt(strings.Split(line, ":")[1], 10, 64); err == nil {
					result["keyspace_hits"] = hits
				}
			} else if strings.HasPrefix(line, "keyspace_misses:") {
				if misses, err := strconv.ParseInt(strings.Split(line, ":")[1], 10, 64); err == nil {
					result["keyspace_misses"] = misses
				}
			}
		}
	}

	// 如果同时有命中和未命中次数，则计算命中率
	if hits, ok := result["keyspace_hits"].(int64); ok {
		if misses, ok := result["keyspace_misses"].(int64); ok {
			total := hits + misses
			if total > 0 {
				hitRate = float64(hits) / float64(total) * 100
			}
		}
	}

	result["total_keys"] = totalKeys
	result["total_commands_processed"] = totalCommands
	result["total_connections_received"] = totalConnections
	result["keyspace_hit_rate_percent"] = hitRate

	return result
}

// calculateHitRate 计算命中率百分比
func calculateHitRate(hits, misses uint32) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total) * 100
}

// GetClient 返回底层的 Redis 客户端用于高级操作
// 这是一个可选方法，提供对原始客户端的访问
func (r *RedisCache) GetClient() *redis.Client {
	return r.client
}

// SetTTL 为现有键设置或更新 TTL
func (r *RedisCache) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	return r.client.Expire(ctx, r.getKey(key), ttl).Err()
}

// GetTTL 返回键的剩余生存时间
func (r *RedisCache) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := r.client.TTL(ctx, r.getKey(key)).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get TTL for key %s: %w", key, err)
	}
	if ttl == -1 {
		return 0, nil // 无过期时间
	}
	return ttl, nil
}

// 辅助函数：将 interface{} 转换为字符串用于 Redis 存储
func interfaceToString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%f", v), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		// 尝试将值序列化为 JSON
		data, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal value: %w", err)
		}
		return string(data), nil
	}
}

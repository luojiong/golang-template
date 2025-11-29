package middleware

import (
    "context"
    "fmt"
    "net/http"
    "strconv"
    "sync"
    "time"

    "go-server/internal/config"
    "go-server/pkg/response"

    "github.com/gin-gonic/gin"
    "github.com/redis/go-redis/v9"
    "github.com/redis/go-redis/v9/maintnotifications"
)

// RateLimiterConfig 速率限制器配置
type RateLimiterConfig struct {
	// Redis 配置
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	RedisPoolSize int

	// 速率限制配置
	AnonymousRequests     int           // 匿名用户请求限制 (100/分钟)
	AuthenticatedRequests int           // 认证用户请求限制 (200/分钟)
	WindowDuration        time.Duration // 时间窗口 (1分钟)

	// Redis 键前缀
	KeyPrefix string

	// 降级配置
	FallbackEnabled bool // 是否启用内存降级
}

// MemoryRateLimiter 内存速率限制器（用于 Redis 不可用时的降级）
type MemoryRateLimiter struct {
	mu      sync.RWMutex
	clients map[string][]time.Time // 客户端IP -> 请求时间列表
	window  time.Duration          // 时间窗口
	maxReqs int                    // 最大请求数
}

// DistributedRateLimiter 分布式速率限制器
type DistributedRateLimiter struct {
	config    RateLimiterConfig
	redis     *redis.Client
	fallback  *MemoryRateLimiter
	anonymous *MemoryRateLimiter // 匿名用户内存限制器
}

// NewMemoryRateLimiter 创建内存速率限制器
func NewMemoryRateLimiter(maxReqs int, window time.Duration) *MemoryRateLimiter {
	return &MemoryRateLimiter{
		clients: make(map[string][]time.Time),
		window:  window,
		maxReqs: maxReqs,
	}
}

// isAllowed 检查内存限制是否允许请求
func (m *MemoryRateLimiter) isAllowed(clientID string) (bool, time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-m.window)

	// 获取或创建客户端请求记录
	requests, exists := m.clients[clientID]
	if !exists {
		requests = []time.Time{}
	}

	// 清理过期的请求记录
	var validRequests []time.Time
	for _, reqTime := range requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}

	// 检查是否超过限制
	if m.maxReqs > 0 && len(validRequests) >= m.maxReqs {
		// 计算最早请求的剩余时间
		if len(validRequests) > 0 {
			oldestRequest := validRequests[0]
			retryAfter := oldestRequest.Add(m.window).Sub(now)
			if retryAfter < 0 {
				retryAfter = 0
			}
			return false, retryAfter
		}
		return false, m.window
	}

	// 如果maxReqs为0，直接拒绝所有请求
	if m.maxReqs == 0 {
		return false, m.window
	}

	// 添加当前请求
	validRequests = append(validRequests, now)
	m.clients[clientID] = validRequests

	return true, 0
}

// NewDistributedRateLimiter 创建分布式速率限制器
func NewDistributedRateLimiter(cfg RateLimiterConfig) *DistributedRateLimiter {
    // 创建 Redis 客户端
    rdb := redis.NewClient(&redis.Options{
        Addr:     cfg.RedisAddr,
        Password: cfg.RedisPassword,
        DB:       cfg.RedisDB,
        PoolSize: cfg.RedisPoolSize,
        MaintNotificationsConfig: &maintnotifications.Config{
            Mode: maintnotifications.ModeDisabled,
        },
    })

	// 创建内存降级限制器
	fallback := NewMemoryRateLimiter(cfg.AuthenticatedRequests, cfg.WindowDuration)
	anonymous := NewMemoryRateLimiter(cfg.AnonymousRequests, cfg.WindowDuration)

	return &DistributedRateLimiter{
		config:    cfg,
		redis:     rdb,
		fallback:  fallback,
		anonymous: anonymous,
	}
}

// isAllowedRedis 使用 Redis 滑动窗口检查速率限制
func (r *DistributedRateLimiter) isAllowedRedis(ctx context.Context, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	now := time.Now().Unix()
	windowSeconds := int64(window.Seconds())

	// 使用 Lua 脚本实现原子性的滑动窗口计数器
	luaScript := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		
		-- 移除过期的记录
		redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)
		
		-- 获取当前窗口内的请求数
		local current = redis.call('ZCARD', key)
		
		if current < limit then
			-- 添加当前请求
			redis.call('ZADD', key, now, now)
			-- 设置过期时间
			redis.call('EXPIRE', key, window + 1)
			return {1, 0}
		else
			-- 获取最早的请求时间
			local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
			if #oldest > 0 then
				local oldest_time = tonumber(oldest[2])
				local retry_after = oldest_time + window - now
				if retry_after < 0 then
					retry_after = 0
				end
				return {0, retry_after}
			else
				return {0, window}
			end
		end
	`

	// 执行 Lua 脚本
	result, err := r.redis.Eval(ctx, luaScript, []string{key}, now, windowSeconds, limit).Result()
	if err != nil {
		return false, 0, err
	}

	// 解析结果
	res := result.([]interface{})
	allowed := res[0].(int64) == 1
	retryAfter := time.Duration(res[1].(int64)) * time.Second

	return allowed, retryAfter, nil
}

// isAllowed 检查请求是否被允许
func (r *DistributedRateLimiter) isAllowed(ctx context.Context, clientID string, isAuthenticated bool) (bool, time.Duration) {
	var limit int

	// 根据用户类型设置不同的限制
	if isAuthenticated {
		limit = r.config.AuthenticatedRequests
	} else {
		limit = r.config.AnonymousRequests
	}

	// 构建 Redis 键
	key := fmt.Sprintf("%s:%s", r.config.KeyPrefix, clientID)

	// 尝试使用 Redis 限制器
	allowed, retryAfter, err := r.isAllowedRedis(ctx, key, limit, r.config.WindowDuration)
	if err == nil {
		return allowed, retryAfter
	}

	// Redis 不可用时，使用内存降级限制器
	if r.config.FallbackEnabled {
		if isAuthenticated {
			return r.fallback.isAllowed(clientID)
		} else {
			return r.anonymous.isAllowed(clientID)
		}
	}

	// 如果没有启用降级，则允许请求（fail-open 策略）
	return true, 0
}

// getClientID 获取客户端标识符
func (r *DistributedRateLimiter) getClientID(c *gin.Context) string {
	// 优先使用用户 ID（如果已认证）
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(string); ok {
			return "user:" + id
		}
	}

	// 否则使用 IP 地址
	clientIP := c.ClientIP()
	if clientIP == "" {
		clientIP = c.Request.RemoteAddr
	}
	return "ip:" + clientIP
}

// isUserAuthenticated 检查用户是否已认证
func (r *DistributedRateLimiter) isUserAuthenticated(c *gin.Context) bool {
	_, exists := c.Get("user_id")
	return exists
}

// Close 关闭 Redis 连接
func (r *DistributedRateLimiter) Close() error {
	if r.redis != nil {
		return r.redis.Close()
	}
	return nil
}

// RateLimiterMiddleware 创建速率限制中间件
func RateLimiterMiddleware(cfg *config.Config) gin.HandlerFunc {
	// 解析时间窗口
	windowDuration, err := time.ParseDuration(cfg.RateLimit.Window)
	if err != nil {
		windowDuration = time.Minute // 默认 1 分钟
	}

	// 创建速率限制器配置
	limiterConfig := RateLimiterConfig{
		RedisAddr:             fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		RedisPassword:         cfg.Redis.Password,
		RedisDB:               cfg.Redis.DB,
		RedisPoolSize:         cfg.Redis.PoolSize,
		AnonymousRequests:     cfg.RateLimit.Requests,     // 使用配置中的匿名用户限制
		AuthenticatedRequests: cfg.RateLimit.Requests * 2, // 认证用户是匿名用户的2倍
		WindowDuration:        windowDuration,
		KeyPrefix:             cfg.RateLimit.RedisKey,
		FallbackEnabled:       true, // 启用降级
	}

	// 创建分布式速率限制器
	limiter := NewDistributedRateLimiter(limiterConfig)

	return func(c *gin.Context) {
		// 如果速率限制被禁用，直接通过
		if !cfg.RateLimit.Enabled {
			c.Next()
			return
		}

		// 获取客户端标识和认证状态
		clientID := limiter.getClientID(c)
		isAuthenticated := limiter.isUserAuthenticated(c)

		// 检查速率限制
		allowed, retryAfter := limiter.isAllowed(c.Request.Context(), clientID, isAuthenticated)

		if !allowed {
			// 设置 Retry-After 头
			if retryAfter > 0 {
				c.Header("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			}

			// 返回 429 状态码和错误信息
			message := "请求过于频繁，请稍后再试"
			if isAuthenticated {
				message = "认证用户请求过于频繁，请稍后再试"
			} else {
				message = "匿名用户请求过于频繁，请稍后再试"
			}

			c.JSON(http.StatusTooManyRequests, response.Response{
				Success: false,
				Message: message,
				Error:   nil,
			})
			c.Abort()
			return
		}

		// 设置速率限制相关的响应头
		c.Header("X-RateLimit-Limit", strconv.Itoa(limiterConfig.AuthenticatedRequests))
		if isAuthenticated {
			c.Header("X-RateLimit-Remaining", strconv.Itoa(limiterConfig.AuthenticatedRequests-1))
		} else {
			c.Header("X-RateLimit-Limit", strconv.Itoa(limiterConfig.AnonymousRequests))
			c.Header("X-RateLimit-Remaining", strconv.Itoa(limiterConfig.AnonymousRequests-1))
		}
		c.Header("X-RateLimit-Reset", strconv.Itoa(int(time.Now().Add(limiterConfig.WindowDuration).Unix())))

		c.Next()
	}
}

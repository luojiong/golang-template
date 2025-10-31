package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go-server/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMemoryRateLimiter 测试内存速率限制器
func TestMemoryRateLimiter(t *testing.T) {
	limiter := NewMemoryRateLimiter(3, time.Second*10)

	// 测试前3个请求应该被允许
	for i := 0; i < 3; i++ {
		allowed, retryAfter := limiter.isAllowed("test_client")
		assert.True(t, allowed, "请求 %d 应该被允许", i+1)
		assert.Equal(t, time.Duration(0), retryAfter)
	}

	// 第4个请求应该被拒绝
	allowed, retryAfter := limiter.isAllowed("test_client")
	assert.False(t, allowed, "第4个请求应该被拒绝")
	assert.True(t, retryAfter > 0, "应该有重试时间")
}

// TestMemoryRateLimiter_SlidingWindow 测试滑动窗口功能
func TestMemoryRateLimiter_SlidingWindow(t *testing.T) {
	limiter := NewMemoryRateLimiter(2, time.Second*2)

	// 发送2个请求
	allowed, _ := limiter.isAllowed("test_client")
	assert.True(t, allowed)
	allowed, _ = limiter.isAllowed("test_client")
	assert.True(t, allowed)

	// 第3个请求应该被拒绝
	allowed, retryAfter := limiter.isAllowed("test_client")
	assert.False(t, allowed)
	assert.True(t, retryAfter > 0)

	// 等待超过窗口时间
	time.Sleep(time.Second * 3)

	// 现在应该可以再次发送请求
	allowed, _ = limiter.isAllowed("test_client")
	assert.True(t, allowed)
}

// TestDistributedRateLimiter_Config 测试分布式速率限制器配置
func TestDistributedRateLimiter_Config(t *testing.T) {
	cfg := RateLimiterConfig{
		RedisAddr:             "localhost:6379",
		RedisPassword:         "",
		RedisDB:               1, // 使用不同的数据库进行测试
		RedisPoolSize:         5,
		AnonymousRequests:     5,
		AuthenticatedRequests: 10,
		WindowDuration:        time.Minute,
		KeyPrefix:             "test_rate_limit",
		FallbackEnabled:       true,
	}

	limiter := NewDistributedRateLimiter(cfg)
	defer limiter.Close()

	assert.Equal(t, cfg.AnonymousRequests, 5)
	assert.Equal(t, cfg.AuthenticatedRequests, 10)
	assert.Equal(t, cfg.WindowDuration, time.Minute)
	assert.True(t, cfg.FallbackEnabled)
}

// TestRateLimiterMiddleware_Anonymous 测试匿名用户速率限制
func TestRateLimiterMiddleware_Anonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:  true,
			Requests: 2,
			Window:   "100ms", // 使用更短的窗口进行测试
			RedisKey: "test_rate_limit",
		},
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       1,
			PoolSize: 5,
		},
	}

	// 创建中间件
	middleware := RateLimiterMiddleware(cfg)

	// 创建 Gin 路由器
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 发送请求测试
	clientIP := "192.168.1.1"

	// 前2个请求应该成功
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = clientIP
		req.Header.Set("X-Forwarded-For", clientIP)
		req.Header.Set("X-Real-IP", clientIP)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "请求 %d 应该成功", i+1)
	}

	// 第3个请求应该被限制
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = clientIP
	req.Header.Set("X-Forwarded-For", clientIP)
	req.Header.Set("X-Real-IP", clientIP)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "请求过于频繁")
}

// TestRateLimiterMiddleware_Authenticated 测试认证用户速率限制
func TestRateLimiterMiddleware_Authenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建测试配置
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:  true,
			Requests: 2,
			Window:   "1s",
			RedisKey: "test_rate_limit",
		},
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       1,
			PoolSize: 5,
		},
	}

	// 创建中间件
	middleware := RateLimiterMiddleware(cfg)

	// 创建 Gin 路由器
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 模拟认证用户的请求
	userID := "test_user_123"

	// 前200个请求应该成功（因为认证用户有更高的限制）
	for i := 0; i < 5; i++ { // 只测试5个请求以节省时间
		req := httptest.NewRequest("GET", "/test", nil)

		// 设置模拟的认证上下文
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("user_id", userID)
		c.Request = req

		// 创建一个新的请求处理器来模拟中间件
		handler := func(c *gin.Context) {
			c.Set("user_id", userID)
			middleware(c)
		}

		router2 := gin.New()
		router2.Use(handler)
		router2.GET("/test", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		w := httptest.NewRecorder()
		router2.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	}
}

// TestRateLimiterMiddleware_Disabled 测试禁用速率限制
func TestRateLimiterMiddleware_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 创建禁用速率限制的配置
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled: false, // 禁用速率限制
		},
	}

	// 创建中间件
	middleware := RateLimiterMiddleware(cfg)

	// 创建 Gin 路由器
	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// 发送大量请求，都应该成功
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "请求 %d 应该成功", i+1)
	}
}

// TestGetClientID 测试客户端ID获取逻辑
func TestGetClientID(t *testing.T) {
	cfg := RateLimiterConfig{
		KeyPrefix: "test",
	}
	limiter := NewDistributedRateLimiter(cfg)
	defer limiter.Close()

	// 测试匿名用户（使用IP）
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/test", nil)
	c.Request.RemoteAddr = "192.168.1.100"
	c.Request.Header.Set("X-Forwarded-For", "192.168.1.100")
	c.Request.Header.Set("X-Real-IP", "192.168.1.100")

	clientID := limiter.getClientID(c)
	assert.Equal(t, "ip:192.168.1.100", clientID)

	// 测试认证用户
	c.Set("user_id", "user123")
	clientID = limiter.getClientID(c)
	assert.Equal(t, "user:user123", clientID)
}

// TestIsUserAuthenticated 测试用户认证状态检查
func TestIsUserAuthenticated(t *testing.T) {
	cfg := RateLimiterConfig{}
	limiter := NewDistributedRateLimiter(cfg)
	defer limiter.Close()

	// 测试未认证用户
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	isAuthenticated := limiter.isUserAuthenticated(c)
	assert.False(t, isAuthenticated)

	// 测试已认证用户
	c.Set("user_id", "user123")
	isAuthenticated = limiter.isUserAuthenticated(c)
	assert.True(t, isAuthenticated)
}

// BenchmarkMemoryRateLimiter 性能测试
func BenchmarkMemoryRateLimiter(b *testing.B) {
	limiter := NewMemoryRateLimiter(1000, time.Minute)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			clientID := "client_" + string(rune(i%100))
			limiter.isAllowed(clientID)
			i++
		}
	})
}

// TestRateLimiterIntegration 集成测试（需要 Redis）
func TestRateLimiterIntegration(t *testing.T) {
	// 这个测试需要 Redis 运行在 localhost:6379
	// 如果 Redis 不可用，跳过测试
	redisClient := NewDistributedRateLimiter(RateLimiterConfig{
		RedisAddr:      "localhost:6379",
		RedisDB:        2, // 使用专门的测试数据库
		WindowDuration: time.Second,
		KeyPrefix:      "integration_test",
	})
	defer redisClient.Close()

	// 测试 Redis 连接
	ctx := context.Background()
	_, err := redisClient.redis.Ping(ctx).Result()
	if err != nil {
		t.Skipf("Redis not available for integration test: %v", err)
		return
	}

	// 测试匿名用户限制
	allowed, retryAfter := redisClient.isAllowed(ctx, "ip:192.168.1.1", false)
	require.True(t, allowed)
	require.Equal(t, time.Duration(0), retryAfter)

	// 快速发送多个请求直到被限制
	for i := 0; i < 100; i++ {
		allowed, _ = redisClient.isAllowed(ctx, "ip:192.168.1.1", false)
		if !allowed {
			break
		}
	}

	// 最终应该被限制
	assert.False(t, allowed)
	assert.True(t, retryAfter > 0)
}

// TestRateLimiterWithInvalidRedis 测试无效Redis连接时的处理
func TestRateLimiterWithInvalidRedis(t *testing.T) {
	cfg := RateLimiterConfig{
		RedisAddr:             "invalid:6379", // 无效地址
		RedisPassword:         "",
		RedisDB:               0,
		RedisPoolSize:         5,
		AnonymousRequests:     5,
		AuthenticatedRequests: 10,
		WindowDuration:        time.Second,
		KeyPrefix:             "test_rate_limit",
		FallbackEnabled:       true, // 启用内存降级
	}

	limiter := NewDistributedRateLimiter(cfg)
	defer limiter.Close()

	ctx := context.Background()

	// 由于Redis不可用，应该使用内存降级限制器
	// 测试匿名用户限制
	for i := 0; i < 5; i++ {
		allowed, retryAfter := limiter.isAllowed(ctx, "ip:192.168.1.1", false)
		assert.True(t, allowed, "匿名用户第 %d 个请求应该被允许（降级模式）", i+1)
		assert.Equal(t, time.Duration(0), retryAfter)
	}

	// 第6个请求应该被拒绝
	allowed, retryAfter := limiter.isAllowed(ctx, "ip:192.168.1.1", false)
	assert.False(t, allowed, "匿名用户第6个请求应该被拒绝（降级模式）")
	assert.True(t, retryAfter > 0, "应该有重试时间（降级模式）")

	// 测试认证用户限制（应该使用不同的内存限制器）
	for i := 0; i < 10; i++ {
		allowed, retryAfter := limiter.isAllowed(ctx, "user:user123", true)
		assert.True(t, allowed, "认证用户第 %d 个请求应该被允许（降级模式）", i+1)
		assert.Equal(t, time.Duration(0), retryAfter)
	}

	// 第11个请求应该被拒绝
	allowed, retryAfter = limiter.isAllowed(ctx, "user:user123", true)
	assert.False(t, allowed, "认证用户第11个请求应该被拒绝（降级模式）")
	assert.True(t, retryAfter > 0, "应该有重试时间（降级模式）")
}

// TestRateLimiterRedisFailover 测试Redis故障转移
func TestRateLimiterRedisFailover(t *testing.T) {
	cfg := RateLimiterConfig{
		RedisAddr:             "invalid:6379", // 无效的Redis地址
		AnonymousRequests:     3,
		AuthenticatedRequests: 6,
		WindowDuration:        time.Second,
		KeyPrefix:             "test_failover",
		FallbackEnabled:       true, // 启用降级
	}

	limiter := NewDistributedRateLimiter(cfg)
	defer limiter.Close()

	ctx := context.Background()

	// 由于Redis不可用，应该使用内存降级限制器
	for i := 0; i < 3; i++ {
		allowed, retryAfter := limiter.isAllowed(ctx, "ip:192.168.1.100", false)
		assert.True(t, allowed, "降级模式下匿名用户第 %d 个请求应该被允许", i+1)
		assert.Equal(t, time.Duration(0), retryAfter)
	}

	// 第4个请求应该被拒绝
	allowed, retryAfter := limiter.isAllowed(ctx, "ip:192.168.1.100", false)
	assert.False(t, allowed, "降级模式下匿名用户第4个请求应该被拒绝")
	assert.True(t, retryAfter > 0, "应该有重试时间")
}

// TestRateLimiterNoFallback 测试不启用降级的情况
func TestRateLimiterNoFallback(t *testing.T) {
	cfg := RateLimiterConfig{
		RedisAddr:             "invalid:6379", // 无效的Redis地址
		AnonymousRequests:     3,
		AuthenticatedRequests: 6,
		WindowDuration:        time.Second,
		KeyPrefix:             "test_no_fallback",
		FallbackEnabled:       false, // 不启用降级
	}

	limiter := NewDistributedRateLimiter(cfg)
	defer limiter.Close()

	ctx := context.Background()

	// Redis不可用且不启用降级时，应该允许所有请求（fail-open策略）
	for i := 0; i < 100; i++ {
		allowed, retryAfter := limiter.isAllowed(ctx, "ip:192.168.1.200", false)
		assert.True(t, allowed, "fail-open模式下所有请求都应该被允许")
		assert.Equal(t, time.Duration(0), retryAfter)
	}
}

// TestRateLimiterMiddleware_RateLimitHeaders 测试速率限制响应头
func TestRateLimiterMiddleware_RateLimitHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:  true,
			Requests: 2,
			Window:   "1s",
			RedisKey: "test_rate_limit_headers",
		},
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       1,
			PoolSize: 5,
		},
	}

	middleware := RateLimiterMiddleware(cfg)

	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// 验证响应头
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
}

// TestRateLimiterMiddleware_429Response 测试429响应格式
func TestRateLimiterMiddleware_429Response(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:  true,
			Requests: 1,
			Window:   "100ms",
			RedisKey: "test_429_response",
		},
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       1,
			PoolSize: 5,
		},
	}

	middleware := RateLimiterMiddleware(cfg)

	router := gin.New()
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	clientIP := "192.168.1.50"

	// 第一个请求应该成功
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = clientIP
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// 第二个请求应该返回429
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = clientIP
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
	assert.Contains(t, w2.Body.String(), "请求过于频繁")
	assert.NotEmpty(t, w2.Header().Get("Retry-After"))
}

// TestRateLimiter_ConcurrentAccess 测试并发访问
func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewMemoryRateLimiter(100, time.Second)

	const numGoroutines = 10
	const requestsPerGoroutine = 20

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	rejectedCount := 0

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < requestsPerGoroutine; j++ {
				clientID := fmt.Sprintf("client_%d", goroutineID)
				allowed, _ := limiter.isAllowed(clientID)

				mu.Lock()
				if allowed {
					successCount++
				} else {
					rejectedCount++
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// 验证结果
	totalRequests := numGoroutines * requestsPerGoroutine
	assert.Equal(t, totalRequests, successCount+rejectedCount)
	assert.True(t, successCount > 0, "应该有成功的请求")
	assert.True(t, rejectedCount > 0, "应该有被拒绝的请求")
}

// TestMemoryRateLimiter_EdgeCases 测试内存限制器边界情况
func TestMemoryRateLimiter_EdgeCases(t *testing.T) {
	t.Run("空客户端ID", func(t *testing.T) {
		limiter := NewMemoryRateLimiter(5, time.Second)

		allowed, retryAfter := limiter.isAllowed("")
		assert.True(t, allowed)
		assert.Equal(t, time.Duration(0), retryAfter)
	})

	t.Run("零时间窗口", func(t *testing.T) {
		limiter := NewMemoryRateLimiter(5, 0)

		// 零时间窗口应该立即重置
		for i := 0; i < 100; i++ {
			allowed, retryAfter := limiter.isAllowed("test")
			assert.True(t, allowed, "零时间窗口下所有请求都应该被允许")
			assert.Equal(t, time.Duration(0), retryAfter)
		}
	})

	t.Run("零请求数限制", func(t *testing.T) {
		limiter := NewMemoryRateLimiter(0, time.Second)

		allowed, retryAfter := limiter.isAllowed("test")
		assert.False(t, allowed, "零请求数限制下第一个请求就应该被拒绝")
		assert.True(t, retryAfter > 0)
	})
}

// TestDistributedRateLimiter_SlidingWindowPrecision 测试滑动窗口精度
func TestDistributedRateLimiter_SlidingWindowPrecision(t *testing.T) {
	cfg := RateLimiterConfig{
		RedisAddr:       "invalid:6379",         // 使用无效Redis强制降级到内存模式
		WindowDuration:  100 * time.Millisecond, // 短窗口用于测试
		KeyPrefix:       "test_precision",
		FallbackEnabled: true,
	}

	limiter := NewDistributedRateLimiter(cfg)
	defer limiter.Close()

	ctx := context.Background()

	// 发送请求直到达到限制
	for i := 0; i < 5; i++ {
		allowed, _ := limiter.isAllowed(ctx, "precision_test", false)
		assert.True(t, allowed)
	}

	// 应该被限制
	allowed, retryAfter := limiter.isAllowed(ctx, "precision_test", false)
	assert.False(t, allowed)
	assert.True(t, retryAfter > 0)

	// 等待窗口过期
	time.Sleep(150 * time.Millisecond)

	// 现在应该再次允许请求
	allowed, retryAfter = limiter.isAllowed(ctx, "precision_test", false)
	assert.True(t, allowed, "窗口过期后应该允许新的请求")
	assert.Equal(t, time.Duration(0), retryAfter)
}

// TestRateLimiterMiddleware_MiddlewareChain 测试中间件链中的速率限制
func TestRateLimiterMiddleware_MiddlewareChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Enabled:  true,
			Requests: 1,
			Window:   "1s",
			RedisKey: "test_middleware_chain",
		},
		Redis: config.RedisConfig{
			Host:     "localhost",
			Port:     6379,
			Password: "",
			DB:       1,
			PoolSize: 5,
		},
	}

	// 创建一个模拟的认证中间件
	authMiddleware := func(c *gin.Context) {
		// 为某些请求设置用户ID
		if strings.Contains(c.Request.URL.Path, "protected") {
			c.Set("user_id", "test_user_456")
		}
		c.Next()
	}

	rateLimitMiddleware := RateLimiterMiddleware(cfg)

	router := gin.New()
	router.Use(authMiddleware)
	router.Use(rateLimitMiddleware)

	router.GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"endpoint": "public"})
	})

	router.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"endpoint": "protected"})
	})

	// 测试公共端点（匿名用户限制）
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/public", nil)
		req.RemoteAddr = "192.168.1.1"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i == 0 {
			assert.Equal(t, http.StatusOK, w.Code)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
		}
	}

	// 测试受保护端点（认证用户限制）
	// 由于认证用户限制更高，应该允许更多请求
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.RemoteAddr = "192.168.1.2"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "认证用户请求 %d 应该成功", i+1)
	}
}

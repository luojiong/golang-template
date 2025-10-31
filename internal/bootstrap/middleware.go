package bootstrap

import (
	"context"
	"fmt"

	"go-server/internal/logger"
	"go-server/internal/middleware"

	"github.com/gin-gonic/gin"
)

// setupMiddlewares 设置中间件栈
func (c *Container) setupMiddlewares() error {
	appLogger := c.Logger.GetLogger("app")

	var middlewares []gin.HandlerFunc

	// 设置Gin模式
	if c.Config.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// 1. 结构化日志中间件（REQ-MW-003）
	middlewares = append(middlewares, middleware.StructuredLoggingMiddleware(c.Config))
	appLogger.Debug(context.Background(), "结构化日志中间件已初始化")

	// 2. 增强恢复中间件
	middlewares = append(middlewares, middleware.RecoveryMiddleware())
	appLogger.Debug(context.Background(), "增强恢复中间件已初始化")

	// 3. CORS中间件
	allowedOrigins := []string{"*"}
	if c.Config.Mode == "production" {
		allowedOrigins = []string{"https://yourdomain.com"}
	}
	middlewares = append(middlewares, middleware.CORSMiddleware(allowedOrigins))
	appLogger.Debug(context.Background(), "CORS中间件已初始化",
		logger.Any("allowed_origins", allowedOrigins))

	// 4. 安全头中间件
	middlewares = append(middlewares, middleware.SecurityHeadersMiddleware(c.Config))
	appLogger.Debug(context.Background(), "安全头部中间件已初始化")

	// 5. 分布式速率限制中间件（REQ-MW-001）
	if c.Config.RateLimit.Enabled {
		middlewares = append(middlewares, middleware.RateLimiterMiddleware(c.Config))

		rateLimitInfo := map[string]interface{}{
			"enabled":  c.Config.RateLimit.Enabled,
			"requests": c.Config.RateLimit.Requests,
			"window":   c.Config.RateLimit.Window,
		}

		if c.Cache != nil {
			rateLimitInfo["redis_integration"] = true
			rateLimitInfo["redis_host"] = fmt.Sprintf("%s:%d", c.Config.Redis.Host, c.Config.Redis.Port)
			rateLimitInfo["redis_db"] = c.Config.Redis.DB
			rateLimitInfo["anonymous_limit"] = c.Config.RateLimit.Requests
			rateLimitInfo["authenticated_limit"] = c.Config.RateLimit.Requests * 2
			rateLimitInfo["key_prefix"] = c.Config.RateLimit.RedisKey
		} else {
			rateLimitInfo["redis_integration"] = false
			rateLimitInfo["fallback"] = "in_memory_only"
			appLogger.Warn(context.Background(), "速率限制将为实例特定，非分布式")
		}

		appLogger.Info(context.Background(), "分布式速率限制中间件已初始化",
			logger.Any("config", rateLimitInfo))
	} else {
		appLogger.Warn(context.Background(), "速率限制中间件已禁用")
	}

	// 6. 压缩中间件（REQ-MW-002）
	if c.Config.Compression.Enabled {
		middlewares = append(middlewares, middleware.CompressionMiddleware(c.Config.Compression.Threshold))

		compressionInfo := map[string]interface{}{
			"enabled":   c.Config.Compression.Enabled,
			"threshold": c.Config.Compression.Threshold,
			"features": []string{
				"gzip_compression",
				"automatic_request_handling",
				"content_encoding_management",
				"intelligent_fallback",
				"skip_compressed_content",
			},
		}

		appLogger.Info(context.Background(), "压缩中间件已初始化",
			logger.Any("config", compressionInfo))
	} else {
		appLogger.Warn(context.Background(), "压缩中间件已禁用",
			logger.String("note", "响应将以未压缩方式发送，带宽使用可能更高"))
	}

	// 7. 请求大小限制中间件
	middlewares = append(middlewares, middleware.RequestSizeLimitMiddleware(10<<20)) // 10MB
	appLogger.Debug(context.Background(), "请求大小限制中间件已初始化",
		logger.Int("limit_mb", 10))

	c.Middlewares = middlewares

	appLogger.Info(context.Background(), "增强的中间件栈已配置完成",
		logger.Int("middleware_count", len(middlewares)),
		logger.Any("features", []string{
			"structured_json_logging",
			"enhanced_error_recovery",
			"cors",
			"security_headers",
			"distributed_rate_limiting",
			"gzip_compression",
			"request_size_protection",
		}))

	return nil
}

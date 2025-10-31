package middleware

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"go-server/internal/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func LoggerMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("%s - [%s] \"%s %s %s %d %s \"%s\" %s\"\n",
			param.ClientIP,
			param.TimeStamp.Format(time.RFC1123),
			param.Method,
			param.Path,
			param.Request.Proto,
			param.StatusCode,
			param.Latency,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}

func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowOrigins = allowedOrigins
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	config.AllowCredentials = true
	config.MaxAge = 12 * time.Hour

	return cors.New(config)
}

// EnhancedRecoveryMiddleware 增强的panic恢复中间件
// 提供关联ID追踪、结构化日志记录和完整的错误上下文捕获
// 满足REQ-EH-001：集中式错误处理要求
func RecoveryMiddleware() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		// 获取关联ID，如果不存在则生成新的
		correlationID := GetCorrelationIDFromContext(c)
		if correlationID == "" {
			correlationID = generateCorrelationID()
			c.Set(correlationIDContextKey, correlationID)
		}

		// 在响应头中设置关联ID，便于客户端追踪
		c.Header(correlationIDHeader, correlationID)

		// 捕获完整的错误上下文信息
		var errorMessage string
		var errorType string

		switch err := recovered.(type) {
		case string:
			errorMessage = err
			errorType = "string_panic"
		case error:
			errorMessage = err.Error()
			errorType = "error_panic"
		default:
			errorMessage = fmt.Sprintf("Unknown panic: %v", err)
			errorType = "unknown_panic"
		}

		// 创建增强的错误日志条目
		errorEntry := LogEntry{
			Timestamp:     time.Now(),
			CorrelationID: correlationID,
			Method:        c.Request.Method,
			Path:          c.Request.URL.Path,
			Protocol:      c.Request.Proto,
			StatusCode:    http.StatusInternalServerError,
			Latency:       0, // 无法准确计算，因为panic发生在处理过程中
			ClientIP:      c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
			Referer:       c.Request.Referer(),
			ErrorMessage:  fmt.Sprintf("Panic recovered [%s]: %s", errorType, errorMessage),
			IsSlowRequest: false,
			Stacktrace:    string(debug.Stack()),
		}

		// 使用新的日志系统记录错误
		if globalLoggerManager != nil && globalLoggerManager.IsStarted() {
			// 在上下文中设置日志管理器
			c.Set(loggerContextKey, globalLoggerManager)
			logEntryWithNewLogger(c, errorEntry)
		} else {
			// 如果日志管理器不可用，回退到简单日志记录
			log.Printf("Recovery panic: %s", errorEntry.ErrorMessage)
		}

		// 在生产环境中记录额外的安全相关信息
		isProduction := gin.Mode() == gin.ReleaseMode
		if isProduction {
			// 记录请求详情用于安全分析（不包含敏感数据）
			log.Printf("[SECURITY] Panic occurred - CorrelationID: %s, Type: %s, Path: %s, IP: %s",
				correlationID, errorType, c.Request.URL.Path, c.ClientIP())
		} else {
			// 开发环境显示详细调试信息
			log.Printf("[DEBUG] [PANIC] [%s] Type: %s, Error: %s, Stack: %s",
				correlationID[:8], errorType, errorMessage, string(debug.Stack()))
		}

		// 返回通用错误响应，不暴露敏感信息
		// 根据环境调整返回消息的详细程度
		var responseMessage string
		if isProduction {
			responseMessage = "An unexpected error occurred. Please try again later."
		} else {
			responseMessage = fmt.Sprintf("Internal server error (Correlation ID: %s)", correlationID[:8])
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"success":        false,
			"message":        responseMessage,
			"correlation_id": correlationID,
			"timestamp":      time.Now().UTC().Format(time.RFC3339),
		})
	})
}

func RequestSizeLimitMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(c.Request.Context())
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}

// SecurityHeadersMiddleware 创建增强的安全头中间件
// 根据环境配置不同的安全策略，包括HSTS和CSP
// 满足REQ-EH-002：输入消毒和验证的安全要求
func SecurityHeadersMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 基础安全头 - 防止常见的Web攻击
		c.Header("X-Content-Type-Options", "nosniff") // 防止MIME类型嗅探攻击
		c.Header("X-Frame-Options", "DENY")           // 防止点击劫持攻击
		c.Header("X-XSS-Protection", "1; mode=block") // 启用浏览器XSS保护过滤器

		// 增强的安全头 - 提供额外的保护层
		c.Header("X-DNS-Prefetch-Control", "off")             // 禁用DNS预取，防止信息泄露
		c.Header("X-Download-Options", "noopen")              // 防止直接下载执行
		c.Header("X-Permitted-Cross-Domain-Policies", "none") // 限制跨域策略
		c.Header("X-Server", "go-server")                     // 服务器信息（最小化暴露）

		// 引用和隐私保护
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin") // 限制引用信息泄露

		// 跨域资源控制 - 防止CSRF和侧信道攻击
		c.Header("Cross-Origin-Embedder-Policy", "require-corp")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")

		// 增强的权限策略 - 禁用更多不必要的API功能
		// 防止设备指纹识别、隐私泄露和侧信道攻击
		c.Header("Permissions-Policy",
			"geolocation=(), "+ // 地理位置访问
				"microphone=(), "+ // 麦克风访问
				"camera=(), "+ // 摄像头访问
				"payment=(), "+ // 支付API
				"usb=(), "+ // USB设备访问
				"magnetometer=(), "+ // 磁力计
				"gyroscope=(), "+ // 陀螺仪
				"accelerometer=(), "+ // 加速度计
				"ambient-light-sensor=(), "+ // 环境光传感器
				"autoplay=(), "+ // 自动播放
				"encrypted-media=(), "+ // 加密媒体
				"fullscreen=(), "+ // 全屏API
				"picture-in-picture=(), "+ // 画中画
				"speaker-selection=(), "+ // 扬声器选择
				"vr=(), "+ // 虚拟现实
				"interest-cohort=()") // FLoC广告分组

		// 缓存控制 - 防止敏感信息被缓存
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			// API端点不缓存，确保获取最新数据
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
			c.Header("Surrogate-Control", "no-store")
		}

		// 根据协议和环境设置HSTS头（仅在HTTPS下生效）
		// HSTS防止协议降级攻击和cookie劫持
		if c.Request.TLS != nil {
			if config.IsProduction(cfg.Mode) {
				// 生产环境启用最严格的传输安全：
				// - max-age=31536000: 有效期1年（最大值）
				// - includeSubDomains: 包含所有子域名
				// - preload: 可提交到浏览器HSTS预加载列表
				c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
			} else if cfg.Mode == "staging" {
				// 预发布环境使用中等强度HSTS设置（1个月）
				c.Header("Strict-Transport-Security", "max-age=2592000; includeSubDomains")
			} else {
				// 开发和测试环境设置较短的HSTS有效期（24小时）
				// 避免在开发过程中因HSTS导致的访问问题
				c.Header("Strict-Transport-Security", "max-age=86400; includeSubDomains")
			}
		}

		// 增强的内容安全策略（CSP）
		// CSP是防止XSS、代码注入等攻击的核心防御机制
		if !strings.Contains(c.Request.URL.Path, "/swagger/") {
			if config.IsProduction(cfg.Mode) {
				// 生产环境使用最严格的安全策略
				// 限制资源加载来源，强制HTTPS，防止混合内容，最小化攻击面
				csp := buildProductionCSP(c.Request.URL.Path)
				c.Header("Content-Security-Policy", csp)

				// 添加CSP违规报告头（可选，用于监控CSP违规）
				c.Header("Content-Security-Policy-Report-Only",
					"default-src 'self'; "+
						"script-src 'self'; "+
						"style-src 'self'; "+
						"img-src 'self'; "+
						"connect-src 'self'; "+
						"font-src 'self'; "+
						"object-src 'none'; "+
						"base-uri 'self'; "+
						"form-action 'self'")
			} else {
				// 开发环境允许更多资源，便于调试和开发
				// 允许WebSocket、HTTP协议、blob URLs等开发常用资源
				csp := buildDevelopmentCSP()
				c.Header("Content-Security-Policy", csp)
			}
		} else {
			// Swagger文档页面使用专门的安全策略
			// Swagger UI需要加载外部字体和样式资源，但仍然保持安全
			c.Header("Content-Security-Policy",
				"default-src 'self'; "+
					"script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.jsdelivr.net; "+
					"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
					"img-src 'self' data: https:; "+
					"font-src 'self' data: https://fonts.gstatic.com; "+
					"connect-src 'self' https:; "+
					"frame-ancestors 'none'; "+
					"base-uri 'self'")
		}

		// 内容类型和安全相关头部
		if c.GetHeader("Content-Type") == "" {
			// 确保设置默认的内容类型
			c.Header("Content-Type", "application/json; charset=utf-8")
		}

		c.Next()
	}
}

// buildProductionCSP 构建生产环境的内容安全策略
func buildProductionCSP(path string) string {
	// 基础生产CSP策略
	baseCSP := "default-src 'self'; " +
		"script-src 'self'; " + // 仅允许来自同源的脚本
		"style-src 'self' 'unsafe-inline'; " + // 允许内联样式（必要时）
		"img-src 'self' data: https:; " + // 允许图片和data URL
		"font-src 'self' data:; " + // 允许字体和data URL
		"connect-src 'self'; " + // 仅允许同源连接
		"object-src 'none'; " + // 禁用插件（Flash等）
		"base-uri 'self'; " + // 限制base标签
		"form-action 'self'; " + // 限制表单提交目标
		"frame-ancestors 'none'; " + // 禁止被嵌入iframe
		"upgrade-insecure-requests" // 强制HTTPS升级

	// 根据路径调整CSP策略
	if strings.Contains(path, "/api/") {
		// API路径使用更严格的策略
		baseCSP = "default-src 'none'; " +
			"script-src 'none'; " +
			"style-src 'none'; " +
			"img-src 'none'; " +
			"connect-src 'self'; " +
			"font-src 'none'; " +
			"object-src 'none'; " +
			"base-uri 'none'; " +
			"form-action 'none'; " +
			"frame-ancestors 'none'"
	}

	return baseCSP
}

// buildDevelopmentCSP 构建开发环境的内容安全策略
func buildDevelopmentCSP() string {
	return "default-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
		"script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; " + // 允许调试工具
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob: https: http:; " + // 允许开发和测试图片
		"font-src 'self' data:; " +
		"connect-src 'self' ws: wss: http: https:; " + // 允许WebSocket和HTTP/HTTPS
		"frame-ancestors 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'"
}

func RateLimitMiddleware() gin.HandlerFunc {
	// Simple in-memory rate limiter
	// In production, use Redis or other distributed solution
	clients := make(map[string][]time.Time)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()

		// Clean old requests (older than 1 minute)
		if requests, exists := clients[clientIP]; exists {
			var validRequests []time.Time
			for _, reqTime := range requests {
				if now.Sub(reqTime) < time.Minute {
					validRequests = append(validRequests, reqTime)
				}
			}
			clients[clientIP] = validRequests
		}

		// Check if client exceeded rate limit (60 requests per minute)
		if len(clients[clientIP]) >= 60 {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "Rate limit exceeded",
			})
			c.Abort()
			return
		}

		// Add current request
		clients[clientIP] = append(clients[clientIP], now)
		c.Next()
	}
}

// SetupMiddlewares configures and returns the middleware stack for the application
// This function implements the enhanced middleware requirements:
// - REQ-MW-001: Distributed Rate Limiting with Redis (100 requests/minute per IP)
// - REQ-MW-002: Request/Response Compression (gzip for responses > 1KB)
// - REQ-MW-003: Structured Logging Middleware (JSON format with correlation IDs)
func SetupMiddlewares(cfg *config.Config) []gin.HandlerFunc {
	var middlewares []gin.HandlerFunc

	// Set Gin mode based on environment configuration
	if cfg.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// 1. Structured logging middleware (REQ-MW-003)
	// Provides structured JSON logging with correlation IDs, method, path, status, duration
	// and slow request monitoring for requests taking longer than 1 second
	middlewares = append(middlewares, StructuredLoggingMiddleware(cfg))

	// 2. Enhanced recovery middleware
	// Handles panics with correlation ID tracking and structured error logging
	middlewares = append(middlewares, RecoveryMiddleware())

	// 3. CORS middleware
	// Configures Cross-Origin Resource Sharing with environment-specific origins
	allowedOrigins := []string{"*"}
	if cfg.Mode == "production" {
		// In production, specify allowed origins for security
		allowedOrigins = []string{"https://yourdomain.com"}
	}
	middlewares = append(middlewares, CORSMiddleware(allowedOrigins))

	// 4. Security headers middleware
	// Enhanced security headers including HSTS, CSP, and other security protections
	middlewares = append(middlewares, SecurityHeadersMiddleware(cfg))

	// 5. Distributed rate limiting middleware (REQ-MW-001)
	// Provides Redis-based rate limiting with fallback to in-memory limiting
	// Implements 100 requests/minute per IP with distributed consistency across instances
	// Returns proper HTTP 429 responses with Retry-After headers
	middlewares = append(middlewares, RateLimiterMiddleware(cfg))

	// 6. Compression middleware (REQ-MW-002)
	// Provides gzip compression for responses > 1KB when client supports it
	// Handles compressed requests and sets appropriate Content-Encoding headers
	// Falls back to uncompressed response if compression would increase size
	if cfg.Compression.Enabled {
		middlewares = append(middlewares, CompressionMiddleware(cfg.Compression.Threshold))
	}

	// 7. Request size limit middleware
	// Prevents resource exhaustion by limiting request body size to 10MB
	middlewares = append(middlewares, RequestSizeLimitMiddleware(10<<20))

	return middlewares
}

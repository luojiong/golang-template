package middleware

import (
	"fmt"
	"net/http"
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

func RequestSizeLimitMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(c.Request.Context())
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}

// SecurityHeadersMiddleware 创建增强的安全头中间件
func SecurityHeadersMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否是 Swagger 路径
		isSwaggerPath := strings.HasPrefix(c.Request.URL.Path, "/swagger/")

		// 基本安全头
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("X-DNS-Prefetch-Control", "off")
		c.Header("X-Download-Options", "noopen")
		c.Header("X-Permitted-Cross-Domain-Policies", "none")

		// Swagger 路径需要更宽松的 CORS 策略
		if !isSwaggerPath {
			c.Header("Cross-Origin-Embedder-Policy", "require-corp")
			c.Header("Cross-Origin-Resource-Policy", "same-origin")
			c.Header("Cross-Origin-Opener-Policy", "same-origin")
		}

		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=(), autoplay=(), encrypted-media=(), fullscreen=(), picture-in-picture=(), interest-cohort=()")

		// 设置服务器信息
		c.Header("X-Server", "go-server")

		// 缓存控制头（仅对API路径）
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
			c.Header("Surrogate-Control", "no-store")
		}

		// HSTS头 (仅限HTTPS请求)
		if c.Request.TLS != nil {
			var hstsMaxAge string
			switch cfg.Mode {
			case "production":
				hstsMaxAge = "max-age=31536000; includeSubDomains; preload"
			case "staging":
				hstsMaxAge = "max-age=2592000; includeSubDomains"
			case "development":
				hstsMaxAge = "max-age=86400; includeSubDomains"
			default:
				hstsMaxAge = "max-age=3600" // Default for unknown modes
			}
			c.Header("Strict-Transport-Security", hstsMaxAge)
		}

		// CSP头
		csp := buildCSP(cfg.Mode, c.Request.URL.Path)
		if csp != "" {
			c.Header("Content-Security-Policy", csp)
		}

		c.Next()
	}
}

// buildCSP 根据环境和路径构建Content-Security-Policy头
func buildCSP(mode, path string) string {
	if strings.HasPrefix(path, "/swagger/") {
		// Swagger UI requires a more relaxed CSP
		return "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'"
	}

	switch mode {
	case "production":
		if strings.HasPrefix(path, "/api/") {
			// API路径使用最严格的CSP
			return "default-src 'none'; script-src 'none'; style-src 'none'; img-src 'none'; connect-src 'self'; font-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
		}
		// 非API路径的标准生产CSP
		return "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; upgrade-insecure-requests"
	case "development", "staging":
		// 开发和Staging环境的CSP (允许更多资源进行调试)
		return "default-src 'self' 'unsafe-inline' 'unsafe-eval'; script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https: http:; font-src 'self' data:; connect-src 'self' ws: wss: http: https:; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
	default:
		return "" // Default to no CSP for unknown modes
	}
}

// RateLimitMiddleware 分布式限流中间件
func RateLimitMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: 实现基于Redis的分布式限流
		c.Next()
	}
}
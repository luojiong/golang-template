package middleware

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"go-server/internal/config"
	"go-server/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// correlationIDHeader 用于在HTTP头中传递关联ID
const correlationIDHeader = "X-Correlation-ID"

// correlationIDContextKey 用于在Gin上下文中存储关联ID
const correlationIDContextKey = "correlation_id"

// loggerContextKey 用于在Gin上下文中存储日志管理器
const loggerContextKey = "logger_manager"

// globalLoggerManager 全局日志管理器实例
var globalLoggerManager *logger.Manager

// LogEntry 结构化日志条目
type LogEntry struct {
	Timestamp     time.Time     `json:"timestamp"`            // 请求时间戳
	CorrelationID string        `json:"correlation_id"`       // 关联ID，用于追踪请求
	Method        string        `json:"method"`               // HTTP方法
	Path          string        `json:"path"`                 // 请求路径
	Protocol      string        `json:"protocol"`             // 协议版本
	StatusCode    int           `json:"status_code"`          // 响应状态码
	Latency       time.Duration `json:"latency"`              // 请求处理延迟
	ClientIP      string        `json:"client_ip"`            // 客户端IP地址
	UserAgent     string        `json:"user_agent"`           // 用户代理
	Referer       string        `json:"referer"`              // 来源页面
	RequestSize   int64         `json:"request_size"`         // 请求体大小（字节）
	ResponseSize  int64         `json:"response_size"`        // 响应体大小（字节）
	ErrorMessage  string        `json:"error_message"`        // 错误信息（如果有）
	IsSlowRequest bool          `json:"is_slow_request"`      // 是否为慢请求（>1秒）
	Stacktrace    string        `json:"stacktrace,omitempty"` // 堆栈跟踪（错误时）
}

// responseBodyWriter 用于捕获响应体的包装器
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// generateCorrelationID 生成新的关联ID
func generateCorrelationID() string {
	return uuid.New().String()
}

// getCorrelationID 从请求头中获取关联ID，如果不存在则生成新的
func getCorrelationID(c *gin.Context) string {
	// 首先尝试从请求头中获取
	corrID := c.GetHeader(correlationIDHeader)
	if corrID == "" {
		// 如果请求头中没有，则生成新的关联ID
		corrID = generateCorrelationID()
	}

	// 将关联ID存储到上下文中
	c.Set(correlationIDContextKey, corrID)

	return corrID
}

// GetCorrelationIDFromContext 从Gin上下文中获取关联ID
func GetCorrelationIDFromContext(c *gin.Context) string {
	if corrID, exists := c.Get(correlationIDContextKey); exists {
		if id, ok := corrID.(string); ok {
			return id
		}
	}
	return ""
}

// GetLoggerFromContext 从Gin上下文中获取日志记录器
func GetLoggerFromContext(c *gin.Context) logger.Logger {
	if loggerManager, exists := c.Get(loggerContextKey); exists {
		if manager, ok := loggerManager.(*logger.Manager); ok && manager.IsStarted() {
			return manager.GetLogger("http")
		}
	}
	// 如果没有找到日志管理器，返回一个空操作的日志记录器
	return &noopLoggerAdapter{}
}

// InitializeLoggerManager 初始化全局日志管理器
func InitializeLoggerManager(cfg *config.Config) error {
	if globalLoggerManager != nil {
		// 如果已经初始化，先停止现有的管理器
		if err := globalLoggerManager.Stop(); err != nil {
			log.Printf("Failed to stop existing logger manager: %v", err)
		}
	}

	// 创建新的日志管理器
	manager, err := logger.NewManager(cfg.Logging)
	if err != nil {
		return fmt.Errorf("failed to create logger manager: %w", err)
	}

	// 启动日志管理器
	if err := manager.Start(); err != nil {
		return fmt.Errorf("failed to start logger manager: %w", err)
	}

	globalLoggerManager = manager
	return nil
}

// UpdateLoggerConfig 更新日志管理器配置（支持热重载）
func UpdateLoggerConfig(newConfig config.LoggingConfig) error {
	if globalLoggerManager == nil {
		return fmt.Errorf("logger manager is not initialized")
	}

	return globalLoggerManager.UpdateConfig(newConfig)
}

// GetLoggerManager 获取全局日志管理器
func GetLoggerManager() *logger.Manager {
	return globalLoggerManager
}

// ShutdownLoggerManager 关闭全局日志管理器
func ShutdownLoggerManager() error {
	if globalLoggerManager != nil {
		err := globalLoggerManager.Stop()
		globalLoggerManager = nil
		return err
	}
	return nil
}

// noopLoggerAdapter 空操作日志记录器适配器，用于兼容性
type noopLoggerAdapter struct{}

func (l *noopLoggerAdapter) Debug(ctx context.Context, message string, fields ...logger.Field) {}
func (l *noopLoggerAdapter) Info(ctx context.Context, message string, fields ...logger.Field)  {}
func (l *noopLoggerAdapter) Warn(ctx context.Context, message string, fields ...logger.Field)  {}
func (l *noopLoggerAdapter) Error(ctx context.Context, message string, fields ...logger.Field) {}
func (l *noopLoggerAdapter) Fatal(ctx context.Context, message string, fields ...logger.Field) {}
func (l *noopLoggerAdapter) WithFields(fields ...logger.Field) logger.Logger                   { return l }
func (l *noopLoggerAdapter) WithModule(module string) logger.Logger                            { return l }
func (l *noopLoggerAdapter) WithCorrelationID(correlationID string) logger.Logger              { return l }
func (l *noopLoggerAdapter) Sync() error                                                       { return nil }

// logEntryWithNewLogger 使用新的日志系统记录HTTP请求日志
func logEntryWithNewLogger(c *gin.Context, entry LogEntry) {
	// 获取日志记录器
	loggerInstance := GetLoggerFromContext(c)

	// 创建上下文并添加关联ID
	ctx := context.Background()
	if entry.CorrelationID != "" {
		ctx = context.WithValue(ctx, "correlation_id", entry.CorrelationID)
	}

	// 准备日志字段
	fields := []logger.Field{
		logger.String("method", entry.Method),
		logger.String("path", entry.Path),
		logger.String("protocol", entry.Protocol),
		logger.Int("status_code", entry.StatusCode),
		logger.Int64("latency_ms", entry.Latency.Milliseconds()),
		logger.String("client_ip", entry.ClientIP),
		logger.String("user_agent", entry.UserAgent),
		logger.String("referer", entry.Referer),
		logger.Int64("request_size", entry.RequestSize),
		logger.Int64("response_size", entry.ResponseSize),
		logger.Bool("is_slow_request", entry.IsSlowRequest),
	}

	// 如果有错误信息，添加错误字段
	if entry.ErrorMessage != "" {
		fields = append(fields, logger.String("error_message", entry.ErrorMessage))
	}

	// 如果有堆栈跟踪，添加堆栈跟踪字段
	if entry.Stacktrace != "" {
		fields = append(fields, logger.Stacktrace("stacktrace", entry.Stacktrace))
	}

	// 根据状态码和错误情况确定日志级别
	var logMessage string
	var logLevel func(context.Context, string, ...logger.Field)

	if entry.StatusCode >= 500 || entry.ErrorMessage != "" {
		// 服务器错误或有错误信息，使用ERROR级别
		logLevel = loggerInstance.Error
		logMessage = fmt.Sprintf("HTTP Request Error: %s %s -> %d", entry.Method, entry.Path, entry.StatusCode)
	} else if entry.StatusCode >= 400 {
		// 客户端错误，使用WARN级别
		logLevel = loggerInstance.Warn
		logMessage = fmt.Sprintf("HTTP Request Warning: %s %s -> %d", entry.Method, entry.Path, entry.StatusCode)
	} else {
		// 成功请求，使用INFO级别
		logLevel = loggerInstance.Info
		logMessage = fmt.Sprintf("HTTP Request: %s %s -> %d", entry.Method, entry.Path, entry.StatusCode)
	}

	// 记录日志
	logLevel(ctx, logMessage, fields...)
}

// StructuredLoggingMiddleware 创建结构化日志中间件
func StructuredLoggingMiddleware(cfg *config.Config) gin.HandlerFunc {
	slowRequestThreshold := time.Second // 1秒阈值用于检测慢请求

	return func(c *gin.Context) {
		// 记录请求开始时间
		startTime := time.Now()

		// 获取或生成关联ID
		correlationID := getCorrelationID(c)

		// 在响应头中设置关联ID，便于客户端追踪
		c.Header(correlationIDHeader, correlationID)

		// 在上下文中设置日志管理器
		if globalLoggerManager != nil && globalLoggerManager.IsStarted() {
			c.Set(loggerContextKey, globalLoggerManager)
		}

		// 读取请求体大小（如果有）
		var requestSize int64
		if c.Request.Body != nil {
			// 读取请求体但不消费它
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil {
				requestSize = int64(len(bodyBytes))
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// 包装响应写入器以捕获响应大小
		responseBodyWriter := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBufferString(""),
		}
		c.Writer = responseBodyWriter

		// 捕获可能的panic
		defer func() {
			if err := recover(); err != nil {
				// 创建错误日志条目
				latency := time.Since(startTime)
				entry := LogEntry{
					Timestamp:     startTime,
					CorrelationID: correlationID,
					Method:        c.Request.Method,
					Path:          c.Request.URL.Path,
					Protocol:      c.Request.Proto,
					StatusCode:    http.StatusInternalServerError,
					Latency:       latency,
					ClientIP:      c.ClientIP(),
					UserAgent:     c.Request.UserAgent(),
					Referer:       c.Request.Referer(),
					RequestSize:   requestSize,
					ResponseSize:  int64(responseBodyWriter.body.Len()),
					ErrorMessage:  fmt.Sprintf("Panic recovered: %v", err),
					IsSlowRequest: latency > slowRequestThreshold,
					Stacktrace:    string(debug.Stack()),
				}

				// 使用新的日志系统记录错误日志
				logEntryWithNewLogger(c, entry)

				// 返回标准错误响应
				c.JSON(http.StatusInternalServerError, gin.H{
					"success":        false,
					"message":        "Internal server error",
					"correlation_id": correlationID,
				})
			}
		}()

		// 处理请求
		c.Next()

		// 计算请求处理延迟
		latency := time.Since(startTime)

		// 获取错误信息（如果有）
		var errorMessage string
		if len(c.Errors) > 0 {
			errorMessage = c.Errors.String()
		}

		// 检查是否为慢请求
		isSlow := latency > slowRequestThreshold

		// 创建结构化日志条目
		entry := LogEntry{
			Timestamp:     startTime,
			CorrelationID: correlationID,
			Method:        c.Request.Method,
			Path:          c.Request.URL.Path,
			Protocol:      c.Request.Proto,
			StatusCode:    c.Writer.Status(),
			Latency:       latency,
			ClientIP:      c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
			Referer:       c.Request.Referer(),
			RequestSize:   requestSize,
			ResponseSize:  int64(responseBodyWriter.body.Len()),
			ErrorMessage:  errorMessage,
			IsSlowRequest: isSlow,
		}

		// 使用新的日志系统记录结构化日志
		logEntryWithNewLogger(c, entry)

		// 如果是慢请求，额外记录警告日志
		if isSlow {
			// 获取日志记录器并记录慢请求警告
			loggerInstance := GetLoggerFromContext(c)
			ctx := context.Background()
			if correlationID != "" {
				ctx = context.WithValue(ctx, "correlation_id", correlationID)
			}

			slowFields := []logger.Field{
				logger.String("method", entry.Method),
				logger.String("path", entry.Path),
				logger.Int64("latency_ms", entry.Latency.Milliseconds()),
				logger.String("client_ip", entry.ClientIP),
			}

			loggerInstance.Warn(ctx, fmt.Sprintf("Slow request detected: %v", entry.Latency), slowFields...)
		}
	}
}

// LoggingWithConfig 返回带有配置的结构化日志中间件
// 这是为了与现有中间件系统保持兼容性
func LoggingWithConfig(cfg *config.Config) gin.HandlerFunc {
	return StructuredLoggingMiddleware(cfg)
}

// UpdateMiddlewareSetup 在 middleware.go 中更新中间件设置函数
// 使用结构化日志中间件替换原有的简单日志中间件
func UpdateMiddlewareSetupWithStructuredLogging(cfg *config.Config) []gin.HandlerFunc {
	var middlewares []gin.HandlerFunc

	// 设置Gin模式
	if cfg.Mode == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// 添加结构化日志中间件（替换原有的LoggerMiddleware）
	middlewares = append(middlewares, StructuredLoggingMiddleware(cfg))

	// 添加恢复中间件
	middlewares = append(middlewares, RecoveryMiddleware())

	// 添加CORS中间件
	allowedOrigins := []string{"*"}
	if cfg.Mode == "production" {
		// 在生产环境中，指定允许的源
		allowedOrigins = []string{"https://yourdomain.com"}
	}
	middlewares = append(middlewares, CORSMiddleware(allowedOrigins))

	// 添加安全头中间件
	middlewares = append(middlewares, SecurityHeadersMiddleware(cfg))

	// 添加分布式限流中间件
	middlewares = append(middlewares, RateLimiterMiddleware(cfg))

	// 添加压缩中间件
	if cfg.Compression.Enabled {
		middlewares = append(middlewares, CompressionMiddleware(cfg.Compression.Threshold))
	}

	// 添加请求大小限制（10MB）
	middlewares = append(middlewares, RequestSizeLimitMiddleware(10<<20))

	return middlewares
}

// OnLoggingConfigChange 处理日志配置变更的回调函数
func OnLoggingConfigChange(oldConfig, newConfig config.LoggingConfig) error {
	// 更新日志管理器配置
	if err := UpdateLoggerConfig(newConfig); err != nil {
		log.Printf("Failed to update logger configuration: %v", err)
		return err
	}

	// 记录配置变更
	if globalLoggerManager != nil && globalLoggerManager.IsStarted() {
		loggerInstance := globalLoggerManager.GetLogger("config")
		loggerInstance.Info(context.Background(), "Logging configuration updated",
			logger.String("old_level", oldConfig.Level),
			logger.String("new_level", newConfig.Level),
			logger.String("old_format", oldConfig.Format),
			logger.String("new_format", newConfig.Format),
			logger.String("old_output", oldConfig.Output),
			logger.String("new_output", newConfig.Output),
		)
	}

	log.Printf("Logging configuration updated: level=%s, format=%s, output=%s",
		newConfig.Level, newConfig.Format, newConfig.Output)

	return nil
}

// GetLoggerForModule 为指定模块获取日志记录器
func GetLoggerForModule(moduleName string) logger.Logger {
	if globalLoggerManager != nil && globalLoggerManager.IsStarted() {
		return globalLoggerManager.GetLogger(moduleName)
	}
	return &noopLoggerAdapter{}
}

package utils

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"go-server/internal/logger"

	"github.com/gin-gonic/gin"
)

// GetLogger 获取默认日志记录器实例，提供便捷的访问方式
// 这个函数简化了在整个应用程序中获取日志记录器的过程
func GetLogger(moduleName ...string) logger.Logger {
	// 默认模块名为 "utils"
	module := "utils"
	if len(moduleName) > 0 && moduleName[0] != "" {
		module = moduleName[0]
	}

	// 尝试从全局日志管理器获取日志记录器
	manager := GetGlobalLoggerManager()
	if manager != nil && manager.IsStarted() {
		return manager.GetLogger(module)
	}

	// 如果全局管理器不可用，返回空操作日志记录器
	return &noopLoggerAdapter{}
}

// GetGlobalLoggerManager 获取全局日志管理器实例
// 这个函数封装了对全局日志管理器的访问
func GetGlobalLoggerManager() *logger.Manager {
	// 这里应该从中间件或配置中获取全局日志管理器
	// 为了避免循环依赖，我们使用一个简单的接口
	return loggerManagerInstance
}

// 全局日志管理器实例变量，由应用程序初始化时设置
var loggerManagerInstance *logger.Manager

// SetGlobalLoggerManager 设置全局日志管理器实例
// 这个函数应该在应用程序启动时调用，以确保日志系统正确初始化
func SetGlobalLoggerManager(manager *logger.Manager) {
	loggerManagerInstance = manager
}

// GetCorrelationIDFromGinContext 从Gin上下文中获取关联ID
func GetCorrelationIDFromGinContext(c *gin.Context) string {
	if c == nil {
		return ""
	}
	// 尝试从请求头中获取关联ID
	if correlationID := c.GetHeader("X-Correlation-ID"); correlationID != "" {
		return correlationID
	}
	// 尝试从Gin的键值对中获取
	if correlationID, exists := c.Get("correlation_id"); exists {
		if id, ok := correlationID.(string); ok {
			return id
		}
	}
	return ""
}

// GetModuleNameFromGinContext 从Gin上下文中获取模块名称
func GetModuleNameFromGinContext(c *gin.Context) string {
	if c == nil {
		return ""
	}
	// 尝试从路径中推断模块名
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/api/v1/auth") {
		return "auth"
	} else if strings.HasPrefix(path, "/api/v1/users") {
		return "user"
	} else if strings.HasPrefix(path, "/api/v1/health") {
		return "health"
	}
	return "api"
}

// WithError 创建带有错误字段的日志记录器
// 这是一个常见的日志模式，用于简化错误日志的记录
func WithError(err error) logger.Field {
	return logger.Error(err)
}

// WithFields 创建带有多个字段的日志记录器
// 这个函数提供了便捷的方式来添加结构化字段到日志中
func WithFields(fields map[string]interface{}) []logger.Field {
	logFields := make([]logger.Field, 0, len(fields))
	for key, value := range fields {
		logFields = append(logFields, logger.Any(key, value))
	}
	return logFields
}

// WithField 创建带有单个字段的日志记录器
// 这是一个便捷函数，用于添加单个字段到日志中
func WithField(key string, value interface{}) logger.Field {
	return logger.Any(key, value)
}

// WithString 创建字符串类型的字段
func WithString(key, value string) logger.Field {
	return logger.String(key, value)
}

// WithInt 创建整数类型的字段
func WithInt(key string, value int) logger.Field {
	return logger.Int(key, value)
}

// WithInt64 创建64位整数类型的字段
func WithInt64(key string, value int64) logger.Field {
	return logger.Int64(key, value)
}

// WithFloat64 创建浮点数类型的字段
func WithFloat64(key string, value float64) logger.Field {
	return logger.Float64(key, value)
}

// WithBool 创建布尔类型的字段
func WithBool(key string, value bool) logger.Field {
	return logger.Bool(key, value)
}

// WithDuration 创建时间间隔字段，以毫秒为单位
func WithDuration(key string, duration time.Duration) logger.Field {
	return logger.Int64(key, duration.Milliseconds())
}

// WithTimestamp 创建时间戳字段
func WithTimestamp(key string, timestamp time.Time) logger.Field {
	return logger.String(key, timestamp.Format(time.RFC3339))
}

// ContextWithLogger 为上下文添加日志记录器
// 这个函数允许在整个请求处理过程中保持一致的日志记录器
func ContextWithLogger(ctx context.Context, log logger.Logger) context.Context {
	return context.WithValue(ctx, "logger", log)
}

// LoggerFromContext 从上下文中获取日志记录器
// 这个函数提供了从上下文中检索日志记录器的便捷方式
func LoggerFromContext(ctx context.Context) logger.Logger {
	if log, ok := ctx.Value("logger").(logger.Logger); ok {
		return log
	}
	return GetLogger("context")
}

// ContextWithCorrelationID 为上下文添加关联ID
// 关联ID用于追踪单个请求在整个系统中的处理过程
func ContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, "correlation_id", correlationID)
}

// CorrelationIDFromContext 从上下文中获取关联ID
// 这个函数用于提取请求的关联ID以便在日志中使用
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value("correlation_id").(string); ok {
		return id
	}
	return ""
}

// GinContextWithLogger 为Gin上下文添加日志记录器
// 这个函数简化了在HTTP请求处理中设置日志记录器的过程
func GinContextWithLogger(c *gin.Context, log logger.Logger) {
	c.Set("logger", log)
}

// LoggerFromGinContext 从Gin上下文中获取日志记录器
// 这个函数提供了从HTTP请求上下文中获取日志记录器的便捷方式
func LoggerFromGinContext(c *gin.Context) logger.Logger {
	if log, exists := c.Get("logger"); exists {
		if loggerInstance, ok := log.(logger.Logger); ok {
			return loggerInstance
		}
	}

	// 如果没有找到日志记录器，尝试从全局管理器获取
	correlationID := GetCorrelationIDFromGinContext(c)
	module := GetModuleNameFromGinContext(c)

	log := GetLogger(module)
	if correlationID != "" {
		log = log.WithCorrelationID(correlationID)
	}

	return log
}

// CorrelationIDFromGinContext 从Gin上下文中获取关联ID
// 这个函数用于从HTTP请求中提取关联ID
func CorrelationIDFromGinContext(c *gin.Context) string {
	if id, exists := c.Get("correlation_id"); exists {
		if correlationID, ok := id.(string); ok {
			return correlationID
		}
	}

	// 尝试从HTTP头中获取
	if correlationID := c.GetHeader("X-Correlation-ID"); correlationID != "" {
		return correlationID
	}

	return ""
}


// LogRequest 记录HTTP请求的开始
// 这个函数用于记录请求的详细信息，便于调试和监控
func LogRequest(c *gin.Context, additionalFields ...logger.Field) {
	log := LoggerFromGinContext(c)

	fields := []logger.Field{
		logger.String("method", c.Request.Method),
		logger.String("path", c.Request.URL.Path),
		logger.String("query", c.Request.URL.RawQuery),
		logger.String("client_ip", c.ClientIP()),
		logger.String("user_agent", c.Request.UserAgent()),
	}

	// 添加额外字段
	fields = append(fields, additionalFields...)

	log.Info(c.Request.Context(), "HTTP request started", fields...)
}

// LogResponse 记录HTTP响应的完成
// 这个函数用于记录响应的详细信息，包括处理时间
func LogResponse(c *gin.Context, startTime time.Time, additionalFields ...logger.Field) {
	log := LoggerFromGinContext(c)

	duration := time.Since(startTime)

	fields := []logger.Field{
		logger.String("method", c.Request.Method),
		logger.String("path", c.Request.URL.Path),
		logger.Int("status_code", c.Writer.Status()),
		WithDuration("duration", duration),
		logger.Int64("response_size", int64(c.Writer.Size())),
	}

	// 添加额外字段
	fields = append(fields, additionalFields...)

	// 根据状态码确定日志级别
	message := fmt.Sprintf("HTTP request completed: %s %s -> %d (%v)",
		c.Request.Method, c.Request.URL.Path, c.Writer.Status(), duration)

	switch {
	case c.Writer.Status() >= 500:
		log.Error(c.Request.Context(), message, fields...)
	case c.Writer.Status() >= 400:
		log.Warn(c.Request.Context(), message, fields...)
	default:
		log.Info(c.Request.Context(), message, fields...)
	}
}

// LogError 记录带有上下文信息的错误
// 这个函数提供了统一的错误日志记录方式，包括堆栈跟踪
func LogError(ctx context.Context, err error, message string, additionalFields ...logger.Field) {
	log := LoggerFromContext(ctx)

	fields := []logger.Field{
		WithError(err),
		GetCallerInfo(),
	}

	// 添加额外字段
	fields = append(fields, additionalFields...)

	log.Error(ctx, message, fields...)
}

// LogPanic 记录panic恢复信息
// 这个函数用于记录panic的详细信息，包括堆栈跟踪
func LogPanic(c *gin.Context, recovered interface{}) {
	log := LoggerFromGinContext(c)

	fields := []logger.Field{
		logger.String("panic", fmt.Sprintf("%v", recovered)),
		logger.String("stacktrace", string(debug.Stack())),
		logger.String("method", c.Request.Method),
		logger.String("path", c.Request.URL.Path),
		logger.String("client_ip", c.ClientIP()),
	}

	log.Error(c.Request.Context(), "Panic recovered", fields...)
}

// GetCallerInfo 获取调用者信息
// 这个函数用于在日志中包含函数调用的上下文信息
func GetCallerInfo() logger.Field {
	// 获取调用者信息，跳过当前函数和调用栈
	pc, file, line, ok := runtime.Caller(2)
	if !ok {
		return logger.String("caller", "unknown")
	}

	funcName := runtime.FuncForPC(pc).Name()
	// 提取函数名，去掉包路径
	if lastSlash := strings.LastIndex(funcName, "/"); lastSlash >= 0 {
		funcName = funcName[lastSlash+1:]
	}
	if lastDot := strings.LastIndex(funcName, "."); lastDot >= 0 {
		funcName = funcName[lastDot+1:]
	}

	// 提取文件名，去掉路径
	if lastSlash := strings.LastIndex(file, "/"); lastSlash >= 0 {
		file = file[lastSlash+1:]
	}
	if lastSlash := strings.LastIndex(file, "\\"); lastSlash >= 0 {
		file = file[lastSlash+1:]
	}

	callerInfo := fmt.Sprintf("%s:%d (%s)", file, line, funcName)
	return logger.String("caller", callerInfo)
}

// PerformanceMetrics 性能监控辅助结构体
type PerformanceMetrics struct {
	Operation  string                 `json:"operation"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time"`
	Duration   time.Duration          `json:"duration"`
	Success    bool                   `json:"success"`
	ErrorCount int                    `json:"error_count"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// NewPerformanceMetrics 创建新的性能监控实例
func NewPerformanceMetrics(operation string) *PerformanceMetrics {
	return &PerformanceMetrics{
		Operation:  operation,
		StartTime:  time.Now(),
		Success:    true,
		ErrorCount: 0,
		Metadata:   make(map[string]interface{}),
	}
}

// Start 开始性能监控
func (pm *PerformanceMetrics) Start() {
	pm.StartTime = time.Now()
}

// End 结束性能监控
func (pm *PerformanceMetrics) End() {
	pm.EndTime = time.Now()
	pm.Duration = pm.EndTime.Sub(pm.StartTime)
}

// RecordError 记录错误
func (pm *PerformanceMetrics) RecordError(err error) {
	pm.Success = false
	pm.ErrorCount++
	if pm.Metadata["errors"] == nil {
		pm.Metadata["errors"] = []string{}
	}
	if errors, ok := pm.Metadata["errors"].([]string); ok {
		pm.Metadata["errors"] = append(errors, err.Error())
	}
}

// AddMetadata 添加元数据
func (pm *PerformanceMetrics) AddMetadata(key string, value interface{}) {
	pm.Metadata[key] = value
}

// Log 记录性能指标
func (pm *PerformanceMetrics) Log(ctx context.Context, additionalFields ...logger.Field) {
	log := LoggerFromContext(ctx)

	pm.End() // 确保结束时间已设置

	fields := []logger.Field{
		logger.String("operation", pm.Operation),
		logger.String("start_time", pm.StartTime.Format(time.RFC3339)),
		logger.String("end_time", pm.EndTime.Format(time.RFC3339)),
		WithDuration("duration", pm.Duration),
		logger.Bool("success", pm.Success),
		logger.Int("error_count", pm.ErrorCount),
	}

	// 添加元数据字段
	for key, value := range pm.Metadata {
		fields = append(fields, logger.Any(key, value))
	}

	// 添加额外字段
	fields = append(fields, additionalFields...)

	// 根据性能指标确定日志级别
	message := fmt.Sprintf("Performance: %s completed in %v", pm.Operation, pm.Duration)

	switch {
	case !pm.Success:
		log.Error(ctx, message, fields...)
	case pm.Duration > 5*time.Second:
		log.Warn(ctx, message, fields...)
	default:
		log.Info(ctx, message, fields...)
	}
}

// LogPerformance 记录性能指标的便捷函数
// 这个函数提供了简单的性能监控和日志记录
func LogPerformance(ctx context.Context, operation string, fn func() error, additionalFields ...logger.Field) error {
	metrics := NewPerformanceMetrics(operation)
	metrics.Start()

	err := fn()
	metrics.End()

	if err != nil {
		metrics.RecordError(err)
	}

	metrics.Log(ctx, additionalFields...)

	return err
}

// DebugWithFields 使用调试级别记录带字段的消息
func DebugWithFields(ctx context.Context, message string, fields map[string]interface{}) {
	log := LoggerFromContext(ctx)
	log.Debug(ctx, message, WithFields(fields)...)
}

// InfoWithFields 使用信息级别记录带字段的消息
func InfoWithFields(ctx context.Context, message string, fields map[string]interface{}) {
	log := LoggerFromContext(ctx)
	log.Info(ctx, message, WithFields(fields)...)
}

// WarnWithFields 使用警告级别记录带字段的消息
func WarnWithFields(ctx context.Context, message string, fields map[string]interface{}) {
	log := LoggerFromContext(ctx)
	log.Warn(ctx, message, WithFields(fields)...)
}

// ErrorWithFields 使用错误级别记录带字段的消息
func ErrorWithFields(ctx context.Context, message string, fields map[string]interface{}) {
	log := LoggerFromContext(ctx)
	log.Error(ctx, message, WithFields(fields)...)
}

// LogHTTPError 记录HTTP错误的便捷函数
// 这个函数专门用于记录HTTP相关的错误信息
func LogHTTPError(c *gin.Context, statusCode int, message string, err error) {
	log := LoggerFromGinContext(c)

	fields := []logger.Field{
		logger.String("method", c.Request.Method),
		logger.String("path", c.Request.URL.Path),
		logger.Int("status_code", statusCode),
		logger.String("client_ip", c.ClientIP()),
		GetCallerInfo(),
	}

	if err != nil {
		fields = append(fields, WithError(err))
	}

	errorMessage := fmt.Sprintf("HTTP Error %d: %s", statusCode, message)

	switch {
	case statusCode >= 500:
		log.Error(c.Request.Context(), errorMessage, fields...)
	case statusCode >= 400:
		log.Warn(c.Request.Context(), errorMessage, fields...)
	default:
		log.Info(c.Request.Context(), errorMessage, fields...)
	}
}

// LogConfigChange 记录配置变更
// 这个函数用于记录系统配置的变更，便于审计和调试
func LogConfigChange(ctx context.Context, component string, oldConfig, newConfig interface{}) {
	log := GetLogger("config")

	fields := []logger.Field{
		logger.String("component", component),
		logger.Any("old_config", oldConfig),
		logger.Any("new_config", newConfig),
		GetCallerInfo(),
	}

	log.Info(ctx, "Configuration changed", fields...)
}

// LogStartup 记录应用程序启动信息
// 这个函数用于记录应用程序启动的详细信息
func LogStartup(appName, version string, additionalFields ...logger.Field) {
	log := GetLogger("startup")

	fields := []logger.Field{
		logger.String("app_name", appName),
		logger.String("version", version),
		logger.String("start_time", time.Now().Format(time.RFC3339)),
		logger.String("go_version", runtime.Version()),
		logger.Int("goroutines", runtime.NumGoroutine()),
	}

	// 添加额外字段
	fields = append(fields, additionalFields...)

	log.Info(context.Background(), "Application starting", fields...)
}

// LogShutdown 记录应用程序关闭信息
// 这个函数用于记录应用程序关闭的详细信息
func LogShutdown(reason string, additionalFields ...logger.Field) {
	log := GetLogger("shutdown")

	fields := []logger.Field{
		logger.String("reason", reason),
		logger.String("shutdown_time", time.Now().Format(time.RFC3339)),
		GetCallerInfo(),
	}

	// 添加额外字段
	fields = append(fields, additionalFields...)

	log.Info(context.Background(), "Application shutting down", fields...)
}

// noopLoggerAdapter 空操作日志记录器适配器，用于兼容性
type noopLoggerAdapter struct{}

func (l *noopLoggerAdapter) Debug(ctx context.Context, message string, fields ...logger.Field) {}
func (l *noopLoggerAdapter) Info(ctx context.Context, message string, fields ...logger.Field)  {}
func (l *noopLoggerAdapter) Warn(ctx context.Context, message string, fields ...logger.Field)  {}
func (l *noopLoggerAdapter) Error(ctx context.Context, message string, fields ...logger.Field) {}
func (l *noopLoggerAdapter) Fatal(ctx context.Context, message string, fields ...logger.Field) {}
func (l *noopLoggerAdapter) Sync() error                                                      { return nil }
func (l *noopLoggerAdapter) WithFields(fields ...logger.Field) logger.Logger                   { return l }
func (l *noopLoggerAdapter) WithModule(module string) logger.Logger                            { return l }
func (l *noopLoggerAdapter) WithCorrelationID(correlationID string) logger.Logger              { return l }

// 需要导入debug包
var debug = struct {
	Stack func() []byte
}{
	Stack: func() []byte {
		buf := make([]byte, 1024)
		for {
			n := runtime.Stack(buf, false)
			if n < len(buf) {
				return buf[:n]
			}
			buf = make([]byte, 2*len(buf))
		}
	},
}

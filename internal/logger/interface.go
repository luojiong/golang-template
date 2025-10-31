package logger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger 定义简化的日志接口，基于 zap 实现
type Logger interface {
	// Debug logs a debug message with context and optional fields
	Debug(ctx context.Context, message string, fields ...Field)

	// Info logs an info message with context and optional fields
	Info(ctx context.Context, message string, fields ...Field)

	// Warn logs a warning message with context and optional fields
	Warn(ctx context.Context, message string, fields ...Field)

	// Error logs an error message with context and optional fields
	Error(ctx context.Context, message string, fields ...Field)

	// Fatal logs a fatal message with context and optional fields, then terminates the application
	Fatal(ctx context.Context, message string, fields ...Field)

	// WithFields returns a new logger with the specified fields always included
	WithFields(fields ...Field) Logger

	// WithModule returns a new logger with the specified module name
	WithModule(module string) Logger

	// WithCorrelationID returns a new logger with the specified correlation ID
	WithCorrelationID(correlationID string) Logger

	// Sync flushes any buffered log entries
	Sync() error
}

// Field represents a key-value pair for structured logging
type Field struct {
	Key   string
	Value interface{}
}

// Helper functions for creating common field types
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

func Float64(key string, value float64) Field {
	return Field{Key: key, Value: value}
}

func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

func Any(key string, value interface{}) Field {
	return Field{Key: key, Value: value}
}

func Error(err error) Field {
	if err == nil {
		return Field{Key: "error", Value: nil}
	}
	return Field{Key: "error", Value: err.Error()}
}

func Stacktrace(key string, stack string) Field {
	return Field{Key: key, Value: stack}
}

// zapLoggerImpl 是基于 zap 的 Logger 实现
type zapLoggerImpl struct {
	logger *zap.Logger
	module string
	fields []Field
}

// NewZapLogger 创建一个新的基于 zap 的日志记录器
func NewZapLogger(zapLogger *zap.Logger) Logger {
	return &zapLoggerImpl{
		logger: zapLogger,
		fields: make([]Field, 0),
	}
}

// Debug logs a debug message with context and optional fields
func (l *zapLoggerImpl) Debug(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, zapcore.DebugLevel, message, fields...)
}

// Info logs an info message with context and optional fields
func (l *zapLoggerImpl) Info(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, zapcore.InfoLevel, message, fields...)
}

// Warn logs a warning message with context and optional fields
func (l *zapLoggerImpl) Warn(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, zapcore.WarnLevel, message, fields...)
}

// Error logs an error message with context and optional fields
func (l *zapLoggerImpl) Error(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, zapcore.ErrorLevel, message, fields...)
}

// Fatal logs a fatal message with context and optional fields, then terminates the application
func (l *zapLoggerImpl) Fatal(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, zapcore.FatalLevel, message, fields...)
}

// WithFields returns a new logger with the specified fields always included
func (l *zapLoggerImpl) WithFields(fields ...Field) Logger {
	// 合并当前字段和新字段
	allFields := make([]Field, 0, len(l.fields)+len(fields))
	allFields = append(allFields, l.fields...)
	allFields = append(allFields, fields...)

	return &zapLoggerImpl{
		logger: l.logger,
		module: l.module,
		fields: allFields,
	}
}

// WithModule returns a new logger with the specified module name
func (l *zapLoggerImpl) WithModule(module string) Logger {
	return &zapLoggerImpl{
		logger: l.logger,
		module: module,
		fields: l.fields,
	}
}

// WithCorrelationID returns a new logger with the specified correlation ID
func (l *zapLoggerImpl) WithCorrelationID(correlationID string) Logger {
	return l.WithFields(String("correlation_id", correlationID))
}

// Sync flushes any buffered log entries
func (l *zapLoggerImpl) Sync() error {
	return l.logger.Sync()
}

// log 是内部日志记录方法
func (l *zapLoggerImpl) log(ctx context.Context, level zapcore.Level, message string, fields ...Field) {
	// 创建 zap 字段
	zapFields := make([]zap.Field, 0, len(l.fields)+len(fields)+2)

	// 添加模块字段
	if l.module != "" {
		zapFields = append(zapFields, zap.String("module", l.module))
	}

	// 添加预设字段
	for _, field := range l.fields {
		zapFields = append(zapFields, l.fieldToZapField(field))
	}

	// 添加方法调用时的字段
	for _, field := range fields {
		zapFields = append(zapFields, l.fieldToZapField(field))
	}

	// 从上下文中提取关联ID
	if correlationID := ctx.Value("correlation_id"); correlationID != nil {
		if id, ok := correlationID.(string); ok {
			zapFields = append(zapFields, zap.String("correlation_id", id))
		}
	}

	// 使用 zap 记录日志
	switch level {
	case zapcore.DebugLevel:
		l.logger.Debug(message, zapFields...)
	case zapcore.InfoLevel:
		l.logger.Info(message, zapFields...)
	case zapcore.WarnLevel:
		l.logger.Warn(message, zapFields...)
	case zapcore.ErrorLevel:
		l.logger.Error(message, zapFields...)
	case zapcore.FatalLevel:
		l.logger.Fatal(message, zapFields...)
	}
}

// fieldToZapField 将自定义 Field 转换为 zap.Field
func (l *zapLoggerImpl) fieldToZapField(field Field) zap.Field {
	switch v := field.Value.(type) {
	case string:
		return zap.String(field.Key, v)
	case int:
		return zap.Int(field.Key, v)
	case int64:
		return zap.Int64(field.Key, v)
	case float64:
		return zap.Float64(field.Key, v)
	case bool:
		return zap.Bool(field.Key, v)
	case []byte:
		return zap.Binary(field.Key, v)
	default:
		return zap.Any(field.Key, v)
	}
}
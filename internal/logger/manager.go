package logger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go-server/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// DateRotatingWriter 按日期轮转的日志写入器
type DateRotatingWriter struct {
	mu          sync.Mutex
	directory   string
	maxAge      int
	compress    bool
	currentDate string
	file        *os.File
}

// NewDateRotatingWriter 创建新的按日期轮转的日志写入器
func NewDateRotatingWriter(directory string, maxAge int, compress bool) *DateRotatingWriter {
	return &DateRotatingWriter{
		directory: directory,
		maxAge:    maxAge,
		compress:  compress,
	}
}

// Write 实现io.Writer接口
func (w *DateRotatingWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 获取当前日期
	today := time.Now().Format("2006-01-02")

	// 如果日期变化了，需要轮转文件
	if today != w.currentDate {
		if err := w.rotate(today); err != nil {
			return 0, err
		}
	}

	// 如果文件还未创建，创建它
	if w.file == nil {
		if err := w.rotate(today); err != nil {
			return 0, err
		}
	}

	return w.file.Write(p)
}

// rotate 轮转日志文件
func (w *DateRotatingWriter) rotate(date string) error {
	// 关闭当前文件
	if w.file != nil {
		w.file.Close()
	}

	// 创建新的日志文件
	filename := filepath.Join(w.directory, fmt.Sprintf("%s.log", date))
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file %s: %w", filename, err)
	}

	w.currentDate = date
	w.file = file

	// 清理旧文件
	go w.cleanOldFiles()

	return nil
}

// cleanOldFiles 清理超过保留期的旧日志文件
func (w *DateRotatingWriter) cleanOldFiles() {
	if w.maxAge <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -w.maxAge)

	files, err := filepath.Glob(filepath.Join(w.directory, "*.log*"))
	if err != nil {
		return
	}

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(file)
		}
	}
}

// Sync 同步缓冲区
func (w *DateRotatingWriter) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Sync()
	}
	return nil
}

// Close 关闭写入器
func (w *DateRotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

// Manager 管理日志记录器实例，简化版本基于 zap
type Manager struct {
	mu         sync.RWMutex
	config     config.LoggingConfig
	zapLogger  *zap.Logger
	logger     Logger
	fileWriter *DateRotatingWriter // 自定义日期轮转写入器
	started    bool
}

// NewManager 创建一个新的日志管理器
func NewManager(cfg config.LoggingConfig) (*Manager, error) {
	var fileWriter *DateRotatingWriter

	// 如果需要文件输出，创建日期轮转写入器
	if cfg.Output == "file" || cfg.Output == "both" {
		fileWriter = NewDateRotatingWriter(cfg.Directory, cfg.MaxAge, cfg.Compress)
	}

	zapLogger, err := buildZapLogger(cfg, fileWriter)
	if err != nil {
		return nil, fmt.Errorf("failed to build zap logger: %w", err)
	}

	logger := NewZapLogger(zapLogger)

	return &Manager{
		config:     cfg,
		zapLogger:  zapLogger,
		logger:     logger,
		fileWriter: fileWriter,
		started:    false,
	}, nil
}

// Start 启动日志管理器
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("logger manager is already started")
	}

	// 确保日志目录存在
	if m.config.Output == "file" || m.config.Output == "both" {
		if err := os.MkdirAll(m.config.Directory, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	m.started = true
	return nil
}

// Stop 停止日志管理器
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil
	}

	// 同步缓冲的日志条目
	if m.zapLogger != nil {
		if err := m.zapLogger.Sync(); err != nil {
			// zap.Sync 可能会返回 "sync: invalid argument" 错误，这是正常的
			fmt.Printf("Warning: failed to sync logger: %v\n", err)
		}
	}

	// 关闭文件写入器
	if m.fileWriter != nil {
		if err := m.fileWriter.Close(); err != nil {
			fmt.Printf("Warning: failed to close file writer: %v\n", err)
		}
	}

	m.started = false
	return nil
}

// GetLogger 返回一个日志记录器实例
func (m *Manager) GetLogger(name string) Logger {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.started {
		// 如果管理器未启动，返回一个空操作的日志记录器
		return &noopLogger{}
	}

	// 返回带有模块名称的日志记录器
	return m.logger.WithModule(name)
}

// UpdateConfig 更新日志配置
func (m *Manager) UpdateConfig(newConfig config.LoggingConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果管理器正在运行，需要先停止
	if m.started {
		// 同步当前日志记录器
		if m.zapLogger != nil {
			m.zapLogger.Sync()
		}
		// 关闭当前的文件写入器
		if m.fileWriter != nil {
			m.fileWriter.Close()
		}
	}

	// 创建新的文件写入器
	var fileWriter *DateRotatingWriter
	if newConfig.Output == "file" || newConfig.Output == "both" {
		fileWriter = NewDateRotatingWriter(newConfig.Directory, newConfig.MaxAge, newConfig.Compress)
	}

	// 构建新的 zap 日志记录器
	newZapLogger, err := buildZapLogger(newConfig, fileWriter)
	if err != nil {
		return fmt.Errorf("failed to build new zap logger: %w", err)
	}

	// 更新配置和日志记录器
	m.config = newConfig
	m.fileWriter = fileWriter
	m.zapLogger = newZapLogger
	m.logger = NewZapLogger(newZapLogger)

	return nil
}

// GetConfig 返回当前的日志配置
func (m *Manager) GetConfig() config.LoggingConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// IsStarted 返回日志管理器是否已启动
func (m *Manager) IsStarted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// buildZapLogger 根据配置构建 zap 日志记录器
func buildZapLogger(cfg config.LoggingConfig, fileWriter *DateRotatingWriter) (*zap.Logger, error) {
	// 解析日志级别
	level, err := parseLogLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}

	var cores []zapcore.Core

	// 根据输出类型创建不同的编码器和核心
	switch cfg.Output {
	case "stdout", "console":
		// 仅控制台输出 - 启用颜色
		encoderConfig := zapcore.EncoderConfig{
			TimeKey:          "timestamp",
			LevelKey:         "level",
			NameKey:          "logger",
			MessageKey:       "message",
			StacktraceKey:    "stacktrace",
			LineEnding:       zapcore.DefaultLineEnding,
			EncodeLevel:      zapcore.CapitalColorLevelEncoder, // 彩色级别
			EncodeTime:       zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
			EncodeDuration:   zapcore.StringDurationEncoder,
			ConsoleSeparator: " ",
		}

		var encoder zapcore.Encoder
		if cfg.Format == "json" {
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		} else {
			encoder = zapcore.NewConsoleEncoder(encoderConfig)
		}

		consoleCore := zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), level)
		cores = append(cores, consoleCore)

	case "file":
		// 仅文件输出 - 不启用颜色
		if fileWriter == nil {
			return nil, fmt.Errorf("file writer is required for file output")
		}

		encoderConfig := zapcore.EncoderConfig{
			TimeKey:          "timestamp",
			LevelKey:         "level",
			NameKey:          "logger",
			MessageKey:       "message",
			StacktraceKey:    "stacktrace",
			LineEnding:       zapcore.DefaultLineEnding,
			EncodeLevel:      zapcore.CapitalLevelEncoder, // 无颜色，但保持大写
			EncodeTime:       zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
			EncodeDuration:   zapcore.StringDurationEncoder,
			ConsoleSeparator: " ",
		}

		var encoder zapcore.Encoder
		if cfg.Format == "json" {
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		} else {
			encoder = zapcore.NewConsoleEncoder(encoderConfig)
		}

		fileCore := zapcore.NewCore(encoder, zapcore.AddSync(fileWriter), level)
		cores = append(cores, fileCore)

	case "both":
		// 同时输出到控制台和文件 - 分别处理
		if fileWriter == nil {
			return nil, fmt.Errorf("file writer is required for both output")
		}

		// 控制台编码器 - 带颜色
		consoleEncoderConfig := zapcore.EncoderConfig{
			TimeKey:          "timestamp",
			LevelKey:         "level",
			NameKey:          "logger",
			MessageKey:       "message",
			StacktraceKey:    "stacktrace",
			LineEnding:       zapcore.DefaultLineEnding,
			EncodeLevel:      zapcore.CapitalColorLevelEncoder, // 彩色级别
			EncodeTime:       zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
			EncodeDuration:   zapcore.StringDurationEncoder,
			ConsoleSeparator: " ",
		}

		var consoleEncoder zapcore.Encoder
		if cfg.Format == "json" {
			consoleEncoder = zapcore.NewJSONEncoder(consoleEncoderConfig)
		} else {
			consoleEncoder = zapcore.NewConsoleEncoder(consoleEncoderConfig)
		}

		// 文件编码器 - 不带颜色
		fileEncoderConfig := zapcore.EncoderConfig{
			TimeKey:          "timestamp",
			LevelKey:         "level",
			NameKey:          "logger",
			MessageKey:       "message",
			StacktraceKey:    "stacktrace",
			LineEnding:       zapcore.DefaultLineEnding,
			EncodeLevel:      zapcore.CapitalLevelEncoder, // 无颜色，但保持大写
			EncodeTime:       zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
			EncodeDuration:   zapcore.StringDurationEncoder,
			ConsoleSeparator: " ",
		}

		var fileEncoder zapcore.Encoder
		if cfg.Format == "json" {
			fileEncoder = zapcore.NewJSONEncoder(fileEncoderConfig)
		} else {
			fileEncoder = zapcore.NewConsoleEncoder(fileEncoderConfig)
		}

		// 创建两个核心
		consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), level)
		fileCore := zapcore.NewCore(fileEncoder, zapcore.AddSync(fileWriter), level)
		cores = append(cores, consoleCore, fileCore)

	default:
		return nil, fmt.Errorf("unsupported output type: %s", cfg.Output)
	}

	// 使用 teeCore 合并多个核心
	var core zapcore.Core
	if len(cores) == 1 {
		core = cores[0]
	} else {
		core = zapcore.NewTee(cores...)
	}

	// 创建日志记录器
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return logger, nil
}

// parseLogLevel 解析日志级别字符串
func parseLogLevel(levelStr string) (zapcore.Level, error) {
	switch levelStr {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	case "fatal":
		return zapcore.FatalLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("unsupported log level: %s", levelStr)
	}
}

// noopLogger 是一个空操作的日志记录器实现
type noopLogger struct{}

func (l *noopLogger) Debug(ctx context.Context, message string, fields ...Field) {}
func (l *noopLogger) Info(ctx context.Context, message string, fields ...Field)  {}
func (l *noopLogger) Warn(ctx context.Context, message string, fields ...Field)  {}
func (l *noopLogger) Error(ctx context.Context, message string, fields ...Field) {}
func (l *noopLogger) Fatal(ctx context.Context, message string, fields ...Field) {}

func (l *noopLogger) WithFields(fields ...Field) Logger             { return l }
func (l *noopLogger) WithModule(module string) Logger               { return l }
func (l *noopLogger) WithCorrelationID(correlationID string) Logger { return l }
func (l *noopLogger) Sync() error                                   { return nil }

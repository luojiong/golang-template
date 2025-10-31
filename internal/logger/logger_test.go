package logger

import (
	"context"
	"os"
	"testing"
	"time"

	"go-server/internal/config"
)

func TestNewManager(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:      "debug",
		Format:     "json",
		Output:     "stdout",
		Directory:  "test_logs",
		MaxSize:    10,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
	}

	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger manager: %v", err)
	}

	// Test starting the manager
	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start logger manager: %v", err)
	}

	// Test getting a logger
	logger := manager.GetLogger("test-module")
	if logger == nil {
		t.Fatal("Failed to get logger")
	}

	// Test logging different levels
	ctx := context.Background()
	logger.Debug(ctx, "Debug message", String("key", "value"))
	logger.Info(ctx, "Info message", Int("number", 42))
	logger.Warn(ctx, "Warning message", Bool("flag", true))
	logger.Error(ctx, "Error message", Error(nil))

	// Test with fields
	fieldLogger := logger.WithFields(
		String("module", "test"),
		Int64("timestamp", time.Now().Unix()),
	)
	fieldLogger.Info(ctx, "Message with fields")

	// Test with correlation ID
	correlationLogger := logger.WithCorrelationID("test-correlation-123")
	correlationLogger.Info(ctx, "Message with correlation ID")

	// Test with module
	moduleLogger := logger.WithModule("custom-module")
	moduleLogger.Info(ctx, "Message from custom module")

	// Test stopping the manager
	if err := manager.Stop(); err != nil {
		t.Fatalf("Failed to stop logger manager: %v", err)
	}
}

func TestNewManagerWithFileOutput(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:      "info",
		Format:     "text",
		Output:     "file",
		Directory:  "test_logs",
		MaxSize:    1,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
	}

	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger manager: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start logger manager: %v", err)
	}

	logger := manager.GetLogger("test-file")
	logger.Info(context.Background(), "This message goes to file")

	// Clean up
	manager.Stop()
	os.RemoveAll("test_logs")
}

func TestNewManagerWithBothOutput(t *testing.T) {
	cfg := config.LoggingConfig{
		Level:      "debug",
		Format:     "json",
		Output:     "both",
		Directory:  "test_logs",
		MaxSize:    1,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
	}

	manager, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("Failed to create logger manager: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start logger manager: %v", err)
	}

	logger := manager.GetLogger("test-both")
	logger.Info(context.Background(), "This message goes to both console and file")

	// Clean up
	manager.Stop()
	os.RemoveAll("test_logs")
}

func TestLogLevels(t *testing.T) {
	testCases := []struct {
		level   string
		wantErr bool
	}{
		{"debug", false},
		{"info", false},
		{"warn", false},
		{"error", false},
		{"fatal", false},
		{"invalid", true},
	}

	for _, tc := range testCases {
		cfg := config.LoggingConfig{
			Level:      tc.level,
			Format:     "json",
			Output:     "stdout",
			Directory:  "test_logs",
			MaxSize:    1,
			MaxBackups: 1,
			MaxAge:     1,
			Compress:   false,
		}

		manager, err := NewManager(cfg)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Expected error for invalid level %s, got nil", tc.level)
			}
			continue
		}

		if err != nil {
			t.Errorf("Failed to create manager with level %s: %v", tc.level, err)
			continue
		}

		if err := manager.Start(); err != nil {
			t.Errorf("Failed to start manager with level %s: %v", tc.level, err)
			manager.Stop()
			continue
		}

		logger := manager.GetLogger("test-levels")
		ctx := context.Background()

		switch tc.level {
		case "debug":
			logger.Debug(ctx, "Debug message")
			fallthrough
		case "info":
			logger.Info(ctx, "Info message")
			fallthrough
		case "warn":
			logger.Warn(ctx, "Warning message")
			fallthrough
		case "error":
			logger.Error(ctx, "Error message")
			fallthrough
		case "fatal":
			// Note: Fatal calls os.Exit, so we can't test it here
		}

		manager.Stop()
	}
}

func TestUpdateConfig(t *testing.T) {
	initialCfg := config.LoggingConfig{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		Directory:  "test_logs",
		MaxSize:    10,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
	}

	manager, err := NewManager(initialCfg)
	if err != nil {
		t.Fatalf("Failed to create logger manager: %v", err)
	}

	if err := manager.Start(); err != nil {
		t.Fatalf("Failed to start logger manager: %v", err)
	}

	// Test updating configuration
	newCfg := config.LoggingConfig{
		Level:      "debug",
		Format:     "text",
		Output:     "stdout",
		Directory:  "test_logs",
		MaxSize:    10,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
	}

	if err := manager.UpdateConfig(newCfg); err != nil {
		t.Errorf("Failed to update config: %v", err)
	}

	// Verify config was updated
	currentCfg := manager.GetConfig()
	if currentCfg.Level != "debug" || currentCfg.Format != "text" {
		t.Errorf("Config not updated correctly: got level=%s, format=%s", currentCfg.Level, currentCfg.Format)
	}

	manager.Stop()
	os.RemoveAll("test_logs")
}

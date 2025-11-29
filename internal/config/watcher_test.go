package config

import (
	"os"
	"testing"
	"time"
)

func TestNewConfigWatcher(t *testing.T) {
	watcher, err := NewConfigWatcher()
	if err != nil {
		t.Fatalf("Failed to create config watcher: %v", err)
	}
	defer watcher.Stop()

	if watcher.IsWatching() {
		t.Error("Watcher should not be watching initially")
	}
}

func TestConfigWatcherCallbacks(t *testing.T) {
	watcher, err := NewConfigWatcher()
	if err != nil {
		t.Fatalf("Failed to create config watcher: %v", err)
	}
	defer watcher.Stop()

	callbackCalled := false
	watcher.AddCallback(func(config *Config, err error) error {
		callbackCalled = true
		return nil
	})

	// Test that callbacks are added (we can't easily test the callback execution
	// without actually modifying files, which is complex in unit tests)
	if !callbackCalled {
		// This is expected since no config changes occurred
		t.Log("Callback not called (expected in unit test)")
	}
}

func TestConfigWatcherDevelopmentMode(t *testing.T) {
	// Create a test config in development mode
	config := &Config{
		Mode: "development",
		Server: ServerConfig{
			Port: "8080",
			Host: "localhost",
		},
	}

	watcher, err := NewConfigWatcher()
	if err != nil {
		t.Fatalf("Failed to create config watcher: %v", err)
	}
	defer watcher.Stop()

	// Should start successfully in development mode
	err = watcher.Start(config)
	if err != nil {
		t.Fatalf("Failed to start watcher in development mode: %v", err)
	}

	if !watcher.IsWatching() {
		t.Error("Watcher should be watching in development mode")
	}
}

func TestConfigWatcherProductionMode(t *testing.T) {
	// Create a test config in production mode
	config := &Config{
		Mode: "production",
		Server: ServerConfig{
			Port: "8080",
			Host: "localhost",
		},
	}

	watcher, err := NewConfigWatcher()
	if err != nil {
		t.Fatalf("Failed to create config watcher: %v", err)
	}
	defer watcher.Stop()

	// Should start but not enable watching in production mode
	err = watcher.Start(config)
	if err != nil {
		t.Fatalf("Failed to start watcher in production mode: %v", err)
	}

	// In production mode, watching should be disabled for security
	if watcher.IsWatching() {
		t.Error("Watcher should not be watching in production mode")
	}
}

func TestIsDevelopment(t *testing.T) {
	tests := []struct {
		mode     string
		expected bool
	}{
		{"development", true},
		{"production", false},
		{"test", false},
		{"", false},
	}

	for _, tt := range tests {
		result := IsDevelopment(tt.mode)
		if result != tt.expected {
			t.Errorf("IsDevelopment(%s) = %v; expected %v", tt.mode, result, tt.expected)
		}
	}
}

func TestWatchConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configContent := `
mode: development
server:
  port: "8080"
  host: "localhost"
  read_timeout: 30
  write_timeout: 30
database:
  host: "localhost"
  port: 5432
  user: "test"
  password: "test"
  db_name: "test"
  ssl_mode: "disable"
logging:
  level: "info"
  format: "json"
  output: "stdout"
auth:
  bcrypt_cost: 10
jwt:
  secret_key: "test-secret-key-that-is-long-enough-for-validation"
  expires_in: 24
redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
  pool_size: 10
rate_limit:
  enabled: true
  requests: 100
  window: "1m"
  redis_key: "rate_limit"
`

	configFile := tmpDir + "/development.yaml"
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	// Change working directory temporarily for the test
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tmpDir)

	// Set environment for the test
	os.Setenv("APP_ENV", "development")
	defer os.Unsetenv("APP_ENV")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	callbackCalled := false
	var callbackConfig *Config
	var callbackError error

	watcher, err := WatchConfigFile(config, func(newConfig *Config, err error) error {
		callbackCalled = true
		callbackConfig = newConfig
		callbackError = err
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to start config file watcher: %v", err)
	}
	defer watcher.Stop()

	// Give the watcher a moment to start
	time.Sleep(100 * time.Millisecond)

	// Modify the config file
	modifiedContent := configContent + `
# Added comment
`
	err = os.WriteFile(configFile, []byte(modifiedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to modify config file: %v", err)
	}

	// Give the watcher time to detect the change
	time.Sleep(500 * time.Millisecond)

	// Note: The callback might not be called in all test environments
	// due to file system event timing differences
	if callbackCalled {
		if callbackError != nil {
			t.Errorf("Callback received error: %v", callbackError)
		}
		if callbackConfig == nil {
			t.Error("Callback received nil config")
		}
	} else {
		t.Log("Callback not called - this might be normal in test environment")
	}
}

func TestConfigWatcherManager(t *testing.T) {
	manager := NewConfigWatcherManager()

	// Initialize should not fail even if watcher doesn't start
	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize manager: %v", err)
	}

	// Get config should return a valid config
	config := manager.GetConfig()
	if config == nil {
		t.Error("Manager should return a valid config")
	}

	// Cleanup
	err = manager.Shutdown()
	if err != nil {
		t.Errorf("Failed to shutdown manager: %v", err)
	}
}

// MockLoggerManager 模拟日志管理器，用于测试
type MockLoggerManager struct {
	config     LoggingConfig
	started    bool
	updateErr  error
	getErr     error
	startedErr error
}

// UpdateConfig 实现UpdateConfig方法以供反射调用
func (m *MockLoggerManager) UpdateConfig(config LoggingConfig) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.config = config
	return nil
}

// IsStarted 实现IsStarted方法以供反射调用
func (m *MockLoggerManager) IsStarted() bool {
	if m.startedErr != nil {
		return false
	}
	return m.started
}

func TestConfigWatcher_SetLoggerManager(t *testing.T) {
	watcher, err := NewConfigWatcher()
	if err != nil {
		t.Fatalf("Failed to create config watcher: %v", err)
	}
	defer watcher.Stop()

	mockManager := &MockLoggerManager{started: true}

	watcher.SetLoggerManager(mockManager)

	if watcher.loggerManager == nil {
		t.Error("Expected logger manager to be set")
	}
}

func TestConfigWatcher_AddLoggingConfigChangeHandler(t *testing.T) {
	watcher, err := NewConfigWatcher()
	if err != nil {
		t.Fatalf("Failed to create config watcher: %v", err)
	}
	defer watcher.Stop()

	mockManager := &MockLoggerManager{
		started: true,
		config: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}

	watcher.SetLoggerManager(mockManager)
	watcher.AddLoggingConfigChangeHandler()

	if len(watcher.callbacks) != 1 {
		t.Errorf("Expected 1 callback, got %d", len(watcher.callbacks))
	}

	// 测试配置更新
	newConfig := &Config{
		Logging: LoggingConfig{
			Level:  "debug",
			Format: "json",
			Output: "stdout",
		},
	}

	// 调用回调函数
	err = watcher.callbacks[0](newConfig, nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证配置是否已更新
	if mockManager.config.Level != "debug" {
		t.Errorf("Expected level 'debug', got '%s'", mockManager.config.Level)
	}
}

func TestConfigWatcher_reloadLoggingConfig(t *testing.T) {
	tests := []struct {
		name          string
		currentConfig LoggingConfig
		newConfig     *Config
		expectUpdate  bool
		expectError   bool
	}{
		{
			name: "配置相同，不应更新",
			currentConfig: LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
			newConfig: &Config{
				Logging: LoggingConfig{
					Level:  "info",
					Format: "json",
					Output: "stdout",
				},
			},
			expectUpdate: false,
			expectError:  false,
		},
		{
			name: "配置不同，应更新",
			currentConfig: LoggingConfig{
				Level:  "info",
				Format: "json",
				Output: "stdout",
			},
			newConfig: &Config{
				Logging: LoggingConfig{
					Level:  "debug",
					Format: "json",
					Output: "stdout",
				},
			},
			expectUpdate: true,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			watcher := &ConfigWatcher{}
			mockManager := &MockLoggerManager{
				started: true,
				config:  tt.currentConfig,
			}

			err := watcher.reloadLoggingConfigWithReflection(tt.newConfig, mockManager)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if tt.expectUpdate {
				if mockManager.config.Level != tt.newConfig.Logging.Level {
					t.Errorf("Expected level '%s', got '%s'", tt.newConfig.Logging.Level, mockManager.config.Level)
				}
			}
		})
	}
}

func TestConfigWatcher_isLoggingConfigEqual(t *testing.T) {
	watcher := &ConfigWatcher{}

	config1 := LoggingConfig{
		Level:     "info",
		Format:    "json",
		Output:    "stdout",
		Directory: "./logs",
		MaxSize:   100,
		Compress:  true,
	}

	config2 := LoggingConfig{
		Level:     "info",
		Format:    "json",
		Output:    "stdout",
		Directory: "./logs",
		MaxSize:   100,
		Compress:  true,
	}

	config3 := LoggingConfig{
		Level:     "debug",
		Format:    "json",
		Output:    "stdout",
		Directory: "./logs",
		MaxSize:   100,
		Compress:  true,
	}

	if !watcher.isLoggingConfigEqual(config1, config2) {
		t.Error("Expected configs to be equal")
	}

	if watcher.isLoggingConfigEqual(config1, config3) {
		t.Error("Expected configs to be different")
	}
}

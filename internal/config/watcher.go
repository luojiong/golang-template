package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigChangeCallback 配置变更时调用的函数类型
type ConfigChangeCallback func(*Config, error) error

// ConfigWatcher 监控配置文件变更并重新加载
type ConfigWatcher struct {
	watcher       *fsnotify.Watcher          // 文件监控器
	config        *Config                    // 当前配置
	callbacks     []ConfigChangeCallback     // 变更回调函数列表
	stopCh        chan struct{}              // 停止通道
	mu            sync.RWMutex               // 读写锁
	enabled       bool                       // 是否启用
	loggerManager interface{}                // 日志管理器引用（使用interface{}避免循环导入）
}

// NewConfigWatcher 创建新的配置文件监控器
func NewConfigWatcher() (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("创建文件监控器失败: %w", err)
	}

	return &ConfigWatcher{
		watcher:   watcher,
		stopCh:    make(chan struct{}),
		callbacks: make([]ConfigChangeCallback, 0),
		enabled:   false,
	}, nil
}

// AddCallback 添加配置变更时的回调函数
func (cw *ConfigWatcher) AddCallback(callback ConfigChangeCallback) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.callbacks = append(cw.callbacks, callback)
}

// SetConfig 设置当前配置实例
func (cw *ConfigWatcher) SetConfig(config *Config) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.config = config
}

// SetLoggerManager 设置日志管理器引用
func (cw *ConfigWatcher) SetLoggerManager(loggerManager interface{}) {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	cw.loggerManager = loggerManager
}

// AddLoggingConfigChangeHandler 添加日志配置变更处理器
func (cw *ConfigWatcher) AddLoggingConfigChangeHandler() {
	cw.AddCallback(func(config *Config, err error) error {
		if err != nil {
			// 配置加载失败时记录错误
			log.Printf("配置重新加载失败，无法更新日志配置: %v", err)
			return nil
		}

		if config == nil {
			log.Printf("配置为空，无法更新日志配置")
			return nil
		}

		// 获取日志管理器引用
		cw.mu.RLock()
		loggerManager := cw.loggerManager
		cw.mu.RUnlock()

		if loggerManager == nil {
			log.Printf("日志管理器未设置，无法更新日志配置")
			return nil
		}

		// 使用反射调用日志管理器的方法
		if reloadErr := cw.reloadLoggingConfigWithReflection(config, loggerManager); reloadErr != nil {
			log.Printf("日志配置热重载失败: %v", reloadErr)
			// 尝试记录配置变更事件
			cw.logConfigChangeEvent(config.Logging, reloadErr)
			return nil
		}

		// 记录成功的配置变更事件
		cw.logConfigChangeEvent(config.Logging, nil)
		log.Printf("日志配置热重载成功")

		return nil
	})
}

// Start starts watching configuration files for changes
func (cw *ConfigWatcher) Start(config *Config) error {
	// Only enable watching in development mode for security
	if !IsDevelopment(config.Mode) {
		log.Printf("Configuration file watching is disabled in %s mode for security", config.Mode)
		return nil
	}

	cw.mu.Lock()
	if cw.enabled {
		cw.mu.Unlock()
		return fmt.Errorf("watcher is already started")
	}
	cw.enabled = true
	cw.config = config
	cw.mu.Unlock()

	// Get configuration file paths
	configPaths := cw.getConfigPaths()

	// Add each configuration file to the watcher
	for _, configPath := range configPaths {
		if _, err := os.Stat(configPath); err == nil {
			// Watch the file directly
			err := cw.watcher.Add(configPath)
			if err != nil {
				log.Printf("Failed to watch config file %s: %v", configPath, err)
				continue
			}
			log.Printf("Watching configuration file: %s", configPath)
		} else {
			log.Printf("Configuration file not found: %s", configPath)
		}
	}

	// Also watch the config directory for new files
	configDir := "./configs"
	if _, err := os.Stat(configDir); err == nil {
		err := cw.watcher.Add(configDir)
		if err != nil {
			log.Printf("Failed to watch config directory %s: %v", configDir, err)
		} else {
			log.Printf("Watching configuration directory: %s", configDir)
		}
	}

	// Start the event loop
	go cw.eventLoop()

	log.Printf("Configuration file watching started in development mode")
	return nil
}

// Stop stops watching configuration files
func (cw *ConfigWatcher) Stop() {
	cw.mu.Lock()
	if !cw.enabled {
		cw.mu.Unlock()
		return
	}
	cw.enabled = false
	cw.mu.Unlock()

	close(cw.stopCh)

	if err := cw.watcher.Close(); err != nil {
		log.Printf("Error closing config watcher: %v", err)
	}

	log.Printf("Configuration file watching stopped")
}

// IsWatching returns true if the watcher is currently active
func (cw *ConfigWatcher) IsWatching() bool {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.enabled
}

// getConfigPaths returns the paths to configuration files to watch
func (cw *ConfigWatcher) getConfigPaths() []string {
	var paths []string

	// Get current environment
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	// Look for configuration files in standard paths
	configDirs := []string{"./configs", "./"}

	for _, dir := range configDirs {
		configFile := filepath.Join(dir, env+".yaml")
		if _, err := os.Stat(configFile); err == nil {
			paths = append(paths, configFile)
		}

		// Also watch common alternative names
		altConfigFile := filepath.Join(dir, env+".yml")
		if _, err := os.Stat(altConfigFile); err == nil {
			paths = append(paths, altConfigFile)
		}
	}

	return paths
}

// eventLoop processes file system events
func (cw *ConfigWatcher) eventLoop() {
	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			cw.handleEvent(event)

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Config watcher error: %v", err)

		case <-cw.stopCh:
			return
		}
	}
}

// handleEvent handles a single file system event
func (cw *ConfigWatcher) handleEvent(event fsnotify.Event) {
	// Check if this is a configuration file we care about
	if !cw.isConfigFile(event.Name) {
		return
	}

	// Handle different event types
	switch {
	case event.Op&fsnotify.Write == fsnotify.Write:
		log.Printf("Configuration file modified: %s", event.Name)
		cw.reloadConfig()

	case event.Op&fsnotify.Create == fsnotify.Create:
		log.Printf("Configuration file created: %s", event.Name)
		// If it's a new config file, start watching it
		if cw.isConfigFile(event.Name) {
			err := cw.watcher.Add(event.Name)
			if err != nil {
				log.Printf("Failed to watch new config file %s: %v", event.Name, err)
			} else {
				log.Printf("Started watching new configuration file: %s", event.Name)
			}
		}
		cw.reloadConfig()

	case event.Op&fsnotify.Remove == fsnotify.Remove:
		log.Printf("Configuration file removed: %s", event.Name)
		// Note: We don't try to reload when a file is removed
		// as it would likely fail. The application will continue
		// with the last loaded configuration.

	case event.Op&fsnotify.Rename == fsnotify.Rename:
		log.Printf("Configuration file renamed: %s", event.Name)
		// Try to reload as this might be a file replacement
		cw.reloadConfig()
	}
}

// isConfigFile checks if a file is a configuration file we should watch
func (cw *ConfigWatcher) isConfigFile(filename string) bool {
	base := filepath.Base(filename)

	// Check for standard config file names and extensions
	configNames := []string{"development.yaml", "production.yaml", "test.yaml"}
	configExts := []string{".yaml", ".yml"}

	// Check exact matches
	for _, name := range configNames {
		if base == name {
			return true
		}
	}

	// Check extension matches
	for _, configExt := range configExts {
		if filepath.Ext(filename) == configExt {
			// Additional check: make sure it's in a config directory
			dir := filepath.Dir(filename)
			if dir == "./configs" || dir == "." {
				return true
			}
		}
	}

	return false
}

// reloadConfig reloads the configuration from files
func (cw *ConfigWatcher) reloadConfig() {
	cw.mu.RLock()
	config := cw.config
	callbacks := make([]ConfigChangeCallback, len(cw.callbacks))
	copy(callbacks, cw.callbacks)
	cw.mu.RUnlock()

	if config == nil {
		log.Printf("No configuration instance available for reload")
		return
	}

	// Reload configuration using viper
	newConfig, err := LoadConfig()
	if err != nil {
		log.Printf("Failed to reload configuration: %v", err)

		// Notify callbacks about the error
		for _, callback := range callbacks {
			if err := callback(nil, err); err != nil {
				log.Printf("Config reload callback error: %v", err)
			}
		}
		return
	}

	// Validate the new configuration
	validator := NewValidator(newConfig)
	validationResult := validator.Validate()

	if !validationResult.Valid {
		log.Printf("Reloaded configuration is invalid: %s", validationResult.FormatErrors())

		// Notify callbacks about validation error
		callbackErr := fmt.Errorf("configuration validation failed: %s", validationResult.FormatErrors())
		for _, callback := range callbacks {
			if err := callback(nil, callbackErr); err != nil {
				log.Printf("Config reload callback error: %v", err)
			}
		}
		return
	}

	// Update the internal config reference
	cw.mu.Lock()
	cw.config = newConfig
	cw.mu.Unlock()

	log.Printf("Configuration successfully reloaded")

	// Notify all callbacks about the successful reload
	for _, callback := range callbacks {
		if err := callback(newConfig, nil); err != nil {
			log.Printf("Config reload callback error: %v", err)
		}
	}
}

// GetConfig returns the current configuration instance
func (cw *ConfigWatcher) GetConfig() *Config {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.config
}

// reloadLoggingConfigWithReflection 使用反射执行日志配置热重载
func (cw *ConfigWatcher) reloadLoggingConfigWithReflection(config *Config, loggerManager interface{}) error {
	// 使用反射获取日志管理器的方法
	managerValue := reflect.ValueOf(loggerManager)
	if managerValue.IsNil() {
		return fmt.Errorf("日志管理器为空")
	}

	// 检查IsStarted方法
	isStartedMethod := managerValue.MethodByName("IsStarted")
	if !isStartedMethod.IsValid() {
		log.Printf("日志管理器没有IsStarted方法，跳过启动检查")
	} else {
		// 调用IsStarted方法
		results := isStartedMethod.Call(nil)
		if len(results) > 0 {
			if started := results[0].Bool(); !started {
				log.Printf("日志管理器未启动，无法更新日志配置")
				return fmt.Errorf("日志管理器未启动")
			}
		}
	}

	// 检查UpdateConfig方法
	updateConfigMethod := managerValue.MethodByName("UpdateConfig")
	if !updateConfigMethod.IsValid() {
		return fmt.Errorf("日志管理器没有UpdateConfig方法")
	}

	// 调用UpdateConfig方法
	results := updateConfigMethod.Call([]reflect.Value{reflect.ValueOf(config.Logging)})
	if len(results) > 0 {
		if err := results[0].Interface(); err != nil {
			return fmt.Errorf("更新日志配置失败: %v", err)
		}
	}

	return nil
}

// isLoggingConfigEqual 比较两个日志配置是否相等
func (cw *ConfigWatcher) isLoggingConfigEqual(oldConfig, newConfig LoggingConfig) bool {
	return oldConfig.Level == newConfig.Level &&
		oldConfig.Format == newConfig.Format &&
		oldConfig.Output == newConfig.Output &&
		oldConfig.Directory == newConfig.Directory &&
		oldConfig.MaxSize == newConfig.MaxSize &&
		oldConfig.MaxBackups == newConfig.MaxBackups &&
		oldConfig.MaxAge == newConfig.MaxAge &&
		oldConfig.Compress == newConfig.Compress
}

// logConfigChangeDetails 记录配置变更的详细信息
func (cw *ConfigWatcher) logConfigChangeDetails(oldConfig, newConfig LoggingConfig) {
	changes := make([]string, 0)

	if oldConfig.Level != newConfig.Level {
		changes = append(changes, fmt.Sprintf("级别: %s -> %s", oldConfig.Level, newConfig.Level))
	}
	if oldConfig.Format != newConfig.Format {
		changes = append(changes, fmt.Sprintf("格式: %s -> %s", oldConfig.Format, newConfig.Format))
	}
	if oldConfig.Output != newConfig.Output {
		changes = append(changes, fmt.Sprintf("输出: %s -> %s", oldConfig.Output, newConfig.Output))
	}
	if oldConfig.Directory != newConfig.Directory {
		changes = append(changes, fmt.Sprintf("目录: %s -> %s", oldConfig.Directory, newConfig.Directory))
	}
	if oldConfig.MaxSize != newConfig.MaxSize {
		changes = append(changes, fmt.Sprintf("最大文件大小: %d -> %d", oldConfig.MaxSize, newConfig.MaxSize))
	}
	if oldConfig.MaxBackups != newConfig.MaxBackups {
		changes = append(changes, fmt.Sprintf("最大备份数: %d -> %d", oldConfig.MaxBackups, newConfig.MaxBackups))
	}
	if oldConfig.MaxAge != newConfig.MaxAge {
		changes = append(changes, fmt.Sprintf("最大保存天数: %d -> %d", oldConfig.MaxAge, newConfig.MaxAge))
	}
	if oldConfig.Compress != newConfig.Compress {
		changes = append(changes, fmt.Sprintf("压缩: %t -> %t", oldConfig.Compress, newConfig.Compress))
	}

	if len(changes) > 0 {
		log.Printf("检测到日志配置变更: %s", strings.Join(changes, ", "))
	}
}

// logConfigChangeEvent 记录配置变更事件
func (cw *ConfigWatcher) logConfigChangeEvent(config LoggingConfig, err error) {
	event := map[string]interface{}{
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
		"event_type":    "logging_config_change",
		"config":        map[string]interface{}{
			"level":         config.Level,
			"format":        config.Format,
			"output":        config.Output,
			"directory":     config.Directory,
			"max_size":      config.MaxSize,
			"max_backups":   config.MaxBackups,
			"max_age":       config.MaxAge,
			"compress":      config.Compress,
		},
		"change_result": "success",
	}

	if err != nil {
		event["change_result"] = "failed"
		event["error"] = err.Error()
		log.Printf("日志配置变更事件: %+v, 错误: %v", event, err)
	} else {
		log.Printf("日志配置变更事件: %+v", event)
	}
}

// WatchConfigFile starts watching configuration files with a simple callback
// This is a convenience function for basic use cases
func WatchConfigFile(config *Config, onReload func(*Config, error) error) (*ConfigWatcher, error) {
	watcher, err := NewConfigWatcher()
	if err != nil {
		return nil, err
	}

	if onReload != nil {
		watcher.AddCallback(onReload)
	}

	if err := watcher.Start(config); err != nil {
		watcher.Stop()
		return nil, err
	}

	return watcher, nil
}

// WatchConfigFileWithLogger starts watching configuration files with logger manager support
// This is a convenience function that automatically sets up logging configuration hot reload
func WatchConfigFileWithLogger(config *Config, loggerManager interface{}) (*ConfigWatcher, error) {
	watcher, err := NewConfigWatcher()
	if err != nil {
		return nil, err
	}

	// 设置日志管理器引用
	watcher.SetLoggerManager(loggerManager)

	// 添加日志配置变更处理器
	watcher.AddLoggingConfigChangeHandler()

	if err := watcher.Start(config); err != nil {
		watcher.Stop()
		return nil, err
	}

	return watcher, nil
}
package config

import (
	"log"
	"time"
)

// ExampleConfigWatcher demonstrates how to use the configuration watcher
// This is an example implementation showing the expected usage pattern
func ExampleConfigWatcher() {
	// Load initial configuration
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load initial configuration: %v", err)
	}

	// Create a new watcher
	watcher, err := NewConfigWatcher()
	if err != nil {
		log.Fatalf("Failed to create config watcher: %v", err)
	}
	defer watcher.Stop()

	// Add a callback function to handle configuration changes
	watcher.AddCallback(func(newConfig *Config, reloadErr error) error {
		if reloadErr != nil {
			log.Printf("Configuration reload failed: %v", reloadErr)
			// Here you might want to:
			// - Send an alert to monitoring system
			// - Continue with the old configuration
			// - Implement fallback logic
			return nil
		}

		if newConfig == nil {
			log.Printf("Received nil configuration during reload")
			return nil
		}

		log.Printf("Configuration reloaded successfully!")
		log.Printf("New server port: %s", newConfig.Server.Port)
		log.Printf("New log level: %s", newConfig.Logging.Level)
		log.Printf("New database host: %s", newConfig.Database.Host)

		// Here you would typically:
		// - Update database connections if needed
		// - Update logging configuration
		// - Update rate limiting settings
		// - Refresh any cached configuration values
		// - Notify other parts of the application

		return nil
	})

	// Start watching
	if err := watcher.Start(config); err != nil {
		log.Fatalf("Failed to start config watcher: %v", err)
	}

	// Keep the application running
	log.Printf("Application started with configuration file watching enabled")
	log.Printf("Modify the configuration file to see hot-reloading in action")

	// In a real application, this would be your main application loop
	select {}
}

// StartConfigWatcherWithGracefulShutdown demonstrates starting the watcher
// with graceful shutdown handling
func StartConfigWatcherWithGracefulShutdown(config *Config) (*ConfigWatcher, error) {
	watcher, err := NewConfigWatcher()
	if err != nil {
		return nil, err
	}

	// Add a comprehensive callback
	watcher.AddCallback(func(newConfig *Config, reloadErr error) error {
		if reloadErr != nil {
			log.Printf("Configuration reload error: %v", reloadErr)
			// In production, you might want to:
			// - Increment a metric for monitoring
			// - Send an alert if critical settings failed to reload
			return reloadErr
		}

		if newConfig == nil {
			log.Printf("Warning: Received nil configuration during reload")
			return nil
		}

		// Validate critical settings that might require immediate action
		if newConfig.Server.Port != config.Server.Port {
			log.Printf("Server port changed from %s to %s", config.Server.Port, newConfig.Server.Port)
			// Note: Port changes typically require server restart
			log.Printf("WARNING: Server port change requires application restart")
		}

		if newConfig.Logging.Level != config.Logging.Level {
			log.Printf("Log level changed from %s to %s", config.Logging.Level, newConfig.Logging.Level)
			// Update logging level dynamically
		}

		if newConfig.Database.Host != config.Database.Host ||
		   newConfig.Database.Port != config.Database.Port {
			log.Printf("Database configuration changed")
			log.Printf("New database: %s:%d", newConfig.Database.Host, newConfig.Database.Port)
			// Database changes typically require reconnection
			log.Printf("WARNING: Database changes may require reconnection")
		}

		// Update the reference config for next comparison
		*config = *newConfig

		return nil
	})

	// Start the watcher
	if err := watcher.Start(config); err != nil {
		watcher.Stop()
		return nil, err
	}

	return watcher, nil
}

// ConfigWatcherManager manages the lifecycle of a configuration watcher
type ConfigWatcherManager struct {
	watcher *ConfigWatcher
	config  *Config
}

// NewConfigWatcherManager creates a new watcher manager
func NewConfigWatcherManager() *ConfigWatcherManager {
	return &ConfigWatcherManager{}
}

// Initialize loads initial configuration and starts watching
func (cwm *ConfigWatcherManager) Initialize() error {
	// Load initial configuration
	config, err := LoadConfig()
	if err != nil {
		return err
	}
	cwm.config = config

	// Start watcher only in development mode
	watcher, err := StartConfigWatcherWithGracefulShutdown(config)
	if err != nil {
		// In production, we don't want to fail if watching fails
		if IsDevelopment(config.Mode) {
			return err
		}
		log.Printf("Configuration watching disabled (production mode): %v", err)
		return nil
	}

	cwm.watcher = watcher
	return nil
}

// Shutdown gracefully stops the configuration watcher
func (cwm *ConfigWatcherManager) Shutdown() error {
	if cwm.watcher != nil {
		cwm.watcher.Stop()
		cwm.watcher = nil
	}
	return nil
}

// GetConfig returns the current configuration
func (cwm *ConfigWatcherManager) GetConfig() *Config {
	if cwm.watcher != nil {
		return cwm.watcher.GetConfig()
	}
	return cwm.config
}

// IsWatching returns true if the watcher is active
func (cwm *ConfigWatcherManager) IsWatching() bool {
	return cwm.watcher != nil && cwm.watcher.IsWatching()
}

// Example usage of ConfigWatcherManager
func ExampleUsage() {
	manager := NewConfigWatcherManager()

	// Initialize configuration and watcher
	if err := manager.Initialize(); err != nil {
		log.Fatalf("Failed to initialize configuration: %v", err)
	}
	defer manager.Shutdown()

	// Application main loop
	for {
		// Your application logic here
		config := manager.GetConfig()

		// Use current configuration
		_ = config // placeholder for actual usage

		// Simulate some work
		time.Sleep(time.Second * 5)

		// In a real application, you would have proper shutdown logic
		// break when application should exit
	}
}
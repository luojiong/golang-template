package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server      ServerConfig      `mapstructure:"server"`
	Database    DatabaseConfig    `mapstructure:"database"`
	Auth        AuthConfig        `mapstructure:"auth"`
	JWT         JWTConfig         `mapstructure:"jwt"`
	Redis       RedisConfig       `mapstructure:"redis"`
	RateLimit   RateLimitConfig   `mapstructure:"rate_limit"`
	Compression CompressionConfig `mapstructure:"compression"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	Mode        string            `mapstructure:"mode"`
}

// ConfigChangeType 表示配置变更的类型
type ConfigChangeType int

const (
	ConfigChangeTypeLogging     ConfigChangeType = iota // 日志配置变更
	ConfigChangeTypeRateLimit                           // 速率限制配置变更
	ConfigChangeTypeServer                              // 服务器配置变更
	ConfigChangeTypeAuth                                // 认证配置变更
	ConfigChangeTypeJWT                                 // JWT配置变更
	ConfigChangeTypeRedis                               // Redis配置变更
	ConfigChangeTypeDatabase                            // 数据库配置变更
	ConfigChangeTypeCompression                         // 压缩配置变更
	ConfigChangeTypeUnknown                             // 未知配置变更
)

// ConfigChange 表示配置变更事件
type ConfigChange struct {
	Type      ConfigChangeType // 变更类型
	OldValue  interface{}      // 旧值
	NewValue  interface{}      // 新值
	Timestamp time.Time        // 变更时间戳
}

// ConfigChangeHandler 是处理配置变更的函数类型
type ConfigChangeHandler func(ctx context.Context, change ConfigChange)

// ConfigManager 管理配置，支持热重载功能
type ConfigManager struct {
	mu          sync.RWMutex                               // 读写锁
	config      *Config                                    // 当前配置
	validConfig *Config                                    // 最后已知的有效配置
	v           *viper.Viper                               // Viper实例
	watcher     *fsnotify.Watcher                          // 文件监控器
	handlers    map[ConfigChangeType][]ConfigChangeHandler // 配置变更处理器
	ctx         context.Context                            // 上下文
	cancel      context.CancelFunc                         // 取消函数
	running     bool                                       // 是否正在运行
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port         string `mapstructure:"port"`          // 端口号
	Host         string `mapstructure:"host"`          // 主机地址
	ReadTimeout  int    `mapstructure:"read_timeout"`  // 读取超时时间（秒）
	WriteTimeout int    `mapstructure:"write_timeout"` // 写入超时时间（秒）
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`     // 主机地址
	Port     int    `mapstructure:"port"`     // 端口号
	User     string `mapstructure:"user"`     // 用户名
	Password string `mapstructure:"password"` // 密码
	DBName   string `mapstructure:"dbname"`   // 数据库名称
	SSLMode  string `mapstructure:"sslmode"`  // SSL模式
	// 连接池设置
	MaxOpenConns    int `mapstructure:"max_open_conns"`    // 最大打开连接数
	MaxIdleConns    int `mapstructure:"max_idle_conns"`    // 最大空闲连接数
	ConnMaxLifetime int `mapstructure:"conn_max_lifetime"` // 连接最大生存时间（秒）
}

// AuthConfig 认证配置
type AuthConfig struct {
	BcryptCost int `mapstructure:"bcrypt_cost"` // bcrypt加密成本
}

// JWTConfig JWT配置
type JWTConfig struct {
	SecretKey string `mapstructure:"secret_key"` // 密钥
	ExpiresIn int    `mapstructure:"expires_in"` // 过期时间（小时）
}

// RedisConfig Redis配置
type RedisConfig struct {
	Host     string `mapstructure:"host"`      // 主机地址
	Port     int    `mapstructure:"port"`      // 端口号
	Password string `mapstructure:"password"`  // 密码
	DB       int    `mapstructure:"db"`        // 数据库编号
	PoolSize int    `mapstructure:"pool_size"` // 连接池大小
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	Enabled  bool   `mapstructure:"enabled"`   // 是否启用
	Requests int    `mapstructure:"requests"`  // 请求次数限制
	Window   string `mapstructure:"window"`    // 时间窗口
	RedisKey string `mapstructure:"redis_key"` // Redis键名前缀
}

// CompressionConfig 压缩配置
type CompressionConfig struct {
	Enabled   bool `mapstructure:"enabled"`   // 是否启用
	Threshold int  `mapstructure:"threshold"` // 压缩阈值（字节）
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `mapstructure:"level"`       // 日志级别
	Format     string `mapstructure:"format"`      // 日志格式
	Output     string `mapstructure:"output"`      // 输出位置
	Directory  string `mapstructure:"directory"`   // 日志文件目录
	MaxSize    int    `mapstructure:"max_size"`    // 单个日志文件最大大小（MB）
	MaxBackups int    `mapstructure:"max_backups"` // 最大备份文件数
	MaxAge     int    `mapstructure:"max_age"`     // 日志文件最大保存天数
	Compress   bool   `mapstructure:"compress"`    // 是否压缩旧日志文件
}

// LoadConfig 加载配置文件
func LoadConfig() (*Config, error) {
	var config Config

	// 设置环境
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	// 设置配置文件路径
	viper.SetConfigName(env)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 设置环境变量前缀
	viper.SetEnvPrefix("APP")
	viper.AutomaticEnv()

	// 设置默认值
	viper.SetDefault("mode", env)
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("auth.bcrypt_cost", 12)
	viper.SetDefault("jwt.secret_key", "your-secret-key-change-in-production")
	viper.SetDefault("jwt.expires_in", 24)

	// 根据环境设置数据库连接池默认值
	if env == "production" {
		viper.SetDefault("database.max_open_conns", 50)
		viper.SetDefault("database.max_idle_conns", 10)
		viper.SetDefault("database.conn_max_lifetime", 600) // 10分钟（秒）
	} else if env == "staging" {
		viper.SetDefault("database.max_open_conns", 75)
		viper.SetDefault("database.max_idle_conns", 15)
		viper.SetDefault("database.conn_max_lifetime", 1800) // 30分钟（秒）
	} else {
		// 开发环境和其他环境
		viper.SetDefault("database.max_open_conns", 100)
		viper.SetDefault("database.max_idle_conns", 10)
		viper.SetDefault("database.conn_max_lifetime", 3600) // 1小时（秒）
	}

	// Redis默认值
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("redis.pool_size", 10)

	// 速率限制默认值
	viper.SetDefault("rate_limit.enabled", true)
	viper.SetDefault("rate_limit.requests", 100)
	viper.SetDefault("rate_limit.window", "1m")
	viper.SetDefault("rate_limit.redis_key", "rate_limit")

	// 压缩默认值
	viper.SetDefault("compression.enabled", true)
	viper.SetDefault("compression.threshold", 1024)

	// 日志默认值
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.directory", "./logs")
	viper.SetDefault("logging.max_size", 100)
	viper.SetDefault("logging.max_backups", 3)
	viper.SetDefault("logging.max_age", 28)
	viper.SetDefault("logging.compress", true)

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Printf("未找到配置文件，使用默认值和环境变量")
		} else {
			return nil, err
		}
	}

	// 解析配置
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	config.Mode = env

	return &config, nil
}

// IsDevelopment 检查是否为开发环境
func IsDevelopment(mode string) bool {
	return mode == "development"
}

// IsProduction 检查是否为生产环境
func IsProduction(mode string) bool {
	return mode == "production"
}

// NewConfigManager 创建一个支持热重载的新配置管理器
func NewConfigManager() (*ConfigManager, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("加载初始配置失败: %w", err)
	}

	// 验证初始配置
	validator := NewValidator(cfg)
	validationResult := validator.Validate()
	if !validationResult.Valid {
		return nil, fmt.Errorf("初始配置验证失败: %s", validationResult.FormatErrors())
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &ConfigManager{
		config:      cfg,
		validConfig: deepCopyConfig(cfg),
		handlers:    make(map[ConfigChangeType][]ConfigChangeHandler),
		ctx:         ctx,
		cancel:      cancel,
		running:     false,
	}, nil
}

// StartWatching 开始监控配置文件变更
func (cm *ConfigManager) StartWatching() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.running {
		return fmt.Errorf("配置监控器已在运行")
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建文件监控器失败: %w", err)
	}
	cm.watcher = watcher

	// 获取配置文件路径
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	configFile := filepath.Join("./configs", env+".yaml")

	// 将配置文件添加到监控器
	err = cm.watcher.Add(configFile)
	if err != nil {
		cm.watcher.Close()
		return fmt.Errorf("将配置文件添加到监控器失败: %w", err)
	}

	// 同时监控configs目录以获取新文件
	err = cm.watcher.Add("./configs")
	if err != nil {
		log.Printf("警告：将configs目录添加到监控器失败: %v", err)
	}

	cm.running = true

	// 启动文件监控goroutine
	go cm.watchFiles()

	log.Printf("开始监控配置文件: %s", configFile)
	return nil
}

// StopWatching 停止配置文件监控
func (cm *ConfigManager) StopWatching() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.running {
		return
	}

	cm.cancel()
	if cm.watcher != nil {
		cm.watcher.Close()
	}
	cm.running = false

	log.Println("已停止配置文件监控")
}

// GetConfig 返回当前配置（线程安全）
func (cm *ConfigManager) GetConfig() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return deepCopyConfig(cm.config)
}

// RegisterHandler 注册配置变更处理器
func (cm *ConfigManager) RegisterHandler(changeType ConfigChangeType, handler ConfigChangeHandler) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.handlers[changeType] == nil {
		cm.handlers[changeType] = make([]ConfigChangeHandler, 0)
	}
	cm.handlers[changeType] = append(cm.handlers[changeType], handler)
}

// watchFiles 监控文件系统事件并重新加载配置
func (cm *ConfigManager) watchFiles() {
	defer cm.watcher.Close()

	for {
		select {
		case event, ok := <-cm.watcher.Events:
			if !ok {
				return
			}

			// 只处理YAML文件的写入事件
			if event.Op&fsnotify.Write == fsnotify.Write && filepath.Ext(event.Name) == ".yaml" {
				log.Printf("配置文件已变更: %s", event.Name)

				// 添加小延迟以处理快速文件写入
				time.Sleep(100 * time.Millisecond)

				if err := cm.reloadConfig(); err != nil {
					log.Printf("重新加载配置失败: %v", err)
				}
			}

		case err, ok := <-cm.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("文件监控器错误: %v", err)

		case <-cm.ctx.Done():
			return
		}
	}
}

// reloadConfig 从文件重新加载配置
func (cm *ConfigManager) reloadConfig() error {
	// 加载新配置
	newConfig, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("加载新配置失败: %w", err)
	}

	// 验证新配置
	validator := NewValidator(newConfig)
	validationResult := validator.Validate()
	if !validationResult.Valid {
		log.Printf("配置验证失败，保持之前的有效配置:\n%s", validationResult.FormatErrors())
		return fmt.Errorf("新配置无效")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 检测变更并通知处理器
	changes := cm.detectChanges(cm.config, newConfig)

	// 更新配置
	cm.validConfig = deepCopyConfig(newConfig)
	cm.config = newConfig

	// 通知处理器变更
	for _, change := range changes {
		cm.notifyHandlers(change)
	}

	log.Printf("配置成功重新加载，共 %d 项变更", len(changes))
	return nil
}

// detectChanges 检测新旧配置之间的差异
func (cm *ConfigManager) detectChanges(oldConfig, newConfig *Config) []ConfigChange {
	var changes []ConfigChange
	now := time.Now()

	// 检查日志配置变更
	if oldConfig.Logging.Level != newConfig.Logging.Level ||
		oldConfig.Logging.Format != newConfig.Logging.Format ||
		oldConfig.Logging.Output != newConfig.Logging.Output ||
		oldConfig.Logging.Directory != newConfig.Logging.Directory ||
		oldConfig.Logging.MaxSize != newConfig.Logging.MaxSize ||
		oldConfig.Logging.MaxBackups != newConfig.Logging.MaxBackups ||
		oldConfig.Logging.MaxAge != newConfig.Logging.MaxAge ||
		oldConfig.Logging.Compress != newConfig.Logging.Compress {
		changes = append(changes, ConfigChange{
			Type:      ConfigChangeTypeLogging,
			OldValue:  oldConfig.Logging,
			NewValue:  newConfig.Logging,
			Timestamp: now,
		})
	}

	// 检查速率限制配置变更
	if oldConfig.RateLimit.Enabled != newConfig.RateLimit.Enabled ||
		oldConfig.RateLimit.Requests != newConfig.RateLimit.Requests ||
		oldConfig.RateLimit.Window != newConfig.RateLimit.Window {
		changes = append(changes, ConfigChange{
			Type:      ConfigChangeTypeRateLimit,
			OldValue:  oldConfig.RateLimit,
			NewValue:  newConfig.RateLimit,
			Timestamp: now,
		})
	}

	// 检查压缩配置变更
	if oldConfig.Compression.Enabled != newConfig.Compression.Enabled ||
		oldConfig.Compression.Threshold != newConfig.Compression.Threshold {
		changes = append(changes, ConfigChange{
			Type:      ConfigChangeTypeCompression,
			OldValue:  oldConfig.Compression,
			NewValue:  newConfig.Compression,
			Timestamp: now,
		})
	}

	// 检查服务器配置变更
	if oldConfig.Server.Host != newConfig.Server.Host ||
		oldConfig.Server.Port != newConfig.Server.Port ||
		oldConfig.Server.ReadTimeout != newConfig.Server.ReadTimeout ||
		oldConfig.Server.WriteTimeout != newConfig.Server.WriteTimeout {
		changes = append(changes, ConfigChange{
			Type:      ConfigChangeTypeServer,
			OldValue:  oldConfig.Server,
			NewValue:  newConfig.Server,
			Timestamp: now,
		})
	}

	// 检查认证配置变更
	if oldConfig.Auth.BcryptCost != newConfig.Auth.BcryptCost {
		changes = append(changes, ConfigChange{
			Type:      ConfigChangeTypeAuth,
			OldValue:  oldConfig.Auth,
			NewValue:  newConfig.Auth,
			Timestamp: now,
		})
	}

	// 检查JWT配置变更
	if oldConfig.JWT.SecretKey != newConfig.JWT.SecretKey ||
		oldConfig.JWT.ExpiresIn != newConfig.JWT.ExpiresIn {
		changes = append(changes, ConfigChange{
			Type:      ConfigChangeTypeJWT,
			OldValue:  oldConfig.JWT,
			NewValue:  newConfig.JWT,
			Timestamp: now,
		})
	}

	// 检查Redis配置变更
	if oldConfig.Redis.Host != newConfig.Redis.Host ||
		oldConfig.Redis.Port != newConfig.Redis.Port ||
		oldConfig.Redis.Password != newConfig.Redis.Password ||
		oldConfig.Redis.DB != newConfig.Redis.DB ||
		oldConfig.Redis.PoolSize != newConfig.Redis.PoolSize {
		changes = append(changes, ConfigChange{
			Type:      ConfigChangeTypeRedis,
			OldValue:  oldConfig.Redis,
			NewValue:  newConfig.Redis,
			Timestamp: now,
		})
	}

	// 检查数据库配置变更
	if oldConfig.Database.Host != newConfig.Database.Host ||
		oldConfig.Database.Port != newConfig.Database.Port ||
		oldConfig.Database.User != newConfig.Database.User ||
		oldConfig.Database.Password != newConfig.Database.Password ||
		oldConfig.Database.DBName != newConfig.Database.DBName ||
		oldConfig.Database.SSLMode != newConfig.Database.SSLMode ||
		oldConfig.Database.MaxOpenConns != newConfig.Database.MaxOpenConns ||
		oldConfig.Database.MaxIdleConns != newConfig.Database.MaxIdleConns ||
		oldConfig.Database.ConnMaxLifetime != newConfig.Database.ConnMaxLifetime {
		changes = append(changes, ConfigChange{
			Type:      ConfigChangeTypeDatabase,
			OldValue:  oldConfig.Database,
			NewValue:  newConfig.Database,
			Timestamp: now,
		})
	}

	return changes
}

// notifyHandlers 通知已注册的处理器配置变更
func (cm *ConfigManager) notifyHandlers(change ConfigChange) {
	handlers, exists := cm.handlers[change.Type]
	if !exists {
		return
	}

	for _, handler := range handlers {
		go func(h ConfigChangeHandler) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("配置变更处理器发生panic: %v", r)
				}
			}()
			h(cm.ctx, change)
		}(handler)
	}
}

// deepCopyConfig 声明已移至 config_utils.go

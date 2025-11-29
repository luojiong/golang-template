package bootstrap

import (
	"context"
	"fmt"

	"go-server/internal/config"
	"go-server/internal/logger"
)

// initializeConfig 初始化配置管理器
func (c *Container) initializeConfig() error {
	// 创建配置管理器（支持热重载）
	configManager, err := config.NewConfigManager()
	if err != nil {
		return fmt.Errorf("创建配置管理器失败: %w", err)
	}

	c.ConfigManager = configManager
	c.Config = configManager.GetConfig()

	return nil
}

// registerConfigHandlers 注册配置变更处理器
func (c *Container) registerConfigHandlers() {
	appLogger := c.Logger.GetLogger("config")

	// 日志配置变更处理器
	c.ConfigManager.RegisterHandler(config.ConfigChangeTypeLogging,
		func(ctx context.Context, change config.ConfigChange) {
			oldConfig := change.OldValue.(config.LoggingConfig)
			newConfig := change.NewValue.(config.LoggingConfig)

			appLogger.Info(ctx, "日志配置已更改",
				logger.String("old_level", oldConfig.Level),
				logger.String("new_level", newConfig.Level),
				logger.String("old_format", oldConfig.Format),
				logger.String("new_format", newConfig.Format),
				logger.String("old_output", oldConfig.Output),
				logger.String("new_output", newConfig.Output))

			// 更新日志管理器配置
			if err := c.Logger.UpdateConfig(newConfig); err != nil {
				appLogger.Error(ctx, "更新日志配置失败", logger.Error(err))
			}
		})

	// 速率限制配置变更处理器
	c.ConfigManager.RegisterHandler(config.ConfigChangeTypeRateLimit,
		func(ctx context.Context, change config.ConfigChange) {
			oldConfig := change.OldValue.(config.RateLimitConfig)
			newConfig := change.NewValue.(config.RateLimitConfig)

			appLogger.Info(ctx, "速率限制配置已更改",
				logger.Bool("old_enabled", oldConfig.Enabled),
				logger.Bool("new_enabled", newConfig.Enabled),
				logger.Int("old_requests", oldConfig.Requests),
				logger.Int("new_requests", newConfig.Requests),
				logger.String("old_window", oldConfig.Window),
				logger.String("new_window", newConfig.Window))
		})

	// 服务器配置变更处理器
	c.ConfigManager.RegisterHandler(config.ConfigChangeTypeServer,
		func(ctx context.Context, change config.ConfigChange) {
			oldConfig := change.OldValue.(config.ServerConfig)
			newConfig := change.NewValue.(config.ServerConfig)

			appLogger.Info(ctx, "服务器配置已更改",
				logger.String("old_host", oldConfig.Host),
				logger.String("new_host", newConfig.Host),
				logger.String("old_port", oldConfig.Port),
				logger.String("new_port", newConfig.Port),
				logger.Int("old_read_timeout", oldConfig.ReadTimeout),
				logger.Int("new_read_timeout", newConfig.ReadTimeout),
				logger.Int("old_write_timeout", oldConfig.WriteTimeout),
				logger.Int("new_write_timeout", newConfig.WriteTimeout))

			appLogger.Warn(ctx, "服务器配置更改（主机/端口）需要重启应用程序才能生效")
		})

	// JWT配置变更处理器
	c.ConfigManager.RegisterHandler(config.ConfigChangeTypeJWT,
		func(ctx context.Context, change config.ConfigChange) {
			oldConfig := change.OldValue.(config.JWTConfig)
			newConfig := change.NewValue.(config.JWTConfig)

			appLogger.Info(ctx, "JWT配置已更改",
				logger.Int("old_expires_in", oldConfig.ExpiresIn),
				logger.Int("new_expires_in", newConfig.ExpiresIn))

			if oldConfig.SecretKey != newConfig.SecretKey {
				appLogger.Warn(ctx, "JWT密钥已更改 - 现有令牌将失效")
			}
		})

	// Redis配置变更处理器
	c.ConfigManager.RegisterHandler(config.ConfigChangeTypeRedis,
		func(ctx context.Context, change config.ConfigChange) {
			oldConfig := change.OldValue.(config.RedisConfig)
			newConfig := change.NewValue.(config.RedisConfig)

			appLogger.Info(ctx, "Redis配置已更改",
				logger.String("old_host", oldConfig.Host),
				logger.String("new_host", newConfig.Host),
				logger.Int("old_port", oldConfig.Port),
				logger.Int("new_port", newConfig.Port),
				logger.Int("old_db", oldConfig.DB),
				logger.Int("new_db", newConfig.DB),
				logger.Int("old_pool_size", oldConfig.PoolSize),
				logger.Int("new_pool_size", newConfig.PoolSize))

			appLogger.Warn(ctx, "Redis连接更改需要重启服务才能生效")
		})

	// 数据库配置变更处理器
	c.ConfigManager.RegisterHandler(config.ConfigChangeTypeDatabase,
		func(ctx context.Context, change config.ConfigChange) {
			oldConfig := change.OldValue.(config.DatabaseConfig)
			newConfig := change.NewValue.(config.DatabaseConfig)

			appLogger.Info(ctx, "数据库配置已更改",
				logger.String("old_host", oldConfig.Host),
				logger.String("new_host", newConfig.Host),
				logger.Int("old_port", oldConfig.Port),
				logger.Int("new_port", newConfig.Port),
				logger.String("old_db_name", oldConfig.DBName),
				logger.String("new_db_name", newConfig.DBName))

			appLogger.Warn(ctx, "数据库配置更改需要重启应用程序才能生效")
		})

	// 认证配置变更处理器
	c.ConfigManager.RegisterHandler(config.ConfigChangeTypeAuth,
		func(ctx context.Context, change config.ConfigChange) {
			oldConfig := change.OldValue.(config.AuthConfig)
			newConfig := change.NewValue.(config.AuthConfig)

			appLogger.Info(ctx, "认证配置已更改",
				logger.Int("old_bcrypt_cost", oldConfig.BcryptCost),
				logger.Int("new_bcrypt_cost", newConfig.BcryptCost))

			appLogger.Info(ctx, "认证配置更改仅影响新密码操作")
		})

	// 压缩配置变更处理器
	c.ConfigManager.RegisterHandler(config.ConfigChangeTypeCompression,
		func(ctx context.Context, change config.ConfigChange) {
			oldConfig := change.OldValue.(config.CompressionConfig)
			newConfig := change.NewValue.(config.CompressionConfig)

			appLogger.Info(ctx, "压缩配置已更改",
				logger.Bool("old_enabled", oldConfig.Enabled),
				logger.Bool("new_enabled", newConfig.Enabled),
				logger.Int("old_threshold", oldConfig.Threshold),
				logger.Int("new_threshold", newConfig.Threshold))

			appLogger.Info(ctx, "压缩配置更改仅影响新请求",
				logger.Bool("enabled", newConfig.Enabled),
				logger.Int("threshold", newConfig.Threshold))
		})
}

package bootstrap

import (
	"context"
	"time"

	"go-server/internal/logger"
	"go-server/pkg/auth"
	"go-server/pkg/cache"
)

// initializeAuth 初始化JWT管理器和黑名单服务
func (c *Container) initializeAuth() error {
	appLogger := c.Logger.GetLogger("app")

	// 初始化基础JWT管理器
	c.JWTManager = auth.NewJWTManager(c.Config.JWT.SecretKey, c.Config.JWT.ExpiresIn)

	// 如果Redis可用，初始化JWT令牌黑名单服务
	if c.Cache != nil {
		blacklistConfig := &cache.BlacklistConfig{
			KeyPrefix:       "jwt_blacklist:",
			CleanupInterval: 1 * time.Hour,
			BatchSize:       100,
		}

		c.BlacklistService = cache.NewBlacklistService(c.Cache, c.JWTManager, blacklistConfig)

		appLogger.Info(context.Background(), "JWT黑名单服务已使用Redis支持初始化",
			logger.String("cleanup_interval", blacklistConfig.CleanupInterval.String()),
			logger.Int("batch_size", blacklistConfig.BatchSize))

		// 使用黑名单支持重新初始化JWT管理器
		c.JWTManager = auth.NewJWTManagerWithBlacklist(
			c.Config.JWT.SecretKey,
			c.Config.JWT.ExpiresIn,
			c.BlacklistService,
		)

		appLogger.Info(context.Background(), "JWT管理器已重新初始化，具有黑名单支持")

		// 启动后台清理过期令牌的goroutine
		go c.startBlacklistCleanup(blacklistConfig.CleanupInterval)

		appLogger.Info(context.Background(), "JWT黑名单清理例程已启动")
	} else {
		appLogger.Warn(context.Background(), "JWT黑名单服务不可用 - Redis缓存未初始化")
		appLogger.Warn(context.Background(), "令牌将仅使用标准JWT验证")
	}

	return nil
}

// startBlacklistCleanup 启动黑名单清理后台任务
func (c *Container) startBlacklistCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	appLogger := c.Logger.GetLogger("app")

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

		if err := c.BlacklistService.CleanupExpiredTokens(ctx); err != nil {
			appLogger.Error(ctx, "清理过期JWT令牌失败", logger.Error(err))
		} else {
			appLogger.Debug(ctx, "JWT黑名单清理完成")
		}

		cancel()
	}
}

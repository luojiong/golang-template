package bootstrap

import (
	"context"

	"go-server/internal/handlers"
	"go-server/internal/logger"
	"go-server/internal/repositories"
	"go-server/internal/services"
)

// initializeRepositories 初始化仓储层
func (c *Container) initializeRepositories() error {
	// 初始化用户仓储
	c.UserRepository = repositories.NewUserRepository(c.Database.DB)

	return nil
}

// initializeServices 初始化服务层
func (c *Container) initializeServices() error {
	appLogger := c.Logger.GetLogger("app")

	// 根据是否有缓存，创建相应的用户服务
	if c.Cache != nil {
		// 使用支持缓存的服务
		c.UserService = services.NewUserServiceWithCache(c.UserRepository, c.Cache)

		appLogger.Info(context.Background(), "用户服务已初始化，支持Redis缓存",
			logger.String("cache_type", "Redis"),
			logger.String("ttl", "5分钟"))
		appLogger.Info(context.Background(), "频繁访问的数据将从Redis缓存提供")
		appLogger.Info(context.Background(), "缓存内存使用将由Redis管理，当内存超过80%时使用LRU淘汰策略")
	} else {
		// 无缓存服务
		c.UserService = services.NewUserService(c.UserRepository)

		appLogger.Info(context.Background(), "用户服务已初始化，不支持缓存",
			logger.String("reason", "Redis不可用"))
		appLogger.Warn(context.Background(), "所有数据将直接从数据库提供 - 性能可能受到影响")
	}

	return nil
}

// initializeHandlers 初始化处理器层
func (c *Container) initializeHandlers() error {
	appLogger := c.Logger.GetLogger("app")

	// 初始化处理器
	c.AuthHandler = handlers.NewAuthHandler(c.JWTManager, c.UserService, c.BlacklistService)
	c.UserHandler = handlers.NewUserHandler(c.UserService)
	c.HealthHandler = handlers.NewHealthHandler(c.Database, c.Cache)

	appLogger.Info(context.Background(), "所有处理器已初始化")

	return nil
}

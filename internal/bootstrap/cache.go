package bootstrap

import (
	"context"
	"fmt"
	"time"

	"go-server/internal/logger"
	"go-server/pkg/cache"
)

// initializeCache 初始化Redis缓存
func (c *Container) initializeCache() error {
	appLogger := c.Logger.GetLogger("app")

	// 创建Redis缓存配置
	redisConfig := &cache.RedisConfig{
		Host:         c.Config.Redis.Host,
		Port:         c.Config.Redis.Port,
		Password:     c.Config.Redis.Password,
		DB:           c.Config.Redis.DB,
		Prefix:       "golang_template:",
		PoolSize:     c.Config.Redis.PoolSize,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	}

	// 初始化Redis缓存
	redisCache, err := cache.NewRedisCache(redisConfig)
	if err != nil {
		return fmt.Errorf("初始化Redis缓存失败: %w", err)
	}

	c.Cache = redisCache

	appLogger.Info(context.Background(), "Redis缓存初始化成功",
		logger.String("host", fmt.Sprintf("%s:%d", c.Config.Redis.Host, c.Config.Redis.Port)),
		logger.Int("database", c.Config.Redis.DB),
		logger.Int("pool_size", c.Config.Redis.PoolSize))

	// 测试Redis连接
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	testKey := "startup_test"
	if err := redisCache.Set(ctx, testKey, "test", 10*time.Second); err != nil {
		appLogger.Warn(context.Background(), "Redis缓存测试操作失败",
			logger.Error(err))
		appLogger.Warn(context.Background(), "缓存可能不稳定 - 建议检查Redis配置")
	} else {
		redisCache.Delete(ctx, testKey) // 清理测试键
		appLogger.Info(context.Background(), "Redis缓存连接验证成功")
	}

	return nil
}

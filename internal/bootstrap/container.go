package bootstrap

import (
	"context"
	"fmt"
	"log"

	"go-server/internal/config"
	"go-server/internal/database"
	"go-server/internal/handlers"
	"go-server/internal/logger"
	"go-server/internal/repositories"
	"go-server/internal/routes"
	"go-server/internal/services"
	"go-server/pkg/auth"
	"go-server/pkg/cache"

	"github.com/gin-gonic/gin"
)

// Container 应用程序依赖注入容器
// 管理所有应用程序组件的生命周期和依赖关系
type Container struct {
	// 配置管理
	ConfigManager *config.ConfigManager
	Config        *config.Config

	// 核心组件
	Logger   *logger.Manager
	Database *database.Database
	Cache    cache.Cache

	// 认证和授权
	JWTManager       *auth.JWTManager
	BlacklistService *cache.BlacklistService

	// 仓储层
	UserRepository repositories.UserRepository

	// 服务层
	UserService services.UserService

	// 处理器层
	AuthHandler   *handlers.AuthHandler
	UserHandler   *handlers.UserHandler
	HealthHandler *handlers.HealthHandler

	// 中间件和路由
	Middlewares []gin.HandlerFunc
	Router      *routes.Router
}

// NewContainer 创建并初始化应用容器
// 按照依赖顺序初始化所有组件：配置 -> 日志 -> 数据库 -> 缓存 -> 服务 -> 处理器
func NewContainer() (*Container, error) {
	c := &Container{}

	// 1. 初始化配置管理器
	if err := c.initializeConfig(); err != nil {
		return nil, fmt.Errorf("初始化配置失败: %w", err)
	}

	// 2. 初始化日志系统
	if err := c.initializeLogger(); err != nil {
		return nil, fmt.Errorf("初始化日志系统失败: %w", err)
	}

	// 3. 初始化数据库
	if err := c.initializeDatabase(); err != nil {
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 4. 初始化缓存（Redis）
	if err := c.initializeCache(); err != nil {
		// 缓存初始化失败不是致命错误，记录警告后继续
		c.Logger.GetLogger("app").Warn(
			context.Background(),
			"缓存初始化失败，将在没有缓存的情况下运行",
			logger.Error(err),
		)
	}

	// 5. 初始化JWT和黑名单服务
	if err := c.initializeAuth(); err != nil {
		return nil, fmt.Errorf("初始化认证服务失败: %w", err)
	}

	// 6. 初始化仓储层
	if err := c.initializeRepositories(); err != nil {
		return nil, fmt.Errorf("初始化仓储层失败: %w", err)
	}

	// 7. 初始化服务层
	if err := c.initializeServices(); err != nil {
		return nil, fmt.Errorf("初始化服务层失败: %w", err)
	}

	// 8. 初始化处理器层
	if err := c.initializeHandlers(); err != nil {
		return nil, fmt.Errorf("初始化处理器层失败: %w", err)
	}

	// 9. 设置中间件
	if err := c.setupMiddlewares(); err != nil {
		return nil, fmt.Errorf("设置中间件失败: %w", err)
	}

	// 10. 初始化路由
	if err := c.initializeRouter(); err != nil {
		return nil, fmt.Errorf("初始化路由失败: %w", err)
	}

	// 11. 注册配置变更处理器
	c.registerConfigHandlers()

	// 12. 启动配置文件监控
	if err := c.ConfigManager.StartWatching(); err != nil {
		c.Logger.GetLogger("app").Warn(
			context.Background(),
			"启动配置文件监控失败",
			logger.Error(err),
		)
	}

	return c, nil
}

// Cleanup 清理所有资源
func (c *Container) Cleanup() {
	ctx := context.Background()
	appLogger := c.Logger.GetLogger("app")

	// 停止配置监控
	if c.ConfigManager != nil {
		c.ConfigManager.StopWatching()
		appLogger.Info(ctx, "配置文件监控已停止")
	}

	// 关闭数据库连接
	if c.Database != nil {
		if err := c.Database.Close(); err != nil {
			appLogger.Error(ctx, "关闭数据库连接失败", logger.Error(err))
		} else {
			appLogger.Info(ctx, "数据库连接已关闭")
		}
	}

	// 关闭缓存连接
	if c.Cache != nil {
		if err := c.Cache.Close(); err != nil {
			appLogger.Error(ctx, "关闭缓存连接失败", logger.Error(err))
		} else {
			appLogger.Info(ctx, "缓存连接已关闭")
		}
	}

	// 关闭日志系统
	if c.Logger != nil {
		if err := c.Logger.Stop(); err != nil {
			log.Printf("关闭日志系统失败: %v", err)
		} else {
			log.Println("日志系统已关闭")
		}
	}
}

// GetEngine 获取 Gin Engine
func (c *Container) GetEngine() *gin.Engine {
	if c.Router != nil {
		return c.Router.GetEngine()
	}
	return nil
}

package bootstrap

import (
	"context"

	"go-server/internal/routes"
)

// initializeRouter 初始化路由
func (c *Container) initializeRouter() error {
	// 创建路由
	c.Router = routes.NewRouter(
		c.AuthHandler,
		c.UserHandler,
		c.HealthHandler,
		c.JWTManager,
		c.Middlewares,
	)

	// 设置路由
	c.Router.SetupRoutes()

	c.Logger.GetLogger("app").Info(context.Background(), "路由系统已初始化")

	return nil
}

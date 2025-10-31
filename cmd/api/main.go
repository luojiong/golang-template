package main

import (
	"context"
	"fmt"

	_ "go-server/docs" // 导入Swagger文档
	"go-server/internal/bootstrap"
	"go-server/internal/config"
	"go-server/internal/logger"
)

// @title Golang 模板 API
// @version 1.0
// @description 一个使用Go和Gin框架构建的RESTful API模板，具有基于Redis的缓存、速率限制和增强的错误处理功能。所有API端点都受到速率限制保护：匿名用户（100次/分钟），认证用户（200次/分钟）。频繁访问的数据（用户配置文件、配置信息）从Redis缓存提供，TTL为5分钟。API具有集中式错误处理，带有用于请求跟踪的关联ID、详细的字段级验证错误（支持国际化消息）和全面的错误上下文。速率限制、缓存头和关联ID都包含在所有响应中。当Redis不可用时，系统会优雅地降级到数据库查询。
// @termsOfService http://swagger.io/terms/

// @contact.name API 支持
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 输入"Bearer"后跟空格和JWT令牌。

func main() {
	// 创建临时日志记录器用于启动时的错误处理
	tempLogger, err := logger.NewManager(config.LoggingConfig{
		Level:    "info",
		Format:   "text",
		Output:   "stdout",
		MaxSize:  100,
		Compress: false,
	})
	if err != nil {
		panic(fmt.Sprintf("创建临时日志记录器失败: %v", err))
	}

	if err := tempLogger.Start(); err != nil {
		panic(fmt.Sprintf("启动临时日志记录器失败: %v", err))
	}
	defer tempLogger.Stop()

	appLogger := tempLogger.GetLogger("main")

	// 创建应用容器，初始化所有组件
	container, err := bootstrap.NewContainer()
	if err != nil {
		appLogger.Fatal(context.Background(), "初始化应用容器失败", logger.Error(err))
	}
	defer container.Cleanup()

	// 运行服务器，处理优雅关闭
	if err := bootstrap.Run(container); err != nil {
		appLogger.Fatal(context.Background(), "服务器运行错误", logger.Error(err))
	}
}

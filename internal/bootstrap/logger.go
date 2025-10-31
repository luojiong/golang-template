package bootstrap

import (
	"context"
	"fmt"
	"log"

	"go-server/internal/config"
	"go-server/internal/logger"
)

// initializeLogger 初始化日志管理器
func (c *Container) initializeLogger() error {
	// 创建日志管理器
	loggerManager, err := logger.NewManager(c.Config.Logging)
	if err != nil {
		return fmt.Errorf("创建日志管理器失败: %w", err)
	}

	// 启动日志管理器
	if err := loggerManager.Start(); err != nil {
		return fmt.Errorf("启动日志管理器失败: %w", err)
	}

	c.Logger = loggerManager

	// 获取应用程序日志记录器
	appLogger := loggerManager.GetLogger("app")

	// 在开发模式下打印配置信息
	if config.IsDevelopment(c.Config.Mode) {
		appLogger.Info(context.Background(), "应用程序启动配置",
			logger.String("mode", c.Config.Mode),
			logger.String("server_address", fmt.Sprintf("%s:%s", c.Config.Server.Host, c.Config.Server.Port)),
			logger.String("database", fmt.Sprintf("%s:%d/%s", c.Config.Database.Host, c.Config.Database.Port, c.Config.Database.DBName)),
		)
	}

	// 将标准log包的输出重定向到我们的日志系统
	log.SetOutput(&StdLogBridge{logger: appLogger})
	log.SetFlags(0) // 禁用标准log的时间戳和文件前缀

	return nil
}

// StdLogBridge 标准log包到自定义logger的桥接
// 用于捕获第三方库使用标准log包输出的日志
type StdLogBridge struct {
	logger logger.Logger
}

// Write 实现 io.Writer 接口
func (b *StdLogBridge) Write(p []byte) (n int, err error) {
	if len(p) > 0 {
		// 去掉末尾的换行符
		message := string(p)
		if message[len(message)-1] == '\n' {
			message = message[:len(message)-1]
		}
		b.logger.Info(context.Background(), message)
	}
	return len(p), nil
}

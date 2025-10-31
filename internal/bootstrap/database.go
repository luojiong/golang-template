package bootstrap

import (
	"context"
	"fmt"

	"go-server/internal/database"
	"go-server/internal/logger"
)

// initializeDatabase 初始化数据库连接
func (c *Container) initializeDatabase() error {
	appLogger := c.Logger.GetLogger("app")

	// 创建数据库连接
	db, err := database.NewDatabase(c.Config, c.Logger)
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	c.Database = db

	appLogger.Info(context.Background(), "数据库连接建立成功",
		logger.String("host", fmt.Sprintf("%s:%d", c.Config.Database.Host, c.Config.Database.Port)),
		logger.String("database", c.Config.Database.DBName),
		logger.Int("max_open_conns", c.Config.Database.MaxOpenConns),
		logger.Int("max_idle_conns", c.Config.Database.MaxIdleConns),
	)

	// 运行数据库迁移
	if err := db.AutoMigrate(); err != nil {
		return fmt.Errorf("运行数据库迁移失败: %w", err)
	}

	appLogger.Info(context.Background(), "数据库迁移完成")

	return nil
}

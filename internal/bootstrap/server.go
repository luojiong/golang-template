package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-server/internal/config"
	"go-server/internal/logger"

	"github.com/gin-gonic/gin"
)

// Server HTTP服务器
type Server struct {
	httpServer *http.Server
	config     *config.Config
	logger     logger.Logger
}

// NewServer 创建新的HTTP服务器
func NewServer(cfg *config.Config, engine *gin.Engine, appLogger logger.Logger) *Server {
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      engine,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	return &Server{
		httpServer: server,
		config:     cfg,
		logger:     appLogger,
	}
}

// Start 启动HTTP服务器
func (s *Server) Start() error {
	s.logger.Info(context.Background(), "启动服务器",
		logger.String("address", s.httpServer.Addr),
		logger.String("swagger_url", fmt.Sprintf("http://%s:%s/swagger/index.html",
			s.config.Server.Host, s.config.Server.Port)))

	s.logger.Info(context.Background(), "健康检查端点可用",
		logger.String("health_url", fmt.Sprintf("http://%s:%s/api/v1/health",
			s.config.Server.Host, s.config.Server.Port)))

	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("服务器启动失败: %w", err)
	}

	return nil
}

// Shutdown 优雅关闭服务器
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info(ctx, "正在优雅关闭服务器...")

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("服务器关闭失败: %w", err)
	}

	s.logger.Info(ctx, "服务器已成功关闭")
	return nil
}

// Run 运行服务器并处理优雅关闭
func Run(container *Container) error {
	appLogger := container.Logger.GetLogger("app")

	// 创建服务器
	server := NewServer(
		container.Config,
		container.GetEngine(),
		appLogger,
	)

	// 记录系统架构摘要
	logSystemSummary(container, appLogger)

	// 在goroutine中启动服务器
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Start()
	}()

	// 等待中断信号或服务器错误
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("服务器错误: %w", err)
	case sig := <-quit:
		appLogger.Info(context.Background(), "收到关闭信号",
			logger.String("signal", sig.String()))

		// 给服务器5秒时间来完成当前正在处理的请求
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("强制关闭服务器: %w", err)
		}

		// 清理资源
		container.Cleanup()

		appLogger.Info(context.Background(), "应用程序已优雅退出")
		return nil
	}
}

// logSystemSummary 记录系统架构摘要
func logSystemSummary(c *Container, appLogger logger.Logger) {
	ctx := context.Background()

	// 记录增强的系统架构摘要
	appLogger.Info(ctx, "=== 增强的系统架构摘要 ===",
		logger.String("database", fmt.Sprintf("PostgreSQL (host: %s:%d, db: %s)",
			c.Config.Database.Host, c.Config.Database.Port, c.Config.Database.DBName)),
		logger.String("authentication", fmt.Sprintf("JWT with %d-hour expiration",
			c.Config.JWT.ExpiresIn)),
		logger.String("environment", c.Config.Mode))

	if c.Cache != nil {
		appLogger.Info(ctx, "Redis缓存状态: 已启用",
			logger.String("host", fmt.Sprintf("%s:%d", c.Config.Redis.Host, c.Config.Redis.Port)),
			logger.Int("database", c.Config.Redis.DB),
			logger.Bool("caching_enabled", true),
			logger.Bool("jwt_blacklisting_enabled", true),
			logger.Bool("distributed_rate_limiting_enabled", true))
	} else {
		appLogger.Warn(ctx, "Redis缓存状态: 已禁用",
			logger.String("reason", "不可用"),
			logger.Bool("caching_enabled", false),
			logger.Bool("jwt_blacklisting_enabled", false),
			logger.Bool("distributed_rate_limiting_enabled", false))
	}

	// 增强中间件功能
	middlewareInfo := map[string]interface{}{
		"structured_logging":    true,
		"panic_recovery":        true,
		"security_headers":      true,
		"cors":                  true,
		"rate_limiting_enabled": c.Config.RateLimit.Enabled,
		"compression_enabled":   c.Config.Compression.Enabled,
	}

	if c.Config.RateLimit.Enabled {
		middlewareInfo["rate_limiting_anonymous"] = c.Config.RateLimit.Requests
		middlewareInfo["rate_limiting_authenticated"] = c.Config.RateLimit.Requests * 2
		middlewareInfo["rate_limiting_window"] = c.Config.RateLimit.Window
	}

	if c.Config.Compression.Enabled {
		middlewareInfo["compression_threshold"] = c.Config.Compression.Threshold
	}

	appLogger.Info(ctx, "增强的中间件栈功能", logger.Any("features", middlewareInfo))
}

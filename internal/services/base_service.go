package services

import (
	"context"
	"fmt"

	"go-server/internal/cache_manager"
)

// BaseService 基础服务接口
type BaseService interface {
	GetCacheManager() cache_manager.Manager
	HandleError(ctx context.Context, err error, operation string) error
}

// baseService 基础服务实现
type baseService struct {
	cache cache_manager.Manager
}

// NewBaseService 创建基础服务
func NewBaseService(cacheManager cache_manager.Manager) BaseService {
	return &baseService{
		cache: cacheManager,
	}
}

// GetCacheManager 获取缓存管理器
func (s *baseService) GetCacheManager() cache_manager.Manager {
	return s.cache
}

// HandleError 统一错误处理
func (s *baseService) HandleError(ctx context.Context, err error, operation string) error {
	if err == nil {
		return nil
	}

	// 这里可以添加日志记录、监控上报等
	return fmt.Errorf("failed to %s: %w", operation, err)
}

// ServiceOptions 服务选项
type ServiceOptions struct {
	CacheManager cache_manager.Manager
}

// DefaultServiceOptions 默认服务选项
func DefaultServiceOptions() ServiceOptions {
	return ServiceOptions{}
}

// WithCacheManager 设置缓存管理器
func (o ServiceOptions) WithCacheManager(cacheManager cache_manager.Manager) ServiceOptions {
	o.CacheManager = cacheManager
	return o
}

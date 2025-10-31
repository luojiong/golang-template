package repositories

import (
	"fmt"
	"time"

	"go-server/internal/cache_manager"
	"go-server/internal/models"

	"gorm.io/gorm"
)

// UserRepositoryV2 简化的用户仓库接口
type UserRepositoryV2 interface {
	Create(user *models.User) error
	GetByID(id string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	GetAll(offset, limit int) ([]*models.User, int64, error)
	Update(user *models.User) error
	Delete(id string) error
	UpdateLastLogin(id string) error
	ExistsByEmail(email string) (bool, error)
	ExistsByUsername(username string) (bool, error)
	Count() (int64, error)
}

// userRepositoryV2 简化的用户仓库实现
type userRepositoryV2 struct {
	db    *gorm.DB
	cache cache_manager.Manager
	ttl   time.Duration
}

// NewUserRepositoryV2 创建新的用户仓库V2
func NewUserRepositoryV2(db *gorm.DB, cacheManager cache_manager.Manager, ttl time.Duration) UserRepositoryV2 {
	return &userRepositoryV2{
		db:    db,
		cache: cacheManager,
		ttl:   ttl,
	}
}

// Create 创建用户
func (r *userRepositoryV2) Create(user *models.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// 使用统一的缓存管理器失效相关缓存
	if r.cache != nil {
		if err := r.cache.InvalidateUserListCache(); err != nil {
			// 记录错误但不影响主流程
		}
	}

	return nil
}

// GetByID 根据ID获取用户
func (r *userRepositoryV2) GetByID(id string) (*models.User, error) {
	// 尝试从缓存获取
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:id:%s", id)
		if user, found := r.cache.GetUserFromCache(cacheKey); found {
			return user, nil
		}
	}

	// 从数据库获取
	var user models.User
	err := r.db.Where("id = ? AND is_active = ?", id, true).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// 缓存结果
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:id:%s", id)
		_ = r.cache.SetUserCache(cacheKey, &user, r.ttl)
	}

	return &user, nil
}

// GetByEmail 根据邮箱获取用户
func (r *userRepositoryV2) GetByEmail(email string) (*models.User, error) {
	// 尝试从缓存获取
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:email:%s", email)
		if user, found := r.cache.GetUserFromCache(cacheKey); found {
			return user, nil
		}
	}

	// 从数据库获取
	var user models.User
	err := r.db.Where("email = ? AND is_active = ?", email, true).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// 缓存结果
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:email:%s", email)
		_ = r.cache.SetUserCache(cacheKey, &user, r.ttl)
	}

	return &user, nil
}

// GetByUsername 根据用户名获取用户
func (r *userRepositoryV2) GetByUsername(username string) (*models.User, error) {
	// 尝试从缓存获取
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:username:%s", username)
		if user, found := r.cache.GetUserFromCache(cacheKey); found {
			return user, nil
		}
	}

	// 从数据库获取
	var user models.User
	err := r.db.Where("username = ? AND is_active = ?", username, true).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// 缓存结果
	if r.cache != nil {
		cacheKey := fmt.Sprintf("user:username:%s", username)
		_ = r.cache.SetUserCache(cacheKey, &user, r.ttl)
	}

	return &user, nil
}

// GetAll 获取用户列表（分页）
func (r *userRepositoryV2) GetAll(offset, limit int) ([]*models.User, int64, error) {
	// 尝试从缓存获取
	if r.cache != nil {
		cacheKey := fmt.Sprintf("users:all:%d:%d", offset, limit)
		if users, total, found := r.cache.GetUserListFromCache(cacheKey); found {
			return users, total, nil
		}
	}

	// 从数据库获取
	var users []*models.User
	var total int64

	// 获取总数
	if err := r.db.Model(&models.User{}).Where("is_active = ?", true).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// 获取分页数据
	err := r.db.Where("is_active = ?", true).
		Offset(offset).
		Limit(limit).
		Order("created_at DESC").
		Find(&users).Error
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get users: %w", err)
	}

	// 缓存结果
	if r.cache != nil {
		cacheKey := fmt.Sprintf("users:all:%d:%d", offset, limit)
		_ = r.cache.SetUserListCache(cacheKey, users, total, r.ttl)
	}

	return users, total, nil
}

// Update 更新用户
func (r *userRepositoryV2) Update(user *models.User) error {
	result := r.db.Where("id = ?", user.ID).Updates(user)
	if result.Error != nil {
		return fmt.Errorf("failed to update user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	// 使用统一的缓存管理器失效用户相关缓存
	if r.cache != nil {
		_ = r.cache.InvalidateUserCache(user.ID)
	}

	return nil
}

// Delete 删除用户
func (r *userRepositoryV2) Delete(id string) error {
	result := r.db.Where("id = ?", id).Delete(&models.User{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	// 使用统一的缓存管理器失效用户相关缓存
	if r.cache != nil {
		_ = r.cache.InvalidateUserCache(id)
	}

	return nil
}

// UpdateLastLogin 更新最后登录时间
func (r *userRepositoryV2) UpdateLastLogin(id string) error {
	result := r.db.Model(&models.User{}).Where("id = ?", id).Update("last_login", "NOW()")
	if result.Error != nil {
		return fmt.Errorf("failed to update last login: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	// 使用统一的缓存管理器失效用户相关缓存
	if r.cache != nil {
		_ = r.cache.InvalidateUserCache(id)
	}

	return nil
}

// ExistsByEmail 检查邮箱是否存在
func (r *userRepositoryV2) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("email = ?", email).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if email exists: %w", err)
	}
	return count > 0, nil
}

// ExistsByUsername 检查用户名是否存在
func (r *userRepositoryV2) ExistsByUsername(username string) (bool, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("username = ?", username).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if username exists: %w", err)
	}
	return count > 0, nil
}

// Count 获取活跃用户总数
func (r *userRepositoryV2) Count() (int64, error) {
	var count int64
	err := r.db.Model(&models.User{}).Where("is_active = ?", true).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
}

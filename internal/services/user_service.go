package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go-server/internal/models"
	"go-server/internal/repositories"
	"go-server/pkg/cache"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UserService defines the interface for user business logic
type UserService interface {
	Register(req *models.RegisterRequest) (*models.User, error)
	Login(req *models.LoginRequest) (*models.User, error)
	GetByID(id string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetAll(page, limit int) ([]*models.User, int64, error)
	Update(id string, req *models.UpdateUserRequest, requesterID string) (*models.User, error)
	Delete(id string, requesterID string) error
	ChangePassword(id string, req *models.ChangePasswordRequest) error
	UpdateLastLogin(id string) error
	ValidateCredentials(email, password string) (*models.User, error)
}

type userService struct {
	userRepo repositories.UserRepository
	cache    cache.Cache // 缓存实例用于显式缓存失效
}

// NewUserService creates a new user service
func NewUserService(userRepo repositories.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

// NewUserServiceWithCache creates a new user service with caching support
// 使用缓存仓库装饰器包装基础仓库以提供缓存功能，并注入缓存实例用于显式失效
func NewUserServiceWithCache(baseRepo repositories.UserRepository, cache cache.Cache) UserService {
	// 使用缓存仓库装饰器包装基础仓库
	cachedRepo := repositories.NewCachedUserRepository(baseRepo, cache)
	return &userService{
		userRepo: cachedRepo,
		cache:    cache,
	}
}

// NewUserServiceWithCacheAndExplicitInvalidation 创建一个带有缓存支持和显式缓存失效的用户服务
// 这个构造函数提供了更细粒度的缓存控制，允许在服务层进行显式缓存失效
func NewUserServiceWithCacheAndExplicitInvalidation(baseRepo repositories.UserRepository, cache cache.Cache) UserService {
	// 使用缓存仓库装饰器包装基础仓库
	cachedRepo := repositories.NewCachedUserRepository(baseRepo, cache)
	return &userService{
		userRepo: cachedRepo,
		cache:    cache,
	}
}

// Register creates a new user
func (s *userService) Register(req *models.RegisterRequest) (*models.User, error) {
	// 检查邮箱是否已存在 - 如果使用缓存仓库，此操作将被缓存
	// Check if user already exists by email - this operation will be cached if using cached repository
	exists, err := s.userRepo.ExistsByEmail(req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check if user exists: %w", err)
	}
	if exists {
		return nil, errors.New("user with this email already exists")
	}

	// 检查用户名是否已存在 - 如果使用缓存仓库，此操作将被缓存
	// Check if username already exists - this operation will be cached if using cached repository
	exists, err = s.userRepo.ExistsByUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to check if username exists: %w", err)
	}
	if exists {
		return nil, errors.New("username already taken")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &models.User{
		ID:        uuid.New().String(),
		Username:  req.Username,
		Email:     req.Email,
		Password:  string(hashedPassword),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		IsActive:  true,
		IsAdmin:   false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 创建用户 - 如果使用缓存仓库，相关的缓存条目将被自动失效
	// Create user - if using cached repository, related cache entries will be automatically invalidated
	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 显式缓存失效 - 确保所有相关缓存条目都被立即失效
	// Explicit cache invalidation - ensure all related cache entries are invalidated immediately
	s.invalidateUserCaches(user)

	// Clear password before returning
	user.Password = ""
	return user, nil
}

// Login validates user credentials
func (s *userService) Login(req *models.LoginRequest) (*models.User, error) {
	user, err := s.ValidateCredentials(req.Email, req.Password)
	if err != nil {
		return nil, err
	}

	// 更新最后登录时间 - 如果使用缓存仓库，相关的缓存条目将被自动失效
	// Update last login - if using cached repository, related cache entries will be automatically invalidated
	if err := s.userRepo.UpdateLastLogin(user.ID); err != nil {
		// Log error but don't fail login
		fmt.Printf("Warning: Failed to update last login: %v\n", err)
	} else {
		// 显式缓存失效 - 登录成功后失效用户的缓存条目以确保登录状态更新
		// Explicit cache invalidation - invalidate user's cache entries after successful login to ensure login status is updated
		s.invalidateUserCachesByID(user.ID)
	}

	// Clear password before returning
	user.Password = ""
	return user, nil
}

// ValidateCredentials validates user credentials
func (s *userService) ValidateCredentials(email, password string) (*models.User, error) {
	// 通过邮箱获取用户 - 如果使用缓存仓库，此操作将从缓存中获取用户数据
	// Get user by email - if using cached repository, this operation will retrieve user data from cache
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.New("account is deactivated")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}

// GetByID gets a user by ID
func (s *userService) GetByID(id string) (*models.User, error) {
	// 通过ID获取用户 - 如果使用缓存仓库，此操作将从缓存中获取用户数据
	// Get user by ID - if using cached repository, this operation will retrieve user data from cache
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Clear password before returning
	user.Password = ""
	return user, nil
}

// GetByEmail gets a user by email
func (s *userService) GetByEmail(email string) (*models.User, error) {
	// 通过邮箱获取用户 - 如果使用缓存仓库，此操作将从缓存中获取用户数据
	// Get user by email - if using cached repository, this operation will retrieve user data from cache
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return nil, err
	}

	// Clear password before returning
	user.Password = ""
	return user, nil
}

// GetAll gets all users with pagination
func (s *userService) GetAll(page, limit int) ([]*models.User, int64, error) {
	offset := (page - 1) * limit
	// 获取所有用户（带分页）- 如果使用缓存仓库，此操作将从缓存中获取用户列表数据
	// Get all users with pagination - if using cached repository, this operation will retrieve user list data from cache
	users, total, err := s.userRepo.GetAll(offset, limit)
	if err != nil {
		return nil, 0, err
	}

	// Clear passwords before returning
	for _, user := range users {
		user.Password = ""
	}

	return users, total, nil
}

// Update updates a user
func (s *userService) Update(id string, req *models.UpdateUserRequest, requesterID string) (*models.User, error) {
	// 获取现有用户 - 如果使用缓存仓库，此操作将从缓存中获取用户数据
	// Get existing user - if using cached repository, this operation will retrieve user data from cache
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	// 检查用户是否正在更新自己的资料或是管理员
	// Check if user is updating their own profile or is admin
	if id != requesterID {
		// 获取请求者信息 - 如果使用缓存仓库，此操作将从缓存中获取用户数据
		// Get requester info - if using cached repository, this operation will retrieve user data from cache
		requester, err := s.userRepo.GetByID(requesterID)
		if err != nil {
			return nil, errors.New("unauthorized")
		}
		if !requester.IsAdmin {
			return nil, errors.New("you can only update your own profile")
		}
	}

	// 检查新用户名是否已被占用 - 如果使用缓存仓库，此操作将被缓存
	// Check if new username is taken - this operation will be cached if using cached repository
	if req.Username != "" && req.Username != user.Username {
		exists, err := s.userRepo.ExistsByUsername(req.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to check if username exists: %w", err)
		}
		if exists {
			return nil, errors.New("username already taken")
		}
		user.Username = req.Username
	}

	// Update fields
	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.Avatar != "" {
		user.Avatar = req.Avatar
	}

	user.UpdatedAt = time.Now()

	// 更新用户 - 如果使用缓存仓库，相关的缓存条目将被自动失效
	// Update user - if using cached repository, related cache entries will be automatically invalidated
	if err := s.userRepo.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// 显式缓存失效 - 确保所有相关缓存条目都被立即失效
	// Explicit cache invalidation - ensure all related cache entries are invalidated immediately
	s.invalidateUserCaches(user)

	// Clear password before returning
	user.Password = ""
	return user, nil
}

// Delete deletes a user
func (s *userService) Delete(id string, requesterID string) error {
	// 获取请求者以检查权限 - 如果使用缓存仓库，此操作将从缓存中获取用户数据
	// Get requester to check permissions - if using cached repository, this operation will retrieve user data from cache
	requester, err := s.userRepo.GetByID(requesterID)
	if err != nil {
		return errors.New("unauthorized")
	}

	// Users can only delete themselves or admins can delete any user
	if id != requesterID && !requester.IsAdmin {
		return errors.New("you can only delete your own account")
	}

	// 删除用户 - 如果使用缓存仓库，相关的缓存条目将被自动失效
	// Delete user - if using cached repository, related cache entries will be automatically invalidated
	if err := s.userRepo.Delete(id); err != nil {
		return err
	}

	// 显式缓存失效 - 确保所有相关缓存条目都被立即失效
	// 由于用户已被删除，我们只能通过ID来失效缓存
	// Explicit cache invalidation - ensure all related cache entries are invalidated immediately
	// Since user is deleted, we can only invalidate cache by ID
	s.invalidateUserCachesByID(id)

	return nil
}

// invalidateUserCaches 失效与用户相关的所有缓存条目
// Invalidate all cache entries related to a user
func (s *userService) invalidateUserCaches(user *models.User) {
	if s.cache == nil {
		return // 如果没有缓存实例，跳过缓存失效
	}

	ctx := context.Background()

	// 构建需要失效的缓存键列表
	// Build list of cache keys to invalidate
	keys := []string{
		fmt.Sprintf("user:id:%s", user.ID),
		fmt.Sprintf("user:email:%s", user.Email),
		fmt.Sprintf("user:username:%s", user.Username),
		fmt.Sprintf("user:exists:email:%s", user.Email),
		fmt.Sprintf("user:exists:username:%s", user.Username),
	}

	// 批量删除用户特定的缓存键
	// Batch delete user-specific cache keys
	if err := s.cache.DeleteMultiple(ctx, keys); err != nil {
		log.Printf("警告：用户缓存失效失败 (用户ID: %s): %v", user.ID, err)
		// Warning: Failed to invalidate user cache (User ID: %s): %v
	} else {
		log.Printf("成功失效用户缓存条目 (用户ID: %s, 邮箱: %s)", user.ID, user.Email)
		// Successfully invalidated user cache entries (User ID: %s, Email: %s)
	}

	// 失效用户列表缓存（可能包含该用户）
	// Invalidate user list caches (might contain this user)
	s.invalidateUserListCaches(ctx)
}

// invalidateUserCachesByID 通过用户ID失效缓存条目
// 当用户对象不可用时使用（如删除操作）
// Invalidate cache entries by user ID
// Used when user object is not available (e.g., in delete operations)
func (s *userService) invalidateUserCachesByID(userID string) {
	if s.cache == nil {
		return // 如果没有缓存实例，跳过缓存失效
	}

	ctx := context.Background()

	// 通过ID失效用户缓存
	// Invalidate user cache by ID
	if err := s.cache.Delete(ctx, fmt.Sprintf("user:id:%s", userID)); err != nil {
		log.Printf("警告：用户ID缓存失效失败 (用户ID: %s): %v", userID, err)
		// Warning: Failed to invalidate user ID cache (User ID: %s): %v
	} else {
		log.Printf("成功失效用户ID缓存条目 (用户ID: %s)", userID)
		// Successfully invalidated user ID cache entry (User ID: %s)
	}

	// 失效用户列表缓存（可能包含该用户）
	// Invalidate user list caches (might contain this user)
	s.invalidateUserListCaches(ctx)
}

// invalidateUserListCaches 失效所有用户列表相关的缓存条目
// Invalidate all user list related cache entries
func (s *userService) invalidateUserListCaches(ctx context.Context) {
	if s.cache == nil {
		return
	}

	// 失效用户列表缓存模式
	// Invalidate user list cache patterns
	patterns := []string{"users:all:*", "users:count"}

	for _, pattern := range patterns {
		keys, err := s.cache.Keys(ctx, pattern)
		if err != nil {
			log.Printf("警告：获取缓存键失败 (模式: %s): %v", pattern, err)
			// Warning: Failed to get cache keys (Pattern: %s): %v
			continue
		}

		if len(keys) > 0 {
			if err := s.cache.DeleteMultiple(ctx, keys); err != nil {
				log.Printf("警告：用户列表缓存失效失败 (模式: %s, 键数量: %d): %v", pattern, len(keys), err)
				// Warning: Failed to invalidate user list cache (Pattern: %s, Key count: %d): %v
			} else {
				log.Printf("成功失效用户列表缓存 (模式: %s, 键数量: %d)", pattern, len(keys))
				// Successfully invalidated user list cache (Pattern: %s, Key count: %d)
			}
		}
	}
}

// ChangePassword changes a user's password
func (s *userService) ChangePassword(id string, req *models.ChangePasswordRequest) error {
	// 获取用户 - 如果使用缓存仓库，此操作将从缓存中获取用户数据
	// Get user - if using cached repository, this operation will retrieve user data from cache
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return err
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return errors.New("old password is incorrect")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash new password: %w", err)
	}

	// Update password
	user.Password = string(hashedPassword)
	user.UpdatedAt = time.Now()

	// 更新用户密码 - 如果使用缓存仓库，相关的缓存条目将被自动失效
	// Update user password - if using cached repository, related cache entries will be automatically invalidated
	if err := s.userRepo.Update(user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// 显式缓存失效 - 密码更改后立即失效所有相关缓存条目
	// 这是重要的安全措施，确保密码更改立即生效
	// Explicit cache invalidation - invalidate all related cache entries immediately after password change
	// This is an important security measure to ensure password changes take effect immediately
	s.invalidateUserCaches(user)

	return nil
}

// UpdateLastLogin updates the last login time for a user
func (s *userService) UpdateLastLogin(id string) error {
	// 更新用户最后登录时间 - 如果使用缓存仓库，相关的缓存条目将被自动失效
	// Update user last login time - if using cached repository, related cache entries will be automatically invalidated
	if err := s.userRepo.UpdateLastLogin(id); err != nil {
		return err
	}

	// 显式缓存失效 - 最后登录时间更新后立即失效相关缓存条目
	// Explicit cache invalidation - invalidate related cache entries immediately after last login time update
	s.invalidateUserCachesByID(id)

	return nil
}

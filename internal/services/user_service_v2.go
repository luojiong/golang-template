package services

import (
	"context"
	"errors"
	"fmt"

	"go-server/internal/cache_manager"
	"go-server/internal/domain/user"
	"go-server/internal/monitoring"
	"go-server/internal/validation"
)

// UserServiceV2 改进的用户服务接口
type UserServiceV2 interface {
	// 用户注册
	Register(ctx context.Context, dto *validation.RegisterUserDTO) (*user.User, error)

	// 用户登录
	Login(ctx context.Context, dto *validation.LoginUserDTO) (*user.User, error)

	// 获取用户信息
	GetByID(ctx context.Context, userID string) (*user.User, error)
	GetByEmail(ctx context.Context, email string) (*user.User, error)
	GetByUsername(ctx context.Context, username string) (*user.User, error)

	// 获取用户列表
	GetAll(ctx context.Context, page, limit int) ([]*user.User, int64, error)

	// 更新用户信息
	Update(ctx context.Context, userID string, dto *validation.UpdateUserDTO, requesterID string) (*user.User, error)

	// 删除用户
	Delete(ctx context.Context, userID, requesterID string) error

	// 修改密码
	ChangePassword(ctx context.Context, userID string, dto *validation.ChangePasswordDTO) error

	// 更新最后登录时间
	UpdateLastLogin(ctx context.Context, userID string) error
}

// userServiceV2 改进的用户服务实现
type userServiceV2 struct {
	domainService user.Service
	repo          user.Repository
	mapper        *user.Mapper
	cacheManager  cache_manager.Manager
	metrics       monitoring.MetricsCollector
	baseService   BaseService
}

// NewUserServiceV2 创建改进的用户服务
func NewUserServiceV2(
	domainService user.Service,
	repo user.Repository,
	cacheManager cache_manager.Manager,
	metrics monitoring.MetricsCollector,
) UserServiceV2 {
	return &userServiceV2{
		domainService: domainService,
		repo:          repo,
		mapper:        user.NewMapper(),
		cacheManager:  cacheManager,
		metrics:       metrics,
		baseService:   NewBaseService(cacheManager),
	}
}

// Register 用户注册
func (s *userServiceV2) Register(ctx context.Context, dto *validation.RegisterUserDTO) (*user.User, error) {
	// 验证DTO
	if err := dto.Validate(); err != nil {
		s.metrics.RecordUserRegistration(false)
		return nil, s.baseService.HandleError(ctx, err, "validate register DTO")
	}

	// 创建值对象
	email, err := user.NewEmail(dto.Email)
	if err != nil {
		s.metrics.RecordUserRegistration(false)
		return nil, s.baseService.HandleError(ctx, err, "create email value object")
	}

	username, err := user.NewUsername(dto.Username)
	if err != nil {
		s.metrics.RecordUserRegistration(false)
		return nil, s.baseService.HandleError(ctx, err, "create username value object")
	}

	password, err := user.NewPassword(dto.Password)
	if err != nil {
		s.metrics.RecordUserRegistration(false)
		return nil, s.baseService.HandleError(ctx, err, "create password value object")
	}

	profile := user.NewUserProfile(dto.FirstName, dto.LastName, dto.Avatar)

	// 调用领域服务注册用户
	domainUser, err := s.domainService.RegisterUser(email, username, password, profile)
	if err != nil {
		s.metrics.RecordUserRegistration(false)
		return nil, s.baseService.HandleError(ctx, err, "register user in domain")
	}

	// 失效相关缓存
	if s.cacheManager != nil {
		_ = s.cacheManager.InvalidateUserListCache()
	}

	s.metrics.RecordUserRegistration(true)
	s.metrics.RecordUserAction("register")

	return domainUser, nil
}

// Login 用户登录
func (s *userServiceV2) Login(ctx context.Context, dto *validation.LoginUserDTO) (*user.User, error) {
	// 验证DTO
	if err := dto.Validate(); err != nil {
		s.metrics.RecordUserLogin(false)
		return nil, s.baseService.HandleError(ctx, err, "validate login DTO")
	}

	// 创建邮箱值对象
	email, err := user.NewEmail(dto.Email)
	if err != nil {
		s.metrics.RecordUserLogin(false)
		return nil, s.baseService.HandleError(ctx, err, "create email value object for login")
	}

	// 调用领域服务认证用户
	domainUser, err := s.domainService.AuthenticateUser(email, dto.Password)
	if err != nil {
		s.metrics.RecordUserLogin(false)
		return nil, s.baseService.HandleError(ctx, err, "authenticate user in domain")
	}

	// 失效相关缓存
	if s.cacheManager != nil && domainUser != nil {
		_ = s.cacheManager.InvalidateUserCache(domainUser.ID().String())
	}

	s.metrics.RecordUserLogin(true)
	s.metrics.RecordUserAction("login")

	return domainUser, nil
}

// GetByID 根据ID获取用户
func (s *userServiceV2) GetByID(ctx context.Context, userID string) (*user.User, error) {
	// 首先尝试从缓存获取
	if s.cacheManager != nil {
		cacheKey := "user:id:" + userID
		if userModel, found := s.cacheManager.GetUserFromCache(cacheKey); found {
			s.metrics.RecordCacheHit("user", "get_by_id")
			domainUser, err := s.mapper.ToDomainModel(userModel)
			if err != nil {
				return nil, s.baseService.HandleError(ctx, err, "convert cached user to domain model")
			}
			return domainUser, nil
		}
		s.metrics.RecordCacheMiss("user", "get_by_id")
	}

	// 创建用户ID值对象
	userIDValue, err := user.NewUserIDFromString(userID)
	if err != nil {
		return nil, s.baseService.HandleError(ctx, err, "create user ID value object")
	}

	// 从数据库获取
	domainUser, err := s.repo.FindByID(userIDValue)
	if err != nil {
		return nil, s.baseService.HandleError(ctx, err, "find user by ID")
	}

	// 缓存结果
	if s.cacheManager != nil && domainUser != nil {
		cacheKey := "user:id:" + userID
		userModel := s.mapper.ToDataModel(domainUser)
		_ = s.cacheManager.SetUserCache(cacheKey, userModel, 0)
	}

	s.metrics.RecordUserAction("get_by_id")
	return domainUser, nil
}

// GetByEmail 根据邮箱获取用户
func (s *userServiceV2) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	// 创建邮箱值对象
	emailValue, err := user.NewEmail(email)
	if err != nil {
		return nil, s.baseService.HandleError(ctx, err, "create email value object")
	}

	// 首先尝试从缓存获取
	if s.cacheManager != nil {
		cacheKey := "user:email:" + emailValue.String()
		if userModel, found := s.cacheManager.GetUserFromCache(cacheKey); found {
			s.metrics.RecordCacheHit("user", "get_by_email")
			domainUser, err := s.mapper.ToDomainModel(userModel)
			if err != nil {
				return nil, s.baseService.HandleError(ctx, err, "convert cached user to domain model")
			}
			return domainUser, nil
		}
		s.metrics.RecordCacheMiss("user", "get_by_email")
	}

	// 从数据库获取
	domainUser, err := s.repo.FindByEmail(emailValue)
	if err != nil {
		return nil, s.baseService.HandleError(ctx, err, "find user by email")
	}

	// 缓存结果
	if s.cacheManager != nil && domainUser != nil {
		cacheKey := "user:email:" + emailValue.String()
		userModel := s.mapper.ToDataModel(domainUser)
		_ = s.cacheManager.SetUserCache(cacheKey, userModel, 0)
	}

	s.metrics.RecordUserAction("get_by_email")
	return domainUser, nil
}

// GetByUsername 根据用户名获取用户
func (s *userServiceV2) GetByUsername(ctx context.Context, username string) (*user.User, error) {
	// 创建用户名值对象
	usernameValue, err := user.NewUsername(username)
	if err != nil {
		return nil, s.baseService.HandleError(ctx, err, "create username value object")
	}

	// 首先尝试从缓存获取
	if s.cacheManager != nil {
		cacheKey := "user:username:" + usernameValue.String()
		if userModel, found := s.cacheManager.GetUserFromCache(cacheKey); found {
			s.metrics.RecordCacheHit("user", "get_by_username")
			domainUser, err := s.mapper.ToDomainModel(userModel)
			if err != nil {
				return nil, s.baseService.HandleError(ctx, err, "convert cached user to domain model")
			}
			return domainUser, nil
		}
		s.metrics.RecordCacheMiss("user", "get_by_username")
	}

	// 从数据库获取
	domainUser, err := s.repo.FindByUsername(usernameValue)
	if err != nil {
		return nil, s.baseService.HandleError(ctx, err, "find user by username")
	}

	// 缓存结果
	if s.cacheManager != nil && domainUser != nil {
		cacheKey := "user:username:" + usernameValue.String()
		userModel := s.mapper.ToDataModel(domainUser)
		_ = s.cacheManager.SetUserCache(cacheKey, userModel, 0)
	}

	s.metrics.RecordUserAction("get_by_username")
	return domainUser, nil
}

// GetAll 获取用户列表
func (s *userServiceV2) GetAll(ctx context.Context, page, limit int) ([]*user.User, int64, error) {
	offset := (page - 1) * limit

	// 首先尝试从缓存获取
	if s.cacheManager != nil {
		cacheKey := fmt.Sprintf("users:all:%d:%d", offset, limit)
		if userModels, total, found := s.cacheManager.GetUserListFromCache(cacheKey); found {
			s.metrics.RecordCacheHit("user", "get_all")
			users, err := s.mapper.ToDomainModels(userModels)
			if err != nil {
				return nil, 0, s.baseService.HandleError(ctx, err, "convert cached users to domain models")
			}
			return users, total, nil
		}
		s.metrics.RecordCacheMiss("user", "get_all")
	}

	// 从数据库获取
	users, total, err := s.repo.FindAll(offset, limit)
	if err != nil {
		return nil, 0, s.baseService.HandleError(ctx, err, "find all users")
	}

	// 缓存结果
	if s.cacheManager != nil {
		cacheKey := fmt.Sprintf("users:all:%d:%d", offset, limit)
		userModels := s.mapper.ToDataModels(users)
		_ = s.cacheManager.SetUserListCache(cacheKey, userModels, total, 0)
	}

	s.metrics.RecordUserAction("get_all")
	return users, total, nil
}

// Update 更新用户信息
func (s *userServiceV2) Update(ctx context.Context, userID string, dto *validation.UpdateUserDTO, requesterID string) (*user.User, error) {
	// 验证DTO
	if err := dto.Validate(); err != nil {
		return nil, s.baseService.HandleError(ctx, err, "validate update DTO")
	}

	// 获取要更新的用户
	domainUser, err := s.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 检查权限（用户只能更新自己的信息，或者管理员可以更新任何用户）
	if userID != requesterID {
		requester, err := s.GetByID(ctx, requesterID)
		if err != nil {
			return nil, errors.New("unauthorized: requester not found")
		}
		if !requester.IsAdmin() {
			return nil, errors.New("unauthorized: can only update your own profile")
		}
	}

	// 更新用户信息
	if dto.Username != "" {
		username, err := user.NewUsername(dto.Username)
		if err != nil {
			return nil, s.baseService.HandleError(ctx, err, "create username value object for update")
		}
		if err := domainUser.ChangeUsername(username); err != nil {
			return nil, s.baseService.HandleError(ctx, err, "change username")
		}
	}

	if dto.FirstName != "" || dto.LastName != "" || dto.Avatar != "" {
		domainUser.UpdateProfile(dto.FirstName, dto.LastName, dto.Avatar)
	}

	// 保存更新
	if err := s.repo.Save(domainUser); err != nil {
		return nil, s.baseService.HandleError(ctx, err, "save updated user")
	}

	// 失效相关缓存
	if s.cacheManager != nil {
		_ = s.cacheManager.InvalidateUserCache(userID)
		_ = s.cacheManager.InvalidateUserListCache()
	}

	s.metrics.RecordUserAction("update")
	return domainUser, nil
}

// Delete 删除用户
func (s *userServiceV2) Delete(ctx context.Context, userID, requesterID string) error {
	// 获取请求者信息
	requester, err := s.GetByID(ctx, requesterID)
	if err != nil {
		return errors.New("unauthorized: requester not found")
	}

	// 检查权限（用户只能删除自己，或者管理员可以删除任何用户）
	if userID != requesterID && !requester.IsAdmin() {
		return errors.New("unauthorized: can only delete your own account")
	}

	// 创建用户ID值对象
	userIDValue, err := user.NewUserIDFromString(userID)
	if err != nil {
		return s.baseService.HandleError(ctx, err, "create user ID value object for deletion")
	}

	// 删除用户
	if err := s.repo.Delete(userIDValue); err != nil {
		return s.baseService.HandleError(ctx, err, "delete user")
	}

	// 失效相关缓存
	if s.cacheManager != nil {
		_ = s.cacheManager.InvalidateUserCache(userID)
		_ = s.cacheManager.InvalidateUserListCache()
	}

	s.metrics.RecordUserAction("delete")
	return nil
}

// ChangePassword 修改密码
func (s *userServiceV2) ChangePassword(ctx context.Context, userID string, dto *validation.ChangePasswordDTO) error {
	// 验证DTO
	if err := dto.Validate(); err != nil {
		return s.baseService.HandleError(ctx, err, "validate change password DTO")
	}

	// 创建用户ID值对象
	userIDValue, err := user.NewUserIDFromString(userID)
	if err != nil {
		return s.baseService.HandleError(ctx, err, "create user ID value object for password change")
	}

	// 调用领域服务修改密码
	if err := s.domainService.ChangeUserPassword(userIDValue, dto.OldPassword, dto.NewPassword); err != nil {
		return s.baseService.HandleError(ctx, err, "change password in domain")
	}

	// 失效相关缓存
	if s.cacheManager != nil {
		_ = s.cacheManager.InvalidateUserCache(userID)
	}

	s.metrics.RecordUserAction("change_password")
	return nil
}

// UpdateLastLogin 更新最后登录时间
func (s *userServiceV2) UpdateLastLogin(ctx context.Context, userID string) error {
	// 获取用户
	domainUser, err := s.GetByID(ctx, userID)
	if err != nil {
		return s.baseService.HandleError(ctx, err, "get user for last login update")
	}

	// 更新最后登录时间
	domainUser.UpdateLastLogin()

	// 保存更新
	if err := s.repo.Save(domainUser); err != nil {
		return s.baseService.HandleError(ctx, err, "save last login update")
	}

	// 失效相关缓存
	if s.cacheManager != nil {
		_ = s.cacheManager.InvalidateUserCache(userID)
	}

	s.metrics.RecordUserAction("update_last_login")
	return nil
}

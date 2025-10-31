package user

import (
	"fmt"
	"time"

	customerrors "go-server/internal/errors"
)

// DomainService 用户领域服务
type DomainService struct {
	repo Repository
}

// NewDomainService 创建领域服务
func NewDomainService(repo Repository) Service {
	return &DomainService{
		repo: repo,
	}
}

// RegisterUser 注册用户
func (s *DomainService) RegisterUser(
	email Email,
	username Username,
	password Password,
	profile UserProfile,
) (*User, error) {
	// 检查邮箱是否已存在
	exists, err := s.repo.ExistsByEmail(email)
	if err != nil {
		return nil, customerrors.NewDatabaseError("failed to check email existence", err)
	}
	if exists {
		return nil, customerrors.NewConflictError("email already registered").WithDetails(map[string]interface{}{
			"field": "email",
			"value": email.String(),
		})
	}

	// 检查用户名是否已存在
	exists, err = s.repo.ExistsByUsername(username)
	if err != nil {
		return nil, customerrors.NewDatabaseError("failed to check username existence", err)
	}
	if exists {
		return nil, customerrors.NewConflictError("username already taken").WithDetails(map[string]interface{}{
			"field": "username",
			"value": username.String(),
		})
	}

	// 生成用户ID（在实际应用中应该使用UUID生成器）
	userID, err := NewUserID(generateUserID())
	if err != nil {
		return nil, customerrors.NewInternalError("failed to generate user ID", err)
	}

	// 创建用户
	user, err := NewUser(
		userID,
		email,
		username,
		password,
		profile,
		UserRoleRegular, // 默认为普通用户
	)
	if err != nil {
		return nil, customerrors.NewValidationError("invalid user data: " + err.Error())
	}

	// 保存用户
	if err := s.repo.Save(user); err != nil {
		return nil, customerrors.NewDatabaseError("failed to save user", err)
	}

	// 在实际应用中，这里可以发布领域事件
	// s.eventBus.Publish(UserRegisteredEvent{...})

	return user, nil
}

// AuthenticateUser 认证用户
func (s *DomainService) AuthenticateUser(email Email, password string) (*User, error) {
	// 查找用户
	user, err := s.repo.FindByEmail(email)
	if err != nil {
		return nil, customerrors.NewUnauthorizedError("invalid credentials")
	}

	// 检查用户是否活跃
	if !user.IsActive() {
		return nil, customerrors.NewUnauthorizedError("account is deactivated")
	}

	// 验证密码
	if !user.VerifyPassword(password) {
		return nil, customerrors.NewUnauthorizedError("invalid credentials")
	}

	// 更新最后登录时间
	user.UpdateLastLogin()
	if err := s.repo.Save(user); err != nil {
		// 记录错误但不影响登录流程
		fmt.Printf("Warning: Failed to update last login: %v\n", err)
	}

	// 发布登录事件
	// s.eventBus.Publish(UserLoggedInEvent{...})

	return user, nil
}

// ChangeUserPassword 更改用户密码
func (s *DomainService) ChangeUserPassword(userID UserID, oldPassword, newPassword string) error {
	// 查找用户
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return customerrors.NewNotFoundError("user")
	}

	// 验证旧密码
	if !user.VerifyPassword(oldPassword) {
		return customerrors.NewValidationError("old password is incorrect")
	}

	// 创建新密码
	newPasswordObj, err := NewPassword(newPassword)
	if err != nil {
		return customerrors.NewValidationError("invalid new password: " + err.Error())
	}

	// 更新密码
	user.ChangePassword(newPasswordObj)

	// 保存用户
	if err := s.repo.Save(user); err != nil {
		return customerrors.NewDatabaseError("failed to update password", err)
	}

	// 发布密码更改事件
	// s.eventBus.Publish(UserPasswordChangedEvent{...})

	return nil
}

// Helper Functions

// generateUserID 生成用户ID（简化版本）
func generateUserID() string {
	// 在实际应用中应该使用UUID生成器
	return fmt.Sprintf("user_%d", time.Now().UnixNano())
}

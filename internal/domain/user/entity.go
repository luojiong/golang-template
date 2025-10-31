package user

import (
	"errors"
	"time"
)

// User 用户领域实体
type User struct {
	id        UserID
	email     Email
	username  Username
	password  Password
	profile   UserProfile
	status    UserStatus
	role      UserRole
	createdAt time.Time
	updatedAt time.Time
	lastLogin *time.Time
}

// NewUser 创建新用户
func NewUser(
	id UserID,
	email Email,
	username Username,
	password Password,
	profile UserProfile,
	role UserRole,
) (*User, error) {
	if id.value == "" {
		return nil, errors.New("user ID is required")
	}

	now := time.Now()

	return &User{
		id:        id,
		email:     email,
		username:  username,
		password:  password,
		profile:   profile,
		status:    UserStatusActive,
		role:      role,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// Getters
func (u *User) ID() UserID            { return u.id }
func (u *User) Email() Email          { return u.email }
func (u *User) Username() Username    { return u.username }
func (u *User) Profile() UserProfile  { return u.profile }
func (u *User) Status() UserStatus    { return u.status }
func (u *User) Role() UserRole        { return u.role }
func (u *User) CreatedAt() time.Time  { return u.createdAt }
func (u *User) UpdatedAt() time.Time  { return u.updatedAt }
func (u *User) LastLogin() *time.Time { return u.lastLogin }

// IsActive 是否活跃
func (u *User) IsActive() bool {
	return u.status == UserStatusActive
}

// IsAdmin 是否是管理员
func (u *User) IsAdmin() bool {
	return u.role.IsAdmin()
}

// Business Methods

// ChangeEmail 更改邮箱
func (u *User) ChangeEmail(newEmail Email) error {
	if newEmail.String() == u.email.String() {
		return errors.New("new email is the same as current email")
	}

	u.email = newEmail
	u.updatedAt = time.Now()

	return nil
}

// ChangeUsername 更改用户名
func (u *User) ChangeUsername(newUsername Username) error {
	if newUsername.String() == u.username.String() {
		return errors.New("new username is the same as current username")
	}

	u.username = newUsername
	u.updatedAt = time.Now()

	return nil
}

// UpdateProfile 更新档案
func (u *User) UpdateProfile(firstName, lastName, avatar string) {
	u.profile.UpdateProfile(firstName, lastName, avatar)
	u.updatedAt = time.Now()
}

// ChangePassword 更改密码
func (u *User) ChangePassword(newPassword Password) {
	u.password = newPassword
	u.updatedAt = time.Now()
}

// VerifyPassword 验证密码
func (u *User) VerifyPassword(plainPassword string) bool {
	return u.password.Verify(plainPassword)
}

// UpdateLastLogin 更新最后登录时间
func (u *User) UpdateLastLogin() {
	now := time.Now()
	u.lastLogin = &now
	u.updatedAt = now
}

// Activate 激活用户
func (u *User) Activate() {
	if u.status != UserStatusActive {
		u.status = UserStatusActive
		u.updatedAt = time.Now()
	}
}

// Deactivate 停用用户
func (u *User) Deactivate() {
	if u.status != UserStatusInactive {
		u.status = UserStatusInactive
		u.updatedAt = time.Now()
	}
}

// Suspend 暂停用户
func (u *User) Suspend() {
	if u.status != UserStatusSuspended {
		u.status = UserStatusSuspended
		u.updatedAt = time.Now()
	}
}

// PromoteToAdmin 提升为管理员
func (u *User) PromoteToAdmin() {
	if !u.IsAdmin() {
		u.role = UserRoleAdmin
		u.updatedAt = time.Now()
	}
}

// DemoteToUser 降级为普通用户
func (u *User) DemoteToUser() {
	if u.IsAdmin() {
		u.role = UserRoleRegular
		u.updatedAt = time.Now()
	}
}

// CanAccessResource 检查用户是否可以访问资源
func (u *User) CanAccessResource(resource string, action string) bool {
	// 基本的访问控制逻辑
	if !u.IsActive() {
		return false
	}

	// 管理员可以访问所有资源
	if u.IsAdmin() {
		return true
	}

	// 这里可以实现更复杂的权限逻辑
	// 暂时允许所有活跃用户访问基本资源
	return true
}

// Repository Interface for User Domain
type Repository interface {
	Save(user *User) error
	FindByID(id UserID) (*User, error)
	FindByEmail(email Email) (*User, error)
	FindByUsername(username Username) (*User, error)
	FindAll(offset, limit int) ([]*User, int64, error)
	Delete(id UserID) error
	ExistsByEmail(email Email) (bool, error)
	ExistsByUsername(username Username) (bool, error)
	Count() (int64, error)
}

// Domain Service Interface
type Service interface {
	RegisterUser(email Email, username Username, password Password, profile UserProfile) (*User, error)
	AuthenticateUser(email Email, password string) (*User, error)
	ChangeUserPassword(userID UserID, oldPassword, newPassword string) error
}

// Domain Events
type UserEvent interface {
	UserID() UserID
	OccurredAt() time.Time
}

type UserRegisteredEvent struct {
	userID     UserID
	email      Email
	username   Username
	occurredAt time.Time
}

func (e *UserRegisteredEvent) UserID() UserID        { return e.userID }
func (e *UserRegisteredEvent) OccurredAt() time.Time { return e.occurredAt }

type UserPasswordChangedEvent struct {
	userID     UserID
	occurredAt time.Time
}

func (e *UserPasswordChangedEvent) UserID() UserID        { return e.userID }
func (e *UserPasswordChangedEvent) OccurredAt() time.Time { return e.occurredAt }

type UserLoggedInEvent struct {
	userID     UserID
	occurredAt time.Time
}

func (e *UserLoggedInEvent) UserID() UserID        { return e.userID }
func (e *UserLoggedInEvent) OccurredAt() time.Time { return e.occurredAt }

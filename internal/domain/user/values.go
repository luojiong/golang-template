package user

import (
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// UserID 用户ID值对象
type UserID struct {
	value string
}

// NewUserID 创建新的用户ID（生成新的UUID）
func NewUserID() UserID {
	return UserID{value: uuid.New().String()}
}

// NewUserIDFromString 从字符串创建用户ID
func NewUserIDFromString(id string) (UserID, error) {
	id = strings.TrimSpace(id)
	if _, err := uuid.Parse(id); err != nil {
		return UserID{}, errors.New("invalid user ID format (must be a UUID)")
	}
	return UserID{value: id}, nil
}

// String 返回字符串值
func (u UserID) String() string {
	return u.value
}

// Equals 比较是否相等
func (u UserID) Equals(other UserID) bool {
	return u.value == other.value
}

// Email 邮箱值对象
type Email struct {
	value string
}

// NewEmail 创建新的邮箱
func NewEmail(email string) (Email, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	if email == "" {
		return Email{}, errors.New("email cannot be empty")
	}

	// 简单的邮箱验证
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return Email{}, errors.New("invalid email format")
	}

	return Email{value: email}, nil
}

// String 返回字符串值
func (e Email) String() string {
	return e.value
}

// Domain 返回域名部分
func (e Email) Domain() string {
	parts := strings.Split(e.value, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// Username 用户名值对象
type Username struct {
	value string
}

// NewUsername 创建新的用户名
func NewUsername(username string) (Username, error) {
	username = strings.TrimSpace(username)

	if username == "" {
		return Username{}, errors.New("username cannot be empty")
	}

	if len(username) < 3 {
		return Username{}, errors.New("username must be at least 3 characters")
	}

	if len(username) > 50 {
		return Username{}, errors.New("username must be at most 50 characters")
	}

	// 用户名只能包含字母、数字、下划线和连字符
	usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !usernameRegex.MatchString(username) {
		return Username{}, errors.New("username can only contain letters, numbers, underscores and hyphens")
	}

	return Username{value: username}, nil
}

// String 返回字符串值
func (u Username) String() string {
	return u.value
}

// Password 密码值对象
type Password struct {
	hashedValue string
}

// NewPassword 创建新密码（自动哈希）
func NewPassword(plainPassword string) (Password, error) {
	if len(plainPassword) < 6 {
		return Password{}, errors.New("password must be at least 6 characters")
	}

	// 这里应该使用实际的哈希算法，暂时简化
	// 在实际实现中应该使用 bcrypt
	hashedValue := hashPassword(plainPassword)

	return Password{hashedValue: hashedValue}, nil
}

// NewPasswordFromHash 从已有哈希值创建密码
func NewPasswordFromHash(hashedValue string) (Password, error) {
	if hashedValue == "" {
		return Password{}, errors.New("hashed password cannot be empty")
	}
	return Password{hashedValue: hashedValue}, nil
}

// Hash 返回哈希值
func (p Password) Hash() string {
	return p.hashedValue
}

// Verify 验证密码
func (p Password) Verify(plainPassword string) bool {
	// 这里应该使用实际的验证算法，暂时简化
	// 在实际实现中应该使用 bcrypt.CompareHashAndPassword
	return verifyPassword(plainPassword, p.hashedValue)
}

// UserProfile 用户档案值对象
type UserProfile struct {
	firstName string
	lastName  string
	avatar    string
}

// NewUserProfile 创建用户档案
func NewUserProfile(firstName, lastName, avatar string) UserProfile {
	return UserProfile{
		firstName: strings.TrimSpace(firstName),
		lastName:  strings.TrimSpace(lastName),
		avatar:    strings.TrimSpace(avatar),
	}
}

// FirstName 返回名
func (p UserProfile) FirstName() string {
	return p.firstName
}

// LastName 返回姓
func (p UserProfile) LastName() string {
	return p.lastName
}

// FullName 返回全名
func (p UserProfile) FullName() string {
	if p.firstName != "" && p.lastName != "" {
		return p.firstName + " " + p.lastName
	}
	if p.firstName != "" {
		return p.firstName
	}
	return p.lastName
}

// Avatar 返回头像URL
func (p UserProfile) Avatar() string {
	return p.avatar
}

// UpdateProfile 更新档案信息
func (p *UserProfile) UpdateProfile(firstName, lastName, avatar string) {
	p.firstName = strings.TrimSpace(firstName)
	p.lastName = strings.TrimSpace(lastName)
	p.avatar = strings.TrimSpace(avatar)
}

// UserStatus 用户状态枚举
type UserStatus int

const (
	UserStatusInactive UserStatus = iota
	UserStatusActive
	UserStatusSuspended
)

// String 返回状态字符串
func (s UserStatus) String() string {
	switch s {
	case UserStatusActive:
		return "active"
	case UserStatusInactive:
		return "inactive"
	case UserStatusSuspended:
		return "suspended"
	default:
		return "unknown"
	}
}

// UserRole 用户角色枚举
type UserRole int

const (
	UserRoleRegular UserRole = iota
	UserRoleAdmin
)

// String 返回角色字符串
func (r UserRole) String() string {
	switch r {
	case UserRoleAdmin:
		return "admin"
	case UserRoleRegular:
		return "user"
	default:
		return "unknown"
	}
}

// IsAdmin 是否是管理员
func (r UserRole) IsAdmin() bool {
	return r == UserRoleAdmin
}

// 临时哈希函数（实际应该使用bcrypt）
func hashPassword(password string) string {
	// 这里应该使用实际的哈希算法
	// 暂时返回简单的哈希值
	return "hashed_" + password
}

// 临时验证函数（实际应该使用bcrypt）
func verifyPassword(plainPassword, hashedPassword string) bool {
	// 这里应该使用实际的验证算法
	// 暂时进行简单的比较
	return hashedPassword == "hashed_"+plainPassword
}

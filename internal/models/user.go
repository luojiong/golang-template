package models

import (
	"time"

	"gorm.io/gorm"
)

// User 系统中的用户模型，包含GORM注解
type User struct {
	ID        string         `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"` // 用户ID
	Username  string         `json:"username" gorm:"type:varchar(50);uniqueIndex;not null"`    // 用户名
	Email     string         `json:"email" gorm:"type:varchar(100);uniqueIndex;not null"`      // 邮箱地址
	Password  string         `json:"-" gorm:"type:varchar(255);not null"`                      // 密码（不序列化）
	FirstName string         `json:"first_name" gorm:"type:varchar(50)"`                        // 名
	LastName  string         `json:"last_name" gorm:"type:varchar(50)"`                         // 姓
	Avatar    string         `json:"avatar" gorm:"type:varchar(255)"`                            // 头像URL
	IsActive  bool           `json:"is_active" gorm:"default:true"`                              // 是否激活
	IsAdmin   bool           `json:"is_admin" gorm:"default:false"`                             // 是否为管理员
	LastLogin *time.Time     `json:"last_login"`                                                // 最后登录时间
	CreatedAt time.Time      `json:"created_at"`                                                // 创建时间
	UpdatedAt time.Time      `json:"updated_at"`                                                // 更新时间
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`                                            // 删除时间（软删除）
}

// TableName 返回User模型的表名
func (User) TableName() string {
	return "users"
}

// BeforeCreate 创建前钩子，如果未提供UUID则生成
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		// 使用github.com/google/uuid生成UUID
		// 这将在服务层处理
	}
	return nil
}

// GetFullName 返回用户全名
func (u *User) GetFullName() string {
	if u.FirstName != "" && u.LastName != "" {
		return u.FirstName + " " + u.LastName
	}
	if u.FirstName != "" {
		return u.FirstName
	}
	if u.LastName != "" {
		return u.LastName
	}
	return u.Username
}

// ToSafeUser 返回不包含敏感信息的用户对象
func (u *User) ToSafeUser() SafeUser {
	return SafeUser{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Avatar:    u.Avatar,
		IsActive:  u.IsActive,
		IsAdmin:   u.IsAdmin,
		LastLogin: u.LastLogin,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// SafeUser 不包含敏感信息的用户对象
type SafeUser struct {
	ID        string     `json:"id"`         // 用户ID
	Username  string     `json:"username"`   // 用户名
	Email     string     `json:"email"`      // 邮箱地址
	FirstName string     `json:"first_name"` // 名
	LastName  string     `json:"last_name"`  // 姓
	Avatar    string     `json:"avatar"`     // 头像URL
	IsActive  bool       `json:"is_active"`  // 是否激活
	IsAdmin   bool       `json:"is_admin"`   // 是否为管理员
	LastLogin *time.Time `json:"last_login"` // 最后登录时间
	CreatedAt time.Time  `json:"created_at"` // 创建时间
	UpdatedAt time.Time  `json:"updated_at"` // 更新时间
}
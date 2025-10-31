package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestUser_ToSafeUser(t *testing.T) {
	// 测试用户转换为安全用户
	user := &User{
		ID:        uuid.New().String(),
		Username:  "testuser",
		Email:     "test@example.com",
		Password:  "hashedpassword",
		FirstName: "Test",
		LastName:  "User",
		IsActive:  true,
		IsAdmin:   false,
		CreatedAt:  time.Now(),
		UpdatedAt: time.Now(),
	}

	safeUser := user.ToSafeUser()

	// SafeUser 不应该有 Password 字段，这是设计的一部分
	// 测试重点是确保其他字段正确传递
	assert.Equal(t, user.ID, safeUser.ID, "ID should be preserved")
	assert.Equal(t, user.Username, safeUser.Username, "Username should be preserved")
	assert.Equal(t, user.Email, safeUser.Email, "Email should be preserved")
	assert.Equal(t, user.FirstName, safeUser.FirstName, "FirstName should be preserved")
	assert.Equal(t, user.LastName, safeUser.LastName, "LastName should be preserved")
	assert.Equal(t, user.IsActive, safeUser.IsActive, "IsActive should be preserved")
	assert.Equal(t, user.IsAdmin, safeUser.IsAdmin, "IsAdmin should be preserved")
}

func TestUser_Validation(t *testing.T) {
	// 测试用户验证函数
	user := &User{
		Username: "testuser", // 添加有效的用户名
		Email:    "invalid-email",
	}

	err := user.Validate()
	assert.Error(t, err, "Invalid email should return error")

	user.Email = "valid@example.com"
	err = user.Validate()
	assert.NoError(t, err, "Valid email should not return error")
}

func TestUser_IsActiveUser(t *testing.T) {
	// 测试用户活跃状态检查
	user := &User{
		IsActive: true,
	}

	assert.True(t, user.IsActiveUser(), "Active user should return true")

	user.IsActive = false
	assert.False(t, user.IsActiveUser(), "Inactive user should return false")
}

func TestUser_GetFullName(t *testing.T) {
	// 测试获取全名
	user := &User{
		FirstName: "John",
		LastName:  "Doe",
	}

	assert.Equal(t, "John Doe", user.GetFullName(), "Should return concatenated full name")

	user.FirstName = ""
	assert.Equal(t, "Doe", user.GetFullName(), "Should return last name when first name is empty")

	user.LastName = ""
	assert.Empty(t, user.GetFullName(), "Should return empty string when both names are empty")
}

func TestUser_GetRoles(t *testing.T) {
	// 测试获取用户角色
	user := &User{
		IsAdmin: true,
	}

	roles := user.GetRoles()
	assert.Contains(t, roles, "admin", "Admin user should have admin role")

	user.IsAdmin = false
	roles = user.GetRoles()
	assert.Contains(t, roles, "user", "Regular user should have user role")
	assert.NotContains(t, roles, "admin", "Non-admin user should not have admin role")
}
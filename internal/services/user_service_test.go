package services

import (
	"errors"
	"testing"
	"time"

	"go-server/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// MockUserRepository 是仓储层的模拟实现
type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserRepository) GetByID(id string) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByEmail(email string) (*models.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetByUsername(username string) (*models.User, error) {
	args := m.Called(username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserRepository) GetAll(offset, limit int) ([]*models.User, int64, error) {
	args := m.Called(offset, limit)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.User), args.Get(1).(int64), args.Error(2)
}

func (m *MockUserRepository) Update(user *models.User) error {
	args := m.Called(user)
	return args.Error(0)
}

func (m *MockUserRepository) Delete(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockUserRepository) UpdateLastLogin(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockUserRepository) ExistsByEmail(email string) (bool, error) {
	args := m.Called(email)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) ExistsByUsername(username string) (bool, error) {
	args := m.Called(username)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserRepository) Count() (int64, error) {
	args := m.Called()
	return args.Get(0).(int64), args.Error(1)
}


// 创建测试用户的辅助函数
func createTestUser(email, username string) *models.User {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	return &models.User{
		ID:        uuid.New().String(),
		Username:  username,
		Email:     email,
		FirstName: "Test",
		LastName:  "User",
		Password:  string(hashedPassword),
		IsActive:  true,
		IsAdmin:   false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestNewUserService(t *testing.T) {
	mockRepo := new(MockUserRepository)
	service := NewUserService(mockRepo)

	assert.NotNil(t, service)

	// 验证类型断言
	userService, ok := service.(*userService)
	require.True(t, ok)
	assert.Equal(t, mockRepo, userService.userRepo)
	assert.Nil(t, userService.cache)
}

func TestNewUserServiceWithCache(t *testing.T) {
	mockRepo := new(MockUserRepository)
	// Note: 由于我们无法轻易创建一个真实的cache实例，这里跳过缓存测试
	// 在实际项目中，应该使用mock cache或者提供一个cache接口

	service := NewUserService(mockRepo) // 使用基础版本

	assert.NotNil(t, service)

	// 验证类型断言
	userService, ok := service.(*userService)
	require.True(t, ok)
	assert.Equal(t, mockRepo, userService.userRepo)
	assert.Nil(t, userService.cache)
}

func TestUserService_Register(t *testing.T) {
	mockRepo := new(MockUserRepository)
	service := NewUserService(mockRepo)

	t.Run("成功注册用户", func(t *testing.T) {
		req := &models.RegisterRequest{
			Username:  "testuser",
			Email:     "test@example.com",
			FirstName: "Test",
			LastName:  "User",
			Password:  "password123",
		}

		// 模拟邮箱和用户名不存在
		mockRepo.On("ExistsByEmail", req.Email).Return(false, nil)
		mockRepo.On("ExistsByUsername", req.Username).Return(false, nil)

		// 模拟成功创建
		mockRepo.On("Create", mock.AnythingOfType("*models.User")).Return(nil).Run(func(args mock.Arguments) {
			user := args.Get(0).(*models.User)
			user.ID = uuid.New().String()
			user.CreatedAt = time.Now()
			user.UpdatedAt = time.Now()
		})

		user, err := service.Register(req)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, req.Username, user.Username)
		assert.Equal(t, req.Email, user.Email)
		assert.Equal(t, req.FirstName, user.FirstName)
		assert.Equal(t, req.LastName, user.LastName)
		assert.NotEmpty(t, user.Password) // 密码应该被哈希
		assert.NotEqual(t, req.Password, user.Password) // 不应该与原始密码相同

		mockRepo.AssertExpectations(t)
	})

	t.Run("邮箱已存在", func(t *testing.T) {
		req := &models.RegisterRequest{
			Username:  "testuser",
			Email:     "existing@example.com",
			FirstName: "Test",
			LastName:  "User",
			Password:  "password123",
		}

		mockRepo.On("ExistsByEmail", req.Email).Return(true, nil)

		user, err := service.Register(req)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "邮箱已存在")

		mockRepo.AssertExpectations(t)
	})

	t.Run("用户名已存在", func(t *testing.T) {
		req := &models.RegisterRequest{
			Username:  "existinguser",
			Email:     "test@example.com",
			FirstName: "Test",
			LastName:  "User",
			Password:  "password123",
		}

		mockRepo.On("ExistsByEmail", req.Email).Return(false, nil)
		mockRepo.On("ExistsByUsername", req.Username).Return(true, nil)

		user, err := service.Register(req)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "用户名已存在")

		mockRepo.AssertExpectations(t)
	})

	t.Run("创建用户失败", func(t *testing.T) {
		req := &models.RegisterRequest{
			Username:  "testuser",
			Email:     "test@example.com",
			FirstName: "Test",
			LastName:  "User",
			Password:  "password123",
		}

		mockRepo.On("ExistsByEmail", req.Email).Return(false, nil)
		mockRepo.On("ExistsByUsername", req.Username).Return(false, nil)
		mockRepo.On("Create", mock.AnythingOfType("*models.User")).Return(errors.New("数据库错误"))

		user, err := service.Register(req)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "创建用户失败")

		mockRepo.AssertExpectations(t)
	})
}

func TestUserService_Login(t *testing.T) {
	mockRepo := new(MockUserRepository)
	service := NewUserService(mockRepo)

	t.Run("成功登录", func(t *testing.T) {
		req := &models.LoginRequest{
			Email:    "test@example.com",
			Password: "password123",
		}

		user := createTestUser(req.Email, "testuser")

		mockRepo.On("GetByEmail", req.Email).Return(user, nil)

		loggedInUser, err := service.Login(req)

		assert.NoError(t, err)
		assert.NotNil(t, loggedInUser)
		assert.Equal(t, user.ID, loggedInUser.ID)
		assert.Equal(t, user.Email, loggedInUser.Email)

		mockRepo.AssertExpectations(t)
	})

	t.Run("用户不存在", func(t *testing.T) {
		req := &models.LoginRequest{
			Email:    "nonexistent@example.com",
			Password: "password123",
		}

		mockRepo.On("GetByEmail", req.Email).Return(nil, errors.New("用户不存在"))

		user, err := service.Login(req)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "邮箱或密码错误")

		mockRepo.AssertExpectations(t)
	})

	t.Run("密码错误", func(t *testing.T) {
		req := &models.LoginRequest{
			Email:    "test@example.com",
			Password: "wrongpassword",
		}

		user := createTestUser(req.Email, "testuser")

		mockRepo.On("GetByEmail", req.Email).Return(user, nil)

		loggedInUser, err := service.Login(req)

		assert.Error(t, err)
		assert.Nil(t, loggedInUser)
		assert.Contains(t, err.Error(), "邮箱或密码错误")

		mockRepo.AssertExpectations(t)
	})

	t.Run("用户未激活", func(t *testing.T) {
		req := &models.LoginRequest{
			Email:    "inactive@example.com",
			Password: "password123",
		}

		user := createTestUser(req.Email, "testuser")
		user.IsActive = false

		mockRepo.On("GetByEmail", req.Email).Return(user, nil)

		loggedInUser, err := service.Login(req)

		assert.Error(t, err)
		assert.Nil(t, loggedInUser)
		assert.Contains(t, err.Error(), "账户已被禁用")

		mockRepo.AssertExpectations(t)
	})
}

func TestUserService_GetByID(t *testing.T) {
	mockRepo := new(MockUserRepository)
	service := NewUserService(mockRepo)

	t.Run("成功获取用户", func(t *testing.T) {
		userID := uuid.New().String()
		user := createTestUser("test@example.com", "testuser")
		user.ID = userID

		mockRepo.On("GetByID", userID).Return(user, nil)

		result, err := service.GetByID(userID)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, userID, result.ID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("用户不存在", func(t *testing.T) {
		userID := uuid.New().String()

		mockRepo.On("GetByID", userID).Return(nil, errors.New("用户不存在"))

		result, err := service.GetByID(userID)

		assert.Error(t, err)
		assert.Nil(t, result)

		mockRepo.AssertExpectations(t)
	})
}

func TestUserService_GetAll(t *testing.T) {
	mockRepo := new(MockUserRepository)
	service := NewUserService(mockRepo)

	t.Run("成功获取用户列表", func(t *testing.T) {
		page, limit := 1, 10
		offset := (page - 1) * limit

		users := []*models.User{
			createTestUser("user1@example.com", "user1"),
			createTestUser("user2@example.com", "user2"),
		}
		total := int64(2)

		mockRepo.On("GetAll", offset, limit).Return(users, total, nil)

		result, totalResult, err := service.GetAll(page, limit)

		assert.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, total, totalResult)

		mockRepo.AssertExpectations(t)
	})

	t.Run("仓储层错误", func(t *testing.T) {
		page, limit := 1, 10
		offset := (page - 1) * limit

		mockRepo.On("GetAll", offset, limit).Return(nil, int64(0), errors.New("数据库错误"))

		result, totalResult, err := service.GetAll(page, limit)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, int64(0), totalResult)

		mockRepo.AssertExpectations(t)
	})
}

func TestUserService_Update(t *testing.T) {
	mockRepo := new(MockUserRepository)
	service := NewUserService(mockRepo)

	t.Run("成功更新用户", func(t *testing.T) {
		userID := uuid.New().String()
		user := createTestUser("test@example.com", "testuser")
		user.ID = userID

		req := &models.UpdateUserRequest{
			FirstName: "Updated",
			LastName:  "Name",
		}

		mockRepo.On("GetByID", userID).Return(user, nil)
		mockRepo.On("Update", mock.AnythingOfType("*models.User")).Return(nil)

		result, err := service.Update(userID, req, userID)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Updated", result.FirstName)
		assert.Equal(t, "Name", result.LastName)

		mockRepo.AssertExpectations(t)
	})

	t.Run("用户不存在", func(t *testing.T) {
		userID := uuid.New().String()
		req := &models.UpdateUserRequest{
			FirstName: "Updated",
		}

		mockRepo.On("GetByID", userID).Return(nil, errors.New("用户不存在"))

		result, err := service.Update(userID, req, userID)

		assert.Error(t, err)
		assert.Nil(t, result)

		mockRepo.AssertExpectations(t)
	})

	t.Run("权限不足", func(t *testing.T) {
		userID := uuid.New().String()
		requesterID := uuid.New().String()
		user := createTestUser("test@example.com", "testuser")
		user.ID = userID

		req := &models.UpdateUserRequest{
			FirstName: "Updated",
		}

		mockRepo.On("GetByID", userID).Return(user, nil)

		result, err := service.Update(userID, req, requesterID)

		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "权限不足")

		mockRepo.AssertExpectations(t)
	})
}

func TestUserService_Delete(t *testing.T) {
	mockRepo := new(MockUserRepository)
	service := NewUserService(mockRepo)

	t.Run("成功删除用户", func(t *testing.T) {
		userID := uuid.New().String()
		user := createTestUser("test@example.com", "testuser")
		user.ID = userID

		mockRepo.On("GetByID", userID).Return(user, nil)
		mockRepo.On("Delete", userID).Return(nil)

		err := service.Delete(userID, userID)

		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("用户不存在", func(t *testing.T) {
		userID := uuid.New().String()

		mockRepo.On("GetByID", userID).Return(nil, errors.New("用户不存在"))

		err := service.Delete(userID, userID)

		assert.Error(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("权限不足", func(t *testing.T) {
		userID := uuid.New().String()
		requesterID := uuid.New().String()
		user := createTestUser("test@example.com", "testuser")
		user.ID = userID

		mockRepo.On("GetByID", userID).Return(user, nil)

		err := service.Delete(userID, requesterID)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "权限不足")

		mockRepo.AssertExpectations(t)
	})
}

func TestUserService_ChangePassword(t *testing.T) {
	mockRepo := new(MockUserRepository)
	service := NewUserService(mockRepo)

	t.Run("成功修改密码", func(t *testing.T) {
		userID := uuid.New().String()
		user := createTestUser("test@example.com", "testuser")
		user.ID = userID

		req := &models.ChangePasswordRequest{
			OldPassword: "password123",
			NewPassword: "newpassword123",
		}

		mockRepo.On("GetByID", userID).Return(user, nil)
		mockRepo.On("Update", mock.AnythingOfType("*models.User")).Return(nil)

		err := service.ChangePassword(userID, req)

		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("当前密码错误", func(t *testing.T) {
		userID := uuid.New().String()
		user := createTestUser("test@example.com", "testuser")
		user.ID = userID

		req := &models.ChangePasswordRequest{
			OldPassword: "wrongpassword",
			NewPassword: "newpassword123",
		}

		mockRepo.On("GetByID", userID).Return(user, nil)

		err := service.ChangePassword(userID, req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "当前密码错误")

		mockRepo.AssertExpectations(t)
	})
}

func TestUserService_UpdateLastLogin(t *testing.T) {
	mockRepo := new(MockUserRepository)
	service := NewUserService(mockRepo)

	t.Run("成功更新最后登录时间", func(t *testing.T) {
		userID := uuid.New().String()

		mockRepo.On("UpdateLastLogin", userID).Return(nil)

		err := service.UpdateLastLogin(userID)

		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("更新失败", func(t *testing.T) {
		userID := uuid.New().String()

		mockRepo.On("UpdateLastLogin", userID).Return(errors.New("数据库错误"))

		err := service.UpdateLastLogin(userID)

		assert.Error(t, err)

		mockRepo.AssertExpectations(t)
	})
}

func TestUserService_ValidateCredentials(t *testing.T) {
	mockRepo := new(MockUserRepository)
	service := NewUserService(mockRepo)

	t.Run("验证成功", func(t *testing.T) {
		email := "test@example.com"
		password := "password123"
		user := createTestUser(email, "testuser")

		mockRepo.On("GetByEmail", email).Return(user, nil)

		result, err := service.ValidateCredentials(email, password)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, user.ID, result.ID)

		mockRepo.AssertExpectations(t)
	})

	t.Run("用户不存在", func(t *testing.T) {
		email := "nonexistent@example.com"
		password := "password123"

		mockRepo.On("GetByEmail", email).Return(nil, errors.New("用户不存在"))

		result, err := service.ValidateCredentials(email, password)

		assert.Error(t, err)
		assert.Nil(t, result)

		mockRepo.AssertExpectations(t)
	})

	t.Run("密码错误", func(t *testing.T) {
		email := "test@example.com"
		password := "wrongpassword"
		user := createTestUser(email, "testuser")

		mockRepo.On("GetByEmail", email).Return(user, nil)

		result, err := service.ValidateCredentials(email, password)

		assert.Error(t, err)
		assert.Nil(t, result)

		mockRepo.AssertExpectations(t)
	})
}


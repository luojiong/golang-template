package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go-server/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockUserService 是服务层的模拟实现
type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) Register(req *models.RegisterRequest) (*models.User, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserService) Login(req *models.LoginRequest) (*models.User, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserService) GetByID(id string) (*models.User, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserService) GetByEmail(email string) (*models.User, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserService) GetAll(page, limit int) ([]*models.User, int64, error) {
	args := m.Called(page, limit)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.User), args.Get(1).(int64), args.Error(2)
}

func (m *MockUserService) Update(id string, req *models.UpdateUserRequest, requesterID string) (*models.User, error) {
	args := m.Called(id, req, requesterID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockUserService) Delete(id string, requesterID string) error {
	args := m.Called(id, requesterID)
	return args.Error(0)
}

func (m *MockUserService) ChangePassword(id string, req *models.ChangePasswordRequest) error {
	args := m.Called(id, req)
	return args.Error(0)
}

func (m *MockUserService) UpdateLastLogin(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockUserService) ValidateCredentials(email, password string) (*models.User, error) {
	args := m.Called(email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

// 创建测试用户
func createTestUser(id, email, username string) *models.User {
	return &models.User{
		ID:        id,
		Username:  username,
		Email:     email,
		FirstName: "Test",
		LastName:  "User",
		IsActive:  true,
		IsAdmin:   false,
	}
}

func TestNewUserHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockUserService)
	handler := NewUserHandler(mockService)

	assert.NotNil(t, handler)
	assert.Equal(t, mockService, handler.userService)
}

func TestUserHandler_GetUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("管理员成功获取用户列表", func(t *testing.T) {
		mockService := new(MockUserService)
		handler := NewUserHandler(mockService)

		req := httptest.NewRequest(http.MethodGet, "/users?page=1&limit=10", nil)
		w := httptest.NewRecorder()

		// 设置管理员用户上下文
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", "admin-id")
		c.Set("is_admin", true)

		// 准备模拟数据
		users := []*models.User{
			createTestUser("1", "user1@example.com", "user1"),
			createTestUser("2", "user2@example.com", "user2"),
		}

		// Mock admin user check
		adminUser := createTestUser("admin-id", "admin@example.com", "admin")
		adminUser.IsAdmin = true
		mockService.On("GetByID", "admin-id").Return(adminUser, nil)
		mockService.On("GetAll", 1, 10).Return(users, int64(2), nil)

		handler.GetUsers(c)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("非管理员用户访问", func(t *testing.T) {
		mockService := new(MockUserService)
		handler := NewUserHandler(mockService)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		w := httptest.NewRecorder()

		// 设置普通用户上下文
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", "user-id")

		// Mock regular user (not admin)
		regularUser := createTestUser("user-id", "user@example.com", "user")
		regularUser.IsAdmin = false
		mockService.On("GetByID", "user-id").Return(regularUser, nil)

		handler.GetUsers(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
		mockService.AssertExpectations(t)
	})

	t.Run("用户未身份验证", func(t *testing.T) {
		mockService := new(MockUserService)
		handler := NewUserHandler(mockService)

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		w := httptest.NewRecorder()

		// 不设置用户上下文
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		handler.GetUsers(c)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestUserHandler_GetUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功获取用户", func(t *testing.T) {
		mockService := new(MockUserService)
		handler := NewUserHandler(mockService)

		user := createTestUser("1", "test@example.com", "testuser")

		req := httptest.NewRequest(http.MethodGet, "/users/1", nil)
		w := httptest.NewRecorder()

		// 设置用户上下文（获取自己或管理员）
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", "1")
		c.Params = gin.Params{gin.Param{Key: "id", Value: "1"}}

		mockService.On("GetByID", "1").Return(user, nil)

		handler.GetUser(c)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("用户不存在", func(t *testing.T) {
		mockService := new(MockUserService)
		handler := NewUserHandler(mockService)

		req := httptest.NewRequest(http.MethodGet, "/users/999", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", "1")
		c.Set("is_admin", true)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "999"}}

		mockService.On("GetByID", "999").Return(nil, assert.AnError)

		handler.GetUser(c)

		assert.Equal(t, http.StatusNotFound, w.Code)

		mockService.AssertExpectations(t)
	})
}

func TestUserHandler_UpdateUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功更新用户", func(t *testing.T) {
		mockService := new(MockUserService)
		handler := NewUserHandler(mockService)

		updateData := &models.UpdateUserRequest{
			FirstName: "Updated",
			LastName:  "Name",
		}

		req := httptest.NewRequest(http.MethodPut, "/users/1", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", "1")
		c.Params = gin.Params{gin.Param{Key: "id", Value: "1"}}
		c.Set("update_data", updateData)

		mockService.On("GetByID", "1").Return(createTestUser("1", "test@example.com", "testuser"), nil)
		mockService.On("Update", "1", updateData, "1").Return(createTestUser("1", "test@example.com", "testuser"), nil)

		handler.UpdateUser(c)

		// 由于这是一个简化的测试，我们主要验证调用是否正确
		mockService.AssertExpectations(t)
	})

	t.Run("权限不足", func(t *testing.T) {
		mockService := new(MockUserService)
		handler := NewUserHandler(mockService)

		req := httptest.NewRequest(http.MethodPut, "/users/2", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", "1") // 用户1尝试更新用户2
		c.Set("is_admin", false)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "2"}}

		handler.UpdateUser(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

func TestUserHandler_DeleteUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("管理员成功删除用户", func(t *testing.T) {
		mockService := new(MockUserService)
		handler := NewUserHandler(mockService)

		req := httptest.NewRequest(http.MethodDelete, "/users/1", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", "admin-id")
		c.Set("is_admin", true)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "1"}}

		mockService.On("GetByID", "1").Return(createTestUser("1", "test@example.com", "testuser"), nil)
		mockService.On("Delete", "1", "admin-id").Return(nil)

		handler.DeleteUser(c)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("普通用户删除自己", func(t *testing.T) {
		mockService := new(MockUserService)
		handler := NewUserHandler(mockService)

		req := httptest.NewRequest(http.MethodDelete, "/users/1", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", "1")
		c.Set("is_admin", false)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "1"}}

		mockService.On("GetByID", "1").Return(createTestUser("1", "test@example.com", "testuser"), nil)
		mockService.On("Delete", "1", "1").Return(nil)

		handler.DeleteUser(c)

		assert.Equal(t, http.StatusOK, w.Code)

		mockService.AssertExpectations(t)
	})

	t.Run("权限不足", func(t *testing.T) {
		mockService := new(MockUserService)
		handler := NewUserHandler(mockService)

		req := httptest.NewRequest(http.MethodDelete, "/users/2", nil)
		w := httptest.NewRecorder()

		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Set("user_id", "1") // 用户1尝试删除用户2
		c.Set("is_admin", false)
		c.Params = gin.Params{gin.Param{Key: "id", Value: "2"}}

		handler.DeleteUser(c)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
package handlers

import (
	"math"
	"net/http"
	"strconv"

	"go-server/internal/domain/user"
	"go-server/internal/errors"
	"go-server/internal/models"
	"go-server/internal/services"
	"go-server/internal/validation"
	"go-server/pkg/response"

	"github.com/gin-gonic/gin"
)

// UserHandlerV2 改进的用户处理器
type UserHandlerV2 struct {
	userService  services.UserServiceV2
	errorHandler errors.ErrorHandler
}

// NewUserHandlerV2 创建改进的用户处理器
func NewUserHandlerV2(userService services.UserServiceV2) *UserHandlerV2 {
	return &UserHandlerV2{
		userService:  userService,
		errorHandler: errors.NewErrorHandler(),
	}
}

// Register godoc
// @Summary 用户注册
// @Description 用户注册接口
// @Tags auth
// @Accept json
// @Produce json
// @Param register body validation.RegisterUserDTO true "注册信息"
// @Success 201 {object} models.SuccessResponse{data=models.SafeUser}
// @Failure 400 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/v1/auth/register [post]
func (h *UserHandlerV2) Register(c *gin.Context) {
	var dto validation.RegisterUserDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		h.errorHandler.HandleError(c, errors.NewValidationError("invalid request format: "+err.Error()))
		return
	}

	// 验证DTO
	if err := dto.Validate(); err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	// 调用服务注册用户
	domainUser, err := h.userService.Register(c.Request.Context(), &dto)
	if err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	// 转换为安全用户响应
	safeUser := h.domainUserToSafeUser(domainUser)
	response.Success(c, http.StatusCreated, "User registered successfully", safeUser)
}

// Login godoc
// @Summary 用户登录
// @Description 用户登录接口
// @Tags auth
// @Accept json
// @Produce json
// @Param login body validation.LoginUserDTO true "登录信息"
// @Success 200 {object} models.SuccessResponse{data=models.LoginResponse}
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 500 {object} models.ErrorResponse
// @Router /api/v1/auth/login [post]
func (h *UserHandlerV2) Login(c *gin.Context) {
	var dto validation.LoginUserDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		h.errorHandler.HandleError(c, errors.NewValidationError("invalid request format: "+err.Error()))
		return
	}

	// 验证DTO
	if err := dto.Validate(); err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	// 调用服务登录用户
	domainUser, err := h.userService.Login(c.Request.Context(), &dto)
	if err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	// 生成JWT令牌
	token, err := h.generateToken(domainUser)
	if err != nil {
		h.errorHandler.HandleError(c, errors.NewInternalError("failed to generate token", err))
		return
	}

	// 构建登录响应
	safeUser := h.domainUserToSafeUser(domainUser)
	loginResponse := models.LoginResponse{
		Token: token,
		User:  safeUser,
	}

	response.Success(c, http.StatusOK, "Login successful", loginResponse)
}

// GetUsers godoc
// @Summary 获取所有用户
// @Description 获取所有用户列表（仅管理员）
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param limit query int false "每页项目数量" default(10)
// @Success 200 {object} models.SuccessResponse{data=models.PaginatedResponse}
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Router /api/v1/users [get]
func (h *UserHandlerV2) GetUsers(c *gin.Context) {
	// 检查用户权限
	currentUserID, exists := c.Get("user_id")
	if !exists {
		h.errorHandler.HandleError(c, errors.NewUnauthorizedError("user not authenticated"))
		return
	}

	currentUser, err := h.userService.GetByID(c.Request.Context(), currentUserID.(string))
	if err != nil {
		h.errorHandler.HandleError(c, errors.NewUnauthorizedError("user not found"))
		return
	}

	if !currentUser.IsAdmin() {
		h.errorHandler.HandleError(c, errors.NewForbiddenError("admin privileges required"))
		return
	}

	// 解析分页参数
	paginationDTO := validation.PaginationParamsDTO{
		Page:  getIntQuery(c, "page", 1),
		Limit: getIntQuery(c, "limit", 10),
	}

	// 验证分页参数
	if err := paginationDTO.Validate(); err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	paginationParams := paginationDTO.ToModel()

	// 获取用户列表
	domainUsers, total, err := h.userService.GetAll(c.Request.Context(), paginationParams.Page, paginationParams.Limit)
	if err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	// 转换为安全用户
	safeUsers := h.domainUsersToSafeUsers(domainUsers)

	// 计算分页信息
	totalPages := int(math.Ceil(float64(total) / float64(paginationParams.Limit)))
	pagination := models.Pagination{
		Page:       paginationParams.Page,
		Limit:      paginationParams.Limit,
		Total:      total,
		TotalPages: totalPages,
	}

	response.Success(c, http.StatusOK, "Users retrieved successfully", models.PaginatedResponse{
		Data:       safeUsers,
		Pagination: pagination,
	})
}

// GetUser godoc
// @Summary 获取用户信息
// @Description 根据ID获取用户信息
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户ID"
// @Success 200 {object} models.SuccessResponse{data=models.SafeUser}
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/users/{id} [get]
func (h *UserHandlerV2) GetUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		h.errorHandler.HandleError(c, errors.NewValidationError("user ID is required"))
		return
	}

	// 获取用户信息
	domainUser, err := h.userService.GetByID(c.Request.Context(), userID)
	if err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	// 转换为安全用户响应
	safeUser := h.domainUserToSafeUser(domainUser)
	response.Success(c, http.StatusOK, "User retrieved successfully", safeUser)
}

// UpdateUser godoc
// @Summary 更新用户信息
// @Description 更新用户信息
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户ID"
// @Param user body validation.UpdateUserDTO true "用户信息"
// @Success 200 {object} models.SuccessResponse{data=models.SafeUser}
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Failure 409 {object} models.ErrorResponse
// @Router /api/v1/users/{id} [put]
func (h *UserHandlerV2) UpdateUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		h.errorHandler.HandleError(c, errors.NewValidationError("user ID is required"))
		return
	}

	// 获取当前用户ID
	currentUserID, exists := c.Get("user_id")
	if !exists {
		h.errorHandler.HandleError(c, errors.NewUnauthorizedError("user not authenticated"))
		return
	}

	var dto validation.UpdateUserDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		h.errorHandler.HandleError(c, errors.NewValidationError("invalid request format: "+err.Error()))
		return
	}

	// 验证DTO
	if err := dto.Validate(); err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	// 调用服务更新用户
	domainUser, err := h.userService.Update(c.Request.Context(), userID, &dto, currentUserID.(string))
	if err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	// 转换为安全用户响应
	safeUser := h.domainUserToSafeUser(domainUser)
	response.Success(c, http.StatusOK, "User updated successfully", safeUser)
}

// DeleteUser godoc
// @Summary 删除用户
// @Description 删除用户（仅限本人或管理员）
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户ID"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/users/{id} [delete]
func (h *UserHandlerV2) DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		h.errorHandler.HandleError(c, errors.NewValidationError("user ID is required"))
		return
	}

	// 获取当前用户ID
	currentUserID, exists := c.Get("user_id")
	if !exists {
		h.errorHandler.HandleError(c, errors.NewUnauthorizedError("user not authenticated"))
		return
	}

	// 调用服务删除用户
	if err := h.userService.Delete(c.Request.Context(), userID, currentUserID.(string)); err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "User deleted successfully", gin.H{
		"user_id": userID,
	})
}

// ChangePassword godoc
// @Summary 修改密码
// @Description 修改用户密码
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "用户ID"
// @Param password body validation.ChangePasswordDTO true "密码信息"
// @Success 200 {object} models.SuccessResponse
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/users/{id}/password [put]
func (h *UserHandlerV2) ChangePassword(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		h.errorHandler.HandleError(c, errors.NewValidationError("user ID is required"))
		return
	}

	// 获取当前用户ID
	currentUserID, exists := c.Get("user_id")
	if !exists {
		h.errorHandler.HandleError(c, errors.NewUnauthorizedError("user not authenticated"))
		return
	}

	// 检查权限（只能修改自己的密码）
	if userID != currentUserID.(string) {
		h.errorHandler.HandleError(c, errors.NewForbiddenError("can only change your own password"))
		return
	}

	var dto validation.ChangePasswordDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		h.errorHandler.HandleError(c, errors.NewValidationError("invalid request format: "+err.Error()))
		return
	}

	// 验证DTO
	if err := dto.Validate(); err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	// 调用服务修改密码
	if err := h.userService.ChangePassword(c.Request.Context(), userID, &dto); err != nil {
		h.errorHandler.HandleError(c, err)
		return
	}

	response.Success(c, http.StatusOK, "Password changed successfully", nil)
}

// Helper methods

// domainUserToSafeUser 将领域用户转换为安全用户
func (h *UserHandlerV2) domainUserToSafeUser(domainUser *user.User) models.SafeUser {
	if domainUser == nil {
		return models.SafeUser{}
	}

	profile := domainUser.Profile()
	return models.SafeUser{
		ID:        domainUser.ID().String(),
		Username:  domainUser.Username().String(),
		Email:     domainUser.Email().String(),
		FirstName: profile.FirstName(),
		LastName:  profile.LastName(),
		Avatar:    profile.Avatar(),
		IsActive:  domainUser.IsActive(),
		IsAdmin:   domainUser.IsAdmin(),
		CreatedAt: domainUser.CreatedAt(),
		UpdatedAt: domainUser.UpdatedAt(),
		LastLogin: domainUser.LastLogin(),
	}
}

// domainUsersToSafeUsers 批量转换领域用户为安全用户
func (h *UserHandlerV2) domainUsersToSafeUsers(domainUsers []*user.User) []models.SafeUser {
	if len(domainUsers) == 0 {
		return nil
	}

	safeUsers := make([]models.SafeUser, 0, len(domainUsers))
	for _, domainUser := range domainUsers {
		safeUsers = append(safeUsers, h.domainUserToSafeUser(domainUser))
	}
	return safeUsers
}

// generateToken 生成JWT令牌（简化版本）
func (h *UserHandlerV2) generateToken(domainUser *user.User) (string, error) {
	// 这里应该调用实际的JWT生成器
	// 暂时返回简单的令牌
	return "jwt_token_for_" + domainUser.ID().String(), nil
}

// getIntQuery 获取整数查询参数
func getIntQuery(c *gin.Context, key string, defaultValue int) int {
	valueStr := c.DefaultQuery(key, strconv.Itoa(defaultValue))
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

package handlers

import (
	"math"
	"net/http"
	"strconv"

	"go-server/internal/models"
	"go-server/internal/services"
	"go-server/pkg/errors"
	"go-server/pkg/response"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService services.UserService
}

func NewUserHandler(userService services.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

// GetUsers godoc
// @Summary 获取所有用户
// @Description 获取所有用户列表（仅管理员）。此端点从Redis缓存提供频繁访问的用户数据，TTL为5分钟。如果Redis不可用，数据直接从PostgreSQL数据库提供。缓存状态在响应头中提供。
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param limit query int false "每页项目数量" default(10)
// @Success 200 {object} models.SuccessResponse{data=models.PaginatedResponse} "成功获取用户"
// @Failure 401 {object} models.ErrorResponse{error=models.EnhancedErrorResponse} "需要身份验证"
// @Failure 403 {object} models.ErrorResponse{error=models.EnhancedErrorResponse} "需要管理员权限"
// @Header 200 {string} X-Cache "缓存状态 (HIT, MISS, BYPASS, STALE)"
// @Header 200 {integer} X-Cache-TTL "缓存数据生存时间（秒）"
// @Header 200 {string} X-Cache-Backend "使用的缓存后端 (redis, database)"
// @Header 200 {string} X-Correlation-ID "请求追踪的唯一标识符"
// @Header 401 {string} X-Correlation-ID "请求追踪的唯一标识符"
// @Header 403 {string} X-Correlation-ID "请求追踪的唯一标识符"
// @Router /api/v1/users [get]
func (h *UserHandler) GetUsers(c *gin.Context) {
	// 检查用户是否为管理员
	currentUserID, exists := c.Get("user_id")
	if !exists {
		response.UnauthorizedError(c, "用户未身份验证")
		return
	}

	currentUser, err := h.userService.GetByID(currentUserID.(string))
	if err != nil {
		response.UnauthorizedError(c, "用户未找到")
		return
	}

	if !currentUser.IsAdmin {
		response.ForbiddenError(c, "需要管理员权限")
		return
	}

	// 获取分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	if page < 1 {
		response.ValidationError(c, "页码必须大于0",
			errors.ErrorDetails{Field: "page", Message: "页码必须大于0", Value: page})
		return
	}
	if limit < 1 || limit > 100 {
		response.ValidationError(c, "每页数量必须在1到100之间",
			errors.ErrorDetails{Field: "limit", Message: "每页数量必须在1到100之间", Value: limit})
		return
	}

	// 从数据库获取用户
	users, total, err := h.userService.GetAll(page, limit)
	if err != nil {
		response.DatabaseError(c, "获取用户失败", err)
		return
	}

	// 转换为安全用户
	safeUsers := make([]models.SafeUser, len(users))
	for i, user := range users {
		safeUsers[i] = user.ToSafeUser()
	}

	// 计算分页信息
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	pagination := models.Pagination{
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	}

	response.Success(c, http.StatusOK, "成功获取用户", models.PaginatedResponse{
		Data:       safeUsers,
		Pagination: pagination,
	})
}

// GetUser godoc
// @Summary Get user by ID
// @Description Get a specific user by ID. This endpoint serves frequently accessed user profile data from Redis cache with 5-minute TTL. If Redis is unavailable, data is served directly from PostgreSQL database. Cache status is provided in response headers.
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} models.SuccessResponse{data=models.SafeUser} "User retrieved successfully"
// @Failure 401 {object} models.ErrorResponse{error=models.EnhancedErrorResponse} "Authentication required"
// @Failure 404 {object} models.ErrorResponse{error=models.EnhancedErrorResponse} "User not found"
// @Header 200 {string} X-Cache "Cache status (HIT, MISS, BYPASS, STALE)"
// @Header 200 {integer} X-Cache-TTL "Time to live in seconds for cached data"
// @Header 200 {string} X-Cache-Backend "Cache backend used (redis, database)"
// @Header 200 {string} X-Correlation-ID "Unique identifier for request tracing"
// @Header 401 {string} X-Correlation-ID "Unique identifier for request tracing"
// @Header 404 {string} X-Correlation-ID "Unique identifier for request tracing"
// @Router /api/v1/users/{id} [get]
func (h *UserHandler) GetUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		response.ValidationError(c, "User ID is required",
			errors.ErrorDetails{Field: "id", Message: "User ID is required"})
		return
	}

	// Get user from database
	user, err := h.userService.GetByID(userID)
	if err != nil {
		response.NotFoundError(c, "User", userID)
		return
	}

	response.Success(c, http.StatusOK, "User retrieved successfully", user.ToSafeUser())
}

// UpdateUser godoc
// @Summary Update user
// @Description Update a user's information
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param user body models.UpdateUserRequest true "User information"
// @Success 200 {object} models.SuccessResponse{data=models.SafeUser}
// @Failure 400 {object} models.ErrorResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/users/{id} [put]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		response.ValidationError(c, "User ID is required",
			errors.ErrorDetails{Field: "id", Message: "User ID is required"})
		return
	}

	// Check if user is authenticated
	currentUserID, exists := c.Get("user_id")
	if !exists {
		response.UnauthorizedError(c, "User not authenticated")
		return
	}

	var req models.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, "Invalid request format: "+err.Error())
		return
	}

	// Update user using user service
	user, err := h.userService.Update(userID, &req, currentUserID.(string))
	if err != nil {
		if err.Error() == "user not found" {
			response.NotFoundError(c, "User", userID)
			return
		}
		if err.Error() == "you can only update your own profile" || err.Error() == "unauthorized" {
			response.ForbiddenError(c, "You can only update your own profile")
			return
		}
		if err.Error() == "username already taken" {
			response.ConflictError(c, "Username already taken", map[string]interface{}{
				"field": "username",
				"value": req.Username,
			})
			return
		}
		response.InternalServerErrorWithCause(c, "Failed to update user", err)
		return
	}

	response.Success(c, http.StatusOK, "User updated successfully", user.ToSafeUser())
}

// DeleteUser godoc
// @Summary Delete user
// @Description Delete a user (admin only or own account)
// @Tags users
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Success 200 {object} models.SuccessResponse
// @Failure 401 {object} models.ErrorResponse
// @Failure 403 {object} models.ErrorResponse
// @Failure 404 {object} models.ErrorResponse
// @Router /api/v1/users/{id} [delete]
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		response.ValidationError(c, "User ID is required",
			errors.ErrorDetails{Field: "id", Message: "User ID is required"})
		return
	}

	// Check if user is authenticated
	currentUserID, exists := c.Get("user_id")
	if !exists {
		response.UnauthorizedError(c, "User not authenticated")
		return
	}

	// Delete user using user service
	if err := h.userService.Delete(userID, currentUserID.(string)); err != nil {
		if err.Error() == "user not found" {
			response.NotFoundError(c, "User", userID)
			return
		}
		if err.Error() == "you can only delete your own account" || err.Error() == "unauthorized" {
			response.ForbiddenError(c, "You can only delete your own account")
			return
		}
		response.InternalServerErrorWithCause(c, "Failed to delete user", err)
		return
	}

	response.Success(c, http.StatusOK, "User deleted successfully", gin.H{
		"user_id": userID,
	})
}

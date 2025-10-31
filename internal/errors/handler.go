package errors

import (
	"context"
	"fmt"
	"net/http"

	"go-server/internal/models"
	"go-server/pkg/response"

	"github.com/gin-gonic/gin"
)

// ErrorHandler 错误处理器接口
type ErrorHandler interface {
	HandleError(c *gin.Context, err error)
	LogError(ctx context.Context, err error)
	GetErrorCode(err error) models.ErrorCode
	GetUserMessage(err error) string
	GetStatusCode(err error) int
}

// errorHandler 错误处理器实现
type errorHandler struct {
	// 可以添加日志记录器或其他依赖
}

// NewErrorHandler 创建错误处理器
func NewErrorHandler() ErrorHandler {
	return &errorHandler{}
}

// HandleError 处理HTTP请求错误
func (h *errorHandler) HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	ctx := c.Request.Context()

	// 记录错误日志
	h.LogError(ctx, err)

	// 根据错误类型返回相应的HTTP响应
	switch h.GetErrorCode(err) {
	case models.ErrorCodeValidation:
		h.handleValidationError(c, err)
	case models.ErrorCodeNotFound:
		h.handleNotFoundError(c, err)
	case models.ErrorCodeUnauthorized:
		h.handleUnauthorizedError(c, err)
	case models.ErrorCodeForbidden:
		h.handleForbiddenError(c, err)
	case models.ErrorCodeConflict:
		h.handleConflictError(c, err)
	case models.ErrorCodeRateLimitExceeded:
		h.handleRateLimitError(c, err)
	case models.ErrorCodeInvalidToken, models.ErrorCodeTokenBlacklisted:
		h.handleAuthError(c, err)
	case models.ErrorCodeTimeout:
		h.handleTimeoutError(c, err)
	case models.ErrorCodeDatabase:
		h.handleDatabaseError(c, err)
	case models.ErrorCodeCache:
		h.handleCacheError(c, err)
	case models.ErrorCodeServiceUnavailable:
		h.handleServiceUnavailableError(c, err)
	default:
		h.handleInternalError(c, err)
	}
}

// LogError 记录错误日志
func (h *errorHandler) LogError(ctx context.Context, err error) {
	// 简化版本，可以后续添加实际的日志记录
	// 这里可以使用标准库的log或其他日志库
	if err != nil {
		fmt.Printf("Error occurred: %v\n", err)
	}
}

// GetErrorCode 获取错误代码
func (h *errorHandler) GetErrorCode(err error) models.ErrorCode {
	if err == nil {
		return models.ErrorCodeInternal
	}

	// 检查是否是自定义错误类型
	if customErr, ok := err.(*CustomError); ok {
		return customErr.Code
	}

	// 根据错误消息判断错误类型
	msg := err.Error()
	switch {
	case contains(msg, []string{"validation", "invalid", "required"}):
		return models.ErrorCodeValidation
	case contains(msg, []string{"not found", "does not exist"}):
		return models.ErrorCodeNotFound
	case contains(msg, []string{"unauthorized", "authentication"}):
		return models.ErrorCodeUnauthorized
	case contains(msg, []string{"forbidden", "permission"}):
		return models.ErrorCodeForbidden
	case contains(msg, []string{"conflict", "already exists", "duplicate"}):
		return models.ErrorCodeConflict
	case contains(msg, []string{"rate limit", "too many requests"}):
		return models.ErrorCodeRateLimitExceeded
	case contains(msg, []string{"invalid token", "token", "jwt"}):
		return models.ErrorCodeInvalidToken
	case contains(msg, []string{"timeout", "deadline exceeded"}):
		return models.ErrorCodeTimeout
	case contains(msg, []string{"database", "sql", "gorm"}):
		return models.ErrorCodeDatabase
	case contains(msg, []string{"cache", "redis"}):
		return models.ErrorCodeCache
	case contains(msg, []string{"service unavailable"}):
		return models.ErrorCodeServiceUnavailable
	default:
		return models.ErrorCodeInternal
	}
}

// GetUserMessage 获取用户友好的错误消息
func (h *errorHandler) GetUserMessage(err error) string {
	if err == nil {
		return "Unknown error occurred"
	}

	// 检查是否是自定义错误类型
	if customErr, ok := err.(*CustomError); ok {
		if customErr.UserMessage != "" {
			return customErr.UserMessage
		}
	}

	// 根据错误代码返回用户友好的消息
	switch h.GetErrorCode(err) {
	case models.ErrorCodeValidation:
		return "Please check your input and try again"
	case models.ErrorCodeNotFound:
		return "The requested resource was not found"
	case models.ErrorCodeUnauthorized:
		return "Authentication is required to access this resource"
	case models.ErrorCodeForbidden:
		return "You don't have permission to access this resource"
	case models.ErrorCodeConflict:
		return "The request conflicts with the current state of the resource"
	case models.ErrorCodeRateLimitExceeded:
		return "Too many requests. Please try again later"
	case models.ErrorCodeInvalidToken:
		return "Invalid or expired authentication token"
	case models.ErrorCodeTokenBlacklisted:
		return "Authentication token has been revoked"
	case models.ErrorCodeTimeout:
		return "Request timed out. Please try again"
	case models.ErrorCodeServiceUnavailable:
		return "Service is temporarily unavailable. Please try again later"
	default:
		return "An unexpected error occurred. Please try again"
	}
}

// GetStatusCode 获取HTTP状态码
func (h *errorHandler) GetStatusCode(err error) int {
	switch h.GetErrorCode(err) {
	case models.ErrorCodeValidation:
		return http.StatusBadRequest
	case models.ErrorCodeNotFound:
		return http.StatusNotFound
	case models.ErrorCodeUnauthorized:
		return http.StatusUnauthorized
	case models.ErrorCodeForbidden:
		return http.StatusForbidden
	case models.ErrorCodeConflict:
		return http.StatusConflict
	case models.ErrorCodeRateLimitExceeded:
		return http.StatusTooManyRequests
	case models.ErrorCodeInvalidToken, models.ErrorCodeTokenBlacklisted:
		return http.StatusUnauthorized
	case models.ErrorCodeTimeout:
		return http.StatusRequestTimeout
	case models.ErrorCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// 具体的错误处理方法
func (h *errorHandler) handleValidationError(c *gin.Context, err error) {
	response.ValidationError(c, h.GetUserMessage(err))
}

func (h *errorHandler) handleNotFoundError(c *gin.Context, err error) {
	response.NotFoundError(c, "Resource", "")
}

func (h *errorHandler) handleUnauthorizedError(c *gin.Context, err error) {
	response.UnauthorizedError(c, h.GetUserMessage(err))
}

func (h *errorHandler) handleForbiddenError(c *gin.Context, err error) {
	response.ForbiddenError(c, h.GetUserMessage(err))
}

func (h *errorHandler) handleConflictError(c *gin.Context, err error) {
	response.ConflictError(c, h.GetUserMessage(err), nil)
}

func (h *errorHandler) handleRateLimitError(c *gin.Context, err error) {
	c.JSON(http.StatusTooManyRequests, models.ErrorResponse{
		Success: false,
		Message: h.GetUserMessage(err),
		Error: &models.EnhancedErrorResponse{
			Code:        h.GetErrorCode(err),
			Message:     h.GetUserMessage(err),
			UserMessage: h.GetUserMessage(err),
		},
	})
}

func (h *errorHandler) handleAuthError(c *gin.Context, err error) {
	response.UnauthorizedError(c, h.GetUserMessage(err))
}

func (h *errorHandler) handleTimeoutError(c *gin.Context, err error) {
	c.JSON(http.StatusRequestTimeout, models.ErrorResponse{
		Success: false,
		Message: h.GetUserMessage(err),
		Error: &models.EnhancedErrorResponse{
			Code:        h.GetErrorCode(err),
			Message:     h.GetUserMessage(err),
			UserMessage: h.GetUserMessage(err),
		},
	})
}

func (h *errorHandler) handleDatabaseError(c *gin.Context, err error) {
	response.DatabaseError(c, h.GetUserMessage(err), err)
}

func (h *errorHandler) handleCacheError(c *gin.Context, err error) {
	response.InternalServerErrorWithCause(c, h.GetUserMessage(err), err)
}

func (h *errorHandler) handleServiceUnavailableError(c *gin.Context, err error) {
	c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
		Success: false,
		Message: h.GetUserMessage(err),
		Error: &models.EnhancedErrorResponse{
			Code:        h.GetErrorCode(err),
			Message:     h.GetUserMessage(err),
			UserMessage: h.GetUserMessage(err),
		},
	})
}

func (h *errorHandler) handleInternalError(c *gin.Context, err error) {
	response.InternalServerErrorWithCause(c, h.GetUserMessage(err), err)
}

// 辅助函数
func contains(s string, substrings []string) bool {
	for _, substring := range substrings {
		if containsSubstring(s, substring) {
			return true
		}
	}
	return false
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

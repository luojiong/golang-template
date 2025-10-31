package response

import (
	"net/http"
	"time"

	"go-server/pkg/errors"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Success       bool           `json:"success"`                  // 请求是否成功
	Message       string         `json:"message"`                  // 响应消息
	Data          interface{}    `json:"data,omitempty"`           // 响应数据
	Error         *ErrorResponse `json:"error,omitempty"`          // 错误信息（仅在失败时存在）
	CorrelationID string         `json:"correlation_id,omitempty"` // 关联ID，用于请求追踪
	Timestamp     time.Time      `json:"timestamp"`                // 响应时间戳
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Code          errors.ErrorCode       `json:"code"`                     // 错误代码
	Message       string                 `json:"message"`                  // 错误消息
	UserMessage   string                 `json:"user_message,omitempty"`   // 用户友好的错误消息
	Details       map[string]interface{} `json:"details,omitempty"`        // 详细错误信息
	InternalError string                 `json:"internal_error,omitempty"` // 内部错误详情（仅开发环境）
}

// Success 发送成功响应
func Success(c *gin.Context, statusCode int, message string, data interface{}) {
	correlationID := getCorrelationID(c)
	response := Response{
		Success:       true,
		Message:       message,
		Data:          data,
		CorrelationID: correlationID,
		Timestamp:     time.Now().UTC(),
	}
	c.JSON(statusCode, response)
}

// Error 发送通用错误响应
func Error(c *gin.Context, statusCode int, message string) {
	correlationID := getCorrelationID(c)
	appError := errors.NewAppError(errors.ErrCodeInternal, message)
	appError.StatusCode = statusCode
	appError.CorrelationID = correlationID
	ErrorWithAppError(c, appError)
}

// SuccessWithData 发送成功响应（默认200状态码）
func SuccessWithData(c *gin.Context, data interface{}) {
	Success(c, http.StatusOK, "Success", data)
}

// Created 发送创建成功响应（201状态码）
func Created(c *gin.Context, message string, data interface{}) {
	Success(c, http.StatusCreated, message, data)
}

// BadRequest 发送400错误响应
func BadRequest(c *gin.Context, message string) {
	ErrorWithAppError(c, errors.NewValidationError(message))
}

// Unauthorized 发送401错误响应
func Unauthorized(c *gin.Context, message string) {
	ErrorWithAppError(c, errors.NewUnauthorizedError(message))
}

// Forbidden 发送403错误响应
func Forbidden(c *gin.Context, message string) {
	ErrorWithAppError(c, errors.NewForbiddenError(message))
}

// NotFound 发送404错误响应
func NotFound(c *gin.Context, message string) {
	ErrorWithAppError(c, errors.NewNotFoundError(message, ""))
}

// InternalServerError 发送500错误响应
func InternalServerError(c *gin.Context, message string) {
	ErrorWithAppError(c, errors.NewInternalError(message, nil))
}

// ========== 新增的增强错误处理函数 ==========

// ErrorWithAppError 使用AppError发送错误响应
func ErrorWithAppError(c *gin.Context, appError *errors.AppError) {
	correlationID := getCorrelationID(c)
	if appError.CorrelationID == "" {
		appError.CorrelationID = correlationID
	}

	response := Response{
		Success: false,
		Message: appError.Message,
		Error: &ErrorResponse{
			Code:          appError.Code,
			Message:       appError.Message,
			UserMessage:   appError.UserMessage,
			Details:       appError.Details,
			InternalError: getInternalErrorMessage(appError),
		},
		CorrelationID: appError.CorrelationID,
		Timestamp:     time.Now().UTC(),
	}

	c.JSON(appError.StatusCode, response)
}

// ValidationError 发送验证错误响应
func ValidationError(c *gin.Context, message string, fieldDetails ...errors.ErrorDetails) {
	appError := errors.NewValidationError(message, fieldDetails...)
	ErrorWithAppError(c, appError)
}

// NotFoundError 发送资源未找到错误响应
func NotFoundError(c *gin.Context, resourceType string, identifier string) {
	appError := errors.NewNotFoundError(resourceType, identifier)
	ErrorWithAppError(c, appError)
}

// UnauthorizedError 发送未授权错误响应
func UnauthorizedError(c *gin.Context, message string) {
	appError := errors.NewUnauthorizedError(message)
	ErrorWithAppError(c, appError)
}

// ForbiddenError 发送禁止访问错误响应
func ForbiddenError(c *gin.Context, message string) {
	appError := errors.NewForbiddenError(message)
	ErrorWithAppError(c, appError)
}

// ConflictError 发送冲突错误响应
func ConflictError(c *gin.Context, message string, details map[string]interface{}) {
	appError := errors.NewConflictError(message, details)
	ErrorWithAppError(c, appError)
}

// RateLimitError 发送速率限制错误响应
func RateLimitError(c *gin.Context, limit int, windowSeconds int) {
	appError := errors.NewRateLimitError(limit, windowSeconds)
	ErrorWithAppError(c, appError)
}

// InternalServerErrorWithCause 发送带原因的内部服务器错误响应
func InternalServerErrorWithCause(c *gin.Context, message string, cause error) {
	appError := errors.NewInternalError(message, cause)
	ErrorWithAppError(c, appError)
}

// DatabaseError 发送数据库错误响应
func DatabaseError(c *gin.Context, message string, cause error) {
	appError := errors.NewDatabaseError(message, cause)
	ErrorWithAppError(c, appError)
}

// CacheError 发送缓存错误响应
func CacheError(c *gin.Context, message string, cause error) {
	appError := errors.NewCacheError(message, cause)
	ErrorWithAppError(c, appError)
}

// ServiceUnavailableError 发送服务不可用错误响应
func ServiceUnavailableError(c *gin.Context, serviceName string, message string) {
	appError := errors.NewServiceUnavailableError(serviceName, message)
	ErrorWithAppError(c, appError)
}

// TimeoutError 发送超时错误响应
func TimeoutError(c *gin.Context, operation string, timeout time.Duration) {
	appError := errors.NewTimeoutError(operation, timeout)
	ErrorWithAppError(c, appError)
}

// InvalidTokenError 发送无效令牌错误响应
func InvalidTokenError(c *gin.Context, reason string) {
	appError := errors.NewInvalidTokenError(reason)
	ErrorWithAppError(c, appError)
}

// TokenBlacklistedError 发送令牌被拉黑错误响应
func TokenBlacklistedError(c *gin.Context) {
	appError := errors.NewTokenBlacklistedError()
	ErrorWithAppError(c, appError)
}

// ========== 辅助函数 ==========

// getCorrelationID 从请求上下文中获取或生成关联ID
func getCorrelationID(c *gin.Context) string {
	// 首先尝试从请求头中获取
	if correlationID := c.GetHeader("X-Correlation-ID"); correlationID != "" {
		return correlationID
	}

	// 尝试从上下文中获取
	if correlationID, exists := c.Get("correlation_id"); exists {
		if id, ok := correlationID.(string); ok {
			return id
		}
	}

	// 生成新的关联ID
	return errors.GenerateCorrelationID()
}

// getInternalErrorMessage 获取内部错误消息（仅在开发环境返回）
func getInternalErrorMessage(appError *errors.AppError) string {
	if isDevelopmentEnvironment() {
		if appError.Cause != nil {
			return appError.Cause.Error()
		}
		return appError.Message
	}
	return ""
}

// isDevelopmentEnvironment 判断是否为开发环境
func isDevelopmentEnvironment() bool {
	// 这里可以根据实际的环境变量配置来判断
	// 暂时返回true，在实际项目中应该从配置中读取
	return true
}

// WithCorrelationID 为响应添加关联ID中间件
func WithCorrelationID() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			correlationID = errors.GenerateCorrelationID()
		}

		// 设置到上下文中，供后续处理使用
		c.Set("correlation_id", correlationID)

		// 在响应头中也添加关联ID
		c.Header("X-Correlation-ID", correlationID)

		c.Next()
	}
}

// WrapError 包装现有错误为响应
func WrapError(c *gin.Context, err error, code errors.ErrorCode, message string) {
	appError := errors.WrapError(err, code, message)
	if appError != nil {
		ErrorWithAppError(c, appError)
	} else {
		InternalServerError(c, "Unknown error occurred")
	}
}

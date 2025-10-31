package errors

import (
	"fmt"

	"go-server/internal/models"
)

// CustomError 自定义错误类型
type CustomError struct {
	Code        models.ErrorCode
	Message     string
	UserMessage string
	Details     map[string]interface{}
	Cause       error
}

// Error 实现error接口
func (e *CustomError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s", e.Message, e.Cause.Error())
	}
	return e.Message
}

// Unwrap 支持错误链
func (e *CustomError) Unwrap() error {
	return e.Cause
}

// NewCustomError 创建自定义错误
func NewCustomError(code models.ErrorCode, message string) *CustomError {
	return &CustomError{
		Code:    code,
		Message: message,
	}
}

// WithUserMessage 设置用户友好消息
func (e *CustomError) WithUserMessage(userMessage string) *CustomError {
	e.UserMessage = userMessage
	return e
}

// WithDetails 设置详细信息
func (e *CustomError) WithDetails(details map[string]interface{}) *CustomError {
	e.Details = details
	return e
}

// WithCause 设置原因错误
func (e *CustomError) WithCause(cause error) *CustomError {
	e.Cause = cause
	return e
}

// 常用错误构造函数
func NewValidationError(message string) *CustomError {
	return NewCustomError(models.ErrorCodeValidation, message)
}

func NewNotFoundError(resource string) *CustomError {
	return NewCustomError(models.ErrorCodeNotFound, fmt.Sprintf("%s not found", resource)).
		WithUserMessage("The requested resource was not found")
}

func NewUnauthorizedError(message string) *CustomError {
	return NewCustomError(models.ErrorCodeUnauthorized, message).
		WithUserMessage("Authentication is required to access this resource")
}

func NewForbiddenError(message string) *CustomError {
	return NewCustomError(models.ErrorCodeForbidden, message).
		WithUserMessage("You don't have permission to access this resource")
}

func NewConflictError(message string) *CustomError {
	return NewCustomError(models.ErrorCodeConflict, message).
		WithUserMessage("The request conflicts with the current state of the resource")
}

func NewInternalError(message string, cause error) *CustomError {
	return NewCustomError(models.ErrorCodeInternal, message).
		WithCause(cause).
		WithUserMessage("An unexpected error occurred. Please try again")
}

func NewDatabaseError(message string, cause error) *CustomError {
	return NewCustomError(models.ErrorCodeDatabase, message).
		WithCause(cause).
		WithUserMessage("Database operation failed")
}

func NewCacheError(message string, cause error) *CustomError {
	return NewCustomError(models.ErrorCodeCache, message).
		WithCause(cause).
		WithUserMessage("Cache operation failed")
}

func NewTimeoutError(message string, cause error) *CustomError {
	return NewCustomError(models.ErrorCodeTimeout, message).
		WithCause(cause).
		WithUserMessage("Request timed out")
}

func NewServiceUnavailableError(message string) *CustomError {
	return NewCustomError(models.ErrorCodeServiceUnavailable, message).
		WithUserMessage("Service is temporarily unavailable")
}

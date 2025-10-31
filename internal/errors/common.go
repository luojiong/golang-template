package errors

import (
	"context"
	"fmt"
	"net/http"
)

// ErrorCode represents a standardized error code
type ErrorCode string

const (
	// Validation errors
	ErrCodeValidation        ErrorCode = "VALIDATION_ERROR"
	ErrCodeInvalidInput      ErrorCode = "INVALID_INPUT"
	ErrCodeMissingField      ErrorCode = "MISSING_FIELD"
	ErrCodeInvalidFormat     ErrorCode = "INVALID_FORMAT"

	// Authentication and Authorization errors
	ErrCodeUnauthorized      ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden         ErrorCode = "FORBIDDEN"
	ErrCodeInvalidToken      ErrorCode = "INVALID_TOKEN"
	ErrCodeTokenExpired      ErrorCode = "TOKEN_EXPIRED"
	ErrCodeTokenBlacklisted  ErrorCode = "TOKEN_BLACKLISTED"

	// Resource errors
	ErrCodeNotFound          ErrorCode = "NOT_FOUND"
	ErrCodeConflict          ErrorCode = "CONFLICT"
	ErrCodeAlreadyExists     ErrorCode = "ALREADY_EXISTS"

	// Rate limiting errors
	ErrCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeTooManyRequests   ErrorCode = "TOO_MANY_REQUESTS"

	// System errors
	ErrCodeInternal          ErrorCode = "INTERNAL_ERROR"
	ErrCodeDatabase          ErrorCode = "DATABASE_ERROR"
	ErrCodeCache             ErrorCode = "CACHE_ERROR"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeTimeout           ErrorCode = "TIMEOUT"
	ErrCodeNetworkError      ErrorCode = "NETWORK_ERROR"

	// Business logic errors
	ErrCodeBusinessLogic     ErrorCode = "BUSINESS_LOGIC_ERROR"
	ErrCodeInvalidOperation  ErrorCode = "INVALID_OPERATION"
	ErrCodePermissionDenied  ErrorCode = "PERMISSION_DENIED"
)

// AppError represents a structured application error
type AppError struct {
	Code          ErrorCode       `json:"code"`
	Message       string          `json:"message"`
	UserMessage   string          `json:"user_message,omitempty"`
	Details       map[string]any `json:"details,omitempty"`
	StatusCode    int             `json:"-"`
	Cause         error           `json:"-"`
	CorrelationID string          `json:"correlation_id,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithDetails adds details to the error
func (e *AppError) WithDetails(details map[string]any) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// WithDetail adds a single detail to the error
func (e *AppError) WithDetail(key string, value any) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]any)
	}
	e.Details[key] = value
	return e
}

// WithCorrelationID adds a correlation ID to the error
func (e *AppError) WithCorrelationID(correlationID string) *AppError {
	e.CorrelationID = correlationID
	return e
}

// WithCause adds an underlying cause to the error
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// NewAppError creates a new application error
func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:        code,
		Message:     message,
		StatusCode: getDefaultStatusCode(code),
	}
}

// NewValidationError creates a validation error
func NewValidationError(message string, details ...ErrorDetails) *AppError {
	err := NewAppError(ErrCodeValidation, message)
	if len(details) > 0 {
		detailsMap := make(map[string]any)
		for _, detail := range details {
			detailsMap[detail.Field] = map[string]any{
				"message":    detail.Message,
				"value":      detail.Value,
				"constraint": detail.Constraint,
			}
		}
		err.WithDetails(detailsMap)
	}
	return err
}

// NewNotFoundError creates a not found error
func NewNotFoundError(resourceType, identifier string) *AppError {
	message := fmt.Sprintf("%s not found", resourceType)
	if identifier != "" {
		message = fmt.Sprintf("%s with identifier '%s' not found", resourceType, identifier)
	}

	return NewAppError(ErrCodeNotFound, message).
		WithDetails(map[string]any{
			"resource_type": resourceType,
			"identifier":    identifier,
		})
}

// NewUnauthorizedError creates an unauthorized error
func NewUnauthorizedError(message string) *AppError {
	if message == "" {
		message = "Authentication required"
	}
	return NewAppError(ErrCodeUnauthorized, message)
}

// NewForbiddenError creates a forbidden error
func NewForbiddenError(message string) *AppError {
	if message == "" {
		message = "Access denied"
	}
	return NewAppError(ErrCodeForbidden, message)
}

// NewConflictError creates a conflict error
func NewConflictError(message string, details map[string]any) *AppError {
	err := NewAppError(ErrCodeConflict, message)
	if details != nil {
		err.WithDetails(details)
	}
	return err
}

// NewInternalError creates an internal server error
func NewInternalError(message string, cause error) *AppError {
	return NewAppError(ErrCodeInternal, message).WithCause(cause)
}

// NewDatabaseError creates a database error
func NewDatabaseError(message string, cause error) *AppError {
	return NewAppError(ErrCodeDatabase, message).WithCause(cause)
}

// NewCacheError creates a cache error
func NewCacheError(message string, cause error) *AppError {
	return NewAppError(ErrCodeCache, message).WithCause(cause)
}

// NewServiceUnavailableError creates a service unavailable error
func NewServiceUnavailableError(serviceName, message string) *AppError {
	if message == "" {
		message = fmt.Sprintf("Service '%s' is currently unavailable", serviceName)
	}
	return NewAppError(ErrCodeServiceUnavailable, message).
		WithDetails(map[string]any{
			"service_name": serviceName,
		})
}

// NewTimeoutError creates a timeout error
func NewTimeoutError(operation string, timeout any) *AppError {
	message := fmt.Sprintf("Operation '%s' timed out", operation)
	return NewAppError(ErrCodeTimeout, message).
		WithDetails(map[string]any{
			"operation": operation,
			"timeout":   timeout,
		})
}

// NewInvalidTokenError creates an invalid token error
func NewInvalidTokenError(reason string) *AppError {
	message := "Invalid authentication token"
	if reason != "" {
		message = fmt.Sprintf("Invalid authentication token: %s", reason)
	}
	return NewAppError(ErrCodeInvalidToken, message).
		WithDetail("reason", reason)
}

// NewTokenBlacklistedError creates a token blacklisted error
func NewTokenBlacklistedError() *AppError {
	return NewAppError(ErrCodeTokenBlacklisted, "Authentication token has been revoked")
}

// NewRateLimitError creates a rate limit error
func NewRateLimitError(limit int, windowSeconds int) *AppError {
	message := fmt.Sprintf("Rate limit exceeded. Maximum %d requests per %d seconds", limit, windowSeconds)
	return NewAppError(ErrCodeRateLimitExceeded, message).
		WithDetails(map[string]any{
			"limit":          limit,
			"window_seconds": windowSeconds,
		})
}

// NewBusinessLogicError creates a business logic error
func NewBusinessLogicError(message string, details map[string]any) *AppError {
	err := NewAppError(ErrCodeBusinessLogic, message)
	if details != nil {
		err.WithDetails(details)
	}
	return err
}

// ErrorDetails represents field-level validation error details
type ErrorDetails struct {
	Field      string      `json:"field"`
	Message    string      `json:"message"`
	Value      interface{} `json:"value,omitempty"`
	Constraint string      `json:"constraint,omitempty"`
}

// WrapError wraps an existing error with additional context
func WrapError(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return NewAppError(code, message)
	}

	// If the error is already an AppError, just add context
	if appErr, ok := err.(*AppError); ok {
		return NewAppError(code, message).WithCause(appErr)
	}

	return NewAppError(code, message).WithCause(err)
}

// IsErrorCode checks if an error matches a specific error code
func IsErrorCode(err error, code ErrorCode) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == code
	}
	return false
}

// GetErrorCode extracts the error code from an error
func GetErrorCode(err error) ErrorCode {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return ErrCodeInternal
}

// GetHTTPStatusCode returns the appropriate HTTP status code for an error
func GetHTTPStatusCode(err error) int {
	if appErr, ok := err.(*AppError); ok {
		return appErr.StatusCode
	}
	return http.StatusInternalServerError
}

// GetUserMessage returns a user-friendly message for an error
func GetUserMessage(err error) string {
	if appErr, ok := err.(*AppError); ok {
		if appErr.UserMessage != "" {
			return appErr.UserMessage
		}
		return getDefaultUserMessage(appErr.Code)
	}
	return "An unexpected error occurred"
}

// getDefaultStatusCode returns the default HTTP status code for an error code
func getDefaultStatusCode(code ErrorCode) int {
	switch code {
	case ErrCodeValidation, ErrCodeInvalidInput, ErrCodeMissingField, ErrCodeInvalidFormat:
		return http.StatusBadRequest
	case ErrCodeUnauthorized, ErrCodeInvalidToken, ErrCodeTokenExpired, ErrCodeTokenBlacklisted:
		return http.StatusUnauthorized
	case ErrCodeForbidden, ErrCodePermissionDenied:
		return http.StatusForbidden
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeConflict, ErrCodeAlreadyExists:
		return http.StatusConflict
	case ErrCodeRateLimitExceeded, ErrCodeTooManyRequests:
		return http.StatusTooManyRequests
	case ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	case ErrCodeTimeout:
		return http.StatusRequestTimeout
	default:
		return http.StatusInternalServerError
	}
}

// getDefaultUserMessage returns a default user-friendly message for an error code
func getDefaultUserMessage(code ErrorCode) string {
	switch code {
	case ErrCodeValidation, ErrCodeInvalidInput, ErrCodeMissingField, ErrCodeInvalidFormat:
		return "Please check your input and try again"
	case ErrCodeUnauthorized, ErrCodeInvalidToken, ErrCodeTokenExpired, ErrCodeTokenBlacklisted:
		return "Please log in to continue"
	case ErrCodeForbidden, ErrCodePermissionDenied:
		return "You don't have permission to perform this action"
	case ErrCodeNotFound:
		return "The requested resource was not found"
	case ErrCodeConflict, ErrCodeAlreadyExists:
		return "The request conflicts with existing data"
	case ErrCodeRateLimitExceeded, ErrCodeTooManyRequests:
		return "Too many requests. Please try again later"
	case ErrCodeServiceUnavailable:
		return "Service temporarily unavailable. Please try again later"
	case ErrCodeTimeout:
		return "Request timed out. Please try again"
	case ErrCodeNetworkError:
		return "Network error. Please check your connection"
	case ErrCodeDatabase:
		return "Database error. Please try again later"
	case ErrCodeCache:
		return "Service temporarily unavailable. Please try again"
	default:
		return "An unexpected error occurred. Please try again"
	}
}

// ErrorHandler defines the interface for error handling
type ErrorHandler interface {
	HandleError(ctx context.Context, err error)
	HandlePanic(ctx context.Context, recovered any)
	LogError(ctx context.Context, err error)
}

// DefaultErrorHandler is a default implementation of ErrorHandler
type DefaultErrorHandler struct {
	logger Logger
}

// Logger defines the logging interface for error handling
type Logger interface {
	Error(ctx context.Context, message string, fields ...interface{})
	Warn(ctx context.Context, message string, fields ...interface{})
	Info(ctx context.Context, message string, fields ...interface{})
}

// NewDefaultErrorHandler creates a new default error handler
func NewDefaultErrorHandler(logger Logger) *DefaultErrorHandler {
	return &DefaultErrorHandler{
		logger: logger,
	}
}

// HandleError handles an error according to its type and severity
func (h *DefaultErrorHandler) HandleError(ctx context.Context, err error) {
	if err == nil {
		return
	}

	h.LogError(ctx, err)

	// You can add additional error handling logic here
	// such as sending alerts, metrics, etc.
}

// HandlePanic handles a recovered panic
func (h *DefaultErrorHandler) HandlePanic(ctx context.Context, recovered any) {
	h.logger.Error(ctx, "Panic recovered",
		"recovered", recovered,
		"stack_trace", getStackTrace())
}

// LogError logs an error with appropriate context
func (h *DefaultErrorHandler) LogError(ctx context.Context, err error) {
	if err == nil {
		return
	}

	if appErr, ok := err.(*AppError); ok {
		fields := []interface{}{
			"error_code", appErr.Code,
			"error_message", appErr.Message,
		}

		if appErr.CorrelationID != "" {
			fields = append(fields, "correlation_id", appErr.CorrelationID)
		}

		if appErr.Details != nil {
			fields = append(fields, "error_details", appErr.Details)
		}

		if appErr.Cause != nil {
			fields = append(fields, "cause", appErr.Cause.Error())
		}

		// Log based on error severity
		switch appErr.Code {
		case ErrCodeValidation, ErrCodeInvalidInput, ErrCodeMissingField, ErrCodeInvalidFormat,
			 ErrCodeUnauthorized, ErrCodeForbidden, ErrCodeNotFound, ErrCodeConflict, ErrCodeAlreadyExists:
			h.logger.Warn(ctx, "Application error", fields...)
		default:
			h.logger.Error(ctx, "Application error", fields...)
		}
	} else {
		h.logger.Error(ctx, "Unexpected error",
			"error", err.Error(),
			"error_type", fmt.Sprintf("%T", err))
	}
}

// getStackTrace returns the current stack trace
func getStackTrace() string {
	// In a real implementation, you would use runtime or debug packages
	// to get the actual stack trace
	return "Stack trace not available"
}
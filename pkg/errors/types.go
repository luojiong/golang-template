package errors

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ErrorCode 定义应用程序错误代码类型
type ErrorCode string

// 预定义的错误代码常量
const (
	// ErrCodeValidation 验证错误 - 客户端提交的数据格式或内容不正确
	ErrCodeValidation ErrorCode = "VALIDATION_ERROR"
	
	// ErrCodeNotFound 资源未找到 - 请求的资源不存在
	ErrCodeNotFound ErrorCode = "NOT_FOUND"
	
	// ErrCodeUnauthorized 未授权 - 用户未认证或认证失败
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	
	// ErrCodeForbidden 禁止访问 - 用户已认证但没有权限访问资源
	ErrCodeForbidden ErrorCode = "FORBIDDEN"
	
	// ErrCodeConflict 冲突 - 请求与当前资源状态冲突
	ErrCodeConflict ErrorCode = "CONFLICT"
	
	// ErrCodeRateLimitExceeded 速率限制超出 - 请求频率超过限制
	ErrCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"
	
	// ErrCodeInternal 内部服务器错误 - 服务器内部错误
	ErrCodeInternal ErrorCode = "INTERNAL_ERROR"
	
	// ErrCodeDatabase 数据库错误 - 数据库操作失败
	ErrCodeDatabase ErrorCode = "DATABASE_ERROR"
	
	// ErrCodeCache 缓存错误 - 缓存操作失败
	ErrCodeCache ErrorCode = "CACHE_ERROR"
	
	// ErrCodeServiceUnavailable 服务不可用 - 依赖服务不可用
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	
	// ErrCodeTimeout 请求超时 - 操作超时
	ErrCodeTimeout ErrorCode = "TIMEOUT"
	
	// ErrCodeInvalidToken 无效令牌 - JWT令牌无效或过期
	ErrCodeInvalidToken ErrorCode = "INVALID_TOKEN"
	
	// ErrCodeTokenBlacklisted 令牌已被拉黑 - JWT令牌已被加入黑名单
	ErrCodeTokenBlacklisted ErrorCode = "TOKEN_BLACKLISTED"

	// ErrCodeBusinessLogic 业务逻辑错误 - 业务规则验证失败
	ErrCodeBusinessLogic ErrorCode = "BUSINESS_LOGIC_ERROR"

	// ErrCodeQuotaExceeded 配额超出 - 资源配额已用完
	ErrCodeQuotaExceeded ErrorCode = "QUOTA_EXCEEDED"

	// ErrCodeMaintenance 维护模式 - 服务正在维护中
	ErrCodeMaintenance ErrorCode = "MAINTENANCE_MODE"

	// ErrCodeThirdPartyService 第三方服务错误 - 外部依赖服务错误
	ErrCodeThirdPartyService ErrorCode = "THIRD_PARTY_SERVICE_ERROR"

	// ErrCodeConfiguration 配置错误 - 配置项错误或缺失
	ErrCodeConfiguration ErrorCode = "CONFIGURATION_ERROR"

	// ErrCodeDependency 依赖错误 - 依赖服务或组件不可用
	ErrCodeDependency ErrorCode = "DEPENDENCY_ERROR"

	// ErrCodeSecurity 安全错误 - 安全相关错误
	ErrCodeSecurity ErrorCode = "SECURITY_ERROR"

	// ErrCodeDataIntegrity 数据完整性错误 - 数据完整性校验失败
	ErrCodeDataIntegrity ErrorCode = "DATA_INTEGRITY_ERROR"
)

// ErrorDetails 错误详细信息结构
type ErrorDetails struct {
	Field         string      `json:"field,omitempty"`         // 出错的字段名
	Message       string      `json:"message,omitempty"`       // 字段级别的错误消息
	UserMessage   string      `json:"user_message,omitempty"`  // 用户友好的错误消息（国际化）
	Value         interface{} `json:"value,omitempty"`         // 导致错误的值
	Constraint    string      `json:"constraint,omitempty"`    // 违反的约束条件
	ErrorCode     string      `json:"error_code,omitempty"`    // 字段级别的错误代码
	Suggestions   []string    `json:"suggestions,omitempty"`   // 修复建议
}

// ErrorContext 错误上下文信息
type ErrorContext struct {
	RequestID     string                 `json:"request_id,omitempty"`     // 请求ID
	UserID        string                 `json:"user_id,omitempty"`        // 用户ID
	Operation     string                 `json:"operation,omitempty"`      // 操作名称
	Resource      string                 `json:"resource,omitempty"`       // 资源标识
	IPAddress     string                 `json:"ip_address,omitempty"`     // 客户端IP
	UserAgent     string                 `json:"user_agent,omitempty"`     // 用户代理
	Metadata      map[string]interface{} `json:"metadata,omitempty"`       // 额外的上下文元数据
}

// AppError 应用程序错误结构
type AppError struct {
	Code           ErrorCode              `json:"code"`             // 错误代码
	Message        string                 `json:"message"`          // 错误消息
	UserMessage    string                 `json:"user_message,omitempty"`   // 用户友好的错误消息（国际化）
	Details        map[string]interface{} `json:"details,omitempty"` // 详细错误信息
	StatusCode     int                    `json:"-"`                // HTTP状态码，不序列化到JSON
	Cause          error                  `json:"-"`                // 原始错误，不序列化到JSON
	CorrelationID  string                 `json:"correlation_id,omitempty"` // 关联ID，用于请求追踪
	RequestID      string                 `json:"request_id,omitempty"`    // 请求ID
	Timestamp      time.Time              `json:"timestamp"`        // 错误发生时间
	InternalError  string                 `json:"internal_error,omitempty"` // 内部错误详情（仅开发环境）
	Context        *ErrorContext          `json:"context,omitempty"`      // 错误上下文信息
	StackTrace     string                 `json:"stack_trace,omitempty"`   // 堆栈跟踪（仅开发环境）
	Severity       string                 `json:"severity,omitempty"`      // 错误严重程度 (low, medium, high, critical)
	Category       string                 `json:"category,omitempty"`      // 错误分类
	Resolved       bool                   `json:"resolved,omitempty"`      // 是否已解决
	Retryable      bool                   `json:"retryable,omitempty"`     // 是否可重试
}

// Error 实现error接口
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 支持errors.Unwrap
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithCorrelationID 设置关联ID
func (e *AppError) WithCorrelationID(correlationID string) *AppError {
	e.CorrelationID = correlationID
	return e
}

// WithDetails 添加详细信息
func (e *AppError) WithDetails(details map[string]interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// WithDetail 添加单个详细信息
func (e *AppError) WithDetail(key string, value interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithCause 设置原始错误
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// WithUserMessage 设置用户友好的错误消息
func (e *AppError) WithUserMessage(message string) *AppError {
	e.UserMessage = message
	return e
}

// WithRequestID 设置请求ID
func (e *AppError) WithRequestID(requestID string) *AppError {
	e.RequestID = requestID
	return e
}

// WithContext 设置错误上下文
func (e *AppError) WithContext(context *ErrorContext) *AppError {
	e.Context = context
	return e
}

// WithSeverity 设置错误严重程度
func (e *AppError) WithSeverity(severity string) *AppError {
	e.Severity = severity
	return e
}

// WithCategory 设置错误分类
func (e *AppError) WithCategory(category string) *AppError {
	e.Category = category
	return e
}

// WithRetryable 设置是否可重试
func (e *AppError) WithRetryable(retryable bool) *AppError {
	e.Retryable = retryable
	return e
}

// WithStackTrace 设置堆栈跟踪（仅开发环境）
func (e *AppError) WithStackTrace(stackTrace string) *AppError {
	e.StackTrace = stackTrace
	return e
}

// WithResolved 设置是否已解决
func (e *AppError) WithResolved(resolved bool) *AppError {
	e.Resolved = resolved
	return e
}

// IsClientError 判断是否为客户端错误（4xx）
func (e *AppError) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// IsServerError 判断是否为服务器错误（5xx）
func (e *AppError) IsServerError() bool {
	return e.StatusCode >= 500
}

// HTTP状态码映射表
var statusCodeMapping = map[ErrorCode]int{
	ErrCodeValidation:         http.StatusBadRequest,
	ErrCodeNotFound:           http.StatusNotFound,
	ErrCodeUnauthorized:       http.StatusUnauthorized,
	ErrCodeForbidden:          http.StatusForbidden,
	ErrCodeConflict:           http.StatusConflict,
	ErrCodeRateLimitExceeded:  http.StatusTooManyRequests,
	ErrCodeQuotaExceeded:      http.StatusTooManyRequests,
	ErrCodeInternal:           http.StatusInternalServerError,
	ErrCodeDatabase:           http.StatusInternalServerError,
	ErrCodeCache:              http.StatusInternalServerError,
	ErrCodeServiceUnavailable: http.StatusServiceUnavailable,
	ErrCodeTimeout:            http.StatusRequestTimeout,
	ErrCodeInvalidToken:       http.StatusUnauthorized,
	ErrCodeTokenBlacklisted:   http.StatusUnauthorized,
	ErrCodeBusinessLogic:      http.StatusBadRequest,
	ErrCodeMaintenance:        http.StatusServiceUnavailable,
	ErrCodeThirdPartyService:  http.StatusBadGateway,
	ErrCodeConfiguration:      http.StatusInternalServerError,
	ErrCodeDependency:         http.StatusServiceUnavailable,
	ErrCodeSecurity:           http.StatusForbidden,
	ErrCodeDataIntegrity:      http.StatusConflict,
}

// getStatusCode 根据错误代码获取对应的HTTP状态码
func getStatusCode(code ErrorCode) int {
	if status, exists := statusCodeMapping[code]; exists {
		return status
	}
	return http.StatusInternalServerError
}

// NewAppError 创建新的应用程序错误
func NewAppError(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: getStatusCode(code),
		Timestamp:  time.Now().UTC(),
	}
}

// NewValidationError 创建验证错误
func NewValidationError(message string, fieldDetails ...ErrorDetails) *AppError {
	err := NewAppError(ErrCodeValidation, message)
	
	if len(fieldDetails) > 0 {
		details := make(map[string]interface{})
		details["validation_errors"] = fieldDetails
		err.Details = details
	}
	
	return err
}

// NewNotFoundError 创建资源未找到错误
func NewNotFoundError(resourceType string, identifier string) *AppError {
	message := fmt.Sprintf("%s not found", resourceType)
	if identifier != "" {
		message = fmt.Sprintf("%s with identifier '%s' not found", resourceType, identifier)
	}
	
	return NewAppError(ErrCodeNotFound, message).
		WithDetail("resource_type", resourceType).
		WithDetail("identifier", identifier)
}

// NewUnauthorizedError 创建未授权错误
func NewUnauthorizedError(message string) *AppError {
	if message == "" {
		message = "Authentication required"
	}
	return NewAppError(ErrCodeUnauthorized, message)
}

// NewForbiddenError 创建禁止访问错误
func NewForbiddenError(message string) *AppError {
	if message == "" {
		message = "Access forbidden"
	}
	return NewAppError(ErrCodeForbidden, message)
}

// NewConflictError 创建冲突错误
func NewConflictError(message string, details map[string]interface{}) *AppError {
	err := NewAppError(ErrCodeConflict, message)
	if details != nil {
		err.Details = details
	}
	return err
}

// NewRateLimitError 创建速率限制错误
func NewRateLimitError(limit int, windowSeconds int) *AppError {
	return NewAppError(ErrCodeRateLimitExceeded, "Rate limit exceeded").
		WithDetail("limit", limit).
		WithDetail("window_seconds", windowSeconds).
		WithDetail("retry_after", windowSeconds)
}

// NewInternalError 创建内部服务器错误
func NewInternalError(message string, cause error) *AppError {
	err := NewAppError(ErrCodeInternal, message)
	if cause != nil {
		err.Cause = cause
	}
	return err
}

// NewDatabaseError 创建数据库错误
func NewDatabaseError(message string, cause error) *AppError {
	err := NewAppError(ErrCodeDatabase, message)
	if cause != nil {
		err.Cause = cause
	}
	return err
}

// NewCacheError 创建缓存错误
func NewCacheError(message string, cause error) *AppError {
	err := NewAppError(ErrCodeCache, message)
	if cause != nil {
		err.Cause = cause
	}
	return err
}

// NewServiceUnavailableError 创建服务不可用错误
func NewServiceUnavailableError(serviceName string, message string) *AppError {
	if message == "" {
		message = fmt.Sprintf("Service '%s' is currently unavailable", serviceName)
	}
	
	return NewAppError(ErrCodeServiceUnavailable, message).
		WithDetail("service_name", serviceName)
}

// NewTimeoutError 创建超时错误
func NewTimeoutError(operation string, timeout time.Duration) *AppError {
	message := fmt.Sprintf("Operation '%s' timed out after %v", operation, timeout)
	return NewAppError(ErrCodeTimeout, message).
		WithDetail("operation", operation).
		WithDetail("timeout_seconds", timeout.Seconds())
}

// NewInvalidTokenError 创建无效令牌错误
func NewInvalidTokenError(reason string) *AppError {
	message := "Invalid token"
	if reason != "" {
		message = fmt.Sprintf("Invalid token: %s", reason)
	}
	
	return NewAppError(ErrCodeInvalidToken, message).
		WithDetail("reason", reason)
}

// NewTokenBlacklistedError 创建令牌被拉黑错误
func NewTokenBlacklistedError() *AppError {
	return NewAppError(ErrCodeTokenBlacklisted, "Token has been blacklisted")
}

// NewBusinessLogicError 创建业务逻辑错误
func NewBusinessLogicError(message string, details map[string]interface{}) *AppError {
	err := NewAppError(ErrCodeBusinessLogic, message)
	if details != nil {
		err.Details = details
	}
	return err.WithRetryable(false)
}

// NewQuotaExceededError 创建配额超出错误
func NewQuotaExceededError(resourceType string, currentLimit int, resetTime time.Time) *AppError {
	message := fmt.Sprintf("Quota exceeded for %s", resourceType)
	return NewAppError(ErrCodeQuotaExceeded, message).
		WithDetail("resource_type", resourceType).
		WithDetail("current_limit", currentLimit).
		WithDetail("reset_time", resetTime).
		WithRetryable(true)
}

// NewMaintenanceError 创建维护模式错误
func NewMaintenanceError(serviceName string, estimatedDowntime time.Duration) *AppError {
	message := fmt.Sprintf("Service '%s' is currently under maintenance", serviceName)
	return NewAppError(ErrCodeMaintenance, message).
		WithDetail("service_name", serviceName).
		WithDetail("estimated_downtime_minutes", int64(estimatedDowntime.Minutes())).
		WithRetryable(true)
}

// NewThirdPartyServiceError 创建第三方服务错误
func NewThirdPartyServiceError(serviceName string, operation string, cause error) *AppError {
	message := fmt.Sprintf("Third party service '%s' failed during %s operation", serviceName, operation)
	err := NewAppError(ErrCodeThirdPartyService, message)
	if cause != nil {
		err.Cause = cause
	}
	return err.WithDetail("service_name", serviceName).
		WithDetail("operation", operation).
		WithRetryable(true)
}

// NewConfigurationError 创建配置错误
func NewConfigurationError(configKey string, expectedType string) *AppError {
	message := fmt.Sprintf("Configuration error for key '%s'", configKey)
	return NewAppError(ErrCodeConfiguration, message).
		WithDetail("config_key", configKey).
		WithDetail("expected_type", expectedType).
		WithRetryable(false)
}

// NewDependencyError 创建依赖错误
func NewDependencyError(dependencyName string, healthCheck string) *AppError {
	message := fmt.Sprintf("Dependency '%s' is unavailable", dependencyName)
	return NewAppError(ErrCodeDependency, message).
		WithDetail("dependency_name", dependencyName).
		WithDetail("health_check", healthCheck).
		WithRetryable(true)
}

// NewSecurityError 创建安全错误
func NewSecurityError(message string, securityContext map[string]interface{}) *AppError {
	err := NewAppError(ErrCodeSecurity, message).
		WithSeverity("high").
		WithRetryable(false)

	if securityContext != nil {
		err.Details = securityContext
	}

	return err
}

// NewDataIntegrityError 创建数据完整性错误
func NewDataIntegrityError(entityType string, entityID string, constraint string) *AppError {
	message := fmt.Sprintf("Data integrity violation for %s", entityType)
	return NewAppError(ErrCodeDataIntegrity, message).
		WithDetail("entity_type", entityType).
		WithDetail("entity_id", entityID).
		WithDetail("constraint", constraint).
		WithRetryable(false)
}

// WrapError 包装现有错误为应用程序错误
func WrapError(err error, code ErrorCode, message string) *AppError {
	if err == nil {
		return nil
	}
	
	// 如果已经是AppError，直接返回
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	
	return NewAppError(code, message).WithCause(err)
}

// GenerateCorrelationID 生成新的关联ID
func GenerateCorrelationID() string {
	return uuid.New().String()[:8]
}

// ErrorWithCorrelation 创建带关联ID的错误
func ErrorWithCorrelation(code ErrorCode, message string) *AppError {
	return NewAppError(code, message).WithCorrelationID(GenerateCorrelationID())
}

// IsErrorCode 检查错误是否为指定的错误代码
func IsErrorCode(err error, code ErrorCode) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == code
	}
	return false
}

// GetErrorCode 获取错误的错误代码
func GetErrorCode(err error) ErrorCode {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return ErrCodeInternal
}

// GetHTTPStatusCode 获取错误对应的HTTP状态码
func GetHTTPStatusCode(err error) int {
	if appErr, ok := err.(*AppError); ok {
		return appErr.StatusCode
	}
	return http.StatusInternalServerError
}

// CreateErrorFromContext 从上下文创建带上下文信息的错误
func CreateErrorFromContext(ctx context.Context, code ErrorCode, message string) *AppError {
	err := NewAppError(code, message)

	// 从上下文中提取信息
	if requestID := ctx.Value("request_id"); requestID != nil {
		if id, ok := requestID.(string); ok {
			err.RequestID = id
		}
	}

	if userID := ctx.Value("user_id"); userID != nil {
		if id, ok := userID.(string); ok {
			err.WithContext(&ErrorContext{
				RequestID: err.RequestID,
				UserID:    id,
			})
		}
	}

	if correlationID := ctx.Value("correlation_id"); correlationID != nil {
		if id, ok := correlationID.(string); ok {
			err.CorrelationID = id
		}
	}

	return err
}

// AddInternationalizedMessages 添加国际化错误消息
func (e *AppError) AddInternationalizedMessages(messages map[string]string) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details["i18n_messages"] = messages
	return e
}

// GetLocalizedMessage 获取本地化错误消息
func (e *AppError) GetLocalizedMessage(languageCode string) string {
	if e.Details != nil {
		if i18nMessages, ok := e.Details["i18n_messages"]; ok {
			if messages, ok := i18nMessages.(map[string]string); ok {
				if msg, exists := messages[languageCode]; exists {
					return msg
				}
				// 如果没有找到指定语言，尝试返回英文
				if msg, exists := messages["en"]; exists {
					return msg
				}
			}
		}
	}

	// 默认返回用户友好消息或原始消息
	if e.UserMessage != "" {
		return e.UserMessage
	}
	return e.Message
}

// CreateBusinessValidationError 创建业务验证错误（支持字段级别的详细错误）
func CreateBusinessValidationError(message string, fieldErrors []ErrorDetails, suggestions []string) *AppError {
	err := NewAppError(ErrCodeBusinessLogic, message)

	details := make(map[string]interface{})
	if len(fieldErrors) > 0 {
		details["field_errors"] = fieldErrors
	}
	if len(suggestions) > 0 {
		details["suggestions"] = suggestions
	}

	return err.WithDetails(details).
		WithUserMessage("Please check your input and try again").
		WithCategory("validation")
}

// ToMap 将错误转换为map格式，便于日志记录
func (e *AppError) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"code":            e.Code,
		"message":         e.Message,
		"status_code":     e.StatusCode,
		"timestamp":       e.Timestamp,
		"correlation_id":  e.CorrelationID,
		"request_id":      e.RequestID,
		"is_client_error": e.IsClientError(),
		"is_server_error": e.IsServerError(),
	}

	if e.UserMessage != "" {
		result["user_message"] = e.UserMessage
	}

	if e.Details != nil && len(e.Details) > 0 {
		result["details"] = e.Details
	}

	if e.Context != nil {
		result["context"] = e.Context
	}

	if e.Severity != "" {
		result["severity"] = e.Severity
	}

	if e.Category != "" {
		result["category"] = e.Category
	}

	if e.Cause != nil {
		result["cause"] = e.Cause.Error()
	}

	return result
}

// LogFormat 返回适合日志记录的错误格式
func (e *AppError) LogFormat() string {
	return fmt.Sprintf("[%s] %s - Code: %s, Status: %d, CorrelationID: %s, RequestID: %s",
		e.Severity, e.Message, e.Code, e.StatusCode, e.CorrelationID, e.RequestID)
}
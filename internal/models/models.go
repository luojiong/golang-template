package models

import (
	"time"
)

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"john@example.com"`    // 邮箱地址
	Password string `json:"password" binding:"required,min=6" example:"password123"`      // 密码
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username  string `json:"username" binding:"required,min=3,max=50" example:"johndoe"`  // 用户名
	Email     string `json:"email" binding:"required,email" example:"john@example.com"`   // 邮箱地址
	Password  string `json:"password" binding:"required,min=6" example:"password123"`     // 密码
	FirstName string `json:"first_name" binding:"max=50" example:"John"`                  // 名
	LastName  string `json:"last_name" binding:"max=50" example:"Doe"`                    // 姓
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Username  string `json:"username" binding:"omitempty,min=3,max=50"`  // 用户名
	FirstName string `json:"first_name" binding:"omitempty,max=50"`       // 名
	LastName  string `json:"last_name" binding:"omitempty,max=50"`        // 姓
	Avatar    string `json:"avatar" binding:"omitempty,url"`             // 头像URL
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token string   `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."` // JWT令牌
	User  SafeUser `json:"user"`                                                           // 安全用户信息
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required" example:"oldpassword123"` // 旧密码
	NewPassword string `json:"new_password" binding:"required,min=6" example:"newpassword123"` // 新密码
}

// HealthResponse 健康检查响应
type HealthResponse struct {
	Status    string            `json:"status" example:"healthy"`                          // 状态
	Timestamp time.Time         `json:"timestamp" example:"2024-01-01T00:00:00Z"`        // 时间戳
	Version   string            `json:"version" example:"1.0.0"`                          // 版本号
	Services  map[string]string `json:"services"`                                          // 服务状态
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Success       bool                   `json:"success" example:"false"`                     // 是否成功
	Message       string                 `json:"message" example:"Request failed"`           // 错误消息
	Error         *EnhancedErrorResponse `json:"error,omitempty"`                            // 增强错误信息
	CorrelationID string                 `json:"correlation_id,omitempty" example:"a1b2c3d4"` // 关联ID
	Timestamp     time.Time              `json:"timestamp" example:"2024-01-01T12:00:00Z"`   // 时间戳
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Success       bool        `json:"success" example:"true"`                         // 是否成功
	Message       string      `json:"message" example:"Operation completed successfully"` // 成功消息
	Data          interface{} `json:"data,omitempty"`                                   // 响应数据
	CorrelationID string      `json:"correlation_id,omitempty" example:"a1b2c3d4"`      // 关联ID
	Timestamp     time.Time   `json:"timestamp" example:"2024-01-01T12:00:00Z"`        // 时间戳
}

// EnhancedErrorResponse 增强错误信息，包含关联ID和详细验证信息
type EnhancedErrorResponse struct {
	Code          ErrorCode               `json:"code" example:"VALIDATION_ERROR"`        // 错误代码
	Message       string                  `json:"message" example:"Invalid input data"`    // 错误消息
	UserMessage   string                  `json:"user_message,omitempty" example:"Please check your input and try again"` // 用户友好消息
	Details       map[string]interface{}  `json:"details,omitempty"`                      // 详细信息
	InternalError string                  `json:"internal_error,omitempty" example:"Database connection failed: connection timeout"` // 内部错误
}

//ErrorCode 机器可读的错误代码
type ErrorCode string

// 错误代码常量
const (
	ErrorCodeValidation        ErrorCode = "VALIDATION_ERROR"        // 验证错误
	ErrorCodeNotFound          ErrorCode = "NOT_FOUND"              // 未找到
	ErrorCodeUnauthorized      ErrorCode = "UNAUTHORIZED"          // 未授权
	ErrorCodeForbidden         ErrorCode = "FORBIDDEN"             // 禁止访问
	ErrorCodeConflict          ErrorCode = "CONFLICT"              // 冲突
	ErrorCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"   // 速率限制超出
	ErrorCodeInternal          ErrorCode = "INTERNAL_ERROR"        // 内部错误
	ErrorCodeDatabase          ErrorCode = "DATABASE_ERROR"        // 数据库错误
	ErrorCodeCache             ErrorCode = "CACHE_ERROR"           // 缓存错误
	ErrorCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"  // 服务不可用
	ErrorCodeTimeout           ErrorCode = "TIMEOUT"               // 超时
	ErrorCodeInvalidToken      ErrorCode = "INVALID_TOKEN"         // 无效令牌
	ErrorCodeTokenBlacklisted  ErrorCode = "TOKEN_BLACKLISTED"     // 令牌已列入黑名单
)

// FieldValidationError 字段级验证错误详细信息
type FieldValidationError struct {
	Field      string      `json:"field" example:"email"`                                    // 字段名
	Message    string      `json:"message" example:"Email must be a valid email address"`   // 错误消息
	Value      interface{} `json:"value,omitempty" example:"invalid-email"`
	Constraint string      `json:"constraint,omitempty" example:"email_format"`
}

// PaginationParams 分页参数
type PaginationParams struct {
	Page  int `json:"page" form:"page" binding:"min=1" example:"1"`         // 页码
	Limit int `json:"limit" form:"limit" binding:"min=1,max=100" example:"10"` // 每页数量
}

// PaginatedResponse 分页响应
type PaginatedResponse struct {
	Data       interface{} `json:"data"`        // 数据
	Pagination Pagination  `json:"pagination"` // 分页信息
}

// Pagination 分页信息
type Pagination struct {
	Page       int   `json:"page" example:"1"`       // 当前页码
	Limit      int   `json:"limit" example:"10"`     // 每页数量
	Total      int64 `json:"total" example:"100"`    // 总记录数
	TotalPages int   `json:"total_pages" example:"10"` // 总页数
}
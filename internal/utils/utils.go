package utils

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
)

// GenerateRandomString 生成指定长度的随机字符串
func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

// IsValidEmail 验证邮箱格式
func IsValidEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	regex := regexp.MustCompile(pattern)
	return regex.MatchString(email)
}

// SanitizeString 移除多余空白并标准化字符串
func SanitizeString(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

// Contains 检查字符串是否存在于切片中
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ErrorResponse 结构化错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`    // 错误代码
	Message string `json:"message"` // 错误消息
	Details string `json:"details,omitempty"` // 详细信息
}

// NewErrorResponse 创建新的错误响应
func NewErrorResponse(code int, message, details string) ErrorResponse {
	return ErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// PaginationParams 分页参数
type PaginationParams struct {
	Page  int `json:"page" form:"page"`  // 页码
	Limit int `json:"limit" form:"limit"` // 每页数量
}

// GetPaginationParams 从查询中提取分页参数
func GetPaginationParams(defaultPage, defaultLimit int) PaginationParams {
	return PaginationParams{
		Page:  defaultPage,
		Limit: defaultLimit,
	}
}

// CalculateOffset 计算分页偏移量
func CalculateOffset(page, limit int) int {
	if page < 1 {
		page = 1
	}
	return (page - 1) * limit
}
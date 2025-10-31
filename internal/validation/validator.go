package validation

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
)

// Validator 验证器接口
type Validator interface {
	ValidateStruct(obj interface{}) error
	ValidateField(fieldName string, value interface{}, rules ...ValidationRule) error
}

// ValidationRule 验证规则接口
type ValidationRule interface {
	Validate(value interface{}) error
	GetMessage() string
}

// RuleFunc 验证规则函数类型
type RuleFunc func(value interface{}) error

// Rule 验证规则实现
type Rule struct {
	ruleFunc RuleFunc
	message  string
}

// NewRule 创建验证规则
func NewRule(ruleFunc RuleFunc, message string) ValidationRule {
	return &Rule{
		ruleFunc: ruleFunc,
		message:  message,
	}
}

// Validate 执行验证
func (r *Rule) Validate(value interface{}) error {
	if r.ruleFunc != nil {
		return r.ruleFunc(value)
	}
	return nil
}

// GetMessage 获取错误消息
func (r *Rule) GetMessage() string {
	return r.message
}

// validator 验证器实现
type validator struct {
	// 可以包含配置项
}

// NewValidator 创建验证器
func NewValidator() Validator {
	return &validator{}
}

// ValidateStruct 验证结构体
func (v *validator) ValidateStruct(obj interface{}) error {
	// 这里可以使用反射来验证结构体字段
	// 为了简化，暂时返回nil
	return nil
}

// ValidateField 验证字段
func (v *validator) ValidateField(fieldName string, value interface{}, rules ...ValidationRule) error {
	for _, rule := range rules {
		if err := rule.Validate(value); err != nil {
			return FieldValidationError{
				Field:   fieldName,
				Value:   value,
				Message: rule.GetMessage(),
				Err:     err,
			}
		}
	}
	return nil
}

// FieldValidationError 字段验证错误
type FieldValidationError struct {
	Field   string      `json:"field"`
	Value   interface{} `json:"value,omitempty"`
	Message string      `json:"message"`
	Err     error       `json:"-"`
}

// Error 实现error接口
func (e FieldValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// Unwrap 返回原始错误
func (e FieldValidationError) Unwrap() error {
	return e.Err
}

// 预定义验证规则

// Required 必填验证
func Required() ValidationRule {
	return NewRule(func(value interface{}) error {
		if value == nil {
			return errors.New("value is required")
		}

		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return errors.New("value is required")
			}
		case []interface{}:
			if len(v) == 0 {
				return errors.New("value is required")
			}
		case map[string]interface{}:
			if len(v) == 0 {
				return errors.New("value is required")
			}
		}

		return nil
	}, "This field is required")
}

// MinLength 最小长度验证
func MinLength(min int) ValidationRule {
	return NewRule(func(value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return errors.New("value must be a string")
		}

		if len(str) < min {
			return fmt.Errorf("value must be at least %d characters", min)
		}

		return nil
	}, fmt.Sprintf("This field must be at least %d characters", min))
}

// MaxLength 最大长度验证
func MaxLength(max int) ValidationRule {
	return NewRule(func(value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return errors.New("value must be a string")
		}

		if len(str) > max {
			return fmt.Errorf("value must be at most %d characters", max)
		}

		return nil
	}, fmt.Sprintf("This field must be at most %d characters", max))
}

// Email 邮箱验证
func Email() ValidationRule {
	return NewRule(func(value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return errors.New("value must be a string")
		}

		if _, err := mail.ParseAddress(str); err != nil {
			return errors.New("invalid email format")
		}

		return nil
	}, "Please enter a valid email address")
}

// Username 用户名验证
func Username() ValidationRule {
	return NewRule(func(value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return errors.New("value must be a string")
		}

		// 用户名规则：3-50字符，只能包含字母、数字、下划线和连字符
		usernameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]{3,50}$`)
		if !usernameRegex.MatchString(str) {
			return errors.New("username must be 3-50 characters and contain only letters, numbers, underscores and hyphens")
		}

		return nil
	}, "Username must be 3-50 characters and contain only letters, numbers, underscores and hyphens")
}

// Password 密码验证
func Password() ValidationRule {
	return NewRule(func(value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return errors.New("value must be a string")
		}

		if len(str) < 6 {
			return errors.New("password must be at least 6 characters")
		}

		// 可以添加更复杂的密码规则
		// 比如包含大小写字母、数字和特殊字符等

		return nil
	}, "Password must be at least 6 characters")
}

// URL URL验证
func URL() ValidationRule {
	return NewRule(func(value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return errors.New("value must be a string")
		}

		// 简单的URL验证
		urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
		if !urlRegex.MatchString(str) {
			return errors.New("invalid URL format")
		}

		return nil
	}, "Please enter a valid URL")
}

// Min 最小值验证（数字）
func Min(min int) ValidationRule {
	return NewRule(func(value interface{}) error {
		var num int
		var err error

		switch v := value.(type) {
		case int:
			num = v
		case string:
			num, err = strconv.Atoi(v)
			if err != nil {
				return errors.New("value must be a number")
			}
		default:
			return errors.New("value must be a number")
		}

		if num < min {
			return fmt.Errorf("value must be at least %d", min)
		}

		return nil
	}, fmt.Sprintf("This field must be at least %d", min))
}

// Max 最大值验证（数字）
func Max(max int) ValidationRule {
	return NewRule(func(value interface{}) error {
		var num int
		var err error

		switch v := value.(type) {
		case int:
			num = v
		case string:
			num, err = strconv.Atoi(v)
			if err != nil {
				return errors.New("value must be a number")
			}
		default:
			return errors.New("value must be a number")
		}

		if num > max {
			return fmt.Errorf("value must be at most %d", max)
		}

		return nil
	}, fmt.Sprintf("This field must be at most %d", max))
}

// In 在指定值范围内验证
func In(values ...interface{}) ValidationRule {
	return NewRule(func(value interface{}) error {
		for _, v := range values {
			if value == v {
				return nil
			}
		}
		return fmt.Errorf("value must be one of: %v", values)
	}, fmt.Sprintf("This field must be one of: %v", values))
}

// Custom 自定义验证规则
func Custom(ruleFunc RuleFunc, message string) ValidationRule {
	return NewRule(ruleFunc, message)
}

package middleware

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"go-server/pkg/errors"
	"go-server/pkg/response"

	"github.com/gin-gonic/gin"
)

// ValidationRule 定义验证规则接口
type ValidationRule interface {
	// Validate 执行验证并返回错误信息
	Validate(value interface{}, field string) *errors.ErrorDetails
}

// RequiredRule 必填字段验证规则
type RequiredRule struct{}

func (r *RequiredRule) Validate(value interface{}, field string) *errors.ErrorDetails {
	if value == nil || value == "" {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s is required", field),
			Value:      value,
			Constraint: "required",
		}
	}
	return nil
}

// MinLengthRule 最小长度验证规则
type MinLengthRule struct {
	MinLength int
}

func (r *MinLengthRule) Validate(value interface{}, field string) *errors.ErrorDetails {
	str, ok := value.(string)
	if !ok {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must be a string", field),
			Value:      value,
			Constraint: "string",
		}
	}

	if utf8.RuneCountInString(str) < r.MinLength {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must be at least %d characters long", field, r.MinLength),
			Value:      value,
			Constraint: fmt.Sprintf("min_length:%d", r.MinLength),
		}
	}
	return nil
}

// MaxLengthRule 最大长度验证规则
type MaxLengthRule struct {
	MaxLength int
}

func (r *MaxLengthRule) Validate(value interface{}, field string) *errors.ErrorDetails {
	str, ok := value.(string)
	if !ok {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must be a string", field),
			Value:      value,
			Constraint: "string",
		}
	}

	if utf8.RuneCountInString(str) > r.MaxLength {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must not exceed %d characters", field, r.MaxLength),
			Value:      value,
			Constraint: fmt.Sprintf("max_length:%d", r.MaxLength),
		}
	}
	return nil
}

// EmailRule 邮箱格式验证规则
type EmailRule struct{}

func (r *EmailRule) Validate(value interface{}, field string) *errors.ErrorDetails {
	str, ok := value.(string)
	if !ok {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must be a string", field),
			Value:      value,
			Constraint: "string",
		}
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(str) {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must be a valid email address", field),
			Value:      value,
			Constraint: "email_format",
		}
	}
	return nil
}

// MinRule 最小值验证规则
type MinRule struct {
	Min int
}

func (r *MinRule) Validate(value interface{}, field string) *errors.ErrorDetails {
	var intValue int
	var err error

	switch v := value.(type) {
	case int:
		intValue = v
	case string:
		intValue, err = strconv.Atoi(v)
		if err != nil {
			return &errors.ErrorDetails{
				Field:      field,
				Message:    fmt.Sprintf("%s must be a number", field),
				Value:      value,
				Constraint: "number",
			}
		}
	default:
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must be a number", field),
			Value:      value,
			Constraint: "number",
		}
	}

	if intValue < r.Min {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must be at least %d", field, r.Min),
			Value:      value,
			Constraint: fmt.Sprintf("min:%d", r.Min),
		}
	}
	return nil
}

// MaxRule 最大值验证规则
type MaxRule struct {
	Max int
}

func (r *MaxRule) Validate(value interface{}, field string) *errors.ErrorDetails {
	var intValue int
	var err error

	switch v := value.(type) {
	case int:
		intValue = v
	case string:
		intValue, err = strconv.Atoi(v)
		if err != nil {
			return &errors.ErrorDetails{
				Field:      field,
				Message:    fmt.Sprintf("%s must be a number", field),
				Value:      value,
				Constraint: "number",
			}
		}
	default:
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must be a number", field),
			Value:      value,
			Constraint: "number",
		}
	}

	if intValue > r.Max {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must not exceed %d", field, r.Max),
			Value:      value,
			Constraint: fmt.Sprintf("max:%d", r.Max),
		}
	}
	return nil
}

// URLRule URL格式验证规则
type URLRule struct{}

func (r *URLRule) Validate(value interface{}, field string) *errors.ErrorDetails {
	str, ok := value.(string)
	if !ok {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must be a string", field),
			Value:      value,
			Constraint: "string",
		}
	}

	urlRegex := regexp.MustCompile(`^https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-zA-Z0-9()]{1,6}\b([-a-zA-Z0-9()@:%_\+.~#?&//=]*)$`)
	if !urlRegex.MatchString(str) {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must be a valid URL", field),
			Value:      value,
			Constraint: "url_format",
		}
	}
	return nil
}

// FieldValidator 字段验证器
type FieldValidator struct {
	Field    string
	Rules    []ValidationRule
	Required bool
	Sanitize bool
}

// Validate 验证字段
func (fv *FieldValidator) Validate(data map[string]interface{}) *errors.ErrorDetails {
	value, exists := data[fv.Field]

	// 检查必填字段
	if !exists || value == nil || value == "" {
		if fv.Required {
			return &errors.ErrorDetails{
				Field:      fv.Field,
				Message:    fmt.Sprintf("%s is required", fv.Field),
				Value:      value,
				Constraint: "required",
			}
		}
		return nil // 非必填字段且值为空，跳过其他验证
	}

	// 数据清理
	if fv.Sanitize {
		value = sanitizeValue(value)
		data[fv.Field] = value
	}

	// 执行验证规则
	for _, rule := range fv.Rules {
		if err := rule.Validate(value, fv.Field); err != nil {
			return err
		}
	}

	return nil
}

// ValidationConfig 验证配置
type ValidationConfig struct {
	Fields   []FieldValidator
	Validate func(c *gin.Context) bool // 可选的额外验证函数
}

// sanitizeValue 清理输入值
func sanitizeValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		// 去除首尾空格
		return strings.TrimSpace(v)
	default:
		return v
	}
}

// sanitizeMap 清理map中的所有字符串值
func sanitizeMap(data map[string]interface{}) {
	for key, value := range data {
		if str, ok := value.(string); ok {
			data[key] = strings.TrimSpace(str)
		}
	}
}

// ValidationMiddleware 创建验证中间件
func ValidationMiddleware(config ValidationConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查请求内容类型
		contentType := c.GetHeader("Content-Type")
		if !strings.Contains(contentType, "application/json") && c.Request.Method != "GET" {
			response.BadRequest(c, "Content-Type must be application/json")
			c.Abort()
			return
		}

		var data map[string]interface{}

		if c.Request.Method == "GET" {
			// GET请求使用查询参数
			data = make(map[string]interface{})
			for key, values := range c.Request.URL.Query() {
				if len(values) > 0 {
					data[key] = values[0] // 只取第一个值
				}
			}
		} else {
			// POST/PUT/DELETE请求使用JSON body
			if err := c.ShouldBindJSON(&data); err != nil {
				response.ValidationError(c, "Invalid JSON format", errors.ErrorDetails{
					Field:      "body",
					Message:    "Invalid JSON format: " + err.Error(),
					Value:      nil,
					Constraint: "valid_json",
				})
				c.Abort()
				return
			}
		}

		// 全局数据清理
		sanitizeMap(data)

		// 执行字段验证
		var validationErrors []errors.ErrorDetails
		for _, fieldValidator := range config.Fields {
			if err := fieldValidator.Validate(data); err != nil {
				validationErrors = append(validationErrors, *err)
			}
		}

		// 执行额外验证（如果配置了）
		if config.Validate != nil {
			if !config.Validate(c) {
				c.Abort()
				return
			}
		}

		// 如果有验证错误，返回错误响应
		if len(validationErrors) > 0 {
			response.ValidationError(c, "Validation failed", validationErrors...)
			c.Abort()
			return
		}

		// 将验证后的数据存储到上下文中
		c.Set("validated_data", data)
		c.Next()
	}
}

// GetValidatedData 从上下文中获取验证后的数据
func GetValidatedData(c *gin.Context) (map[string]interface{}, bool) {
	data, exists := c.Get("validated_data")
	if !exists {
		return nil, false
	}

	validatedData, ok := data.(map[string]interface{})
	return validatedData, ok
}

// ========== 预定义的验证配置 ==========

// LoginValidation 登录验证配置
func LoginValidation() ValidationConfig {
	return ValidationConfig{
		Fields: []FieldValidator{
			{
				Field:    "email",
				Required: true,
				Sanitize: true,
				Rules: []ValidationRule{
					&EmailRule{},
				},
			},
			{
				Field:    "password",
				Required: true,
				Rules: []ValidationRule{
					&MinLengthRule{MinLength: 6},
				},
			},
		},
	}
}

// RegisterValidation 注册验证配置
func RegisterValidation() ValidationConfig {
	return ValidationConfig{
		Fields: []FieldValidator{
			{
				Field:    "username",
				Required: true,
				Sanitize: true,
				Rules: []ValidationRule{
					&MinLengthRule{MinLength: 3},
					&MaxLengthRule{MaxLength: 50},
				},
			},
			{
				Field:    "email",
				Required: true,
				Sanitize: true,
				Rules: []ValidationRule{
					&EmailRule{},
				},
			},
			{
				Field:    "password",
				Required: true,
				Rules: []ValidationRule{
					&MinLengthRule{MinLength: 6},
				},
			},
			{
				Field:    "first_name",
				Required: false,
				Sanitize: true,
				Rules: []ValidationRule{
					&MaxLengthRule{MaxLength: 50},
				},
			},
			{
				Field:    "last_name",
				Required: false,
				Sanitize: true,
				Rules: []ValidationRule{
					&MaxLengthRule{MaxLength: 50},
				},
			},
		},
	}
}

// UpdateUserValidation 更新用户验证配置
func UpdateUserValidation() ValidationConfig {
	return ValidationConfig{
		Fields: []FieldValidator{
			{
				Field:    "username",
				Required: false,
				Sanitize: true,
				Rules: []ValidationRule{
					&MinLengthRule{MinLength: 3},
					&MaxLengthRule{MaxLength: 50},
				},
			},
			{
				Field:    "first_name",
				Required: false,
				Sanitize: true,
				Rules: []ValidationRule{
					&MaxLengthRule{MaxLength: 50},
				},
			},
			{
				Field:    "last_name",
				Required: false,
				Sanitize: true,
				Rules: []ValidationRule{
					&MaxLengthRule{MaxLength: 50},
				},
			},
			{
				Field:    "avatar",
				Required: false,
				Sanitize: true,
				Rules: []ValidationRule{
					&URLRule{},
				},
			},
		},
	}
}

// ChangePasswordValidation 修改密码验证配置
func ChangePasswordValidation() ValidationConfig {
	return ValidationConfig{
		Fields: []FieldValidator{
			{
				Field:    "old_password",
				Required: true,
				Rules: []ValidationRule{
					&RequiredRule{},
				},
			},
			{
				Field:    "new_password",
				Required: true,
				Rules: []ValidationRule{
					&MinLengthRule{MinLength: 6},
				},
			},
		},
	}
}

// PaginationValidation 分页参数验证配置
func PaginationValidation() ValidationConfig {
	return ValidationConfig{
		Fields: []FieldValidator{
			{
				Field:    "page",
				Required: false,
				Rules: []ValidationRule{
					&MinRule{Min: 1},
				},
			},
			{
				Field:    "limit",
				Required: false,
				Rules: []ValidationRule{
					&MinRule{Min: 1},
					&MaxRule{Max: 100},
				},
			},
		},
	}
}

// UserIDValidation 用户ID验证配置
func UserIDValidation() ValidationConfig {
	return ValidationConfig{
		Validate: func(c *gin.Context) bool {
			userID := c.Param("id")
			if userID == "" {
				response.NotFoundError(c, "user", "")
				return false
			}
			c.Set("user_id", userID)
			return true
		},
	}
}

// StructValidation 基于结构体标签的验证中间件
func StructValidation(model interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查请求内容类型
		contentType := c.GetHeader("Content-Type")
		if !strings.Contains(contentType, "application/json") && c.Request.Method != "GET" {
			response.BadRequest(c, "Content-Type must be application/json")
			c.Abort()
			return
		}

		// 创建模型实例的副本用于绑定
		modelValue := reflect.ValueOf(model)
		if modelValue.Kind() == reflect.Ptr {
			modelValue = modelValue.Elem()
		}

		// 创建新的实例
		newModel := reflect.New(modelValue.Type()).Interface()

		// 绑定请求数据
		if err := c.ShouldBindJSON(newModel); err != nil {
			response.ValidationError(c, "Invalid request format", errors.ErrorDetails{
				Field:      "body",
				Message:    "Invalid JSON format: " + err.Error(),
				Value:      nil,
				Constraint: "valid_json",
			})
			c.Abort()
			return
		}

		// 使用Gin的内置验证
		if err := c.ShouldBind(newModel); err != nil {
			response.ValidationError(c, "Validation failed", errors.ErrorDetails{
				Field:      "validation",
				Message:    err.Error(),
				Value:      newModel,
				Constraint: "struct_validation",
			})
			c.Abort()
			return
		}

		// 将验证后的数据存储到上下文中
		c.Set("validated_model", newModel)
		c.Next()
	}
}

// GetValidatedModel 从上下文中获取验证后的模型
func GetValidatedModel(c *gin.Context) (interface{}, bool) {
	data, exists := c.Get("validated_model")
	if !exists {
		return nil, false
	}

	return data, true
}

// ========== 自定义验证规则示例 ==========

// PasswordStrengthRule 密码强度验证规则
type PasswordStrengthRule struct {
	RequireUppercase bool
	RequireLowercase bool
	RequireNumber    bool
	RequireSymbol    bool
	MinLength        int
}

func (r *PasswordStrengthRule) Validate(value interface{}, field string) *errors.ErrorDetails {
	str, ok := value.(string)
	if !ok {
		return &errors.ErrorDetails{
			Field:      field,
			Message:    fmt.Sprintf("%s must be a string", field),
			Value:      value,
			Constraint: "string",
		}
	}

	var errorMessages []string

	if utf8.RuneCountInString(str) < r.MinLength {
		errorMessages = append(errorMessages, fmt.Sprintf("at least %d characters", r.MinLength))
	}

	if r.RequireUppercase && !regexp.MustCompile(`[A-Z]`).MatchString(str) {
		errorMessages = append(errorMessages, "uppercase letter")
	}

	if r.RequireLowercase && !regexp.MustCompile(`[a-z]`).MatchString(str) {
		errorMessages = append(errorMessages, "lowercase letter")
	}

	if r.RequireNumber && !regexp.MustCompile(`\d`).MatchString(str) {
		errorMessages = append(errorMessages, "number")
	}

	if r.RequireSymbol && !regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`).MatchString(str) {
		errorMessages = append(errorMessages, "special symbol")
	}

	if len(errorMessages) > 0 {
		message := fmt.Sprintf("%s must contain ", field)
		if len(errorMessages) == 1 {
			message += errorMessages[0]
		} else {
			message += strings.Join(errorMessages[:len(errorMessages)-1], ", ") + " and " + errorMessages[len(errorMessages)-1]
		}

		return &errors.ErrorDetails{
			Field:      field,
			Message:    message,
			Value:      value,
			Constraint: "password_strength",
		}
	}

	return nil
}

// StrongPasswordValidation 强密码验证配置
func StrongPasswordValidation() ValidationConfig {
	return ValidationConfig{
		Fields: []FieldValidator{
			{
				Field:    "password",
				Required: true,
				Rules: []ValidationRule{
					&PasswordStrengthRule{
						RequireUppercase: true,
						RequireLowercase: true,
						RequireNumber:    true,
						RequireSymbol:    true,
						MinLength:        8,
					},
				},
			},
		},
	}
}

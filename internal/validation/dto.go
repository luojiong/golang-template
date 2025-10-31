package validation

import (
	"errors"
	"fmt"
	"strings"

	"go-server/internal/models"
)

// DTO (Data Transfer Object) 层定义

// RegisterUserDTO 用户注册DTO
type RegisterUserDTO struct {
	Username  string `json:"username" form:"username"`
	Email     string `json:"email" form:"email"`
	Password  string `json:"password" form:"password"`
	FirstName string `json:"first_name" form:"first_name"`
	LastName  string `json:"last_name" form:"last_name"`
	Avatar    string `json:"avatar" form:"avatar"`
}

// Validate 验证注册DTO
func (dto *RegisterUserDTO) Validate() error {
	validator := NewValidator()

	if err := validator.ValidateField("username", dto.Username,
		Required(),
		MinLength(3),
		MaxLength(50),
		Username(),
	); err != nil {
		return err
	}

	if err := validator.ValidateField("email", dto.Email,
		Required(),
		Email(),
	); err != nil {
		return err
	}

	if err := validator.ValidateField("password", dto.Password,
		Required(),
		MinLength(6),
		Password(),
	); err != nil {
		return err
	}

	if err := validator.ValidateField("first_name", dto.FirstName,
		MaxLength(50),
	); err != nil {
		return err
	}

	if err := validator.ValidateField("last_name", dto.LastName,
		MaxLength(50),
	); err != nil {
		return err
	}

	if err := validator.ValidateField("avatar", dto.Avatar,
		MaxLength(255),
		URL(),
	); err != nil && dto.Avatar != "" {
		return err
	}

	return nil
}

// ToModel 转换为模型
func (dto *RegisterUserDTO) ToModel() *models.RegisterRequest {
	return &models.RegisterRequest{
		Username:  dto.Username,
		Email:     dto.Email,
		Password:  dto.Password,
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
	}
}

// LoginUserDTO 用户登录DTO
type LoginUserDTO struct {
	Email    string `json:"email" form:"email"`
	Password string `json:"password" form:"password"`
}

// Validate 验证登录DTO
func (dto *LoginUserDTO) Validate() error {
	validator := NewValidator()

	if err := validator.ValidateField("email", dto.Email,
		Required(),
		Email(),
	); err != nil {
		return err
	}

	if err := validator.ValidateField("password", dto.Password,
		Required(),
	); err != nil {
		return err
	}

	return nil
}

// ToModel 转换为模型
func (dto *LoginUserDTO) ToModel() *models.LoginRequest {
	return &models.LoginRequest{
		Email:    dto.Email,
		Password: dto.Password,
	}
}

// UpdateUserDTO 更新用户DTO
type UpdateUserDTO struct {
	Username  string `json:"username" form:"username"`
	FirstName string `json:"first_name" form:"first_name"`
	LastName  string `json:"last_name" form:"last_name"`
	Avatar    string `json:"avatar" form:"avatar"`
}

// Validate 验证更新用户DTO
func (dto *UpdateUserDTO) Validate() error {
	validator := NewValidator()

	if dto.Username != "" {
		if err := validator.ValidateField("username", dto.Username,
			MinLength(3),
			MaxLength(50),
			Username(),
		); err != nil {
			return err
		}
	}

	if err := validator.ValidateField("first_name", dto.FirstName,
		MaxLength(50),
	); err != nil {
		return err
	}

	if err := validator.ValidateField("last_name", dto.LastName,
		MaxLength(50),
	); err != nil {
		return err
	}

	if err := validator.ValidateField("avatar", dto.Avatar,
		MaxLength(255),
		URL(),
	); err != nil && dto.Avatar != "" {
		return err
	}

	return nil
}

// ToModel 转换为模型
func (dto *UpdateUserDTO) ToModel() *models.UpdateUserRequest {
	return &models.UpdateUserRequest{
		Username:  dto.Username,
		FirstName: dto.FirstName,
		LastName:  dto.LastName,
		Avatar:    dto.Avatar,
	}
}

// ChangePasswordDTO 修改密码DTO
type ChangePasswordDTO struct {
	OldPassword string `json:"old_password" form:"old_password"`
	NewPassword string `json:"new_password" form:"new_password"`
}

// Validate 验证修改密码DTO
func (dto *ChangePasswordDTO) Validate() error {
	validator := NewValidator()

	if err := validator.ValidateField("old_password", dto.OldPassword,
		Required(),
	); err != nil {
		return err
	}

	if err := validator.ValidateField("new_password", dto.NewPassword,
		Required(),
		MinLength(6),
		Password(),
	); err != nil {
		return err
	}

	// 确保新旧密码不同
	if dto.OldPassword == dto.NewPassword {
		return errors.New("new password must be different from old password")
	}

	return nil
}

// ToModel 转换为模型
func (dto *ChangePasswordDTO) ToModel() *models.ChangePasswordRequest {
	return &models.ChangePasswordRequest{
		OldPassword: dto.OldPassword,
		NewPassword: dto.NewPassword,
	}
}

// PaginationParamsDTO 分页参数DTO
type PaginationParamsDTO struct {
	Page  int `json:"page" form:"page"`
	Limit int `json:"limit" form:"limit"`
}

// Validate 验证分页参数DTO
func (dto *PaginationParamsDTO) Validate() error {
	validator := NewValidator()

	if err := validator.ValidateField("page", dto.Page,
		Min(1),
	); err != nil {
		return err
	}

	if err := validator.ValidateField("limit", dto.Limit,
		Min(1),
		Max(100),
	); err != nil {
		return err
	}

	return nil
}

// ToModel 转换为模型
func (dto *PaginationParamsDTO) ToModel() *models.PaginationParams {
	// 设置默认值
	page := dto.Page
	if page <= 0 {
		page = 1
	}

	limit := dto.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	return &models.PaginationParams{
		Page:  page,
		Limit: limit,
	}
}

// QueryParamsDTO 查询参数DTO
type QueryParamsDTO struct {
	Search string `json:"search" form:"search"`
	Sort   string `json:"sort" form:"sort"`
	Order  string `json:"order" form:"order"`
}

// Validate 验证查询参数DTO
func (dto *QueryParamsDTO) Validate() error {
	validator := NewValidator()

	if err := validator.ValidateField("sort", dto.Sort,
		In("username", "email", "created_at", "updated_at"),
	); err != nil && dto.Sort != "" {
		return err
	}

	if err := validator.ValidateField("order", dto.Order,
		In("asc", "desc"),
	); err != nil && dto.Order != "" {
		return err
	}

	return nil
}

// GetSortBy 获取排序字段
func (dto *QueryParamsDTO) GetSortBy() string {
	if dto.Sort == "" {
		return "created_at"
	}
	return dto.Sort
}

// GetSortOrder 获取排序方向
func (dto *QueryParamsDTO) GetSortOrder() string {
	if dto.Order == "" {
		return "desc"
	}
	return dto.Order
}

// ValidationErrors 验证错误集合
type ValidationErrors struct {
	Errors []FieldValidationError `json:"errors"`
}

// Error 实现error接口
func (ve ValidationErrors) Error() string {
	if len(ve.Errors) == 0 {
		return "validation failed"
	}

	messages := make([]string, len(ve.Errors))
	for i, err := range ve.Errors {
		messages[i] = err.Error()
	}

	return fmt.Sprintf("validation failed: %s", strings.Join(messages, "; "))
}

// AddError 添加验证错误
func (ve *ValidationErrors) AddError(err FieldValidationError) {
	ve.Errors = append(ve.Errors, err)
}

// HasErrors 是否有错误
func (ve *ValidationErrors) HasErrors() bool {
	return len(ve.Errors) > 0
}

// ToErrorDetails 转换为错误详情
func (ve *ValidationErrors) ToErrorDetails() []map[string]interface{} {
	details := make([]map[string]interface{}, len(ve.Errors))
	for i, err := range ve.Errors {
		details[i] = map[string]interface{}{
			"field":   err.Field,
			"message": err.Message,
			"value":   err.Value,
		}
	}
	return details
}

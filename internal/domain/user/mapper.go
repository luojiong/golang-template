package user

import (
	"go-server/internal/models"
)

// Mapper 领域模型与数据模型之间的映射器
type Mapper struct{}

// NewMapper 创建新的映射器
func NewMapper() *Mapper {
	return &Mapper{}
}

// ToDomainModel 将数据模型转换为领域模型
func (m *Mapper) ToDomainModel(userModel *models.User) (*User, error) {
	if userModel == nil {
		return nil, nil
	}

	// 转换值对象
	userID, err := NewUserIDFromString(userModel.ID)
	if err != nil {
		return nil, err
	}

	email, err := NewEmail(userModel.Email)
	if err != nil {
		return nil, err
	}

	username, err := NewUsername(userModel.Username)
	if err != nil {
		return nil, err
	}

	password, err := NewPasswordFromHash(userModel.Password)
	if err != nil {
		return nil, err
	}

	profile := NewUserProfile(userModel.FirstName, userModel.LastName, userModel.Avatar)

	// 转换状态
	var status UserStatus
	switch userModel.IsActive {
	case true:
		status = UserStatusActive
	default:
		status = UserStatusInactive
	}

	// 转换角色
	var role UserRole
	if userModel.IsAdmin {
		role = UserRoleAdmin
	} else {
		role = UserRoleRegular
	}

	// 创建领域实体
	user := &User{
		id:        userID,
		email:     email,
		username:  username,
		password:  password,
		profile:   profile,
		status:    status,
		role:      role,
		createdAt: userModel.CreatedAt,
		updatedAt: userModel.UpdatedAt,
		lastLogin: userModel.LastLogin,
	}

	return user, nil
}

// ToDataModel 将领域模型转换为数据模型
func (m *Mapper) ToDataModel(user *User) *models.User {
	if user == nil {
		return nil
	}

	return &models.User{
		ID:        user.ID().String(),
		Email:     user.Email().String(),
		Username:  user.Username().String(),
		Password:  user.password.Hash(), // 使用密码的哈希值
		FirstName: user.Profile().FirstName(),
		LastName:  user.Profile().LastName(),
		Avatar:    user.Profile().Avatar(),
		IsActive:  user.IsActive(),
		IsAdmin:   user.IsAdmin(),
		CreatedAt: user.CreatedAt(),
		UpdatedAt: user.UpdatedAt(),
		LastLogin: user.LastLogin(),
	}
}

// ToDomainModels 批量转换数据模型到领域模型
func (m *Mapper) ToDomainModels(userModels []*models.User) ([]*User, error) {
	if len(userModels) == 0 {
		return nil, nil
	}

	users := make([]*User, 0, len(userModels))
	for _, userModel := range userModels {
		user, err := m.ToDomainModel(userModel)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// ToDataModels 批量转换领域模型到数据模型
func (m *Mapper) ToDataModels(users []*User) []*models.User {
	if len(users) == 0 {
		return nil
	}

	userModels := make([]*models.User, 0, len(users))
	for _, user := range users {
		userModel := m.ToDataModel(user)
		if userModel != nil {
			userModels = append(userModels, userModel)
		}
	}

	return userModels
}

// ToSafeUser 将领域模型转换为安全用户响应
func (m *Mapper) ToSafeUser(user *User) models.SafeUser {
	if user == nil {
		return models.SafeUser{}
	}

	return models.SafeUser{
		ID:        user.ID().String(),
		Username:  user.Username().String(),
		Email:     user.Email().String(),
		FirstName: user.Profile().FirstName(),
		LastName:  user.Profile().LastName(),
		Avatar:    user.Profile().Avatar(),
		IsActive:  user.IsActive(),
		IsAdmin:   user.IsAdmin(),
		CreatedAt: user.CreatedAt(),
		UpdatedAt: user.UpdatedAt(),
		LastLogin: user.LastLogin(),
	}
}

// ToSafeUsers 批量转换领域模型到安全用户响应
func (m *Mapper) ToSafeUsers(users []*User) []models.SafeUser {
	if len(users) == 0 {
		return nil
	}

	safeUsers := make([]models.SafeUser, 0, len(users))
	for _, user := range users {
		safeUsers = append(safeUsers, m.ToSafeUser(user))
	}

	return safeUsers
}

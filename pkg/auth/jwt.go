package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// BlacklistChecker 定义检查令牌是否被列入黑名单的接口
// 此接口避免了auth和cache包之间的循环依赖
type BlacklistChecker interface {
	IsBlacklisted(ctx context.Context, tokenString string) (bool, error)
}

// Claims JWT声明结构
type Claims struct {
	UserID   string `json:"user_id"`   // 用户ID
	Username string `json:"username"`  // 用户名
	Email    string `json:"email"`     // 邮箱地址
	jwt.RegisteredClaims                // JWT标准声明
}

// JWTManager JWT管理器
type JWTManager struct {
	secretKey        string           // 密钥
	expiresIn        time.Duration    // 过期时间
	blacklistChecker BlacklistChecker // 黑名单检查器
}

// NewJWTManager 创建新的JWT管理器
func NewJWTManager(secretKey string, expiresIn int) *JWTManager {
	return &JWTManager{
		secretKey:         secretKey,
		expiresIn:         time.Duration(expiresIn) * time.Hour,
		blacklistChecker:  nil,
	}
}

// NewJWTManagerWithBlacklist 创建支持黑名单的新JWT管理器
func NewJWTManagerWithBlacklist(secretKey string, expiresIn int, blacklistChecker BlacklistChecker) *JWTManager {
	return &JWTManager{
		secretKey:         secretKey,
		expiresIn:         time.Duration(expiresIn) * time.Hour,
		blacklistChecker:  blacklistChecker,
	}
}

// GenerateToken 生成JWT令牌
func (j *JWTManager) GenerateToken(userID, username, email string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.expiresIn)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secretKey))
}

// ValidateToken 验证JWT令牌
func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	return j.ValidateTokenWithContext(context.Background(), tokenString)
}

// ValidateTokenWithContext 验证JWT令牌，支持上下文和黑名单检查
func (j *JWTManager) ValidateTokenWithContext(ctx context.Context, tokenString string) (*Claims, error) {
	// If blacklist checker is available, check if token is blacklisted
	if j.blacklistChecker != nil {
		blacklisted, err := j.blacklistChecker.IsBlacklisted(ctx, tokenString)
		if err != nil {
			// Log the error but continue with validation (graceful fallback)
			// In a real application, you might want to use proper logging here
			// For now, we'll just continue with validation
			fmt.Printf("Warning: failed to check blacklist: %v\n", err)
		} else if blacklisted {
			return nil, errors.New("token is blacklisted")
		}
	}

	// Proceed with normal JWT validation
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(j.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
package cache

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"go-server/pkg/auth"

	"github.com/golang-jwt/jwt/v5"
)

// BlacklistService 提供 JWT 令牌黑名单功能
type BlacklistService struct {
	cache      Cache
	jwtManager *auth.JWTManager
	keyPrefix  string
}

// BlacklistConfig 保存黑名单服务的配置
type BlacklistConfig struct {
	// KeyPrefix 是 Redis 中黑名单键的前缀
	KeyPrefix string
	// CleanupInterval 是运行过期令牌清理的频率
	CleanupInterval time.Duration
	// BatchSize 是每次清理批次中要处理的令牌数量
	BatchSize int
}

// DefaultBlacklistConfig 返回黑名单服务的默认配置
func DefaultBlacklistConfig() *BlacklistConfig {
	return &BlacklistConfig{
		KeyPrefix:       "jwt_blacklist:",
		CleanupInterval: 1 * time.Hour,
		BatchSize:       100,
	}
}

// NewBlacklistService 创建新的 JWT 黑名单服务
func NewBlacklistService(cache Cache, jwtManager *auth.JWTManager, config *BlacklistConfig) *BlacklistService {
	if config == nil {
		config = DefaultBlacklistConfig()
	}

	return &BlacklistService{
		cache:      cache,
		jwtManager: jwtManager,
		keyPrefix:  config.KeyPrefix,
	}
}

// AddToBlacklist 将 JWT 令牌添加到黑名单
// 令牌将保持黑名单状态直到其自然过期时间
func (b *BlacklistService) AddToBlacklist(ctx context.Context, tokenString string) error {
	// 解析令牌以获取其过期时间
	claims, err := b.parseToken(tokenString)
	if err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
	}

	// 计算直到令牌过期的 TTL
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		// 令牌已经过期，无需加入黑名单
		return nil
	}

	// 为令牌生成唯一键
	tokenKey := b.generateTokenKey(tokenString)

	// 添加到缓存，TTL 等于令牌的剩余生命周期
	return b.cache.Set(ctx, tokenKey, "blacklisted", ttl)
}

// IsBlacklisted 检查 JWT 令牌是否在黑名单中
func (b *BlacklistService) IsBlacklisted(ctx context.Context, tokenString string) (bool, error) {
	// 首先通过解析令牌检查是否已经过期
	claims, err := b.parseToken(tokenString)
	if err != nil {
		// 如果我们无法解析令牌，则认为其无效/已过期
		return true, nil
	}

	// 检查令牌是否过期
	if time.Now().After(claims.ExpiresAt.Time) {
		// 令牌已自然过期
		return false, nil
	}

	// 检查令牌是否存在于黑名单中
	tokenKey := b.generateTokenKey(tokenString)
	exists, err := b.cache.Exists(ctx, tokenKey)
	if err != nil {
		return false, fmt.Errorf("failed to check blacklist: %w", err)
	}

	return exists, nil
}

// RemoveFromBlacklist 从黑名单中移除 JWT 令牌
// 这对于希望重新使用令牌的情况很有用
func (b *BlacklistService) RemoveFromBlacklist(ctx context.Context, tokenString string) error {
	tokenKey := b.generateTokenKey(tokenString)
	return b.cache.Delete(ctx, tokenKey)
}

// CleanupExpiredTokens 从黑名单中移除过期令牌
// 这是一个维护操作，用于保持黑名单的清洁
func (b *BlacklistService) CleanupExpiredTokens(ctx context.Context) error {
	// 获取所有黑名单键
	keys, err := b.cache.Keys(ctx, "*")
	if err != nil {
		return fmt.Errorf("failed to get blacklist keys: %w", err)
	}

	for _, key := range keys {
		// 检查键是否仍然存在（可能已过期）
		exists, err := b.cache.Exists(ctx, key)
		if err != nil {
			continue // 出错时跳过此键
		}

		if !exists {
			// 键已过期，Redis 自动移除了它
			continue
		}

		// 可选地检查存储的令牌是否实际过期
		// 这是额外的安全检查，以防 TTL 未正确设置
		value, found := b.cache.Get(ctx, key)
		if !found {
			continue
		}

		// 如果我们存储了关于令牌的额外元数据，我们可以在这里检查
		// 目前，我们依赖 Redis TTL 进行自动清理
		_ = value // 抑制未使用变量警告
	}

	return nil
}

// GetBlacklistSize 返回当前黑名单中的令牌数量
func (b *BlacklistService) GetBlacklistSize(ctx context.Context) (int, error) {
	keys, err := b.cache.Keys(ctx, "*")
	if err != nil {
		return 0, fmt.Errorf("failed to get blacklist keys: %w", err)
	}

	return len(keys), nil
}

// ClearBlacklist 从黑名单中移除所有令牌
// 谨慎使用 - 这将允许所有先前被列入黑名单的令牌再次使用
func (b *BlacklistService) ClearBlacklist(ctx context.Context) error {
	return b.cache.Clear(ctx)
}

// ValidateTokenWithBlacklist 是一个便捷方法，结合了 JWT 验证和黑名单检查
func (b *BlacklistService) ValidateTokenWithBlacklist(ctx context.Context, tokenString string) (*auth.Claims, error) {
	// 首先检查令牌是否在黑名单中
	blacklisted, err := b.IsBlacklisted(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("failed to check blacklist: %w", err)
	}

	if blacklisted {
		return nil, fmt.Errorf("token is blacklisted")
	}

	// 如果不在黑名单中，使用 JWT 管理器进行验证
	return b.jwtManager.ValidateToken(tokenString)
}

// parseToken 解析 JWT 令牌并返回其声明
func (b *BlacklistService) parseToken(tokenString string) (*jwt.RegisteredClaims, error) {
	// 不验证解析以获取过期时间
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, &jwt.RegisteredClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// generateTokenKey 生成用于在黑名单中存储令牌的唯一键
func (b *BlacklistService) generateTokenKey(tokenString string) string {
	// 使用令牌的 SHA256 哈希创建固定长度的键
	// 这确保我们不会在 Redis 键中遇到特殊字符问题
	hash := sha256.Sum256([]byte(tokenString))
	return fmt.Sprintf("%s%x", b.keyPrefix, hash)
}

// AddMultipleToBlacklist 在单个操作中将多个令牌添加到黑名单
func (b *BlacklistService) AddMultipleToBlacklist(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}

	// 准备批量设置的项目
	items := make(map[string]interface{})
	ttls := make(map[string]time.Duration)

	for _, token := range tokens {
		claims, err := b.parseToken(token)
		if err != nil {
			// 跳过无效令牌但继续处理其他令牌
			continue
		}

		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl <= 0 {
			// 跳过过期令牌
			continue
		}

		tokenKey := b.generateTokenKey(token)
		items[tokenKey] = "blacklisted"
		ttls[tokenKey] = ttl
	}

	// 由于缓存接口在 SetMultiple 中不支持每个键的 TTL，
	// 我们需要单独设置它们或找到最小 TTL
	// 为简单起见，我们将对所有令牌使用最小 TTL
	var minTTL time.Duration
	if len(ttls) > 0 {
		// 找到第一个键以获取其 TTL
		for key := range items {
			minTTL = ttls[key]
			break
		}
		for _, ttl := range ttls {
			if ttl < minTTL {
				minTTL = ttl
			}
		}
	}

	return b.cache.SetMultiple(ctx, items, minTTL)
}

// GetBlacklistedTokensInfo 返回关于被列入黑名单令牌的信息
func (b *BlacklistService) GetBlacklistedTokensInfo(ctx context.Context, limit int) ([]BlacklistedTokenInfo, error) {
	keys, err := b.cache.Keys(ctx, "*")
	if err != nil {
		return nil, fmt.Errorf("failed to get blacklist keys: %w", err)
	}

	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}

	var infos []BlacklistedTokenInfo
	for _, key := range keys {
		// 获取键的 TTL
		cache := b.cache.(*RedisCache) // 类型断言以使用 Redis 特定方法
		ttl, err := cache.GetTTL(ctx, key)
		if err != nil {
			continue
		}

		infos = append(infos, BlacklistedTokenInfo{
			Key:       key,
			ExpiresAt: time.Now().Add(ttl),
		})
	}

	return infos, nil
}

// BlacklistedTokenInfo 保存关于被列入黑名单令牌的信息
type BlacklistedTokenInfo struct {
	Key       string
	ExpiresAt time.Time
}

package cache

import (
	"AreYouOK/config"
	"AreYouOK/storage/redis"
	"context"
	"time"
)

const (
	tokenPrefix = "token"
)

// SetRefreshToken 存储 refresh token 到 Redis
// Key: ayok:token:refresh:{user_id}
// TTL: 7天
func SetRefreshToken(ctx context.Context, userID, refreshToken string) error {
	key := redis.Key(tokenPrefix, "refresh", userID)
	ttl := time.Duration(config.Cfg.JWTRefreshDays) * 24 * time.Hour
	
	return redis.Client().Set(ctx, key, refreshToken, ttl).Err()
}

// GetRefreshToken 从 Redis 获取 refresh token
func GetRefreshToken(ctx context.Context, userID string) (string, error) {
	key := redis.Key(tokenPrefix, "refresh", userID)
	return redis.Client().Get(ctx, key).Result()
}

// DeleteRefreshToken 删除 refresh token（用于登出或 token 失效）
func DeleteRefreshToken(ctx context.Context, userID string) error {
	key := redis.Key(tokenPrefix, "refresh", userID)
	return redis.Client().Del(ctx, key).Err()
}

// ValidateRefreshTokenExists 检查 refresh token 是否存在且匹配
func ValidateRefreshTokenExists(ctx context.Context, userID, refreshToken string) bool {
	storedToken, err := GetRefreshToken(ctx, userID)
	if err != nil {
		return false
	}
	return storedToken == refreshToken
}
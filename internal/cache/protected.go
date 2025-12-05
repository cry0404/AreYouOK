package cache

import (
	"AreYouOK/storage/redis"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	ri "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"AreYouOK/pkg/logger"
)

const (
	// 空值缓存标识
	emptyValueFlag = "__EMPTY__"
	// 空值缓存TTL，较短时间避免长期占用
	emptyValueTTL = 5 * time.Minute
	// 防雪崩随机延迟范围
	breakerRandomDelayMax = 200 * time.Millisecond
)

// ProtectedCache 带保护的缓存包装器
type ProtectedCache struct {
	keyPrefix string
	ttl       time.Duration
	emptyTTL  time.Duration
}

// NewProtectedCache 创建受保护的缓存实例
func NewProtectedCache(keyPrefix string, ttl time.Duration) *ProtectedCache {
	return &ProtectedCache{
		keyPrefix: keyPrefix,
		ttl:       ttl,
		emptyTTL:  emptyValueTTL,
	}
}

// Set 设置缓存（带空值保护）
func (pc *ProtectedCache) Set(ctx context.Context, key string, value interface{}) error {
	cacheKey := redis.Key(pc.keyPrefix, key)

	var data string
	var ttl time.Duration

	if value == nil {
		// 空值保护：存储特殊标识，使用较短TTL
		data = emptyValueFlag
		ttl = pc.emptyTTL
	} else {
		// 正常值：序列化后存储
		dataBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("failed to marshal cache value: %w", err)
		}
		data = string(dataBytes)
		ttl = pc.ttl
	}

	return redis.Client().Set(ctx, cacheKey, data, ttl).Err()
}

// Get 获取缓存（带空值保护和防雪崩）
func (pc *ProtectedCache) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	cacheKey := redis.Key(pc.keyPrefix, key)


	if err := pc.addBreakerDelay(ctx); err != nil {
		logger.Logger.Warn("Failed to add breaker delay",
			zap.String("key", key),
			zap.Error(err),
		)
	}

	data, err := redis.Client().Get(ctx, cacheKey).Result()
	if err != nil {
		if err == ri.Nil {
			return false, nil // 缓存未命中
		}
		return false, fmt.Errorf("failed to get cache: %w", err)
	}

	// 检查是否为空值标识
	if data == emptyValueFlag {
		return true, nil // 空值命中
	}

	// 反序列化正常值
	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return false, fmt.Errorf("failed to unmarshal cache value: %w", err)
	}

	return true, nil
}


func (pc *ProtectedCache) BatchGet(ctx context.Context, keys []string, destFunc func(string) interface{}) (map[string]interface{}, error) {
	if len(keys) == 0 {
		return make(map[string]interface{}), nil
	}

	if err := pc.addBreakerDelay(ctx); err != nil {
		logger.Logger.Warn("Failed to add batch breaker delay", zap.Error(err))
	}

	pipe := redis.Client().Pipeline()
	cmds := make(map[string]*ri.StringCmd)
	cacheKeys := make([]string, len(keys))

	// 构建批量查询
	for i, key := range keys {
		cacheKey := redis.Key(pc.keyPrefix, key)
		cacheKeys[i] = cacheKey
		cmds[key] = pipe.Get(ctx, cacheKey)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != ri.Nil {
		return nil, fmt.Errorf("failed to batch get cache: %w", err)
	}

	result := make(map[string]interface{})
	for key, cmd := range cmds {
		data, err := cmd.Result()
		if err != nil {
			if err == ri.Nil {
				continue // 缓存未命中，跳过
			}
			logger.Logger.Warn("Failed to get cache item in batch",
				zap.String("key", key),
				zap.Error(err),
			)
			continue
		}

		// 检查是否为空值标识
		if data == emptyValueFlag {
			result[key] = nil // 空值命中
			continue
		}

		// 反序列化值
		dest := destFunc(key)
		if dest == nil {
			logger.Logger.Warn("Dest function returned nil for key",
				zap.String("key", key),
			)
			continue
		}

		if err := json.Unmarshal([]byte(data), dest); err != nil {
			logger.Logger.Warn("Failed to unmarshal cache item in batch",
				zap.String("key", key),
				zap.Error(err),
			)
			continue
		}

		result[key] = dest
	}

	return result, nil
}

// Delete 删除缓存
func (pc *ProtectedCache) Delete(ctx context.Context, key string) error {
	cacheKey := redis.Key(pc.keyPrefix, key)
	return redis.Client().Del(ctx, cacheKey).Err()
}

// BatchDelete 批量删除缓存
func (pc *ProtectedCache) BatchDelete(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	pipe := redis.Client().Pipeline()
	for _, key := range keys {
		cacheKey := redis.Key(pc.keyPrefix, key)
		pipe.Del(ctx, cacheKey)
	}

	_, err := pipe.Exec(ctx)
	return err
}

// addBreakerDelay 添加防雪崩随机延迟
func (pc *ProtectedCache) addBreakerDelay(ctx context.Context) error {

	delay := time.Duration(rand.Intn(int(breakerRandomDelayMax)))

	// 使用 context 支持取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// IsEmptyValue 检查值是否为空值缓存
func IsEmptyValue(value interface{}) bool {
	return value == nil
}

// 预定义的缓存实例
var (
	UserSettingsProtectedCache = NewProtectedCache("user:settings", 24*time.Hour)
	CaptchaProtectedCache      = NewProtectedCache("captcha", 1*time.Minute)
	MessageProtectedCache      = NewProtectedCache("message", 24*time.Hour)
	QuotaProtectedCache        = NewProtectedCache("quota", 1*time.Hour)
)

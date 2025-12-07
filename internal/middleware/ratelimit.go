package middleware

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	redislib "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/response"
	"AreYouOK/storage/redis"
)

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	// 时间窗口（秒）
	Window int
	// 时间窗口内最大请求数
	MaxRequests int
	// 限流键前缀
	KeyPrefix string
	// 是否按用户ID限流（需要认证）
	ByUserID bool
	// 是否按IP限流
	ByIP bool
	// 阻塞时长（秒），超过限制后禁止访问的时间
	BlockDuration int
	// 错误消息
	ErrorMessage string
}

// DefaultRateLimitConfig 默认限流配置
var DefaultRateLimitConfig = RateLimitConfig{
	Window:        60,    // 60秒
	MaxRequests:   100,   // 100次请求
	KeyPrefix:     "rate:limit",
	ByUserID:      true,
	ByIP:          true,
	BlockDuration: 300,   // 阻塞5分钟
	ErrorMessage:  "请求过于频繁，请稍后再试",
}

// UserSettingsRateLimitConfig 用户设置修改限流配置
var UserSettingsRateLimitConfig = RateLimitConfig{
	Window:        10,  // 10 min
	MaxRequests:   1,     // 5次修改
	KeyPrefix:     "user:settings:rate",
	ByUserID:      true,
	ByIP:          false,
	BlockDuration: 3600,  // 阻塞1小时
	ErrorMessage:  "设置修改过于频繁，请稍后再试",
}

var JourneySettingRateLimitConfig = RateLimitConfig{
	Window:        600,  // 10 min
	MaxRequests:   1,     // 5次修改
	KeyPrefix:     "user:settings:rate",
	ByUserID:      true,
	ByIP:          false,
	BlockDuration: 3600,  // 阻塞1小时
	ErrorMessage:  "设置修改过于频繁，请稍后再试",
}

// RateLimiter 限流器
type RateLimiter struct {
	config RateLimitConfig
}

func NewRateLimiter(config RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		config: config,
	}
}

// getKey 生成限流键
func (rl *RateLimiter) getKey(ctx context.Context, c *app.RequestContext) string {
	var identifier string


	if rl.config.ByUserID {
		if userID, exists := GetUserID(ctx, c); exists {
			identifier = fmt.Sprintf("user:%s", userID)
		}
	}

	if identifier == "" && rl.config.ByIP {
		identifier = fmt.Sprintf("ip:%s", c.ClientIP())
	}

	return redis.Key(rl.config.KeyPrefix, identifier)
}



// Allow 检查是否允许请求，使用滑动窗口算法
func (rl *RateLimiter) Allow(ctx context.Context, c *app.RequestContext) (bool, int, error) {
	key := rl.getKey(ctx, c)
	now := time.Now()
	windowStart := now.Add(-time.Duration(rl.config.Window) * time.Second) // 基于配置的限流器时长，来测算有关的滑动窗口部分


	// zset 来实现滑动窗口限流
	client := redis.Client()
	pipe := client.Pipeline()

	// 移除窗口开始时间之前的所有请求记录，每次请求会先移除
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

	// 添加当前请求（使用时间戳作为 score 和 member）
	pipe.ZAdd(ctx, key, redislib.Z{
		Score:  float64(now.UnixNano()),
		Member: now.UnixNano(),
	})

	// 获取当前窗口内的请求数
	zcardCmd := pipe.ZCard(ctx, key)

	pipe.Expire(ctx, key, time.Duration(rl.config.Window+10)*time.Second)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	count := int(zcardCmd.Val())
	allowed := count <= rl.config.MaxRequests

	return allowed, count, nil
}


func (rl *RateLimiter) Block(ctx context.Context, c *app.RequestContext) error {
	key := redis.Key(rl.config.KeyPrefix+"block", rl.getKey(ctx, c))
	return redis.Client().Set(ctx, key, "1", time.Duration(rl.config.BlockDuration)*time.Second).Err()
}

func (rl *RateLimiter) IsBlocked(ctx context.Context, c *app.RequestContext) (bool, error) {
	key := redis.Key(rl.config.KeyPrefix+"block", rl.getKey(ctx, c))
	result, err := redis.Client().Exists(ctx, key).Result()
	return result > 0, err
}

// RateLimitMiddleware 创建限流中间件
func RateLimitMiddleware(config RateLimitConfig) app.HandlerFunc {
	limiter := NewRateLimiter(config)

	return func(ctx context.Context, c *app.RequestContext) {
		// 检查是否被阻塞
		blocked, err := limiter.IsBlocked(ctx, c)
		if err != nil {
			logger.Logger.Error("Failed to check block status", zap.Error(err))
			c.AbortWithStatus(consts.StatusInternalServerError)
			return
		}

		if blocked {
			c.AbortWithStatus(consts.StatusTooManyRequests)
			response.Error(ctx, c, &errors.TooManyRequests)
			return
		}

		allowed, count, err := limiter.Allow(ctx, c) 
		if err != nil {
			logger.Logger.Error("Failed to check rate limit", zap.Error(err))
			c.AbortWithStatus(consts.StatusInternalServerError)
			return
		}

		c.Response.Header.Set("X-RateLimit-Limit", strconv.Itoa(config.MaxRequests))
		c.Response.Header.Set("X-RateLimit-Remaining", strconv.Itoa(config.MaxRequests-count))
		c.Response.Header.Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Duration(config.Window)*time.Second).Unix(), 10))

		if !allowed {
			if err := limiter.Block(ctx, c); err != nil {
				logger.Logger.Error("Failed to block user", zap.Error(err))
			}

			c.AbortWithStatus(consts.StatusTooManyRequests)
			response.Error(ctx, c, &errors.TooManyRequests)
			return
		}

		c.Next(ctx)
	}
}

// GeneralRateLimitMiddleware 通用限流中间件（适用于所有需要认证的路由）
func GeneralRateLimitMiddleware() app.HandlerFunc {
	return RateLimitMiddleware(DefaultRateLimitConfig)
}

// UserSettingsRateLimitMiddleware 用户设置修改限流中间件
func UserSettingsRateLimitMiddleware() app.HandlerFunc {
	return RateLimitMiddleware(UserSettingsRateLimitConfig)
}

func JourneySettingsRateLimitMiddleware() app.HandlerFunc {
	return RateLimitMiddleware(JourneySettingRateLimitConfig)
}

// CaptchaRateLimitMiddleware 验证码限流中间件
func CaptchaRateLimitMiddleware() app.HandlerFunc {
	config := RateLimitConfig{
		Window:        60,    // 60秒
		MaxRequests:   5,     // 3次尝试
		KeyPrefix:     "captcha:rate",
		ByUserID:      false, // 按 IP 限流
		ByIP:          true,
		BlockDuration: 1800,  // 阻塞30分钟
		ErrorMessage:  "验证码尝试次数过多，请稍后再试",
	}
	return RateLimitMiddleware(config)
}

// AuthRateLimitMiddleware 认证相关限流（登录、注册等）
func AuthRateLimitMiddleware() app.HandlerFunc {
	config := RateLimitConfig{
		Window:        60,    // 60秒
		MaxRequests:   5,     // 5次尝试
		KeyPrefix:     "auth:rate",
		ByUserID:      false, // 按 IP 限流
		ByIP:          true,
		BlockDuration: 900,   // 阻塞15分钟
		ErrorMessage:  "认证尝试过于频繁，请稍后再试",
	}
	return RateLimitMiddleware(config)
}


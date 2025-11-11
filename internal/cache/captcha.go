package cache

import (
	//"AreYouOK/config" 后续考虑要不要可定义配置过期时间
	"AreYouOK/config"
	"AreYouOK/storage/redis"
	"context"
	"time"

	ri "github.com/redis/go-redis/v9"
)

/*
用户请求发送验证码
    ↓
[Handler] 参数校验（手机号格式、scene）
    ↓
[Service] 发送验证码逻辑
    ├─ 手机号哈希化（用于 Redis key）
    ├─ 检查每日发送次数（Redis）
    ├─ 判断是否需要滑块验证
    ├─ 验证滑块 token（如果需要）
    ├─ 生成验证码（6位数字）
    ├─ 存储验证码到 Redis（60秒TTL）
    ├─ 调用短信服务发送
    └─ 更新发送计数
*/

// 验证码存储：ayok:captcha:{phoneHash}
// TTL: 60秒

// 每日发送计数：ayok:captcha:count:{phoneHash}:{date}
// TTL: 24小时（自动过期）

// 滑块 token：ayok:slider:token:{token}
// TTL: 5分钟

// 滑块验证 token：ayok:slider:verify:{phoneHash}
// TTL: 10分钟（验证通过后生成，用于后续发送验证码）
const (
	cap = "captcha"
)


// SetCaptcha 存储验证码
// Key: ayok:captcha:{phoneHash}
// TTL: 120秒 
// scene: 代表具体的场景，注册还是登录，对应不同的短信模板
 func SetCaptcha(ctx context.Context, phoneHash, scene, code string) error {
	key := redis.Key(cap, phoneHash, scene)
	ttl := time.Duration(config.Cfg.CaptchaExpireSeconds) * time.Second

	return redis.Client().Set(ctx, key, code, ttl).Err()
 }

 func GetCaptcha(ctx context.Context, phoneHash, scene string) (string, error) {
	key := redis.Key(cap, phoneHash,scene)
	return redis.Client().Get(ctx, key).Result()
 }


func DeleteCaptcha(ctx context.Context, phoneHash,scene string) error {
	key := redis.Key(cap, phoneHash, scene)
	return redis.Client().Del(ctx, key).Err()
}

// IncrCaptchaCount 增加今日发送计数，返回当前次数
// Key: ayok:captcha:count:{phoneHash}:{date}
// TTL: 24小时

func IncrCaptchaCount(ctx context.Context, phoneHash string) (int, error) {
	date := time.Now().Format("2006-01-02")
	key := redis.Key(cap, "count", phoneHash, date)

	count, err := redis.Client().Incr(ctx, key).Result()

	if err != nil {
		return 0, err //具体在业务层处理报错
	}

	if count ==1 { //说明这是今天第一次访问，设定第二天才过期
		now := time.Now()
		tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1,0,0,0,0, now.Location())
		ttl := tomorrow.Sub(now)
		redis.Client().Expire(ctx, key, ttl)
	}

	return int(count), nil
}

func GetCaptchaCount(ctx context.Context, phoneHash string) (int, error) {
	date := time.Now().Format("2006-01-02")
	key := redis.Key(cap, "count", phoneHash, date)

	count, err := redis.Client().Get(ctx, key).Int()
	if err == ri.Nil {
		return 0, nil //调用顺序逻辑问题，理论上来讲不可能发生这种情况
	}

	return count, err
}

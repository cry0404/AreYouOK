package cache

import (
	"AreYouOK/storage/redis"
	"context"
	"time"
	"github.com/google/uuid"
)
/*
1. 用户请求发送验证码
   POST /v1/auth/phone/send-captcha
   ↓
2. 后端检查：发送次数 >= 2？
   ↓ 是
3. 返回 429，slider_required: true
   ↓
4. 前端显示第三方滑块组件（极验/腾讯云等）
   用户完成滑块操作
   ↓
5. 前端获取滑块组件的 token（slider_token）
   ↓
6. 前端调用验证接口
   POST /v1/auth/phone/verify-slider
   Body: { "phone": "...", "slider_token": "第三方返回的token" }
   ↓
7. 后端验证 slider_token（调用第三方 API 验证）
   ↓
8. 后端返回 slider_verification_token
   ↓
9. 前端再次调用 send-captcha，携带 slider_verification_token
   POST /v1/auth/phone/send-captcha
   Body: { "phone": "...", "slider_token": "后端返回的verification_token" }
   ↓
10. 后端验证 verification_token，发送验证码
*/

// SetSliderToken 存储滑块 token（前端传入的原始 token）
// Key: ayok:slider:token:{token}
// TTL: 5分钟
const (
	sli = "slider"
)

func SetSliderToken(ctx context.Context, token string) error {
	key := redis.Key(sli, "token", token)
	return redis.Client().Set(ctx, key, "1", 5 * time.Minute).Err()
}

func ValidateSliderToken(ctx context.Context, token string) bool {
    key := redis.Key(sli, "token", token)
    exists, _ := redis.Client().Exists(ctx, key).Result()
    return exists > 0
}

func DeleteSliderToken(ctx context.Context, token string) error {
   key := redis.Key(sli, "token", token)
   return redis.Client().Del(ctx, key).Err()
}

// SetSliderVerificationToken 存储滑块验证通过后的 token
// Key: ayok:slider:verify:{phoneHash}
// TTL: 10分钟（用于后续发送验证码时验证）
func SetSliderVerificationToken(ctx context.Context, phoneHash string) (string, error) {
    token := generateRandomToken() // 生成随机 token
    key := redis.Key(sli, "verify", phoneHash)
    err := redis.Client().Set(ctx, key, token, 10*time.Minute).Err()
    return token, err
}

// ValidateSliderVerificationToken 验证滑块验证 token
func ValidateSliderVerificationToken(ctx context.Context, phoneHash, token string) bool {
    key := redis.Key(sli, "verify", phoneHash)
    storedToken, err := redis.Client().Get(ctx, key).Result()
    if err != nil {
        return false
    }
    return storedToken == token
}


func generateRandomToken() string {
	return uuid.New().String()
}
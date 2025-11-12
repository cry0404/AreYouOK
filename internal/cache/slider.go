package cache

import (
	"AreYouOK/storage/redis"
	"context"
	"time"

	"github.com/google/uuid"
)

/*
滑块验证流程：

1. 用户请求发送验证码
   POST /v1/auth/phone/send-captcha
   ↓
2. 后端检查：发送次数 >= CaptchaSliderThreshold？
   ↓ 是
3. 返回 429，slider_required: true
   ↓
4. 前端显示阿里云滑块组件
   用户完成滑块操作
   ↓
5. 前端获取滑块组件的 captchaVerifyToken
   ↓
6. 前端调用验证接口
   POST /v1/auth/phone/verify-slider
   Body: { "phone": "...", "slider_token": "阿里云返回的captchaVerifyToken" }
   ↓
7. 后端使用阿里云 SDK 验证 captchaVerifyToken
   ↓
8. 验证成功后，后端生成 slider_verification_token 并存储到 Redis
   返回 slider_verification_token 给前端
   ↓
9. 前端再次调用 send-captcha，携带 slider_verification_token
   POST /v1/auth/phone/send-captcha
   Body: { "phone": "...", "slider_token": "后端返回的slider_verification_token" }
   ↓
10. 后端验证 slider_verification_token（从 Redis 读取），发送验证码

*/

const (
	sli = "slider"
)

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

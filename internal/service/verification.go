package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"AreYouOK/config"
	"AreYouOK/internal/cache"
	pkgerrors "AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/slider"
	"AreYouOK/pkg/sms"
	"AreYouOK/utils"
)

// 单例模式参考，记住了
var (
	verificationService *VerificationService
	verifyOnce          sync.Once
)

func Verification() *VerificationService {
	verifyOnce.Do(func() {
		verificationService = &VerificationService{
			// 这里还得初始化 sms 服务
		}
	})

	return verificationService
}

type VerificationService struct {
	// 可以考虑在这里接入别的服务， sms 部分、 生成滑块部分
}

func generateCaptchaCode() string {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func (s *VerificationService) SendCaptcha(
	ctx context.Context,
	phone string,
	sceneid string,
	captchaVerifyParam string,
) error {
	phoneHash := utils.HashPhone(phone)

	count, err := cache.IncrCaptchaCount(ctx, phoneHash)
	if err != nil {
		return fmt.Errorf("failed to check captcha count: %w", err)
	}

	if count > config.Cfg.CaptchaMaxDaily {
		return pkgerrors.CaptchaRateLimited
	}

	needSlider := count > config.Cfg.CaptchaSliderThreshold

	if needSlider {
		if captchaVerifyParam == "" {
			return pkgerrors.VerificationSliderRequired
		} // 这个地方还需要考虑前端那边需不需要重复统计，不然默认超过两次后还是会请求这个节点一次, 可以考虑返回后再是否加载

		if !cache.ValidateSliderVerificationToken(ctx, phoneHash, captchaVerifyParam) {
			return pkgerrors.VerificationSliderFailed
		}
	}

	code := generateCaptchaCode()

	if err := cache.SetCaptcha(ctx, phoneHash, code); err != nil {
		return fmt.Errorf("failed to store captcha: %w", err)
	}

	// 发送短信验证码
	// 短信发送失败，验证码也已经存储，用户仍可以通过其他方式获取, 考虑提供一个服务层的服务发送失败时删除验证码，然后返回给前端错误

	if err := sms.SendCaptchaSMS(ctx, phone, code); err != nil {
		cache.DeleteCaptcha(ctx, phoneHash)
		logger.Logger.Error("Failed to send captcha SMS",
			zap.String("phone", phone),
			zap.Error(err),
		)

		if config.Cfg.IsDevelopment() {
			return fmt.Errorf("failed to send SMS: %w", err)
		}
	}

	return nil
}

func (s *VerificationService) VerifySlider(
	ctx context.Context,
	phone string,
	sceneid string,
	captchaVerifyParam string,

) (string, time.Time, error) {
	phoneHash := utils.HashPhone(phone)

	// 使用阿里云验证码 SDK 验证滑块 token
	// sliderToken 是前端滑块组件返回的 captchaVerifyToken
	if sceneid != config.Cfg.CaptchaSceneId {
		return "场景 id 篡改", time.Time{}, pkgerrors.VerificationSliderFailed
	}

	valid, err := slider.Verify(ctx, captchaVerifyParam, sceneid)
	if err != nil {
		logger.Logger.Error("Failed to verify slider token",
			zap.String("phone", phone),
			zap.Error(err),
		)
		return "", time.Time{}, pkgerrors.VerificationSliderFailed
	}

	if !valid {
		logger.Logger.Warn("Slider verification failed",
			zap.String("phone", phone),
		)
		return "", time.Time{}, pkgerrors.VerificationSliderFailed
	}

	// 验证成功后，生成并存储验证 token
	verifyToken, err := cache.SetSliderVerificationToken(ctx, phoneHash)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate verify token: %w", err)
	}

	expiresAt := time.Now().Add(5 * time.Minute)
	// 持有 verifyToken 后即可持续请求
	return verifyToken, expiresAt, nil
}

// VerifyCaptcha 验证验证码
// 流程：
// 1. 从 Redis 获取验证码
// 2. 比较验证码
// 3. 验证成功后删除验证码
// 4. 返回验证结果

func (s *VerificationService) VerifyCaptcha(
	ctx context.Context,
	phone string,
	scene string,
	code string,
) error {
	phoneHash := utils.HashPhone(phone)

	storedCode, err := cache.GetCaptcha(ctx, phoneHash)

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return pkgerrors.VerificationCodeExpired
		}
		return fmt.Errorf("failed to get captcha: %w", err)
	}

	if storedCode != code {
		return pkgerrors.VerificationCodeInvalid
	}

	cache.DeleteCaptcha(ctx, phoneHash)
	// 验证成功后删除
	return nil
}

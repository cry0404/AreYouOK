package service

import (
	"AreYouOK/config"
	"AreYouOK/internal/cache"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/slider"
	"AreYouOK/pkg/sms"
	"AreYouOK/utils"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 单例模式参考，记住了
var (
	verificationService *VerificationService
	verifyOnce          sync.Once
)

func Verification() *VerificationService {
	verifyOnce.Do(func() {
		verificationService = &VerificationService{
			//这里还得初始化 sms 服务
		}
	})

	return verificationService
}

type VerificationService struct {
	//可以考虑在这里接入别的服务， sms 部分、 生成滑块部分
}

func generateCaptchaCode() string {
	max := big.NewInt(1000000)
	n, _ := rand.Int(rand.Reader, max)
	return fmt.Sprintf("%06d", n.Int64())
}

func (s *VerificationService) SendCaptcha(
	ctx context.Context,
	phone string,
	scene string,
	sliderToken string,
) error {
	phoneHash := utils.HashPhone(phone)

	count, err := cache.IncrCaptchaCount(ctx, phoneHash)
	if err != nil {
		return fmt.Errorf("failed to check captcha count: %w", err)
	}

	if count > config.Cfg.CaptchaMaxDaily {
		return errors.CaptchaRateLimited
	}

	needSlider := count > config.Cfg.CaptchaSliderThreshold

	if needSlider {
		if sliderToken == "" {
			return errors.VerificationSliderRequired
		} //这个地方还需要考虑前端那边需不需要重复统计，不然默认超过两次后还是会请求这个节点一次, 可以考虑返回后再是否加载

		if !cache.ValidateSliderVerificationToken(ctx, phoneHash, sliderToken) {
			return errors.VerificationSliderFailed
		}
	}

	code := generateCaptchaCode()

	if err := cache.SetCaptcha(ctx, phoneHash, scene, code); err != nil {
		return fmt.Errorf("failed to store captcha: %w", err)
	}

	// 发送短信验证码
	// 短信发送失败，验证码也已经存储，用户仍可以通过其他方式获取, 考虑提供一个服务层的服务发送失败时删除验证码，然后返回给前端错误

	if err := sms.SendCaptchaSMS(ctx, phone, code, scene); err != nil {
		cache.DeleteCaptcha(ctx, phoneHash, scene)
		logger.Logger.Error("Failed to send captcha SMS",
			zap.String("phone", phone),
			zap.String("scene", scene),
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
	sliderToken string,
	remoteIp string,
) (string, time.Time, error) {
	phoneHash := utils.HashPhone(phone)

	// 使用阿里云验证码 SDK 验证滑块 token
	// sliderToken 是前端滑块组件返回的 captchaVerifyToken
	scene := "default" // 可以根据业务场景调整
	valid, err := slider.Verify(ctx, sliderToken, remoteIp, scene)
	if err != nil {
		logger.Logger.Error("Failed to verify slider token",
			zap.String("phone", phone),
			zap.Error(err),
		)
		return "", time.Time{}, errors.VerificationSliderFailed
	}

	if !valid {
		logger.Logger.Warn("Slider verification failed",
			zap.String("phone", phone),
		)
		return "", time.Time{}, errors.VerificationSliderFailed
	}

	// 验证成功后，生成并存储验证 token
	verifyToken, err := cache.SetSliderVerificationToken(ctx, phoneHash)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate verify token: %w", err)
	}

	expiresAt := time.Now().Add(5 * time.Minute)
	//持有 verifyToken 后即可持续请求
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

	storedCode, err := cache.GetCaptcha(ctx, phoneHash, scene)

	if err != nil {
		if err == redis.Nil {
			return errors.VerificationCodeExpired
		}
		return fmt.Errorf("failed to get captcha: %w", err)
	}

	if storedCode != code {
		return errors.VerificationCodeInvalid
	}

	cache.DeleteCaptcha(ctx, phoneHash, scene)
	//验证成功后删除
	return nil
}

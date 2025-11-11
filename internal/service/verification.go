package service

import (
	"AreYouOK/config"
	"AreYouOK/internal/cache"
	"AreYouOK/pkg/errors"
	"AreYouOK/utils"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
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

	//TODO: 短信发送服务
	//根据对应的 scene 来选择发送的短信模板
	//对应的模板，发送的报错，是否发送成功，届时可以硬编码做测试
	return nil
}

func (s *VerificationService) VerifySlider(
	ctx context.Context,
	phone string,
	sliderToken string,
) (string, time.Time, error) {
	phoneHash := utils.HashPhone(phone)

	if !cache.ValidateSliderToken(ctx, sliderToken) {
		return "", time.Time{}, errors.VerificationSliderFailed
	}

	cache.DeleteSliderToken(ctx, sliderToken) //删除对应 token 防止复用

	verifyToken, err := cache.SetSliderVerificationToken(ctx, phoneHash)

	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate verify token: %w", err)
	}

	expiresAt := time.Now().Add(5 * time.Minute) //

	return verifyToken, expiresAt, nil
	//持有 verifyToken 后即可持续请求

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

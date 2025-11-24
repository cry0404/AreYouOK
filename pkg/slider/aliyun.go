package slider

import (
	"context"
	"fmt"

	captcha "github.com/alibabacloud-go/captcha-20230305/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	credential "github.com/aliyun/credentials-go/credentials"
	"go.uber.org/zap"

	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
)

// AliyunClient 阿里云验证码客户端实现
type AliyunClient struct {
	client *captcha.Client
}

func NewAliyunClient() (*AliyunClient, error) {
	cred, err := credential.NewCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create aliyun credential: %w", err)
	}

	clientConfig := &openapi.Config{
		Credential: cred,
		Endpoint:   tea.String("captcha.cn-shanghai.aliyuncs.com"),
	}

	client, err := captcha.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create captcha client: %w", err)
	}

	return &AliyunClient{
		client: client,
	}, nil
}

// Verify 验证滑块 token
// 根据阿里云验证码 API 文档：https://next.api.aliyun.com/api-tools/sdk/captcha
// captchaVerifyToken: 前端滑块组件返回的验证 token（CaptchaVerifyParam）
// remoteIp: 用户 IP 地址（当前版本 API 不需要此参数）， 暂时只是用于日志记录
// scene: 验证场景（SceneId）
func (c *AliyunClient) Verify(ctx context.Context, captchaVerifyParam, sceneid string) (bool, error) {
	if captchaVerifyParam == "" {
		return false, errors.ErrCaptchaTokenRequired
	}

	logger.Logger.Info("Verifying captcha",
		zap.String("sceneid", sceneid),
		zap.Int("captcha_param_length", len(captchaVerifyParam)),
		zap.String("captcha_param_preview", getParamPreview(captchaVerifyParam)),
	)

	request := &captcha.VerifyIntelligentCaptchaRequest{
		CaptchaVerifyParam: tea.String(captchaVerifyParam),
		SceneId:            tea.String(sceneid),
	}

	response, err := c.client.VerifyIntelligentCaptcha(request)
	if err != nil {
		logger.Logger.Error("Failed to verify captcha",
			zap.String("sceneid", sceneid),
			zap.Error(err),
		)
		return false, fmt.Errorf("failed to verify captcha: %w", err)
	}

	if response == nil || response.Body == nil {
		logger.Logger.Error("Captcha response is nil",
			zap.String("sceneid", sceneid),
		)
		return false, errors.ErrCaptchaResponseNil
	}

	body := response.Body

	// 记录完整的响应信息（用于调试）- 使用强制输出确保能看到
	logger.Logger.Info("=== Captcha verification response ===",
		zap.String("sceneid", sceneid),
	)

	// 分别记录各个字段，避免 Any 类型序列化问题
	if body.Code != nil {
		logger.Logger.Info("Response code", zap.String("code", *body.Code))
	}
	if body.Message != nil {
		logger.Logger.Info("Response message", zap.String("message", *body.Message))
	}
	if body.Result != nil {
		logger.Logger.Info("Response result exists", zap.Bool("result_not_nil", true))
		if body.Result.VerifyResult != nil {
			logger.Logger.Info("VerifyResult value", zap.Bool("verify_result", *body.Result.VerifyResult))
		} else {
			logger.Logger.Warn("VerifyResult is nil")
		}
		// 记录 VerifyCode 字段（如果存在）
		if body.Result.VerifyCode != nil {
			logger.Logger.Info("VerifyCode value", zap.String("verify_code", *body.Result.VerifyCode))
		} else {
			logger.Logger.Warn("VerifyCode is nil")
		}
	} else {
		logger.Logger.Warn("Response result is nil")
	}

	if body.Result != nil {
		verifyResult := false
		verifyCode := ""

		if body.Result.VerifyResult != nil {
			verifyResult = *body.Result.VerifyResult
		}
		if body.Result.VerifyCode != nil {
			verifyCode = *body.Result.VerifyCode
		}

		logger.Logger.Info("Checking verification result",
			zap.Bool("verify_result", verifyResult),
			zap.String("verify_code", verifyCode),
			zap.String("sceneid", sceneid),
		)

		// 优先检查 VerifyResult，如果为 true 则认为验证成功
		// T001 才是成功
		if verifyResult {
			// VerifyResult 为 true，验证成功
			logger.Logger.Info("Captcha verification successful",
				zap.String("verify_code", verifyCode),
				zap.Bool("verify_result", verifyResult),
				zap.String("sceneid", sceneid),
			)
			return true, nil
		} else if verifyCode == "T001" {
			// VerifyCode 为成功代码，即使 VerifyResult 为 false 也认为成功（兼容处理）
			logger.Logger.Info("Captcha verification successful by code",
				zap.String("verify_code", verifyCode),
				zap.Bool("verify_result", verifyResult),
				zap.String("sceneid", sceneid),
			)
			return true, nil
		} else {
			// 验证失败，记录详细信息
			logger.Logger.Warn("Captcha verification failed",
				zap.String("verify_code", verifyCode),
				zap.Bool("verify_result", verifyResult),
				zap.String("sceneid", sceneid),
				zap.Any("code", body.Code),
				zap.Any("message", body.Message),
				zap.String("captcha_param_preview", getParamPreview(captchaVerifyParam)),
				zap.Int("captcha_param_length", len(captchaVerifyParam)),
			)

			return false, errors.ErrCaptchaVerificationFailed
		}
	}

	logger.Logger.Warn("Captcha verification failed: no result field",
		zap.String("sceneid", sceneid),
		zap.Any("body", body),
	)
	return false, errors.ErrCaptchaVerificationFailed
}

// getParamPreview 获取参数预览（用于日志，避免泄露完整 token）
func getParamPreview(param string) string {
	if len(param) <= 50 {
		return param
	}
	return param[:50] + "..."
}

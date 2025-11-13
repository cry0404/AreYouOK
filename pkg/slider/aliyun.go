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
		Endpoint:   tea.String("captcha.cn-hangzhou.aliyuncs.com"),
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
func (c *AliyunClient) Verify(ctx context.Context, captchaVerifyToken, remoteIp, scene string) (bool, error) {
	if captchaVerifyToken == "" {
		return false, errors.ErrCaptchaTokenRequired
	}

	// 直接使用前端返回的 token 作为 CaptchaVerifyParam
	// 前端滑块组件返回的 token 已经包含了所有必要的验证信息（包括 appkey、verifyToken 等）
	request := &captcha.VerifyIntelligentCaptchaRequest{
		CaptchaVerifyParam: tea.String(captchaVerifyToken),
		SceneId:            tea.String(scene),
	}

	response, err := c.client.VerifyIntelligentCaptcha(request)
	if err != nil {
		logger.Logger.Error("Failed to verify captcha",
			zap.String("scene", scene),
			zap.String("remoteIp", remoteIp),
			zap.Error(err),
		)
		return false, fmt.Errorf("failed to verify captcha: %w", err)
	}

	// 检查响应
	if response == nil || response.Body == nil {
		return false, errors.ErrCaptchaResponseNil
	}

	body := response.Body

	// 检查验证结果
	if body.Result != nil && body.Result.VerifyResult != nil && *body.Result.VerifyResult {
		logger.Logger.Info("Captcha verification successful",
			zap.String("scene", scene),
			zap.String("remoteIp", remoteIp),
		)
		return true, nil
	}

	// 检查错误码
	if body.Code != nil && *body.Code != "200" {
		message := ""
		if body.Message != nil {
			message = *body.Message
		}
		logger.Logger.Warn("Captcha verification failed",
			zap.String("code", *body.Code),
			zap.String("message", message),
			zap.String("scene", scene),
		)
		return false, fmt.Errorf("%w: %s - %s", errors.ErrCaptchaVerificationFailed, *body.Code, message)
	}

	logger.Logger.Warn("Captcha verification failed: verify result is false",
		zap.String("scene", scene),
	)
	return false, errors.ErrCaptchaVerificationFailed
}

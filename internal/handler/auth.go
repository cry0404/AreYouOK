package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	//"AreYouOK/internal/middleware"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/service"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/response"
	"AreYouOK/utils"
)

// TODO：可能还需要一个 get 当前 status 的部分，来给前端告诉内测名额还剩余多少

// ExchangeAlipayAuth 支付宝授权换取， 这里可以直接绑定
// POST /v1/auth/miniapp/alipay/exchange
// uid 是 是 public_id, 也就是生成的雪花 id
func ExchangeAlipayAuth(ctx context.Context, c *app.RequestContext) {
	var req dto.AuthExchangeRequest

	if err := c.BindAndValidate(&req); err != nil {
		response.BindError(ctx, c, err)
		// 这里是对于 open_id 部分
		return // 判断 binding "required" 是否填写
	}
	var (
		openid string
		err    error
	)
	if req.AlipayOpenID == "" {
		openid, err = utils.ExchangeAlipayAuthCode(ctx, req.AuthCode)
		if err != nil {
			response.Error(ctx, c, errors.Definition{
				Code:    "CANNOT_GET_OPENID",
				Message: "cannot get open_id, please provide valid auth_code",
			})
			return
		}
	} else {
		openid = req.AlipayOpenID
	}

	if openid == "" {
		response.Error(ctx, c, errors.Definition{
			Code:    "CANNOT_GET_OPENID",
			Message: "open_id is required and cannot be empty",
		})
		return
	}

	encryptedData := req.EncryptedData
	if encryptedData == "" {
		if req.Response == "" {
			response.Error(ctx, c, errors.Definition{
				Code:    "INVALID_REQUEST",
				Message: "missing encrypted data (need encrypted_data or response/sign)",
			})
			return
		}
		if req.Sign == "" {
			response.Error(ctx, c, errors.Definition{
				Code:    "INVALID_REQUEST",
				Message: "sign is required when using response format",
			})
			return
		}

		alipayResp := map[string]string{
			"response":     req.Response,
			"sign":         req.Sign,
			"sign_type":    strings.ToUpper(defaultString(req.SignType, "RSA2")),
			"encrypt_type": strings.ToUpper(defaultString(req.EncryptType, "AES")),
			"charset":      defaultString(req.Charset, "UTF-8"),
		}

		jsonBytes, err := json.Marshal(alipayResp)
		if err != nil {
			response.Error(ctx, c, errors.Definition{
				Code:    "INVALID_REQUEST",
				Message: fmt.Sprintf("Failed to marshal alipay response: %v", err),
			})
			return
		}
		encryptedData = string(jsonBytes)
	}

	authService := service.Auth()
	//到注册这里是一定有 openid 的
	result, err := authService.ExchangeAlipayAuthCode(ctx, encryptedData, req.Device, openid)

	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
} // 先去完善存储层

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// SendCaptcha 发送验证码
// POST /v1/auth/phone/send-captcha
func SendCaptcha(ctx context.Context, c *app.RequestContext) {
	var req dto.SendCaptchaRequest

	if err := c.BindAndValidate(&req); err != nil {
		response.BindError(ctx, c, err)
		return
	}

	if !utils.ValidatePhone(req.Phone) {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_PHONE",
			Message: "Invalid phone number format",
		})
		return
	}

	verifiService := service.Verification()
	if err := verifiService.SendCaptcha(ctx, req.Phone, req.SceneId, req.VerifyToken); err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, map[string]interface{}{
		"message": "Captcha sent successfully",
	})
}

// VerifySlider 滑块验证， 滑块验证不需要区分对应的场景
// POST /v1/auth/phone/verify-slider

// encrypted_data 在这个部分验证
func VerifySlider(ctx context.Context, c *app.RequestContext) {
	var req dto.VerifySliderRequest

	if err := c.BindAndValidate(&req); err != nil {
		response.BindError(ctx, c, err)
		return
	}

	if !utils.ValidatePhone(req.Phone) {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_PHONE",
			Message: "Invalid phone number format",
		})
		return
	}

	verifiService := service.Verification()
	token, expiresAt, err := verifiService.VerifySlider(ctx, req.Phone, req.SceneId, req.CaptchaVerifyParam)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, dto.VerifySliderResponse{
		SliderVerificationToken: token,
		ExpiresAt:               expiresAt,
	})
} // slider 后还是需要继续验证码验证的

// VerifyCaptcha 验证码验证，支持两种场景：
// 1. 无需登录：手机号+验证码直接注册/登录
// 2. 已登录：绑定/更换手机号
// POST /v1/auth/phone/verify
func VerifyCaptcha(ctx context.Context, c *app.RequestContext) {
	var req dto.VerifyCaptchaRequest

	if err := c.BindAndValidate(&req); err != nil {
		response.BindError(ctx, c, err)
		return
	}
	

	openid, e := utils.ExchangeAlipayAuthCode(ctx, req.AuthCode)
		logger.Logger.Info("成功获取到 openid", zap.String("open id 的值:", openid))
		if e != nil || openid == "" {
			response.Error(ctx, c, errors.Definition{
				Code:    "CANNOT_GET_OPENID",
				Message: "cannot get open_id, please provide valid auth_code",
			})
			return
		}

	if !utils.ValidatePhone(req.Phone) {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_PHONE",
			Message: "Invalid phone number format",
		})
		return
	}

	authService := service.Auth()

	var result *dto.VerifyCaptchaResponse
	var err error

	result, err = authService.VerifyPhoneCaptchaAndLogin(ctx, req.Phone, req.VerifyCode, openid)

	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}

// RefreshToken 刷新访问令牌
// POST /v1/auth/token/refresh
func RefreshToken(ctx context.Context, c *app.RequestContext) {
	var req dto.RefreshTokenRequest

	if err := c.BindAndValidate(&req); err != nil {
		response.BindError(ctx, c, err)
		return
	}

	authService := service.Auth()
	result, err := authService.RefreshToken(ctx, req.RefreshToken)

	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}

// 不再需要
// // GetWaitlistStatus 查询内测排队状态
// // GET /v1/auth/waitlist/status
func GetWaitlistStatus(ctx context.Context, c *app.RequestContext) {
	userService := service.User()
	result, err := userService.GetWaitlistStatus(ctx)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}

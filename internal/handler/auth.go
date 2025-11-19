package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"

	"AreYouOK/internal/middleware"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/service"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/response"
	"AreYouOK/utils"
)

// ExchangeAlipayAuth 支付宝授权换取， 这里可以直接绑定
// POST /v1/auth/miniapp/alipay/exchange
// uid 是 是 public_id, 也就是生成的雪花 id
func ExchangeAlipayAuth(ctx context.Context, c *app.RequestContext) {
	var req dto.AuthExchangeRequest

	if err := c.BindAndValidate(&req); err != nil {
		response.BindError(ctx, c, err)
		return // 判断 binding “required” 是否填写
	}

	authService := service.Auth()
	result, err := authService.ExchangeAlipayAuthCode(ctx, req.EncryptedData, req.IV, req.Device)

	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
} // 先去完善存储层


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
	if err := verifiService.SendCaptcha(ctx, req.Phone, req.SceneId, req.CaptchaVerifyParam); err != nil {
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

// VerifyCaptcha 验证码验证， 这里也是一个可以直接实现的部分，不需要验证码
// POST /v1/auth/phone/verify
func VerifyCaptcha(ctx context.Context, c *app.RequestContext) {
	var req dto.VerifyCaptchaRequest

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

	userID, ok := middleware.GetUserID(ctx, c)
	if !ok {
		c.JSON(500, response.ErrorResponse{
			Error: response.ErrorDetail{
				Code:    "INTERANL_ERROR",
				Message: "Server can not find userID in context",
			}})

		return
	}
	// 并入到了 auth 当中去了
	// verifiService := service.Verification()
	// if err := verifiService.VerifyCaptcha(ctx, req.Phone, req.Scene, req.VerifyCode); err != nil {
	// 	response.Error(ctx, c, err)
	// 	return
	// }

	authService := service.Auth()
	result, err := authService.VerifyPhoneCaptchaAndBind(ctx, userID, req.Phone, req.VerifyCode, req.SceneId)

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
// func GetWaitlistStatus(ctx context.Context, c *app.RequestContext) {
// 	// TODO: 实现查询排队状态逻辑
// }

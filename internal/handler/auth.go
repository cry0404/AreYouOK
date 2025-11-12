package handler

import (
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/service"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/response"
	"AreYouOK/utils"
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)



// ExchangeAlipayAuth 支付宝授权换取
// POST /v1/auth/miniapp/alipay/exchange
// uid 是 user_id
func ExchangeAlipayAuth(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现支付宝授权换取逻辑
} //先去完善存储层


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

	//调用对应的认证 service
	verifiService := service.Verification()
	if err := verifiService.SendCaptcha(ctx, req.Phone, req.Scene, req.SliderToken); err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, map[string]interface{}{
		"message": "Captcha sent successfully",
	})

}

// VerifySlider 滑块验证， 滑块验证不需要区分对应的场景
// POST /v1/auth/phone/verify-slider
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

	// 获取客户端 IP
	remoteIp := c.ClientIP()
	if remoteIp == "" {
		remoteIp = c.RemoteAddr().String()
	}

	verifiService := service.Verification()
	token, expiresAt, err := verifiService.VerifySlider(ctx, req.Phone, req.SliderToken, remoteIp)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, dto.VerifySliderResponse{
		SliderVerificationToken: token,
		ExpiresAt:               expiresAt,
	})
}

// VerifyCaptcha 验证码验证
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

	verifiService := service.Verification()
	if err := verifiService.VerifyCaptcha(ctx, req.Phone, req.Scene, req.VerifyCode); err != nil {
		response.Error(ctx, c, err)
		return
	}

	// TODO: 验证成功后，调用 auth service 绑定手机号并返回 JWT
	// authService := service.NewAuthService()
	// result, err := authService.BindPhoneAndLogin(ctx, req.Phone)
	// if err != nil {
	//     response.Error(ctx, c, err)
	//     return
	// }
	// response.Success(ctx, c, result)

	// 临时响应（验证成功但还未实现登录逻辑）
	response.Success(ctx, c, map[string]interface{}{
		"message": "Captcha verified successfully",
	})
}

// RefreshToken 刷新访问令牌
// POST /v1/auth/token/refresh
func RefreshToken(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现刷新 token 逻辑
}

// GetWaitlistStatus 查询内测排队状态
// GET /v1/auth/waitlist/status
func GetWaitlistStatus(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现查询排队状态逻辑
}


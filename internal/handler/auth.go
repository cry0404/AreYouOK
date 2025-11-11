package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

// ExchangeAlipayAuth 支付宝授权换取
// POST /v1/auth/miniapp/alipay/exchange
func ExchangeAlipayAuth(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现支付宝授权换取逻辑
}

// SendCaptcha 发送验证码
// POST /v1/auth/phone/send-captcha
func SendCaptcha(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现发送验证码逻辑
}

// VerifySlider 滑块验证
// POST /v1/auth/phone/verify-slider
func VerifySlider(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现滑块验证逻辑
}

// VerifyCaptcha 验证码验证
// POST /v1/auth/phone/verify
func VerifyCaptcha(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现验证码验证逻辑
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


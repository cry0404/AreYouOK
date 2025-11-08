package handler

import (
	"AreYouOK/internal/model"
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// ExchangeAlipay 处理支付宝身份换取请求。
func ExchangeAlipay(ctx context.Context, c *app.RequestContext) {
	var req model.ExchangeAlipayRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PAYLOAD", "Invalid payload", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// SendPhoneCaptcha 处理发送短信验证码请求。
func SendPhoneCaptcha(ctx context.Context, c *app.RequestContext) {
	var req model.SendCaptchaRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PAYLOAD", "Invalid payload", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// VerifySlider 处理滑块验证请求。
func VerifySlider(ctx context.Context, c *app.RequestContext) {
	var req model.VerifySliderRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PAYLOAD", "Invalid payload", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// VerifyPhone 处理手机号验证码验证请求。
func VerifyPhone(ctx context.Context, c *app.RequestContext) {
	var req model.VerifyPhoneRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PAYLOAD", "Invalid payload", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// RefreshToken 处理访问令牌刷新请求。
func RefreshToken(ctx context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// GetWaitlistStatus 查询内测排队状态。
func GetWaitlistStatus(ctx context.Context, c *app.RequestContext) {
	var req model.WaitlistStatusRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_QUERY", "Invalid query", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

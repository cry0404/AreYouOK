package dto

import "time"

// ========== Auth 相关 DTO ==========

// AuthExchangeRequest 支付宝授权换取请求
type AuthExchangeRequest struct {
	AuthCode string     `json:"auth_code" binding:"required"`
	Device   DeviceInfo `json:"device"`
}

// DeviceInfo 设备信息
type DeviceInfo struct {
	Platform   string `json:"platform"`
	Model      string `json:"model"`
	AppVersion string `json:"app_version"`
}

// AuthExchangeResponse 支付宝授权换取响应
type AuthExchangeResponse struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	ExpiresIn    int              `json:"expires_in"`
	User         AuthUserSnapshot `json:"user"`
}

// AuthUserSnapshot 授权时的用户快照
type AuthUserSnapshot struct {
	ID            string       `json:"id"`
	Nickname      string       `json:"nickname"`
	Status        string       `json:"status"`
	PhoneVerified bool         `json:"phone_verified"`
	IsNewUser     bool         `json:"is_new_user"`
	Waitlist      WaitlistInfo `json:"waitlist"`
}

// WaitlistInfo 内测排队信息
type WaitlistInfo struct {
	Priority    int        `json:"priority"`
	Position    int        `json:"position,omitempty"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
}

// SendCaptchaRequest 发送验证码请求
type SendCaptchaRequest struct {
	Phone       string `json:"phone" binding:"required"`
	Scene       string `json:"scene" binding:"required"` //场景，是基于登录还是注册时，可以切换不同的短信模板
	SliderToken string `json:"slider_token,omitempty"`
}

// VerifySliderRequest 滑块验证请求
type VerifySliderRequest struct {
	Phone       string `json:"phone" binding:"required"`
	SliderToken string `json:"slider_token" binding:"required"`
}

// VerifySliderResponse 滑块验证响应
type VerifySliderResponse struct {
	SliderVerificationToken string    `json:"slider_verification_token"`
	ExpiresAt               time.Time `json:"expires_at"`
}

// VerifyCaptchaRequest 验证码验证请求
type VerifyCaptchaRequest struct {
	Phone       string `json:"phone" binding:"required"`
	VerifyCode  string `json:"verify_code" binding:"required"`
	Scene       string `json:"scene" binding:"required"` 
	SliderToken string `json:"slider_token,omitempty"`
}

// VerifyCaptchaResponse 验证码验证响应
// 验证成功后返回新的 JWT token 和用户信息
type VerifyCaptchaResponse struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	ExpiresIn    int              `json:"expires_in"`
	User         AuthUserSnapshot `json:"user"`
}

// RefreshTokenRequest 刷新 token 请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// WaitlistStatusResponse 内测排队状态响应
type WaitlistStatusResponse struct {
	Status      string     `json:"status"`
	Priority    int        `json:"priority"`
	Position    int        `json:"position,omitempty"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
}

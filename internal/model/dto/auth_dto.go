package dto

import "time"

// ========== Auth 相关 DTO ==========
type AuthExchangeRequest struct {

	EncryptedData string `json:"encrypted_data" binding:"required"`



	Response    string `json:"response,omitempty"`     // AES 加密的数据（base64 编码）
	Sign        string `json:"sign,omitempty"`         // RSA2 签名（base64 编码）
	SignType    string `json:"sign_type,omitempty"`    // 签名算法，通常为 "RSA2"
	EncryptType string `json:"encrypt_type,omitempty"` // 加密算法，通常为 "AES"
	Charset     string `json:"charset,omitempty"`      // 字符集，通常为 "UTF-8"


	IV     string     `json:"iv,omitempty"`
	Device DeviceInfo `json:"device"`
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
	User         AuthUserSnapshot `json:"user"`
	ExpiresIn    int              `json:"expires_in"`
}

// AuthUserSnapshot 授权时的用户快照
type AuthUserSnapshot struct {
	ID            string `json:"id"`
	Nickname      string `json:"nickname"`
	Status        string `json:"status"`
	PhoneVerified bool   `json:"phone_verified"` // 据此来判断 status
	IsNewUser     bool   `json:"is_new_user"`
	// Waitlist      WaitlistInfo `json:"waitlist"` //不再需要，直接返回
}

// // WaitlistInfo 内测排队信息
// type WaitlistInfo struct {
// 	Priority    int        `json:"priority"`
// 	Position    int        `json:"position,omitempty"`
// 	ActivatedAt *time.Time `json:"activated_at,omitempty"`
// }

// SendCaptchaRequest 发送验证码请求
type SendCaptchaRequest struct {
	Phone       string `json:"phone" binding:"required"`
	SceneId     string `json:"scene_id" binding:"required"` // 唯一 id
	VerifyToken string `json:"verify_token,omitempty"`      // 滑块验证参数（超过阈值时需要）
}

// VerifySliderRequest 滑块验证请求
type VerifySliderRequest struct {
	Phone              string `json:"phone" binding:"required"`
	CaptchaVerifyParam string `json:"captcha_verify_param" binding:"required"`
	SceneId            string `json:"scene_id" binding:"required"`
}

// VerifySliderResponse 滑块验证响应
type VerifySliderResponse struct {
	ExpiresAt               time.Time `json:"expires_at"`
	SliderVerificationToken string    `json:"slider_verification_token"`
}

// VerifyCaptchaRequest 验证码验证请求
type VerifyCaptchaRequest struct {
	Phone      string `json:"phone" binding:"required"`
	VerifyCode string `json:"verify_code" binding:"required"`
}

// VerifyCaptchaResponse 验证码验证响应
// 验证成功后返回新的 JWT token 和用户信息
type VerifyCaptchaResponse struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	User         AuthUserSnapshot `json:"user"`
	ExpiresIn    int              `json:"expires_in"`
}

// RefreshTokenRequest 刷新 token 请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// WaitlistStatusResponse 内测排队状态响应
type WaitlistStatusResponse struct {
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
	Status      string     `json:"status"`
	Priority    int        `json:"priority"`
	Position    int        `json:"position,omitempty"`
}

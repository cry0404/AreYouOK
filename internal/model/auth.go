package model

// AuthDeviceInfo 表示从小程序端上报的设备信息。
type AuthDeviceInfo struct {
	Platform   string `json:"platform"`
	Model      string `json:"model"`
	AppVersion string `json:"app_version"`
}

// ExchangeAlipayRequest 表示支付宝换取用户身份的请求体。
type ExchangeAlipayRequest struct {
	AuthCode string         `json:"auth_code"`
	Device   AuthDeviceInfo `json:"device"`
}

// AuthUserSnapshot 表示授权后返回的用户概览信息。
type AuthUserSnapshot struct {
	ID            string       `json:"id"`
	Nickname      string       `json:"nickname"`
	Status        string       `json:"status"`
	PhoneVerified bool         `json:"phone_verified"`
	IsNewUser     bool         `json:"is_new_user"`
	Waitlist      WaitlistInfo `json:"waitlist"`
}

// ExchangeAlipayResponseData 表示支付宝换取身份接口的响应数据。
type ExchangeAlipayResponseData struct {
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	ExpiresIn    int              `json:"expires_in"`
	User         AuthUserSnapshot `json:"user"`
}

// SendCaptchaRequest 表示发送短信验证码的请求体。
type SendCaptchaRequest struct {
	Phone       string `json:"phone"`
	Scene       string `json:"scene"`
	SliderToken string `json:"slider_token"`
}

// SliderChallengeInfo 表示需要进行滑块验证时的响应数据。
type SliderChallengeInfo struct {
	SliderRequired    bool `json:"slider_required"`
	AttemptCount      int  `json:"attempt_count"`
	RemainingAttempts int  `json:"remaining_attempts"`
}

// VerifySliderRequest 表示滑块验证的请求体。
type VerifySliderRequest struct {
	Phone       string `json:"phone"`
	SliderToken string `json:"slider_token"`
}

// VerifySliderResponseData 表示滑块验证成功后的响应数据。
type VerifySliderResponseData struct {
	SliderVerificationToken string `json:"slider_verification_token"`
	ExpiresAt               string `json:"expires_at"`
}

// VerifyPhoneRequest 表示验证码验证并绑定手机号的请求体。
type VerifyPhoneRequest struct {
	Phone       string `json:"phone"`
	VerifyCode  string `json:"verify_code"`
	SliderToken string `json:"slider_token"`
}

// WaitlistStatusRequest 表示排队状态查询的请求参数。
type WaitlistStatusRequest struct {
	AlipayUserID string `query:"alipay_user_id"`
}

// WaitlistStatusData 表示排队状态接口的响应数据。
type WaitlistStatusData struct {
	Status string `json:"status"`
	WaitlistInfo
}

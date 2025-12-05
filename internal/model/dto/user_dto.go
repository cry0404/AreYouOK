package dto

// ========== User 相关 DTO ==========

// UserProfileData 用户资料数据
type UserProfileData struct {
	ID       string    `json:"id"`
	PublicID string    `json:"public_id"`
	Nickname string    `json:"nickname"`
	Phone    PhoneInfo `json:"phone"`
	Status   string    `json:"status"`

	Settings UserSettingsDTO `json:"settings"`
	// Waitlist WaitlistInfo    `json:"waitlist"`
	Quotas QuotaBalance `json:"quotas"`
}

// PhoneInfo 手机号信息
type PhoneInfo struct {
	NumberMasked string `json:"number_masked"`
	Verified     bool   `json:"verified"`
}

// UserSettingsDTO 用户设置
type UserSettingsDTO struct {
	DailyCheckInRemindAt   string `json:"daily_check_in_remind_at"`
	DailyCheckInDeadline   string `json:"daily_check_in_deadline"`
	DailyCheckInGraceUntil string `json:"daily_check_in_grace_until"`
	Timezone               string `json:"timezone"`
	DailyCheckInEnabled    bool   `json:"daily_check_in_enabled"`
	JourneyAutoNotify      bool   `json:"journey_auto_notify"`
}

// UpdateUserSettingsRequest 更新用户设置请求
type UpdateUserSettingsRequest struct {
	NickName               *string `json:"nick_name"`
	DailyCheckInEnabled    *bool   `json:"daily_check_in_enabled"`
	DailyCheckInRemindAt   *string `json:"daily_check_in_remind_at"`
	DailyCheckInDeadline   *string `json:"daily_check_in_deadline"`
	DailyCheckInGraceUntil *string `json:"daily_check_in_grace_until"`
	JourneyAutoNotify      *bool   `json:"journey_auto_notify"`
	Timezone               *string `json:"timezone"`
}

// UserStatusData 用户状态数据
type UserStatusData struct {
	Status        string `json:"status"`
	PhoneVerified bool   `json:"phone_verified"`
	HasContacts   bool   `json:"has_contacts"`
	// Waitlist      WaitlistInfo `json:"waitlist"`
}

// QuotaBalance 额度余额
type QuotaBalance struct {
	SMSBalance int `json:"sms_balance"`
	//VoiceBalance   int     `json:"voice_balance"`
	SMSUnitPrice float32 `json:"sms_unit_price,omitempty"`
	//VoiceUnitPrice float32 `json:"voice_unit_price,omitempty"`
}

type WaitlistStatusData struct {
	UserCount int `json:"user_count"`
}

type WaitlistRequest struct {
	AuthCode     string `json:"auth_code" binding:"required"`               // 支付宝 authCode
	AlipayOpenID string `json:"alipay_open_id,omitempty" binding:"omitempty"` // 可选：前端已解析的 open_id（无则后端用 authCode 交换）
}

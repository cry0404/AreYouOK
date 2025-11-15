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
	DailyCheckInDeadline   string `json:"daily_check_in_deadline"`
	DailyCheckInGraceUntil string `json:"daily_check_in_grace_until"`
	Timezone               string `json:"timezone"`
	DailyCheckInEnabled    bool   `json:"daily_check_in_enabled"`
	JourneyAutoNotify      bool   `json:"journey_auto_notify"`
}

// UpdateUserSettingsRequest 更新用户设置请求
type UpdateUserSettingsRequest struct {
	DailyCheckInEnabled    *bool   `json:"daily_check_in_enabled"`
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
	SMSBalance     int     `json:"sms_balance"`
	VoiceBalance   int     `json:"voice_balance"`
	SMSUnitPrice   float32 `json:"sms_unit_price,omitempty"`
	VoiceUnitPrice float32 `json:"voice_unit_price,omitempty"`
}

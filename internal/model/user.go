package model

// UserPhoneInfo 表示用户手机号的概览信息。
type UserPhoneInfo struct {
	NumberMasked string `json:"number_masked"`
	Verified     bool   `json:"verified"`
}

// WaitlistInfo 表示用户的内测排队信息。
type WaitlistInfo struct {
	Priority    int     `json:"priority"`
	Position    *int    `json:"position,omitempty"`
	ActivatedAt *string `json:"activated_at,omitempty"`
}

// UserSettings 表示用户个性化设置。
type UserSettings struct {
	DailyCheckInEnabled    bool   `json:"daily_check_in_enabled"`
	DailyCheckInDeadline   string `json:"daily_check_in_deadline"`
	DailyCheckInGraceUntil string `json:"daily_check_in_grace_until"`
	Timezone               string `json:"timezone"`
	JourneyAutoNotify      bool   `json:"journey_auto_notify"`
}

// UserQuotas 表示用户剩余额度。
type UserQuotas struct {
	SMSBalance     int `json:"sms_balance"`
	VoiceBalance   int `json:"voice_balance"`
	SMSUnitPrice   int `json:"sms_unit_price"`
	VoiceUnitPrice int `json:"voice_unit_price"`
}

// UserProfileData 表示用户信息接口的响应数据。
type UserProfileData struct {
	ID       string        `json:"id"`
	PublicID string        `json:"public_id"`
	Nickname string        `json:"nickname"`
	Phone    UserPhoneInfo `json:"phone"`
	Status   string        `json:"status"`
	Waitlist WaitlistInfo  `json:"waitlist"`
	Quotas   UserQuotas    `json:"quotas"`
	Settings UserSettings  `json:"settings"`
}

// UpdateUserSettingsRequest 表示更新设置的请求体。
type UpdateUserSettingsRequest struct {
	DailyCheckInEnabled    *bool   `json:"daily_check_in_enabled"`
	DailyCheckInDeadline   *string `json:"daily_check_in_deadline"`
	DailyCheckInGraceUntil *string `json:"daily_check_in_grace_until"`
	JourneyAutoNotify      *bool   `json:"journey_auto_notify"`
	Timezone               *string `json:"timezone"`
}

// UserStatusData 表示用户状态接口的响应数据。
type UserStatusData struct {
	Status        string       `json:"status"`
	PhoneVerified bool         `json:"phone_verified"`
	HasContacts   bool         `json:"has_contacts"`
	Waitlist      WaitlistInfo `json:"waitlist"`
}

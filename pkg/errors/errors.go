package errors

func (d Definition) Error() string {
	return d.Message
}

// Definition 表示业务错误码及默认信息。
type Definition struct {
	Code    string
	Message string
}

// 认证相关错误。
var (
	AuthCodeInvalid            = Definition{Code: "AUTH_CODE_INVALID", Message: "Auth code invalid"}
	PhoneAlreadyRegistered     = Definition{Code: "PHONE_ALREADY_REGISTERED", Message: "Phone already registered"}
	CaptchaRateLimited         = Definition{Code: "CAPTCHA_RATE_LIMITED", Message: "Captcha rate limited"}
	VerificationCodeExpired    = Definition{Code: "VERIFICATION_CODE_EXPIRED", Message: "Verification code expired"}
	VerificationCodeInvalid    = Definition{Code: "VERIFICATION_CODE_INVALID", Message: "Verification code invalid"}
	VerificationSliderRequired = Definition{Code: "VERIFICATION_SLIDER_REQUIRED", Message: "Slider verification required"}
	VerificationSliderFailed   = Definition{Code: "VERIFICATION_SLIDER_FAILED", Message: "Slider verification failed"}
	Unauthorized               = Definition{Code: "UNAUTHORIZED", Message: "Unauthorized"}
	InvalidUserID              = Definition{Code: "INVALID_USER_ID", Message: "Invalid user ID format"}
)

// 联系人模块错误。
var (
	ContactLimitReached     = Definition{Code: "CONTACT_LIMIT_REACHED", Message: "Contact limit reached"}
	ContactPriorityConflict = Definition{Code: "CONTACT_PRIORITY_CONFLICT", Message: "Contact priority conflict"}
)

// 平安打卡模块错误。
var (
	CheckInDisabled    = Definition{Code: "CHECK_IN_DISABLED", Message: "Check-in disabled"}
	CheckInAlreadyDone = Definition{Code: "CHECK_IN_ALREADY_DONE", Message: "Check-in already done"}
)

// 行程报备模块错误。
var (
	JourneyOverlap       = Definition{Code: "JOURNEY_OVERLAP", Message: "Journey overlap"}
	JourneyNotModifiable = Definition{Code: "JOURNEY_NOT_MODIFIABLE", Message: "Journey not modifiable"}
)

// 通知模块错误。
var (
	NotifyAckInvalid = Definition{Code: "NOTIFY_ACK_INVALID", Message: "Notification acknowledgement invalid"}
)

// 额度模块错误。
var (
	QuotaInsufficient   = Definition{Code: "QUOTA_INSUFFICIENT", Message: "Quota insufficient"}
	QuotaChannelInvalid = Definition{Code: "QUOTA_CHANNEL_INVALID", Message: "Quota channel invalid"}
)

// 内测排队错误。
var (
	WaitlistFull       = Definition{Code: "WAITLIST_FULL", Message: "Waitlist full"}
	WaitlistNotInvited = Definition{Code: "WAITLIST_NOT_INVITED", Message: "Waitlist not invited"}
)

// 引导流程错误。
var (
	OnboardingStepInvalid = Definition{Code: "ONBOARDING_STEP_INVALID", Message: "Onboarding step invalid"}
)

// Lookup 提供错误码查询能力。
var Lookup = map[string]Definition{
	AuthCodeInvalid.Code:            AuthCodeInvalid,
	PhoneAlreadyRegistered.Code:     PhoneAlreadyRegistered,
	CaptchaRateLimited.Code:         CaptchaRateLimited,
	VerificationCodeExpired.Code:    VerificationCodeExpired,
	VerificationCodeInvalid.Code:    VerificationCodeInvalid,
	VerificationSliderRequired.Code: VerificationSliderRequired,
	VerificationSliderFailed.Code:   VerificationSliderFailed,
	ContactLimitReached.Code:        ContactLimitReached,
	ContactPriorityConflict.Code:    ContactPriorityConflict,
	CheckInDisabled.Code:            CheckInDisabled,
	CheckInAlreadyDone.Code:         CheckInAlreadyDone,
	JourneyOverlap.Code:             JourneyOverlap,
	JourneyNotModifiable.Code:       JourneyNotModifiable,
	NotifyAckInvalid.Code:           NotifyAckInvalid,
	QuotaInsufficient.Code:          QuotaInsufficient,
	QuotaChannelInvalid.Code:        QuotaChannelInvalid,
	WaitlistFull.Code:               WaitlistFull,
	WaitlistNotInvited.Code:         WaitlistNotInvited,
	OnboardingStepInvalid.Code:      OnboardingStepInvalid,
}

// Get 根据错误码返回 Definition，若不存在则返回空 Definition。
func Get(code string) Definition {
	if def, ok := Lookup[code]; ok {
		return def
	}
	return Definition{Code: code, Message: "Unexpected error"}
}

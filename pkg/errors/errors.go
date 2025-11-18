package errors

import "fmt"

func (d Definition) Error() string {
	return d.Message
}

// Definition 表示业务错误码及默认信息。
type Definition struct {
	Code    string
	Message string
}

// Wrap 包装错误，保留原始错误信息
func (d Definition) Wrap(err error) error {
	return fmt.Errorf("%s: %w", d.Message, err)
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
	InvalidPhone               = Definition{Code: "INVALID_PHONE", Message: "Invalid phone number format"}
)

// 打卡相关错误

// 联系人模块错误。
var (
	ContactLimitReached     = Definition{Code: "CONTACT_LIMIT_REACHED", Message: "Contact limit reached"}
	ContactPriorityConflict = Definition{Code: "CONTACT_PRIORITY_CONFLICT", Message: "Contact priority conflict"}
	ContactMinRequired      = Definition{Code: "CONTACT_MIN_REQUIRED", Message: "At least one contact is required"}
)

// 平安打卡模块错误。
var (
	CheckInDisabled    = Definition{Code: "CHECK_IN_DISABLED", Message: "Check-in disabled"}
	CheckInAlreadyDone = Definition{Code: "CHECK_IN_ALREADY_DONE", Message: "Check-in already done"}
	CheckInExpired     = Definition{Code: "CHECK_IN_EXPIRED", Message: "Check-in time has expired"}
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

// 系统错误（用于内部错误，通常需要包装）
var (
	ErrTokenGeneratorNotInitialized = Definition{Code: "TOKEN_GENERATOR_NOT_INITIALIZED", Message: "Token generator not initialized, call token.Init() first"}
	ErrInvalidToken                 = Definition{Code: "INVALID_TOKEN", Message: "Invalid token"}
	ErrInvalidTokenClaims           = Definition{Code: "INVALID_TOKEN_CLAIMS", Message: "Invalid token claims"}
	ErrInvalidTokenType             = Definition{Code: "INVALID_TOKEN_TYPE", Message: "Invalid token type, expected refresh token"}
	ErrUserIDNotFound               = Definition{Code: "USER_ID_NOT_FOUND", Message: "User ID not found in token"}
	ErrUnexpectedSigningMethod      = Definition{Code: "UNEXPECTED_SIGNING_METHOD", Message: "Unexpected signing method"}
	ErrUserNotFound                 = Definition{Code: "USER_NOT_FOUND", Message: "User not found"}
	ErrDatabaseConnectionNil        = Definition{Code: "DATABASE_CONNECTION_NIL", Message: "Database connection is nil"}
	ErrFailedToUnmarshalJSONB       = Definition{Code: "FAILED_TO_UNMARSHAL_JSONB", Message: "Failed to unmarshal JSONB value"}
	ErrCaptchaTokenRequired         = Definition{Code: "CAPTCHA_TOKEN_REQUIRED", Message: "captchaVerifyToken is required"}
	ErrCaptchaResponseNil           = Definition{Code: "CAPTCHA_RESPONSE_NIL", Message: "captcha verification response is nil"}
	ErrCaptchaVerificationFailed    = Definition{Code: "CAPTCHA_VERIFICATION_FAILED", Message: "captcha verification failed"}
	ErrUnsupportedCaptchaProvider   = Definition{Code: "UNSUPPORTED_CAPTCHA_PROVIDER", Message: "Unsupported captcha provider"}
	ErrSignNameRequired             = Definition{Code: "SIGN_NAME_REQUIRED", Message: "signName is required"}
	ErrTemplateCodeRequired         = Definition{Code: "TEMPLATE_CODE_REQUIRED", Message: "templateCode is required"}
	ErrPhonesListEmpty              = Definition{Code: "PHONES_LIST_EMPTY", Message: "phones list is empty"}
	ErrTemplateParamsMismatch       = Definition{Code: "TEMPLATE_PARAMS_MISMATCH", Message: "templateParams count must match phones count"}
	ErrPhonesCodesMismatch          = Definition{Code: "PHONES_CODES_MISMATCH", Message: "phones and codes count mismatch"}
	ErrTencentSMSNotImplemented     = Definition{Code: "TENCENT_SMS_NOT_IMPLEMENTED", Message: "tencent SMS provider not implemented yet"}
	ErrUnsupportedSMSProvider       = Definition{Code: "UNSUPPORTED_SMS_PROVIDER", Message: "Unsupported SMS provider"}
)

// Lookup 提供错误码查询能力。
var Lookup = map[string]Definition{
	AuthCodeInvalid.Code:                 AuthCodeInvalid,
	PhoneAlreadyRegistered.Code:          PhoneAlreadyRegistered,
	CaptchaRateLimited.Code:              CaptchaRateLimited,
	VerificationCodeExpired.Code:         VerificationCodeExpired,
	VerificationCodeInvalid.Code:         VerificationCodeInvalid,
	VerificationSliderRequired.Code:      VerificationSliderRequired,
	VerificationSliderFailed.Code:        VerificationSliderFailed,
	ContactLimitReached.Code:             ContactLimitReached,
	ContactMinRequired.Code:              ContactMinRequired,
	ContactPriorityConflict.Code:         ContactPriorityConflict,
	CheckInDisabled.Code:                 CheckInDisabled,
	CheckInAlreadyDone.Code:              CheckInAlreadyDone,
	JourneyOverlap.Code:                  JourneyOverlap,
	JourneyNotModifiable.Code:            JourneyNotModifiable,
	NotifyAckInvalid.Code:                NotifyAckInvalid,
	QuotaInsufficient.Code:               QuotaInsufficient,
	QuotaChannelInvalid.Code:             QuotaChannelInvalid,
	WaitlistFull.Code:                    WaitlistFull,
	WaitlistNotInvited.Code:              WaitlistNotInvited,
	OnboardingStepInvalid.Code:           OnboardingStepInvalid,
	ErrTokenGeneratorNotInitialized.Code: ErrTokenGeneratorNotInitialized,
	ErrInvalidToken.Code:                 ErrInvalidToken,
	ErrInvalidTokenClaims.Code:           ErrInvalidTokenClaims,
	ErrInvalidTokenType.Code:             ErrInvalidTokenType,
	ErrUserIDNotFound.Code:               ErrUserIDNotFound,
	ErrUnexpectedSigningMethod.Code:      ErrUnexpectedSigningMethod,
	ErrUserNotFound.Code:                 ErrUserNotFound,
	InvalidPhone.Code:                    InvalidPhone,
	ErrDatabaseConnectionNil.Code:        ErrDatabaseConnectionNil,
	ErrFailedToUnmarshalJSONB.Code:       ErrFailedToUnmarshalJSONB,
	ErrCaptchaTokenRequired.Code:         ErrCaptchaTokenRequired,
	ErrCaptchaResponseNil.Code:           ErrCaptchaResponseNil,
	ErrCaptchaVerificationFailed.Code:    ErrCaptchaVerificationFailed,
	ErrUnsupportedCaptchaProvider.Code:   ErrUnsupportedCaptchaProvider,
	ErrSignNameRequired.Code:             ErrSignNameRequired,
	ErrTemplateCodeRequired.Code:         ErrTemplateCodeRequired,
	ErrPhonesListEmpty.Code:              ErrPhonesListEmpty,
	ErrTemplateParamsMismatch.Code:       ErrTemplateParamsMismatch,
	ErrPhonesCodesMismatch.Code:          ErrPhonesCodesMismatch,
	ErrTencentSMSNotImplemented.Code:     ErrTencentSMSNotImplemented,
	ErrUnsupportedSMSProvider.Code:       ErrUnsupportedSMSProvider,
}

// Get 根据错误码返回 Definition，若不存在则返回空 Definition。
func Get(code string) Definition {
	if def, ok := Lookup[code]; ok {
		return def
	}
	return Definition{Code: code, Message: "Unexpected error"}
}

// SkipMessageError 表示消息应该被跳过（ACK 但不处理）
type SkipMessageError struct {
	Reason string
}

func (e *SkipMessageError) Error() string {
	return fmt.Sprintf("skip message: %s", e.Reason)
}

func IsSkipMessageError(err error) bool {
	_, ok := err.(*SkipMessageError)
	return ok
}

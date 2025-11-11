package model

import "time"

// UserStatus 用户状态枚举
type UserStatus string

const (
	UserStatusWaitlisted UserStatus = "waitlisted" // 内测排队中
	UserStatusOnboarding UserStatus = "onboarding" // 引导中，这里是登录验证部分
	UserStatusContact    UserStatus = "contact"    //这里是填写紧急联系人部分
	UserStatusActive     UserStatus = "active"     // 正常使用
)

// User 用户模型

type User struct {
	BaseModel
	PublicID          int64             `gorm:"uniqueIndex;not null" json:"public_id"`
	AlipayUserID      string            `gorm:"uniqueIndex;type:varchar(64);not null" json:"alipay_user_id"`
	Nickname          string            `gorm:"type:varchar(64);not null;default:''" json:"nickname"`
	PhoneCipher       []byte            `gorm:"type:bytea" json:"-"`                // 手机号密文，不对外暴露
	PhoneHash         *string           `gorm:"uniqueIndex;type:char(64)" json:"-"` // 手机号哈希，用于查询
	Status            UserStatus        `gorm:"type:varchar(16);not null;default:'waitlisted';index:idx_users_status" json:"status"`
	EmergencyContacts EmergencyContacts `gorm:"type:jsonb;default:'[]'" json:"emergency_contacts"` // 紧急联系人数组（JSONB）

	//自定义设置部分
	Timezone               string    `gorm:"type:varchar(64);not null;default:'Asia/Shanghai'" json:"timezone"`
	DailyCheckInEnabled    bool      `gorm:"not null;default:false" json:"daily_check_in_enabled"`
	DailyCheckInDeadline   time.Time `gorm:"type:time;not null;default:'20:00:00'" json:"daily_check_in_deadline"`
	DailyCheckInGraceUntil time.Time `gorm:"type:time;not null;default:'21:00:00'" json:"daily_check_in_grace_until"`
	DailyCheckInRemindAt   time.Time `gorm:"type:time;not null;default:'20:00:00'" json:"daily_check_in_remind_at"`
	JourneyAutoNotify      bool      `gorm:"not null;default:true" json:"journey_auto_notify"`

	// 额度（根据 schema.sql，这些字段仍在 users 表中）
	SMSBalance   int `gorm:"not null;default:0" json:"sms_balance"`
	VoiceBalance int `gorm:"not null;default:0" json:"voice_balance"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// EmergencyContacts 紧急联系人数组（JSONB）
type EmergencyContacts []EmergencyContact

// EmergencyContact 紧急联系人结构（存储在 users.emergency_contacts JSONB 中）
type EmergencyContact struct {
	DisplayName       string `json:"display_name"`
	Relationship      string `json:"relationship"`
	PhoneCipherBase64 string `json:"phone_cipher_base64"` // base64 编码的密文
	PhoneHash         string `json:"phone_hash"`
	Priority          int    `json:"priority"` // 1-3
	CreatedAt         string `json:"created_at"`
}

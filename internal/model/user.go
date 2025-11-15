package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// UserStatus 用户状态枚举
type UserStatus string

const (
	UserStatusWaitlisted UserStatus = "waitlisted" // 内测排队中
	UserStatusOnboarding UserStatus = "onboarding" // 引导中，这里是登录验证部分
	UserStatusContact    UserStatus = "contact"    //这里是填写紧急联系人部分
	UserStatusActive     UserStatus = "active"     // 正常使用
)

var StatusToStringMap = map[UserStatus]string{
	UserStatusWaitlisted: "waitlisted",
	UserStatusOnboarding: "onboarding",
	UserStatusContact:    "contact",
	UserStatusActive:     "active",
}

// User 用户模型
type User struct {
	DailyCheckInRemindAt   string  `gorm:"type:time without time zone;not null;default:'20:00:00'" json:"daily_check_in_remind_at"`
	DailyCheckInGraceUntil string  `gorm:"type:time without time zone;not null;default:'21:00:00'" json:"daily_check_in_grace_until"`
	DailyCheckInDeadline   string  `gorm:"type:time without time zone;not null;default:'20:00:00'" json:"daily_check_in_deadline"`
	PhoneHash              *string `gorm:"uniqueIndex;type:char(64)" json:"-"`
	BaseModel
	Status              UserStatus        `gorm:"type:varchar(16);not null;default:'waitlisted';index:idx_users_status" json:"status"`
	Timezone            string            `gorm:"type:varchar(64);not null;default:'Asia/Shanghai'" json:"timezone"`
	Nickname            string            `gorm:"type:varchar(64);not null;default:''" json:"nickname"`
	AlipayOpenID        string            `gorm:"uniqueIndex;type:varchar(64);not null" json:"alipay_open_id"`
	PhoneCipher         []byte            `gorm:"type:bytea" json:"-"`
	EmergencyContacts   EmergencyContacts `gorm:"type:jsonb;default:'[]'" json:"emergency_contacts"`
	PublicID            int64             `gorm:"uniqueIndex;not null" json:"public_id"`
	DailyCheckInEnabled bool              `gorm:"not null;default:false" json:"daily_check_in_enabled"`
	JourneyAutoNotify   bool              `gorm:"not null;default:true" json:"journey_auto_notify"`
}

func (User) TableName() string {
	return "users"
}

// EmergencyContacts 紧急联系人数组（JSONB）
type EmergencyContacts []EmergencyContact

// Scan 实现 sql.Scanner 接口，用于从数据库读取 JSONB 数据
func (ec *EmergencyContacts) Scan(value interface{}) error {
	if value == nil {
		*ec = EmergencyContacts{}
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("cannot scan non-string value into EmergencyContacts")
	}

	if len(bytes) == 0 {
		*ec = EmergencyContacts{}
		return nil
	}

	return json.Unmarshal(bytes, ec)
}

// Value 实现 driver.Valuer 接口，用于将数据写入数据库
func (ec EmergencyContacts) Value() (driver.Value, error) {
	if ec == nil {
		return "[]", nil
	}
	return json.Marshal(ec)
}

// EmergencyContact 紧急联系人结构（存储在 users.emergency_contacts JSONB 中）
type EmergencyContact struct {
	DisplayName       string `json:"display_name"`
	Relationship      string `json:"relationship"`
	PhoneCipherBase64 string `json:"phone_cipher_base64"`
	PhoneHash         string `json:"phone_hash"`
	CreatedAt         string `json:"created_at"`
	Priority          int    `json:"priority"`
}

package model

import (
	"time"
)

// ContactAttemptStatus 通知尝试状态枚举
type ContactAttemptStatus string

const (
	ContactAttemptStatusPending ContactAttemptStatus = "pending" // 待处理
	ContactAttemptStatusSuccess ContactAttemptStatus = "success" // 成功
	ContactAttemptStatusFailed  ContactAttemptStatus = "failed"  // 失败
)

// ContactAttempt 通知尝试模型
type ContactAttempt struct {
	BaseModel
	TaskID            int64                `gorm:"not null;index:idx_contact_attempts_task" json:"task_id"`
	ContactPriority   int                  `gorm:"type:smallint;not null" json:"contact_priority"`
	ContactPhoneHash  string               `gorm:"type:char(64);not null;index:idx_contact_attempts_contact" json:"contact_phone_hash"`
	Channel           NotificationChannel  `gorm:"type:varchar(16);not null" json:"channel"`
	Status            ContactAttemptStatus `gorm:"type:varchar(16);not null;default:'pending'" json:"status"`
	ResponseCode      *string              `gorm:"type:varchar(32)" json:"response_code,omitempty"`
	ResponseMessage   *string              `gorm:"type:varchar(255)" json:"response_message,omitempty"`
	CostCents         int                  `gorm:"not null;default:0" json:"cost_cents"`
	Deducted          bool                 `gorm:"not null;default:false" json:"deducted"`
	AttemptedAt       time.Time            `gorm:"type:timestamptz;not null;default:now()" json:"attempted_at"`
}

// TableName 指定表名
func (ContactAttempt) TableName() string {
	return "contact_attempts"
}


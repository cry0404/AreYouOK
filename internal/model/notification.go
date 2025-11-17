package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"AreYouOK/pkg/errors"
)

// NotificationCategory 通知类别枚举
type NotificationCategory string

const (
	NotificationCategoryCheckInReminder NotificationCategory = "check_in_reminder" // 打卡提醒
	NotificationCategoryJourneyReminder NotificationCategory = "journey_reminder"  // 行程提醒
)

// NotificationChannel 通知渠道枚举
type NotificationChannel string

const (
	NotificationChannelSMS   NotificationChannel = "sms"
	NotificationChannelVoice NotificationChannel = "voice"
)

// NotificationTaskStatus 通知任务状态枚举
type NotificationTaskStatus string

const (
	NotificationTaskStatusPending    NotificationTaskStatus = "pending"    // 待处理
	NotificationTaskStatusProcessing NotificationTaskStatus = "processing" // 处理中
	NotificationTaskStatusSuccess    NotificationTaskStatus = "success"    // 成功
	NotificationTaskStatusFailed     NotificationTaskStatus = "failed"     // 失败
)

// NotificationTask 通知任务模型
type NotificationTask struct {
	ScheduledAt      time.Time              `gorm:"type:timestamptz;not null;index:idx_notification_tasks_status" json:"scheduled_at"`
	ProcessedAt      *time.Time             `gorm:"type:timestamptz" json:"processed_at,omitempty"`
	ContactPriority  *int                   `gorm:"type:smallint;index:idx_notification_tasks_contact" json:"contact_priority,omitempty"`
	ContactPhoneHash *string                `gorm:"type:char(64)" json:"contact_phone_hash,omitempty"`
	Payload          JSONB                  `gorm:"type:jsonb;not null" json:"payload"`
	Channel          NotificationChannel    `gorm:"type:varchar(16);not null" json:"channel"`
	Category         NotificationCategory   `gorm:"type:varchar(32);not null" json:"category"`
	Status           NotificationTaskStatus `gorm:"type:varchar(16);not null;default:'pending';index:idx_notification_tasks_status" json:"status"`
	BaseModel
	RetryCount int   `gorm:"type:smallint;not null;default:0" json:"retry_count"`
	UserID     int64 `gorm:"not null;index:idx_notification_tasks_contact" json:"user_id"`
	TaskCode   int64 `gorm:"uniqueIndex;not null" json:"task_code"`
	CostCents  int   `gorm:"not null;default:0" json:"cost_cents"`
	Deducted   bool  `gorm:"not null;default:false" json:"deducted"`
}

// TableName 指定表名
func (NotificationTask) TableName() string {
	return "notification_tasks"
}

// ContactAttemptStatus 通知尝试状态枚举
type ContactAttemptStatus string

const (
	ContactAttemptStatusPending ContactAttemptStatus = "pending" // 待处理
	ContactAttemptStatusSuccess ContactAttemptStatus = "success" // 成功
	ContactAttemptStatusFailed  ContactAttemptStatus = "failed"  // 失败
)

// ContactAttempt 通知尝试模型
type ContactAttempt struct {
	AttemptedAt      time.Time            `gorm:"type:timestamptz;not null;default:now()" json:"attempted_at"`
	ResponseCode     *string              `gorm:"type:varchar(32)" json:"response_code,omitempty"`
	ResponseMessage  *string              `gorm:"type:varchar(255)" json:"response_message,omitempty"`
	ContactPhoneHash string               `gorm:"type:char(64);not null;index:idx_contact_attempts_contact" json:"contact_phone_hash"`
	Channel          NotificationChannel  `gorm:"type:varchar(16);not null" json:"channel"`
	Status           ContactAttemptStatus `gorm:"type:varchar(16);not null;default:'pending'" json:"status"`
	BaseModel
	TaskID          int64 `gorm:"not null;index:idx_contact_attempts_task" json:"task_id"`
	ContactPriority int   `gorm:"type:smallint;not null" json:"contact_priority"`
	CostCents       int   `gorm:"not null;default:0" json:"cost_cents"`
	Deducted        bool  `gorm:"not null;default:false" json:"deducted"`
}

// TableName 指定表名
func (ContactAttempt) TableName() string {
	return "contact_attempts"
}

// JSONB 自定义 JSONB 类型
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("%w: invalid type %T", errors.ErrFailedToUnmarshalJSONB, value)
	}
	return json.Unmarshal(bytes, j)
}

package model

import "time"

// JourneyStatus 行程状态枚举
type JourneyStatus string

const (
	JourneyStatusOngoing JourneyStatus = "ongoing" // 进行中
	JourneyStatusEnded   JourneyStatus = "ended"   // 已结束
	JourneyStatusTimeout JourneyStatus = "timeout" // 超时
)

// AlertStatus 提醒状态枚举
type AlertStatus string

const (
	AlertStatusPending    AlertStatus = "pending"    // 待触发
	AlertStatusTriggered  AlertStatus = "triggered"  // 已触发
	AlertStatusProcessing AlertStatus = "processing" // 处理中
	AlertStatusSuccess    AlertStatus = "success"    // 成功
	AlertStatusFailed     AlertStatus = "failed"     // 失败
)

// Journey 行程报备模型
type Journey struct {
	ExpectedReturnTime time.Time     `gorm:"type:timestamptz;not null;index:idx_journeys_expected" json:"expected_return_time"`
	ActualReturnTime   *time.Time    `gorm:"type:timestamptz" json:"actual_return_time,omitempty"`
	ReminderSentAt     *time.Time    `gorm:"type:timestamptz" json:"reminder_sent_at,omitempty"`
	AlertTriggeredAt   *time.Time    `gorm:"type:timestamptz" json:"alert_triggered_at,omitempty"`
	AlertLastAttemptAt *time.Time    `gorm:"type:timestamptz" json:"alert_last_attempt_at,omitempty"`
	Title              string        `gorm:"type:varchar(64);not null" json:"title"`
	Note               string        `gorm:"type:text;not null;default:''" json:"note"`
	Status             JourneyStatus `gorm:"type:varchar(16);not null;default:'ongoing';index:idx_journeys_user_status" json:"status"`
	AlertStatus        AlertStatus   `gorm:"type:varchar(16);not null;default:'pending'" json:"alert_status"`

	// P0.7: 延迟消息追踪（用于取消未触发的超时检查）
	TimeoutMessageID *string `gorm:"type:varchar(128);index:idx_journeys_timeout_message_id" json:"timeout_message_id,omitempty"` // 延迟消息的 message_id，用于在 consumer 中检查行程状态

	BaseModel
	UserID        int64 `gorm:"not null;index:idx_journeys_user_status" json:"user_id"`
	AlertAttempts int   `gorm:"not null;default:0" json:"alert_attempts"`
}

// TableName 指定表名
func (Journey) TableName() string {
	return "journeys"
}

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
	AlertStatusProcessing AlertStatus = "processing" // 处理中
	AlertStatusSuccess    AlertStatus = "success"    // 成功
	AlertStatusFailed     AlertStatus = "failed"     // 失败
)

// Journey 行程报备模型
// 注意：根据 schema.sql，提醒状态字段仍在 journeys 表中
type Journey struct {
	BaseModel
	UserID             int64         `gorm:"not null;index:idx_journeys_user_status" json:"user_id"`
	Title              string        `gorm:"type:varchar(64);not null" json:"title"`
	Note               string        `gorm:"type:varchar(255);not null;default:''" json:"note"`
	ExpectedReturnTime time.Time     `gorm:"type:timestamptz;not null;index:idx_journeys_expected" json:"expected_return_time"`
	ActualReturnTime   *time.Time    `gorm:"type:timestamptz" json:"actual_return_time,omitempty"`
	Status             JourneyStatus `gorm:"type:varchar(16);not null;default:'ongoing';index:idx_journeys_user_status" json:"status"`
	ReminderSentAt     *time.Time    `gorm:"type:timestamptz" json:"reminder_sent_at,omitempty"`
	AlertTriggeredAt   *time.Time    `gorm:"type:timestamptz" json:"alert_triggered_at,omitempty"`

	// 行程提醒执行状态（根据 schema.sql，这些字段仍在 journeys 表中）
	AlertStatus        AlertStatus `gorm:"type:varchar(16);not null;default:'pending'" json:"alert_status"`
	AlertAttempts      int         `gorm:"not null;default:0" json:"alert_attempts"`
	AlertLastAttemptAt *time.Time  `gorm:"type:timestamptz" json:"alert_last_attempt_at,omitempty"`
}

// TableName 指定表名
func (Journey) TableName() string {
	return "journeys"
}

package model

import "time"

// CheckInStatus 打卡状态枚举
type CheckInStatus string

const (
	CheckInStatusPending CheckInStatus = "pending" // 未打卡
	CheckInStatusDone    CheckInStatus = "done"    // 已打卡
	CheckInStatusTimeout CheckInStatus = "timeout" // 超时
)

// DailyCheckIn 平安打卡记录模型
type DailyCheckIn struct {
	BaseModel
	UserID           int64         `gorm:"not null;index:idx_daily_check_ins_user_date_status" json:"user_id"`
	CheckInDate      time.Time     `gorm:"type:date;not null;index:idx_daily_check_ins_user_date_status" json:"check_in_date"`
	Status           CheckInStatus `gorm:"type:varchar(16);not null;default:'pending';index:idx_daily_check_ins_user_date_status" json:"status"`
	CheckInAt        *time.Time    `gorm:"type:timestamptz" json:"check_in_at,omitempty"`
	ReminderSentAt   *time.Time    `gorm:"type:timestamptz" json:"reminder_sent_at,omitempty"`
	AlertTriggeredAt *time.Time    `gorm:"type:timestamptz;index:idx_daily_check_ins_alert" json:"alert_triggered_at,omitempty"`
}

// TableName 指定表名
func (DailyCheckIn) TableName() string {
	return "daily_check_ins"
}

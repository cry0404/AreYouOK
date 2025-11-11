package model

import "time"

// ========== CheckIn 相关 DTO ==========

// CheckInStatusData 打卡状态数据
type CheckInStatusData struct {
	Date             string     `json:"date"`
	Status           string     `json:"status"`
	Deadline         time.Time  `json:"deadline"`
	GraceUntil       time.Time  `json:"grace_until"`
	ReminderSentAt   *time.Time `json:"reminder_sent_at,omitempty"`
	AlertTriggeredAt *time.Time `json:"alert_triggered_at,omitempty"`
}

// CompleteCheckInResponse 完成打卡响应
type CompleteCheckInResponse struct {
	Date         string    `json:"date"`
	Status       string    `json:"status"`
	CompletedAt  time.Time `json:"completed_at"`
	StreakDays   int       `json:"streak_days,omitempty"`
	RewardPoints int       `json:"reward_points,omitempty"`
}

// CheckInHistoryQuery 打卡历史查询参数
type CheckInHistoryQuery struct {
	From   string `form:"from"`
	To     string `form:"to"`
	Status string `form:"status"`
	Limit  int    `form:"limit"`
	Cursor string `form:"cursor"`
}

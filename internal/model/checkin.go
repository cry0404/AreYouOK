package model

// CheckInTodayData 表示当日打卡状态。
type CheckInTodayData struct {
	Date             string  `json:"date"`
	Status           string  `json:"status"`
	Deadline         string  `json:"deadline"`
	GraceUntil       string  `json:"grace_until"`
	ReminderSentAt   *string `json:"reminder_sent_at"`
	AlertTriggeredAt *string `json:"alert_triggered_at"`
}

// CompleteCheckInData 表示打卡完成后的响应数据。
type CompleteCheckInData struct {
	Date         string `json:"date"`
	Status       string `json:"status"`
	CompletedAt  string `json:"completed_at"`
	StreakDays   int    `json:"streak_days"`
	RewardPoints int    `json:"reward_points"`
}

// CheckInHistoryQuery 表示历史记录查询参数。
type CheckInHistoryQuery struct {
	From   string `query:"from"`
	To     string `query:"to"`
	Status string `query:"status"`
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit"`
}

// CheckInRecord 表示单条打卡历史记录。
type CheckInRecord struct {
	Date        string `json:"date"`
	Status      string `json:"status"`
	CompletedAt string `json:"completed_at,omitempty"`
}


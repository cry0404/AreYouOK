package model

import "time"

// ========== Journey 相关 DTO ==========

// JourneyItem 行程项
type JourneyItem struct {
	ID                string     `json:"id"`
	Title             string     `json:"title"`
	Note              string     `json:"note"`
	Status            string     `json:"status"`
	ExpectedReturnTime time.Time `json:"expected_return_time"`
	ActualReturnTime  *time.Time `json:"actual_return_time,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

// CreateJourneyRequest 创建行程请求
type CreateJourneyRequest struct {
	Title             string    `json:"title" binding:"required"`
	ExpectedReturnTime time.Time `json:"expected_return_time" binding:"required"`
	Note              string    `json:"note"`
}

// UpdateJourneyRequest 更新行程请求
type UpdateJourneyRequest struct {
	Title             *string    `json:"title"`
	ExpectedReturnTime *time.Time `json:"expected_return_time"`
	Note              *string    `json:"note"`
}

// JourneyDetail 行程详情
type JourneyDetail struct {
	JourneyItem
	ReminderSentAt    *time.Time `json:"reminder_sent_at,omitempty"`
	AlertTriggeredAt  *time.Time `json:"alert_triggered_at,omitempty"`
	AlertStatus       string     `json:"alert_status"`
	AlertAttempts     int        `json:"alert_attempts"`
	AlertLastAttemptAt *time.Time `json:"alert_last_attempt_at,omitempty"`
}

// JourneyAlertData 行程提醒数据
type JourneyAlertData struct {
	AlertStatus       string     `json:"alert_status"`
	AlertAttempts     int        `json:"alert_attempts"`
	AlertLastAttemptAt *time.Time `json:"alert_last_attempt_at,omitempty"`
}

// JourneyListQuery 行程列表查询参数
type JourneyListQuery struct {
	Status string `form:"status"`
	Limit  int    `form:"limit"`
	Cursor string `form:"cursor"`
}


package model

import "time"

// ========== Notification 相关 DTO ==========

// NotificationTaskItem 通知任务项
type NotificationTaskItem struct {
	ID          string                `json:"id"`
	TaskCode    string                `json:"task_code"`
	Category    string                `json:"category"`
	Channel     string                `json:"channel"`
	Status      string                `json:"status"`
	CostCents   int                   `json:"cost_cents"`
	Deducted    bool                  `json:"deducted"`
	ScheduledAt time.Time             `json:"scheduled_at"`
	ProcessedAt *time.Time            `json:"processed_at,omitempty"`
	Contact     NotificationContactInfo `json:"contact,omitempty"`
}

// NotificationContactInfo 通知联系人信息
type NotificationContactInfo struct {
	Priority    int    `json:"priority"`
	DisplayName string `json:"display_name"`
	PhoneMasked string `json:"phone_masked"`
}

// NotificationTaskDetail 通知任务详情
type NotificationTaskDetail struct {
	ID          string                `json:"id"`
	TaskCode    string                `json:"task_code"`
	Category    string                `json:"category"`
	Channel     string                `json:"channel"`
	Status      string                `json:"status"`
	RetryCount  int                   `json:"retry_count"`
	CostCents   int                   `json:"cost_cents"`
	Deducted    bool                  `json:"deducted"`
	Payload     map[string]interface{} `json:"payload"`
	ScheduledAt time.Time             `json:"scheduled_at"`
	ProcessedAt *time.Time            `json:"processed_at,omitempty"`
	Attempts    []NotificationAttempt `json:"attempts"`
}

// NotificationAttempt 通知尝试记录
type NotificationAttempt struct {
	ID               string     `json:"id"`
	ContactPriority  int        `json:"contact_priority"`
	ContactPhoneHash string     `json:"contact_phone_hash"`
	Channel          string     `json:"channel"`
	Status           string     `json:"status"`
	ResponseCode     *string    `json:"response_code,omitempty"`
	ResponseMessage  *string    `json:"response_message,omitempty"`
	CostCents        int        `json:"cost_cents"`
	Deducted         bool       `json:"deducted"`
	AttemptedAt      time.Time  `json:"attempted_at"`
}

// NotificationTaskQuery 通知任务查询参数
type NotificationTaskQuery struct {
	Category string `form:"category"`
	Channel  string `form:"channel"`
	Status   string `form:"status"`
	Limit    int    `form:"limit"`
	Cursor   string `form:"cursor"`
}

// AckNotificationRequest 确认通知请求
type AckNotificationRequest struct {
	TaskID string `json:"task_id" binding:"required"`
}


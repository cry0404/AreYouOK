package dto

import "time"

// ========== Notification 相关 DTO ==========

// NotificationTaskItem 通知任务项
type NotificationTaskItem struct {
	ScheduledAt time.Time               `json:"scheduled_at"`
	ProcessedAt *time.Time              `json:"processed_at,omitempty"`
	ID          string                  `json:"id"`
	TaskCode    string                  `json:"task_code"`
	Category    string                  `json:"category"`
	Channel     string                  `json:"channel"`
	Status      string                  `json:"status"`
	Contact     NotificationContactInfo `json:"contact,omitempty"`
	CostCents   int                     `json:"cost_cents"`
	Deducted    bool                    `json:"deducted"`
}

// NotificationContactInfo 通知联系人信息
type NotificationContactInfo struct {
	DisplayName string `json:"display_name"`
	PhoneMasked string `json:"phone_masked"`
	Priority    int    `json:"priority"`
}

// NotificationTaskDetail 通知任务详情
type NotificationTaskDetail struct {
	ScheduledAt time.Time              `json:"scheduled_at"`
	Payload     map[string]interface{} `json:"payload"`
	ProcessedAt *time.Time             `json:"processed_at,omitempty"`
	ID          string                 `json:"id"`
	TaskCode    string                 `json:"task_code"`
	Category    string                 `json:"category"`
	Channel     string                 `json:"channel"`
	Status      string                 `json:"status"`
	Attempts    []NotificationAttempt  `json:"attempts"`
	RetryCount  int                    `json:"retry_count"`
	CostCents   int                    `json:"cost_cents"`
	Deducted    bool                   `json:"deducted"`
}

// NotificationAttempt 通知尝试记录
type NotificationAttempt struct {
	AttemptedAt      time.Time `json:"attempted_at"`
	ResponseCode     *string   `json:"response_code,omitempty"`
	ResponseMessage  *string   `json:"response_message,omitempty"`
	ID               string    `json:"id"`
	ContactPhoneHash string    `json:"contact_phone_hash"`
	Channel          string    `json:"channel"`
	Status           string    `json:"status"`
	ContactPriority  int       `json:"contact_priority"`
	CostCents        int       `json:"cost_cents"`
	Deducted         bool      `json:"deducted"`
}

// NotificationTaskQuery 通知任务查询参数
type NotificationTaskQuery struct {
	Category string `form:"category"`
	Channel  string `form:"channel"`
	Status   string `form:"status"`
	Cursor   string `form:"cursor"`
	Limit    int    `form:"limit"`
}

// AckNotificationRequest 确认通知请求
type AckNotificationRequest struct {
	TaskID string `json:"task_id" binding:"required"`
}

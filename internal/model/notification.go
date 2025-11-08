package model

// NotificationTaskQuery 表示通知任务查询参数。
type NotificationTaskQuery struct {
	Category string `query:"category"`
	Channel  string `query:"channel"`
	Status   string `query:"status"`
	Limit    int    `query:"limit"`
	Cursor   string `query:"cursor"`
}

// NotificationContactInfo 表示任务关联的联系人信息。
type NotificationContactInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	PhoneMasked string `json:"phone_masked"`
}

// NotificationTask 表示单个通知任务。
type NotificationTask struct {
	ID          string                  `json:"id"`
	TaskCode    string                  `json:"task_code"`
	Category    string                  `json:"category"`
	Channel     string                  `json:"channel"`
	Status      string                  `json:"status"`
	CostCents   int                     `json:"cost_cents"`
	Deducted    bool                    `json:"deducted"`
	ScheduledAt string                  `json:"scheduled_at"`
	ProcessedAt string                  `json:"processed_at"`
	Contact     NotificationContactInfo `json:"contact"`
}

// NotificationAttempt 表示通知的单次尝试记录。
type NotificationAttempt struct {
	ID           string `json:"id"`
	ContactID    string `json:"contact_id"`
	Channel      string `json:"channel"`
	Status       string `json:"status"`
	ResponseCode string `json:"response_code"`
	CostCents    int    `json:"cost_cents"`
	Deducted     bool   `json:"deducted"`
	AttemptedAt  string `json:"attempted_at"`
}

// NotificationTaskDetail 表示通知任务详情。
type NotificationTaskDetail struct {
	ID         string                 `json:"id"`
	TaskCode   string                 `json:"task_code"`
	Category   string                 `json:"category"`
	Channel    string                 `json:"channel"`
	Status     string                 `json:"status"`
	RetryCount int                    `json:"retry_count"`
	CostCents  int                    `json:"cost_cents"`
	Deducted   bool                   `json:"deducted"`
	Payload    map[string]interface{} `json:"payload"`
	Template   NotificationTemplate   `json:"template"`
	Attempts   []NotificationAttempt  `json:"attempts"`
}

// NotificationTemplate 表示通知模板信息。
type NotificationTemplate struct {
	ID       int    `json:"id"`
	Category string `json:"category"`
	Channel  string `json:"channel"`
	Content  string `json:"content"`
	Version  int    `json:"version"`
	Status   string `json:"status"`
}

// NotificationTemplateQuery 表示模板查询参数。
type NotificationTemplateQuery struct {
	Category string `query:"category"`
	Channel  string `query:"channel"`
	Status   string `query:"status"`
}

// NotificationAckRequest 表示通知确认请求体。
type NotificationAckRequest struct {
	TaskID     string `json:"task_id"`
	Channel    string `json:"channel"`
	ReceivedAt string `json:"received_at"`
}

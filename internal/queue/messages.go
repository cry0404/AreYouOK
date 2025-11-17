package queue

// CheckInReminderMessage 打卡提醒消息
type CheckInReminderMessage struct {
	BatchID      string  `json:"batch_id"`
	CheckInDate  string  `json:"check_in_date"`
	ScheduledAt  string  `json:"scheduled_at"`
	UserIDs      []int64 `json:"user_ids"` // 原定于
	DelaySeconds int     `json:"delay_seconds"`
}

// CheckInTimeoutMessage 打卡超时消息, 这个地方还需要防止打卡超时过多发送
type CheckInTimeoutMessage struct {
	BatchID      string  `json:"batch_id"`
	CheckInDate  string  `json:"check_in_date"`
	ScheduledAt  string  `json:"scheduled_at"`
	UserIDs      []int64 `json:"user_ids"`
	DelaySeconds int     `json:"delay_seconds"`
}

// JourneyTimeoutMessage 行程超时消息
type JourneyTimeoutMessage struct {
	ScheduledAt  string `json:"scheduled_at"`
	JourneyID    int64  `json:"journey_id"`
	UserID       int64  `json:"user_id"`
	DelaySeconds int    `json:"delay_seconds"`
}

// NotificationMessage 通知任务消息
type NotificationMessage struct {
	Payload         map[string]interface{} `json:"payload"`
	Category        string                 `json:"category"`
	Channel         string                 `json:"channel"`
	PhoneHash       string                 `json:"phone_hash"`
	TaskID          int64                  `json:"task_id"`
	UserID          int64                  `json:"user_id"`
	ContactPriority int                    `json:"contact_priority"`
}

// EventMessage 事件消息（用于事件总线）
type EventMessage struct {
	Payload    map[string]interface{} `json:"payload"`
	EventKey   string                 `json:"event_key"`
	EventType  string                 `json:"event_type"`
	OccurredAt string                 `json:"occurred_at"`
}

package queue


type TimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// CheckInReminderMessage 打卡提醒消息， 同一批用户的提醒时间，考虑按照时间段来分组
type CheckInReminderMessage struct {
	BatchID      string  `json:"batch_id"`
	CheckInDate  string  `json:"check_in_date"`
	ScheduledAt  string  `json:"scheduled_at"`
	UserIDs      []int64 `json:"user_ids"` // 原定于

	// 用户设置快照（扫描时的设置）
    // Key: userID (string), Value: 用户设置快照
	UserSettings map[string]UserSettingSnapshot `json:"user_settings"`
	DelaySeconds int     `json:"delay_seconds"`
}

type UserSettingSnapshot struct {
    RemindAt      string     `json:"remind_at"`      // 原始提醒时间 "20:00:00"
    Deadline      string     `json:"deadline"`       // 原始截止时间 "20:00:00"
    GraceUntil    string     `json:"grace_until"`    // 原始宽限期 "21:00:00"
    //TimeRange     *TimeRange `json:"time_range,omitempty"` // 原始时间段 {"start": "08:00:00", "end": "20:00:00"}
    Timezone      string     `json:"timezone"`       // 用户时区
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
	TaskCode        int64                  `json:"task_code"` //幂等性帮助
	UserID          int64                  `json:"user_id"`
	ContactPriority int                    `json:"contact_priority"`
	CheckInDate     string                 `json:"check_in_date,omitempty"` // 打卡日期（仅用于 check_in_reminder 类别）
}

// EventMessage 事件消息（用于事件总线）
type EventMessage struct {
	Payload    map[string]interface{} `json:"payload"`
	EventKey   string                 `json:"event_key"`
	EventType  string                 `json:"event_type"`
	OccurredAt string                 `json:"occurred_at"`
}

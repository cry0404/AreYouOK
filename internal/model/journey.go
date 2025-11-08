package model

// JourneyQuery 表示行程列表查询参数。
type JourneyQuery struct {
	Status string `query:"status"`
	Limit  int    `query:"limit"`
	Cursor string `query:"cursor"`
}

// JourneyData 表示行程的基础信息。
type JourneyData struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Status             string `json:"status"`
	ExpectedReturnTime string `json:"expected_return_time"`
	Note               string `json:"note"`
}

// CreateJourneyRequest 表示创建行程的请求体。
type CreateJourneyRequest struct {
	Title              string `json:"title"`
	ExpectedReturnTime string `json:"expected_return_time"`
	Note               string `json:"note"`
}

// UpdateJourneyRequest 表示更新行程的请求体。
type UpdateJourneyRequest struct {
	Title              *string `json:"title"`
	ExpectedReturnTime *string `json:"expected_return_time"`
	Note               *string `json:"note"`
}

// JourneyAlertData 表示行程提醒执行状态。
type JourneyAlertData struct {
	AlertStatus        string  `json:"alert_status"`
	AlertAttempts      int     `json:"alert_attempts"`
	AlertLastAttemptAt *string `json:"alert_last_attempt_at"`
}

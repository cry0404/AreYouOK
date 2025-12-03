package metrics

import (
	"context"
)

// RegisterSMSMetrics 注册短信相关监控指标
// 这个函数保持兼容性，现在使用 OpenTelemetry 而不是 Prometheus
func RegisterSMSMetrics() {
	// 不再需要注册到 Prometheus 注册表
	// 指标将在 InitMetrics() 中初始化
}

// RecordSMSSent 记录短信发送成功
func RecordSMSSent(template, provider string, duration float64, costCents int) {
	ctx := context.Background()
	m := GetMetrics()
	if m != nil {
		m.RecordSMSSent(ctx, template, provider, duration, int64(costCents))
	}
}

// RecordSMSFailed 记录短信发送失败
func RecordSMSFailed(template, provider, statusCode string, duration float64) {
	ctx := context.Background()
	m := GetMetrics()
	if m != nil {
		m.RecordSMSFailed(ctx, template, provider, statusCode, duration)
	}
}

// RecordSMSQuotaInsufficient 记录额度不足
func RecordSMSQuotaInsufficient(userID int64, template string) {
	ctx := context.Background()
	m := GetMetrics()
	if m != nil {
		m.RecordSMSQuotaInsufficient(ctx, userID, template)
	}
}

// RecordSMSRetry 记录短信重试
func RecordSMSRetry(template, reason string) {
	ctx := context.Background()
	m := GetMetrics()
	if m != nil {
		m.RecordSMSRetry(ctx, template, reason)
	}
}

// UpdateSMSActiveTasks 更新活跃短信任务数
func UpdateSMSActiveTasks(status, category string, count float64) {
	ctx := context.Background()
	m := GetMetrics()
	if m != nil {
		m.UpdateSMSActiveTasks(ctx, status, category, int64(count))
	}
}

// UpdateSMSQueueLength 更新短信队列长度
func UpdateSMSQueueLength(queueName string, length float64) {
	ctx := context.Background()
	m := GetMetrics()
	if m != nil {
		m.SetSMSQueueLength(ctx, queueName, int64(length))
	}
}

// AddSMSActiveTask 增加活跃短信任务
func AddSMSActiveTask(status, category string) {
	ctx := context.Background()
	m := GetMetrics()
	if m != nil {
		m.AddSMSActiveTask(ctx, status, category)
	}
}

// SubtractSMSActiveTask 减少活跃短信任务
func SubtractSMSActiveTask(status, category string) {
	ctx := context.Background()
	m := GetMetrics()
	if m != nil {
		m.SubtractSMSActiveTask(ctx, status, category)
	}
}
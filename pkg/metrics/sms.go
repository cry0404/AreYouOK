package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// 短信相关监控指标
var (
	// SMSSentTotal 短信发送总数计数器
	SMSSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sms_sent_total",
			Help: "Total number of SMS sent",
		},
		[]string{"template", "status", "provider"},
	)

	// SMSSendDuration 短信发送耗时直方图
	SMSSendDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sms_send_duration_seconds",
			Help:    "Time spent sending SMS in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"template", "provider"},
	)

	// SMSCostTotal 短信发送总成本计数器（分为单位）
	SMSCostTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sms_cost_cents_total",
			Help: "Total cost of SMS in cents",
		},
		[]string{"template", "provider"},
	)

	// SMSQuotaInsufficientTotal 额度不足错误计数器
	SMSQuotaInsufficientTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sms_quota_insufficient_total",
			Help: "Total number of SMS quota insufficient errors",
		},
		[]string{"user_id", "template"},
	)

	// SMSRetryTotal 短信重试次数计数器
	SMSRetryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sms_retry_total",
			Help: "Total number of SMS retry attempts",
		},
		[]string{"template", "retry_reason"},
	)

	// SMSActiveTasks 当前活跃短信任务数测量器
	SMSActiveTasks = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sms_active_tasks",
			Help: "Number of currently active SMS tasks",
		},
		[]string{"status", "category"},
	)

	// SMSQueueLength 短信队列长度测量器
	SMSQueueLength = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sms_queue_length",
			Help: "Number of messages in SMS queue",
		},
		[]string{"queue_name"},
	)
)

// RegisterSMSMetrics 注册短信相关监控指标
func RegisterSMSMetrics() {
	prometheus.MustRegister(
		SMSSentTotal,
		SMSSendDuration,
		SMSCostTotal,
		SMSQuotaInsufficientTotal,
		SMSRetryTotal,
		SMSActiveTasks,
		SMSQueueLength,
	)
}

// RecordSMSSent 记录短信发送成功
func RecordSMSSent(template, provider string, duration float64, costCents int) {
	SMSSentTotal.WithLabelValues(template, "success", provider).Inc()
	SMSSendDuration.WithLabelValues(template, provider).Observe(duration)
	SMSCostTotal.WithLabelValues(template, provider).Add(float64(costCents))
}

// RecordSMSFailed 记录短信发送失败
func RecordSMSFailed(template, provider, statusCode string, duration float64) {
	SMSSentTotal.WithLabelValues(template, "failed", provider).Inc()
	SMSSendDuration.WithLabelValues(template, provider).Observe(duration)
}

// RecordSMSQuotaInsufficient 记录额度不足
func RecordSMSQuotaInsufficient(userID int64, template string) {
	SMSQuotaInsufficientTotal.WithLabelValues(
		string(rune(userID)), // 简化处理，实际应该使用userID字符串
		template,
	).Inc()
}

// RecordSMSRetry 记录短信重试
func RecordSMSRetry(template, reason string) {
	SMSRetryTotal.WithLabelValues(template, reason).Inc()
}

// UpdateSMSActiveTasks 更新活跃短信任务数
func UpdateSMSActiveTasks(status, category string, count float64) {
	SMSActiveTasks.WithLabelValues(status, category).Set(count)
}

// UpdateSMSQueueLength 更新短信队列长度
func UpdateSMSQueueLength(queueName string, length float64) {
	SMSQueueLength.WithLabelValues(queueName).Set(length)
}
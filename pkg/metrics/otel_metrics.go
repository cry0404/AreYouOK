package metrics

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// OTelMetrics OpenTelemetry 指标集合
type OTelMetrics struct {
	// 短信相关指标
	SMSSentTotal            metric.Int64Counter
	SMSSendDuration         metric.Float64Histogram
	SMSCostTotal            metric.Int64Counter
	SMSQuotaInsufficientTotal metric.Int64Counter
	SMSRetryTotal           metric.Int64Counter
	SMSActiveTasks          metric.Int64UpDownCounter
	SMSQueueLength          metric.Int64UpDownCounter

	// HTTP 相关指标
	HTTPServerRequestTotal   metric.Int64Counter
	HTTPServerDuration       metric.Float64Histogram
	HTTPServerRequestSize    metric.Int64Histogram
	HTTPServerResponseSize   metric.Int64Histogram
	HTTPServerActiveRequests metric.Int64UpDownCounter
}

var (
	// 全局指标实例
	metrics *OTelMetrics
	// meter 用于创建指标
	meter = otel.Meter("areyouok")
)

// InitMetrics 初始化 OpenTelemetry 指标
func InitMetrics() error {
	var err error

	metrics = &OTelMetrics{}

	// 短信相关指标
	metrics.SMSSentTotal, err = meter.Int64Counter(
		"sms_sent_total",
		metric.WithDescription("Total number of SMS sent"),
		metric.WithUnit("{sms}"),
	)
	if err != nil {
		return err
	}

	metrics.SMSSendDuration, err = meter.Float64Histogram(
		"sms_send_duration_seconds",
		metric.WithDescription("Time spent sending SMS in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	metrics.SMSCostTotal, err = meter.Int64Counter(
		"sms_cost_cents_total",
		metric.WithDescription("Total cost of SMS in cents"),
		metric.WithUnit("{cents}"),
	)
	if err != nil {
		return err
	}

	metrics.SMSQuotaInsufficientTotal, err = meter.Int64Counter(
		"sms_quota_insufficient_total",
		metric.WithDescription("Total number of SMS quota insufficient errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}

	metrics.SMSRetryTotal, err = meter.Int64Counter(
		"sms_retry_total",
		metric.WithDescription("Total number of SMS retry attempts"),
		metric.WithUnit("{retry}"),
	)
	if err != nil {
		return err
	}

	metrics.SMSActiveTasks, err = meter.Int64UpDownCounter(
		"sms_active_tasks",
		metric.WithDescription("Number of currently active SMS tasks"),
		metric.WithUnit("{task}"),
	)
	if err != nil {
		return err
	}

	metrics.SMSQueueLength, err = meter.Int64UpDownCounter(
		"sms_queue_length",
		metric.WithDescription("Number of messages in SMS queue"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return err
	}

	return nil
}

// GetMetrics 获取全局指标实例
func GetMetrics() *OTelMetrics {
	return metrics
}

// RecordSMSSent 记录短信发送成功
func (m *OTelMetrics) RecordSMSSent(ctx context.Context, template, provider string, duration float64, costCents int64) {
	attrs := []attribute.KeyValue{
		attribute.String("template", template),
		attribute.String("status", "success"),
		attribute.String("provider", provider),
	}

	m.SMSSentTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.SMSSendDuration.Record(ctx, duration, metric.WithAttributes(
		attribute.String("template", template),
		attribute.String("provider", provider),
	))
	m.SMSCostTotal.Add(ctx, costCents, metric.WithAttributes(
		attribute.String("template", template),
		attribute.String("provider", provider),
	))
}

// RecordSMSFailed 记录短信发送失败
func (m *OTelMetrics) RecordSMSFailed(ctx context.Context, template, provider, statusCode string, duration float64) {
	attrs := []attribute.KeyValue{
		attribute.String("template", template),
		attribute.String("status", "failed"),
		attribute.String("provider", provider),
	}

	m.SMSSentTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.SMSSendDuration.Record(ctx, duration, metric.WithAttributes(
		attribute.String("template", template),
		attribute.String("provider", provider),
	))
}

// RecordSMSQuotaInsufficient 记录额度不足
func (m *OTelMetrics) RecordSMSQuotaInsufficient(ctx context.Context, userID int64, template string) {
	m.SMSQuotaInsufficientTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("user_id", fmt.Sprintf("%d", userID)),
		attribute.String("template", template),
	))
}

// RecordSMSRetry 记录短信重试
func (m *OTelMetrics) RecordSMSRetry(ctx context.Context, template, reason string) {
	m.SMSRetryTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("template", template),
		attribute.String("retry_reason", reason),
	))
}

// UpdateSMSActiveTasks 更新活跃短信任务数
func (m *OTelMetrics) UpdateSMSActiveTasks(ctx context.Context, status, category string, count int64) {
	m.SMSActiveTasks.Add(ctx, count, metric.WithAttributes(
		attribute.String("status", status),
		attribute.String("category", category),
	))
}

// SetSMSQueueLength 设置短信队列长度
func (m *OTelMetrics) SetSMSQueueLength(ctx context.Context, queueName string, length int64) {
	m.SMSQueueLength.Add(ctx, length, metric.WithAttributes(
		attribute.String("queue_name", queueName),
	))
}

// AddSMSActiveTask 增加活跃短信任务
func (m *OTelMetrics) AddSMSActiveTask(ctx context.Context, status, category string) {
	m.UpdateSMSActiveTasks(ctx, status, category, 1)
}

// SubtractSMSActiveTask 减少活跃短信任务
func (m *OTelMetrics) SubtractSMSActiveTask(ctx context.Context, status, category string) {
	m.UpdateSMSActiveTasks(ctx, status, category, -1)
}
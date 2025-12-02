package mq

import (
	"context"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	// RabbitMQ 相关指标
	mqMessagesTotal   metric.Int64Counter
	mqMessageDuration metric.Float64Histogram
	mqPublishErrors   metric.Int64Counter
	mqConsumeErrors   metric.Int64Counter
	mqQueueSize       metric.Int64UpDownCounter
)

// InitMQMetrics 初始化 RabbitMQ 指标
func InitMQMetrics(meter metric.Meter) error {
	var err error

	// 消息总数
	mqMessagesTotal, err = meter.Int64Counter(
		"mq.messages.total",
		metric.WithDescription("Total number of RabbitMQ messages"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return err
	}

	// 消息处理耗时
	mqMessageDuration, err = meter.Float64Histogram(
		"mq.message.duration",
		metric.WithDescription("RabbitMQ message processing duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5),
	)
	if err != nil {
		return err
	}

	// 发布错误数
	mqPublishErrors, err = meter.Int64Counter(
		"mq.publish.errors",
		metric.WithDescription("Number of RabbitMQ publish errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}

	// 消费错误数
	mqConsumeErrors, err = meter.Int64Counter(
		"mq.consume.errors",
		metric.WithDescription("Number of RabbitMQ consume errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}

	// 队列大小
	mqQueueSize, err = meter.Int64UpDownCounter(
		"mq.queue.size",
		metric.WithDescription("RabbitMQ queue size"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return err
	}

	return nil
}

// InstrumentedChannel 包装 amqp.Channel 以添加 OpenTelemetry 支持
type InstrumentedChannel struct {
	ch          *amqp.Channel
	serviceName string
	propagators propagation.TextMapPropagator
	tracer      trace.Tracer
}

// NewInstrumentedChannel 创建带有 OpenTelemetry 支持的 Channel
func NewInstrumentedChannel(ch *amqp.Channel, serviceName string) *InstrumentedChannel {
	return &InstrumentedChannel{
		ch:          ch,
		serviceName: serviceName,
		propagators: otel.GetTextMapPropagator(),
		tracer:      otel.Tracer(serviceName + ".rabbitmq"),
	}
}

// PublishWithContext 发布消息并添加追踪
func (ic *InstrumentedChannel) PublishWithContext(
	ctx context.Context,
	exchange, routingKey string,
	mandatory, immediate bool,
	msg amqp.Publishing,
) error {
	startTime := time.Now()

	// 创建 Span
	spanName := "rabbitmq.publish"
	if exchange != "" {
		spanName = "rabbitmq.publish." + exchange
	}

	ctx, span := ic.tracer.Start(ctx, spanName, trace.WithAttributes(
		semconv.MessagingSystem("rabbitmq"),
		attribute.String("messaging.destination.kind", "exchange"),
		attribute.String("messaging.destination.name", exchange),
		attribute.String("messaging.rabbitmq.routing_key", routingKey),
		attribute.String("messaging.rabbitmq.exchange", exchange),
		attribute.String("service.name", ic.serviceName),
	))
	defer span.End()

	// 注入追踪上下文到消息头
	headers := make(amqp.Table)
	if msg.Headers != nil {
		for k, v := range msg.Headers {
			headers[k] = v
		}
	}

	ic.propagators.Inject(ctx, &MessageHeaderCarrier{Headers: headers})
	msg.Headers = headers

	// 发布消息
	err := ic.ch.PublishWithContext(ctx, exchange, routingKey, mandatory, immediate, msg)
	duration := time.Since(startTime).Seconds()

	// 记录指标
	status := "success"
	if err != nil {
		status = "error"
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		mqPublishErrors.Add(ctx, 1)
	} else {
		span.SetStatus(codes.Ok, "Message published successfully")
	}

	labels := []attribute.KeyValue{
		semconv.MessagingSystem("rabbitmq"),
		attribute.String("messaging.operation", "publish"),
		attribute.String("messaging.rabbitmq.exchange", exchange),
		attribute.String("messaging.rabbitmq.routing_key", routingKey),
		attribute.String("messaging.status", status),
	}

	mqMessagesTotal.Add(ctx, 1, metric.WithAttributes(labels...))
	mqMessageDuration.Record(ctx, duration, metric.WithAttributes(labels...))

	return err
}

// Consume 消费消息并添加追踪
func (ic *InstrumentedChannel) Consume(
	queue, consumer string,
	autoAck, exclusive, noLocal, noWait bool,
	args amqp.Table,
) (<-chan amqp.Delivery, error) {
	// 创建消费 Span
	ctx := context.Background()
	spanName := "rabbitmq.consume"
	if queue != "" {
		spanName = "rabbitmq.consume." + queue
	}

	ctx, span := ic.tracer.Start(ctx, spanName, trace.WithAttributes(
		semconv.MessagingSystem("rabbitmq"),
		attribute.String("messaging.destination.kind", "queue"),
		semconv.MessagingDestinationName(queue),
		attribute.String("messaging.rabbitmq.queue", queue),
		attribute.String("service.name", ic.serviceName),
		attribute.String("messaging.operation", "consume"),
	))
	defer span.End()

	// 开始消费
	msgs, err := ic.ch.Consume(queue, consumer, autoAck, exclusive, noLocal, noWait, args)

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		mqConsumeErrors.Add(ctx, 1)
		return nil, err
	}

	span.SetStatus(codes.Ok, "Consumer started successfully")

	// 创建包装的 channel 来追踪每个消息
	wrappedMsgs := make(chan amqp.Delivery)
	go func() {
		defer close(wrappedMsgs)
		for msg := range msgs {
			// 为每个消息创建处理 Span
			ic.processDelivery(ctx, msg)
			wrappedMsgs <- msg
		}
	}()

	return wrappedMsgs, nil
}

// processDelivery 处理单个消息的追踪
func (ic *InstrumentedChannel) processDelivery(ctx context.Context, msg amqp.Delivery) {
	startTime := time.Now()

	// 提取追踪上下文
	msgCtx := ic.propagators.Extract(ctx, &MessageHeaderCarrier{Headers: msg.Headers})

	// 创建消息处理 Span
	_, span := ic.tracer.Start(msgCtx, "rabbitmq.message.process", trace.WithAttributes(
		semconv.MessagingSystem("rabbitmq"),
		attribute.String("messaging.destination.kind", "queue"),
		attribute.String("messaging.rabbitmq.exchange", msg.Exchange),
		semconv.MessagingRabbitmqDestinationRoutingKey(msg.RoutingKey),
		semconv.MessagingMessageID(msg.MessageId),
		attribute.String("messaging.rabbitmq.queue", msg.RoutingKey), // 假设 routing key 是队列名
		attribute.String("service.name", ic.serviceName),
	))

	span.End()

	// 记录消息指标（这里只是示例，实际应该在消息处理完成后记录）
	duration := time.Since(startTime).Seconds()

	labels := []attribute.KeyValue{
		semconv.MessagingSystem("rabbitmq"),
		attribute.String("messaging.operation", "consume"),
		attribute.String("messaging.rabbitmq.exchange", msg.Exchange),
		attribute.String("messaging.rabbitmq.routing_key", msg.RoutingKey),
		attribute.String("messaging.status", "received"),
	}

	mqMessagesTotal.Add(ctx, 1, metric.WithAttributes(labels...))
	mqMessageDuration.Record(ctx, duration, metric.WithAttributes(labels...))
}

// Channel 返回原始的 amqp.Channel
func (ic *InstrumentedChannel) Channel() *amqp.Channel {
	return ic.ch
}

// MessageHeaderCarrier 实现 propagation.TextMapCarrier 接口
type MessageHeaderCarrier struct {
	Headers amqp.Table
}

func (m *MessageHeaderCarrier) Get(key string) string {
	if val, ok := m.Headers[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (m *MessageHeaderCarrier) Set(key, value string) {
	if m.Headers == nil {
		m.Headers = make(amqp.Table)
	}
	m.Headers[key] = value
}

func (m *MessageHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(m.Headers))
	for k := range m.Headers {
		keys = append(keys, k)
	}
	return keys
}

// PublishWithTracing 便捷函数：发布消息并添加追踪
func PublishWithTracing(
	ctx context.Context,
	ch *amqp.Channel,
	serviceName, exchange, routingKey string,
	msg amqp.Publishing,
) error {
	ic := NewInstrumentedChannel(ch, serviceName)
	return ic.PublishWithContext(ctx, exchange, routingKey, false, false, msg)
}

// ConsumeWithTracing 便捷函数：消费消息并添加追踪
func ConsumeWithTracing(
	ch *amqp.Channel,
	serviceName, queue, consumer string,
	autoAck, exclusive, noLocal, noWait bool,
	args amqp.Table,
) (<-chan amqp.Delivery, error) {
	ic := NewInstrumentedChannel(ch, serviceName)
	return ic.Consume(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
}

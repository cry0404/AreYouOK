package mq

import (
	"context"
	"fmt"

	"AreYouOK/pkg/errors"

	amqp "github.com/rabbitmq/amqp091-go"

	"go.uber.org/zap"

	"AreYouOK/pkg/logger"
)

type MessageHandler func([]byte) error

const (
	MaxRetryCount = 3
)

type ConsumeOptions struct {
	Handler       MessageHandler
	Queue         string
	ConsumerTag   string
	PrefetchCount int
	MaxRetries    int
	Context       context.Context // 用于优雅关闭
}

// 设计模式，策略型？
func Consume(opts ConsumeOptions) error {
	// 服务对 rabbitmq 建立的连接而已
	conn := Connection()

	if conn == nil {
		return fmt.Errorf("RabbitMQ connection is nil")
	}

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	defer ch.Close()

	if opts.PrefetchCount > 0 {
		if err := ch.Qos(opts.PrefetchCount, 0, false); err != nil {
			return fmt.Errorf("failed to set QoS: %w", err)
		}
	}

	msgs, err := ch.Consume(
		opts.Queue,
		opts.ConsumerTag,
		false, // auto-ack = false
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	logger.Logger.Info("Started consuming messages",
		zap.String("queue", opts.Queue),
		zap.String("consumer_tag", opts.ConsumerTag),
		zap.Int("prefetch_count", opts.PrefetchCount),
	)

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	for {
		select {
		case <-ctx.Done():
			logger.Logger.Info("Consumer context cancelled, stopping",
				zap.String("queue", opts.Queue),
				zap.String("consumer_tag", opts.ConsumerTag),
			)
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				// 消息通道已关闭
				logger.Logger.Info("Message channel closed, stopping consumer",
					zap.String("queue", opts.Queue),
					zap.String("consumer_tag", opts.ConsumerTag),
				)
				return nil
			}

			err := opts.Handler(msg.Body)

			if err != nil {
				if errors.IsSkipMessageError(err) {

					logger.Logger.Info("Skipping message",
						zap.String("queue", opts.Queue),
						zap.String("reason", err.Error()),
					)
					msg.Ack(false)
					continue
				}

				// 检查重试次数
				maxRetries := opts.MaxRetries
				if maxRetries == 0 {
					maxRetries = MaxRetryCount
				}

				retryCount := getRetryCount(msg.Headers)

				if retryCount >= maxRetries {
					// 超过最大重试次数，拒绝消息（requeue=false）让它进入死信队列
					logger.Logger.Error("Message exceeded max retry count, sending to DLQ",
						zap.String("queue", opts.Queue),
						zap.String("consumer_tag", opts.ConsumerTag),
						zap.Int("retry_count", retryCount),
						zap.Int("max_retries", maxRetries),
						zap.Error(err),
					)
					msg.Nack(false, false) // requeue = false，消息会进入死信队列
					continue
				}

				// 未超过最大重试次数，增加重试次数并重新入队
				// 重新发布消息，通过消息头判断是否超过重试次数
				retryCount++
				logger.Logger.Warn("Message processing failed, retrying",
					zap.String("queue", opts.Queue),
					zap.String("consumer_tag", opts.ConsumerTag),
					zap.Int("retry_count", retryCount),
					zap.Int("max_retries", maxRetries),
					zap.Error(err),
				)

				// 重新发布消息并更新重试次数
				if err := republishWithRetryCount(ch, msg, opts.Queue, retryCount); err != nil {
					logger.Logger.Error("Failed to republish message with retry count",
						zap.String("queue", opts.Queue),
						zap.Error(err),
					)
					// 如果重新发布失败，直接 Nack(requeue=true) 作为降级方案
					msg.Nack(false, true)
				} else {
					// 重新发布成功，确认原消息
					msg.Ack(false)
				}
				continue
			}

			msg.Ack(false)
		}
	}
}

// getRetryCount 从消息头获取重试次数
func getRetryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}

	if retryCount, ok := headers["x-retry-count"]; ok {
		if count, ok := retryCount.(int); ok {
			return count
		}
		if count, ok := retryCount.(int32); ok {
			return int(count)
		}
		if count, ok := retryCount.(int64); ok {
			return int(count)
		}
	}

	return 0
}

// republishWithRetryCount 重新发布消息并更新重试次数
func republishWithRetryCount(ch *amqp.Channel, originalMsg amqp.Delivery, queue string, retryCount int) error {
	// 更新消息头
	headers := originalMsg.Headers
	if headers == nil {
		headers = make(amqp.Table)
	}
	headers["x-retry-count"] = retryCount
	headers["x-original-queue"] = queue

	// 重新发布消息
	err := ch.Publish(
		"",    // exchange (空字符串表示使用默认交换机)
		queue, // routing key (队列名)
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:      headers,
			ContentType:  originalMsg.ContentType,
			Body:         originalMsg.Body,
			DeliveryMode: originalMsg.DeliveryMode,
			Priority:     originalMsg.Priority,
			MessageId:    originalMsg.MessageId,
			Timestamp:    originalMsg.Timestamp,
		},
	)

	return err
}

package mq

import (
	"context"
	"encoding/json"
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
					// 在进入死信队列前，尝试清理 Redis 标记，避免标记残留
					messageID := extractMessageID(msg.Body)
					if messageID != "" {

						logger.Logger.Warn("错误的消息提取，考虑引入消息清理部分",
							zap.String("message_id", messageID),
							zap.String("queue", opts.Queue),
						)
					}

					logger.Logger.Error("Message exceeded max retry count, sending to DLQ",
						zap.String("queue", opts.Queue),
						zap.String("consumer_tag", opts.ConsumerTag),
						zap.Int("retry_count", retryCount),
						zap.Int("max_retries", maxRetries),
						zap.String("message_id", messageID),
						zap.Error(err),
					)
					msg.Nack(false, false) // requeue = false，消息会进入死信队列
					continue
				}

				retryCount++
				logger.Logger.Warn("Message processing failed, retrying",
					zap.String("queue", opts.Queue),
					zap.String("consumer_tag", opts.ConsumerTag),
					zap.Int("retry_count", retryCount),
					zap.Int("max_retries", maxRetries),
					zap.Error(err),
				)

				// 这里是重新发布存在错误的地方
				if err := republishWithRetryCount(ch, msg, opts.Queue, retryCount); err != nil {
					logger.Logger.Error("Failed to republish message with retry count",
						zap.String("queue", opts.Queue),
						zap.Error(err),
					)
					msg.Nack(false, true)
				} else {
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

// extractMessageID 从消息体中尝试提取 message_id 字段
// 这是一个辅助函数，用于在消息进入死信队列时清理 Redis 标记
func extractMessageID(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// 尝试解析为 JSON 并提取 message_id
	var msgMap map[string]interface{}
	if err := json.Unmarshal(body, &msgMap); err != nil {
		// 解析失败，返回空字符串
		return ""
	}

	// 尝试提取 message_id 字段（支持 message_id 和 messageId 两种格式）
	if msgID, ok := msgMap["message_id"].(string); ok && msgID != "" {
		return msgID
	}
	if msgID, ok := msgMap["messageId"].(string); ok && msgID != "" {
		return msgID
	}

	return ""
}

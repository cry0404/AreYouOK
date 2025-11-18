package mq

import (
	// "encoding/json"
	"fmt"

	// amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"AreYouOK/pkg/errors"

	"AreYouOK/pkg/logger"
)

type MessageHandler func([]byte) error

type ConsumeOptions struct {
	Handler       MessageHandler
	Queue         string
	ConsumerTag   string
	PrefetchCount int
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

	for msg := range msgs {
		err := opts.Handler(msg.Body)
	
	if err != nil {

		if errors.IsSkipMessageError(err) {
			// 当前消息被取消了，可能是已经消费，也可能是更改了对应的状态
			logger.Logger.Info("Skipping message",
				zap.String("queue", opts.Queue),
				zap.String("reason", err.Error()),
			)
			msg.Ack(false)
			continue
		}
		
		// 真正的处理失败，Nack 并 requeue
		logger.Logger.Error("Failed to process message",
			zap.String("queue", opts.Queue),
			zap.String("consumer_tag", opts.ConsumerTag),
			zap.Error(err),
		)
		msg.Nack(false, true) // requeue = true
		continue
	}

		msg.Ack(false)
	}

	return nil
}

package mq

import (
	//"encoding/json"
	"fmt"
	
	//amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"AreYouOK/pkg/logger"
)

type MessageHandler func([]byte) error

type ConsumeOptions struct {
	Queue         string
	ConsumerTag   string
	PrefetchCount int
	Handler       MessageHandler
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
		false, //auto-ack = false
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
		if err := opts.Handler(msg.Body); err != nil {
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

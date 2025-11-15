package mq

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"

	"AreYouOK/config"
)

var (
	conn *amqp.Connection
)

func Init() error {
	var err error
	url := config.Cfg.GetRabbitMQURL()
	conn, err = amqp.Dial(url)

	if err != nil {
		return err
	}

	_, err = conn.Channel()

	return err
}

// Connection 返回 RabbitMQ 连接
func Connection() *amqp.Connection {
	return conn
}

// Close 关闭 RabbitMQ 连接
func Close(ctx context.Context) error {
	if conn == nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- conn.Close()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

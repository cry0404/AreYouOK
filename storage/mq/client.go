package mq

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"AreYouOK/config"
	"AreYouOK/pkg/logger"
)

// rabbitmq 设计，先声明 conn，一个 conn 多个 channel

	//conn *amqp.Connection //唯一的 connection


func Init() error {
	var err error
	url := config.Cfg.GetRabbitMQURL()
	conn, err = amqp.Dial(url)

	if err != nil {
		return err
	}

	_, err = conn.Channel()

	if err != nil {
		return fmt.Errorf("failed to open test channel: %w", err)
	}

	if err := InitExchangesAndQueues(); err != nil {
		return fmt.Errorf("failed to init exchanges and queues: %w", err)
	}

	logger.Logger.Info("RabbitMQ initialized successful",
		zap.String("url", url))

	return err
}

// 声明交换机 exchange，然后声明交换机对应的队列，然后根据 key 绑定队列到交换机
func InitExchangesAndQueues() error {	
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	defer ch.Close()


	if err := declareExchanges(ch); err != nil {
		return err
	}


	if err := declareQueues(ch); err != nil {
		return err
	}


	if err := bindQueues(ch); err != nil {
		return err
	}
	return nil
}

func declareExchanges(ch *amqp.Channel) error {
	exchanges := []struct {
		name       string
		kind       string
		durable    bool
		autoDelete bool
		internal   bool
		args       amqp.Table
	}{

		//通知交换机
		{
			name:    "notification.topic",
			kind:    "topic",
			durable: true,
		},

		// 延迟消息交换机
		{
			name:    "scheduler.delayed",
			kind:    "x-delayed-message",
			durable: true,
			args: amqp.Table{
				"x-delayed-type": "direct",
			},
		},

		// 事件交换机
		{
			name:    "events.topic",
			kind:    "topic",
			durable: true,
		},
	}

	//开始注册

	for _, ex := range exchanges {
		if err := ch.ExchangeDeclare(
			ex.name,
			ex.kind,
			ex.durable,
			ex.autoDelete,
			ex.internal,
			false,
			ex.args,
		); err != nil {
			return fmt.Errorf("failed to declare exchange %s: %w", ex.name, err)
		}
	}
	return nil
}

// 声明每个 exchange 下要注册的队列

func declareQueues(ch *amqp.Channel) error {
	queues := []struct {
		name       string
		durable    bool
		exclusive  bool
		autoDelete bool
		args       amqp.Table
	}{
		// 通知队列
		{"notification.sms", true, false, false, nil},
		{"notification.voice", true, false, false, nil},
		{"notification.sms.dlq", true, false, false, nil},
		{"notification.voice.dlq", true, false, false, nil},

		// 定时任务队列
		{"scheduler.check_in.reminder", true, false, false, nil},
		{"scheduler.check_in.timeout", true, false, false, nil},
		{"scheduler.journey.timeout", true, false, false, nil},

		// 事件队列
		{"events.check_in.timeout", true, false, false, nil},
		{"events.journey.timeout", true, false, false, nil},
		{"events.quota.depleted", true, false, false, nil},
	}

	for _, q := range queues {
		_, err := ch.QueueDeclare(
			q.name,
			q.durable,
			q.autoDelete,
			q.exclusive,
			false, // no-wait
			q.args,
		)
		if err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", q.name, err)
		}
	}

	return nil
}

// bindQueues 绑定队列到交换机
func bindQueues(ch *amqp.Channel) error {
	bindings := []struct {
		queue      string
		routingKey string
		exchange   string
	}{
		// 通知队列绑定
		{"notification.sms", "notification.sms.*", "notification.topic"},
		{"notification.voice", "notification.voice.*", "notification.topic"},

		// 定时任务队列绑定（延迟消息）
		{"scheduler.check_in.reminder", "scheduler.check_in.reminder", "scheduler.delayed"},
		{"scheduler.check_in.timeout", "scheduler.check_in.timeout", "scheduler.delayed"},
		{"scheduler.journey.timeout", "scheduler.journey.timeout", "scheduler.delayed"},

		// 事件队列绑定
		{"events.check_in.timeout", "check_in.timeout", "events.topic"},
		{"events.journey.timeout", "journey.timeout", "events.topic"},
		{"events.quota.depleted", "quota.depleted", "events.topic"},
	}

	for _, b := range bindings {
		if err := ch.QueueBind(
			b.queue,
			b.routingKey,
			b.exchange,
			false, // no-wait
			nil,
		); err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s: %w", b.queue, b.exchange, err)
		}
	}

	return nil
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

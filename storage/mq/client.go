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

// conn *amqp.Connection //唯一的 connection

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
		args       amqp.Table
		name       string
		kind       string
		durable    bool
		autoDelete bool
		internal   bool
	}{
		// 这个是对应触发的部分
		// 通知交换机，用于发送 sms，voice， 根据 category 递送到不同的队列
		// 对应的两个消费者函数 `StartSMSNotificationConsumer`、StartVoiceNotificationConsumer ，考虑是否可以根据 category 直接投递
		{
			name:    "notification.topic",
			kind:    "topic",
			durable: true,
		},

		// 用于打卡提醒，延迟到提醒时间，定时任务的延迟投递， schedule 是否也需要死信队列呢
		{
			name:    "scheduler.delayed",
			kind:    "x-delayed-message",
			durable: true,
			args: amqp.Table{
				"x-delayed-type": "direct",
			},
		},

		// 事件交换机，主要放重试等事件，打卡超时，充值扣费等额度
		{
			name:    "events.topic",
			kind:    "topic",
			durable: true,
		},

		{
			name:    "notification.dlx", //考虑什么时候丢弃对应的通知
			kind:    "direct",
			durable: true,
		},
	}

	// 开始注册对应的交换机
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

// 声明每个 exchange 下对应的队列

func declareQueues(ch *amqp.Channel) error {
	queues := []struct {
		name       string
		durable    bool
		exclusive  bool
		autoDelete bool
		args       amqp.Table
	}{
		// 通知队列
		{
			name:       "notification.sms",
			durable:    true,
			exclusive:  false,
			autoDelete: false,
			args: amqp.Table{
				"x-dead-letter-exchange":    "notification.dlx",
				"x-dead-letter-routing-key": "notification.sms.dlq",
				"x-message-ttl":             3600000,
			},
		},

		{
			name:       "notification.voice",
			durable:    true,
			exclusive:  false, //排他队列，只允许当前链接使用，理论上应该是
			autoDelete: false,
			args: amqp.Table{
				"x-dead-letter-exchange":    "notification.dlx",
				"x-dead-letter-routing-key": "notification.voice.dlq",
				"x-message-ttl":             3600000,
			},
		},

		//死信队列, 定时任务队列可能也需要配置个死信队列
		{"notification.sms.dlq", true, false, false, nil},
		{"notification.voice.dlq", true, false, false, nil},

		// 定时任务队列
		{"scheduler.check_in.reminder", true, false, false, nil},
		{"scheduler.check_in.timeout", true, false, false, nil},
		{"scheduler.journey.reminder", true, false, false, nil},
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

		// 死信队列
		{"notification.sms.dlq", "notification.sms.dlq", "notification.dlx"},
		{"notification.voice.dlq", "notification.voice.dlq", "notification.dlx"},

		// 定时任务队列绑定（延迟消息）
		{"scheduler.check_in.reminder", "scheduler.check_in.reminder", "scheduler.delayed"},
		{"scheduler.check_in.timeout", "scheduler.check_in.timeout", "scheduler.delayed"},
		{"scheduler.journey.reminder", "scheduler.journey.reminder", "scheduler.delayed"},
		{"scheduler.journey.timeout", "scheduler.journey.timeout", "scheduler.delayed"},

		// 事件队列绑定
		{"events.check_in.timeout", "check_in.timeout", "events.topic"},
		{"events.journey.timeout", "journey.timeout", "events.topic"},
		{"events.quota.depleted", "quota.depleted", "events.topic"},
	}
	// 根据路由键绑定到对应的队列
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

func Connection() *amqp.Connection {
	return conn
}

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

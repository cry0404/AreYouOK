package mq

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"

	"AreYouOK/pkg/logger"
)

// 底层的实现，将对应包装的部分推送即可

// 单例 + 锁，思考实现需求，为什么这样实现

var (
	publisherCh *amqp.Channel
	pubMutex    sync.RWMutex // 读写锁，读多写少
)

// PublishDelayedMessage 发送延迟消息
func getPublisherChannel() (*amqp.Channel, error) {
	// 先读锁检查
	pubMutex.RLock()
	if publisherCh != nil && publisherCh.IsClosed() {
		ch := publisherCh
		pubMutex.RUnlock()
		return ch, nil
	}
	pubMutex.RUnlock()

	pubMutex.Lock()
	defer pubMutex.Unlock()

	if publisherCh != nil && !publisherCh.IsClosed() {
		return publisherCh, nil
	}

	if conn == nil {
		return nil, fmt.Errorf("RabbitMQ connection is nil")
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open publish channel")
	}

	publisherCh = ch

	go func() {
		closeChan := make(chan *amqp.Error, 1)
		closeChan = publisherCh.NotifyClose(closeChan)
		<-closeChan

		pubMutex.Lock()
		publisherCh = nil
		pubMutex.Unlock()

		logger.Logger.Warn("Publisher channerl closed, will recreate on next publish",
			zap.String("component", "rabbitmq"),
		)
	}()

	logger.Logger.Info("Publisher channel created",
		zap.String("component", "rabbitmq"),
	)

	return publisherCh, nil
}

// 发送对应的延迟消息
func PublishDelayedMessage(exchange, routingKey string,
	delay time.Duration,
	body interface{},
) error {
	ch, err := getPublisherChannel()
	if err != nil {
		return err
	}

	bodyBytes, err := json.Marshal(body)

	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 确保延迟不为负数，负数会被当作 0（立即投递）, 处理重新投递部分的错误逻辑
	delayMs := delay.Milliseconds()
	if delayMs < 0 {
		logger.Logger.Warn("Negative delay detected, setting to 0 for immediate delivery",
			zap.String("exchange", exchange),
			zap.String("routing_key", routingKey),
			zap.Duration("original_delay", delay),
		)
		delayMs = 0
	}

	err = ch.Publish(
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        bodyBytes,
			Headers: amqp.Table{
				"x-delay": delayMs, // RabbitMQ delayed message exchange 使用毫秒
			},
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)

	if err != nil {
		return fmt.Errorf("failed to publish delayed message: %w", err)
	}
	return nil
}

// PublishMessage 发送普通消息
func PublishMessage(exchange, routingKey string, body interface{}) error {
	ch, err := getPublisherChannel()
	if err != nil {
		return err
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = ch.Publish(
		exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         bodyBytes,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)

	return err
}

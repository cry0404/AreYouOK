package mq

import (
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

//底层的实现，将对应包装的部分推送即可

// 单例 + 锁，思考实现需求，为什么这样实现

var (
	conn        *amqp.Connection
	publisherCh *amqp.Channel
	pubMutex	sync.RWMutex //读写锁，读多写少
)

// PublishDelayedMessage 发送延迟消息
func getPublisherChannel() (*amqp.Channel, error) {
	//先读锁检查
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


	return nil, nil
}
func PublishDelayedMessage(exchange, routinngKey string,
	delay time.Duration,
	body interface{},
) error {

	

	return nil
}

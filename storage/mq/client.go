package mq

import (
	amqp "github.com/rabbitmq/amqp091-go"

	"AreYouOK/config"
)

func Init() error {
	var err error
	url := config.Cfg.GetRabbitMQURL()
	Conn, err := amqp.Dial(url)

	if err != nil {
		return err
	}

	_, err = Conn.Channel()

	return err
}

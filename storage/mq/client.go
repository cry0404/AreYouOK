package mq

import (
	"AreYouOK/config"
	amqp "github.com/rabbitmq/amqp091-go"
)


func Init(addr, username, password string) error {
	var err error 
	url := config.Cfg.GetRabbitMQURL()
	Conn, err := amqp.Dial(url)

	if err != nil {
		return err
	}

	_, err = Conn.Channel()

	return err

}
package storage

import (
	"AreYouOK/storage/database"
	"AreYouOK/storage/mq"
	"AreYouOK/storage/redis"
)

// 统一 init storage 层

func Init() error {
	if err := database.Init(); err != nil {
		return err
	}

	if err := redis.Init(); err != nil {
		return err
	}

	if err := mq.Init(); err != nil {
		return err
	}

	return nil
}

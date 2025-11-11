package storage

import (
	"AreYouOK/storage/database"
	"AreYouOK/storage/redis"
	"AreYouOK/storage/mq"
)

//统一 init storage 层

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
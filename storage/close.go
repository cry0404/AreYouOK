package storage

import (
	"context"
	"time"

	"go.uber.org/zap"

	"AreYouOK/pkg/logger"
	"AreYouOK/storage/database"
	"AreYouOK/storage/mq"
	"AreYouOK/storage/redis"
)

func Close() {
	// 按顺序关闭，避免消息投递错误
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	logger.Logger.Info("Closing storage connections...")

	if err := mq.Close(ctx); err != nil {
		logger.Logger.Error("Failed to close message queue", zap.Error(err))
	} else {
		logger.Logger.Info("Message queue closed successfully")
	}

	if err := redis.Close(ctx); err != nil {
		logger.Logger.Error("Failed to close Redis connection", zap.Error(err))
	} else {
		logger.Logger.Info("Redis connection closed successfully")
	}

	if err := database.Close(ctx); err != nil {
		logger.Logger.Error("Failed to close database connection", zap.Error(err))
	} else {
		logger.Logger.Info("Database connection closed successfully")
	}

	logger.Logger.Info("All storage connections closed")
}

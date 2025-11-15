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

// Close 优雅关闭所有存储连接
// 关闭顺序：MQ -> Redis -> Database
// 这样可以确保：
// 1. 先停止接收新消息（MQ）
// 2. 然后关闭缓存连接（Redis）
// 3. 最后关闭数据库连接（Database），确保数据持久化完成
func Close() {
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

package database

import (
	"AreYouOK/internal/model"
	"AreYouOK/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Migrate 运行数据库迁移，创建所有表
func Migrate() error {
	db := DB()
	if db == nil {
		return gorm.ErrInvalidDB
	}

	logger.Logger.Info("Starting database migration...")

	// 迁移所有模型
	err := db.AutoMigrate(
		&model.User{},
		&model.DailyCheckIn{},
		&model.Journey{},
		&model.NotificationTask{},
		&model.ContactAttempt{},
		&model.QuotaTransaction{},
	)

	if err != nil {
		logger.Logger.Error("Database migration failed", zap.Error(err))
		return err
	}

	logger.Logger.Info("Database migration completed successfully")
	return nil
}

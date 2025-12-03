package database

import (

	"go.uber.org/zap"
	"gorm.io/gorm"

	"AreYouOK/internal/model"
	"AreYouOK/pkg/logger"
)

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
		&model.QuotaWallet{},
	)

	if err != nil {
		logger.Logger.Error("Database migration failed", zap.Error(err))
	}

	

	logger.Logger.Info("Database migration completed successfully")
	return nil
}

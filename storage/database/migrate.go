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
	)

	if err != nil {
		logger.Logger.Error("Database migration failed", zap.Error(err))
		return err
	}

	// 确保 daily_check_ins 表的唯一约束存在（AutoMigrate 可能不会自动添加）
	// 使用 DO 块来安全地检查并添加约束
	sqlDB, err := db.DB()
	if err == nil {
		_, err = sqlDB.Exec(`
			DO $$ 
			BEGIN
				IF NOT EXISTS (
					SELECT 1 FROM pg_constraint c
					JOIN pg_class t ON c.conrelid = t.oid
					WHERE t.relname = 'daily_check_ins'
					AND c.contype = 'u'
					AND array_length(c.conkey, 1) = 2
					AND EXISTS (
						SELECT 1 FROM pg_attribute a1, pg_attribute a2
						WHERE a1.attrelid = c.conrelid AND a1.attnum = c.conkey[1] AND a1.attname = 'user_id'
						AND a2.attrelid = c.conrelid AND a2.attnum = c.conkey[2] AND a2.attname = 'check_in_date'
					)
				) THEN
					ALTER TABLE daily_check_ins 
					ADD CONSTRAINT daily_check_ins_user_id_check_in_date_key 
					UNIQUE (user_id, check_in_date);
				END IF;
			END $$;
		`)
		if err != nil {
			logger.Logger.Warn("Failed to ensure unique constraint on daily_check_ins",
				zap.Error(err),
			)
		} else {
			logger.Logger.Info("Ensured unique constraint on daily_check_ins (user_id, check_in_date)")
		}
	}

	logger.Logger.Info("Database migration completed successfully")
	return nil
}

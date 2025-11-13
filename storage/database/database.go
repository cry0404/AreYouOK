package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"AreYouOK/config"
	"AreYouOK/internal/repository/query"
	"AreYouOK/pkg/logger"
)

var (
	db     *gorm.DB
	dbOnce sync.Once
	dbErr  error
)

func Init() error {
	dbOnce.Do(func() {
		dsn := buildDSN()
		gormCfg := &gorm.Config{
			//Logger:                                   newLogger(),
			DisableForeignKeyConstraintWhenMigrating: true,
			PrepareStmt:                              true,
			SkipDefaultTransaction:                   true,
		}

		var gormDB *gorm.DB
		gormDB, dbErr = gorm.Open(postgres.Open(dsn), gormCfg)
		if dbErr != nil {
			logger.Logger.Error("Failed to open database", zap.String("dsn", "please check databse connection"), zap.Error(dbErr))
			return
		}

		sqlDB, err := gormDB.DB()
		if err != nil {
			dbErr = err
			logger.Logger.Error("Failed to get sql.DB from gorm", zap.Error(err))
			return
		}

		configureConnectionPool(sqlDB)

		if err := sqlDB.Ping(); err != nil {
			dbErr = err
			logger.Logger.Error("Failed to ping database", zap.Error(err))
			return
		}

		db = gormDB
		if err := Migrate(); err != nil {
			logger.Logger.Fatal("failed to run database migration: %w", zap.Error(err))
		}
		query.SetDefault(db)
		logger.Logger.Info("Database initialized successfully")
	})

	return dbErr
}

func DB() *gorm.DB {
	return db // 可以被导出到 repo 层做 service 中的结构
}

func Close(ctx context.Context) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- sqlDB.Close()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func buildDSN() string {
	cfg := config.Cfg
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s search_path=%s",
		cfg.PostgreSQLHost,
		cfg.PostgreSQLPort,
		cfg.PostgreSQLUser,
		cfg.PostgreSQLPassword,
		cfg.PostgreSQLDatabase,
		cfg.PostgreSQLSSLMode,
		cfg.PostgreSQLSchema,
	)
}

func configureConnectionPool(sqlDB *sql.DB) {
	cfg := config.Cfg

	sqlDB.SetMaxIdleConns(cfg.PostgreSQLMaxIdle)
	sqlDB.SetMaxOpenConns(cfg.PostgreSQLMaxOpen)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	sqlDB.SetConnMaxLifetime(2 * time.Hour)
}

// func newLogger() gormlogger.Interface {
// 	var level gormlogger.LogLevel
// 	switch config.Cfg.LoggerLevel {
// 	case "DEBUG":
// 		level = gormlogger.Info
// 	case "WARN":
// 		level = gormlogger.Warn
// 	case "ERROR":
// 		level = gormlogger.Error
// 	default:
// 		level = gormlogger.Warn
// 	}

// 	if config.Cfg.IsDevelopment() {
// 		level = gormlogger.Info
// 	}

// 	return gormlogger.New(zapWriter{}, gormlogger.Config{
// 		SlowThreshold:             200 * time.Millisecond,
// 		LogLevel:                  level,
// 		IgnoreRecordNotFoundError: true,
// 		Colorful:                  false,
// 	})
// }

// type zapWriter struct{}

// func (zapWriter) Printf(format string, args ...interface{}) {
// 	logger.Logger.Sugar().Infof(format, args...)
// }

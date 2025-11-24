package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"AreYouOK/config"
	"AreYouOK/internal/queue"
	"AreYouOK/internal/service"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/sms"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/storage"
)

func main() {

	logger.Init()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Logger.Info("Received shutdown signal",
			zap.String("signal", sig.String()),
		)
		cancel()
	}()

	if err := storage.Init(); err != nil {
		logger.Logger.Fatal("Failed to initialize storage", zap.Error(err))
	}
	defer storage.Close()

	// 初始化 Snowflake ， 根据 传入的 machinID 来初始化最好，考虑是否要单独启用环境配置
	// 考虑之后循环启动不同的 snowflakeID
	if err := snowflake.Init(config.Cfg.SnowflakeMachineID, config.Cfg.SnowflakeDataCenter); err != nil {
		logger.Logger.Fatal("Failed to initialize snowflake", zap.Error(err))
	}

	if err := sms.Init(); err != nil {
		logger.Logger.Warn("Failed to initialize SMS service", zap.Error(err))
		logger.Logger.Info("SMS service will be disabled, SMS features may not work")
	}

	//if err := voice.Init()

	// 设置通知服务, 因为所有消费者都需要这一环节
	queue.SetNotificationService(service.Notification())

	logger.Logger.Info("Worker service starting",
		zap.String("service", "areyouok-worker"),
		zap.String("environment", config.Cfg.Environment),
	)

	//启动所有的消费者部分
	queue.StartAllConsumers(ctx)

	logger.Logger.Info("Worker service shutting down gracefully")
}

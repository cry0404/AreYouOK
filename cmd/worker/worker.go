package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"AreYouOK/config"
	"AreYouOK/internal/queue"
	"AreYouOK/internal/service"
	"AreYouOK/pkg/logger"
	pkgmq "AreYouOK/pkg/mq"
	pkgtel "AreYouOK/pkg/otel"
	pkredis "AreYouOK/pkg/redis"
	pkdatabase "AreYouOK/pkg/database"
	"AreYouOK/pkg/sms"
	"AreYouOK/pkg/metrics"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/storage"
)

func main() {

	logger.Init()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化 OpenTelemetry
	otelCleanup, err := pkgtel.InitOpenTelemetry(ctx, pkgtel.Config{
		ServiceName:    "areyouok-worker",
		ServiceVersion: "1.0.0",
		Environment:    config.Cfg.Environment,
		OTLPEndpoint:   config.Cfg.OTELEXPORTERENDPOINT,
		SampleRatio:    0.1,
	})
	if err != nil {
		logger.Logger.Fatal("Failed to initialize OpenTelemetry", zap.Error(err))
	}
	defer func() {
		if err := otelCleanup(context.Background()); err != nil {
			logger.Logger.Error("Failed to shutdown OpenTelemetry", zap.Error(err))
		}
	}()

	// 初始化 SMS 相关的 OpenTelemetry 指标
	if err := metrics.InitMetrics(); err != nil {
		logger.Logger.Warn("Failed to initialize OpenTelemetry metrics", zap.Error(err))
	}

	// 初始化 Worker 相关指标
	meter := otel.Meter("areyouok-worker")
	if err := pkgmq.InitMQMetrics(meter); err != nil {
		logger.Logger.Warn("Failed to initialize RabbitMQ metrics", zap.Error(err))
	}
	if err := pkredis.InitRedisMetrics(meter); err != nil {
		logger.Logger.Warn("Failed to initialize Redis metrics", zap.Error(err))
	}
	if err := pkdatabase.InitDatabaseMetrics(meter); err != nil {
		logger.Logger.Warn("Failed to initialize database metrics", zap.Error(err))
	}

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

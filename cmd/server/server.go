package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "net/http/pprof"
	"github.com/cloudwego/hertz/pkg/app/server"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"AreYouOK/config"
	pkgmiddleware "AreYouOK/internal/middleware"
	"AreYouOK/internal/router"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/metrics"
	pkgtel "AreYouOK/pkg/otel"
	"AreYouOK/pkg/slider"
	"AreYouOK/pkg/sms"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/pkg/token"
	"AreYouOK/storage"
)

// 实际上 otel 只是包了一层类似于中间件的东西来不中
func main() {
	// 日志部分
	logger.Init()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if config.Cfg.Environment == "development" {
		go func() {
			http.ListenAndServe("localhost:6060", nil)
		}()
	}

	otelCleanup, err := pkgtel.InitOpenTelemetry(ctx, pkgtel.Config{
		ServiceName:    config.Cfg.ServiceName,
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

	if err := metrics.InitMetrics(); err != nil {
		logger.Logger.Warn("Failed to initialize OpenTelemetry metrics", zap.Error(err))
	}

	meter := otel.Meter(config.Cfg.ServiceName)
	if err := pkgmiddleware.InitMetrics(meter); err != nil {
		logger.Logger.Warn("Failed to initialize HTTP metrics", zap.Error(err))
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

	if err := snowflake.Init(config.Cfg.SnowflakeMachineID, config.Cfg.SnowflakeDataCenter); err != nil {
		logger.Logger.Fatal("Failed to initialize snowflake", zap.Error(err))
	}

	if err := sms.Init(); err != nil {
		logger.Logger.Warn("Failed to initialize SMS service", zap.Error(err))
		logger.Logger.Info("SMS service will be disabled, SMS features may not work")
	}

	if err := slider.Init(); err != nil {
		logger.Logger.Warn("Failed to initialize slider service", zap.Error(err))
		logger.Logger.Info("Slider service will be disabled, slider verification may not work")
	}

	if err := token.Init(); err != nil {
		logger.Logger.Fatal("Failed to initialize token package", zap.Error(err))
	} // token 在中间件前初始化，middleware 依赖 token

	if err := pkgmiddleware.Init(); err != nil {
		logger.Logger.Fatal("Failed to initialize middlewares", zap.Error(err))
	}

	logger.Logger.Info("Server starting",
		zap.String("service", config.Cfg.ServiceName),
		zap.String("port", config.Cfg.ServerPort),
		zap.String("environment", config.Cfg.Environment),
	)

	addr := net.JoinHostPort(config.Cfg.ServerHost, config.Cfg.ServerPort)
	h := server.Default(server.WithHostPorts(addr))

	router.Register(h)

	go func() {
		<-ctx.Done()
		logger.Logger.Info("Initiating graceful shutdown...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.Shutdown(shutdownCtx); err != nil {
			logger.Logger.Error("Failed to shutdown HTTP server", zap.Error(err))
		}
	}()

	logger.Logger.Info("HTTP server listening", zap.String("addr", addr))

	h.Spin()

	logger.Logger.Info("Server shutting down gracefully")
}

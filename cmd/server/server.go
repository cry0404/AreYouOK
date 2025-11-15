package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"go.uber.org/zap"

	"AreYouOK/config"
	"AreYouOK/internal/middleware"
	"AreYouOK/internal/router"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/slider"
	"AreYouOK/pkg/sms"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/pkg/token"
	"AreYouOK/storage"
)

func main() {
	// 日志部分
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

	// 初始化存储层，记得关闭外部连接
	if err := storage.Init(); err != nil {
		logger.Logger.Fatal("Failed to initialize storage", zap.Error(err))
	}

	defer storage.Close()

	if err := snowflake.Init(); err != nil {
		logger.Logger.Fatal("Failed to initialize snowflake", zap.Error(err))
	}

	// 初始化 SMS 服务
	if err := sms.Init(); err != nil {
		logger.Logger.Warn("Failed to initialize SMS service", zap.Error(err))
		logger.Logger.Info("SMS service will be disabled, SMS features may not work")
	}

	// 初始化滑块验证服务
	if err := slider.Init(); err != nil {
		logger.Logger.Warn("Failed to initialize slider service", zap.Error(err))
		logger.Logger.Info("Slider service will be disabled, slider verification may not work")
	}

	if err := token.Init(); err != nil {
		logger.Logger.Fatal("Failed to initialize token package", zap.Error(err))
	} // token 在中间件前初始化，middleware 依赖 token

	// 初始化中间件
	if err := middleware.Init(); err != nil {
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

	// 优雅关闭：在单独的 goroutine 中监听关闭信号并调用 Shutdown
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

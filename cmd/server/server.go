package main

import (
	"AreYouOK/config"
	"AreYouOK/internal/middleware"
	"AreYouOK/internal/router"
	"AreYouOK/pkg/logger"
	"AreYouOK/storage"
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"go.uber.org/zap"
)

func main() {
	//日志部分
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

	//初始化存储层
	if err := storage.Init(); err != nil {
		logger.Logger.Fatal("Failed to load something ", zap.Error(err))
	}

	defer storage.Close()

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

	go func() {
		<-ctx.Done()
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

package main

import (
	"AreYouOK/config"
	"AreYouOK/internal/router"
	"AreYouOK/pkg/logger"
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
	// 1. 初始化日志（必须在最前面，因为其他初始化可能需要日志）
	logger.Init()
	defer logger.Sync()

	// 2. 优雅退出处理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 监听系统信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Logger.Info("Received shutdown signal",
			zap.String("signal", sig.String()),
		)
		cancel()
	}()

	// 3. 初始化其他组件（数据库、Redis、RabbitMQ等）
	// TODO: 在这里初始化数据库连接
	// TODO: 在这里初始化 Redis 连接
	// TODO: 在这里初始化 RabbitMQ 连接

	logger.Logger.Info("Server starting",
		zap.String("service", config.Cfg.ServiceName),
		zap.String("port", config.Cfg.ServerPort),
		zap.String("environment", config.Cfg.Environment),
	)

	// 4. 启动 Hertz HTTP 服务
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

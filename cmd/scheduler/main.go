package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"AreYouOK/config"
	"AreYouOK/internal/schedule"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/storage"
)


func main() {

	logger.Init()
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Logger.Info("Scheduler received shutdown signal",
			zap.String("signal", sig.String()),
		)
		cancel()
	}()


	if err := storage.Init(); err != nil {
		logger.Logger.Fatal("Failed to initialize storage for scheduler", zap.Error(err))
	}
	defer storage.Close()

	// 考虑与 worker 和 server 作区分
	if err := snowflake.Init(config.Cfg.SnowflakeMachineID, config.Cfg.SnowflakeDataCenter); err != nil {
		logger.Logger.Fatal("Failed to initialize snowflake for scheduler", zap.Error(err))
	}

	logger.Logger.Info("Scheduler service starting",
		zap.String("service", "areyouok-scheduler"),
		zap.String("environment", config.Cfg.Environment),
	)


	go runDailyCheckinLoop(ctx)
	go runJourneyTimeoutLoop(ctx)
	go runOverdueJourneyLoop(ctx)


	<-ctx.Done()

	logger.Logger.Info("Scheduler service shutting down gracefully")
}

// runDailyCheckinLoop 每天固定时间执行一次每日打卡调度
// 当前实现：每天本地时间 00:05 触发一次
func runDailyCheckinLoop(ctx context.Context) {
	s := schedule.GetScheduler()

	// 在 development 环境下，为了方便本地调试，将每日调度改为每 1 分钟执行一次
	if config.Cfg.Environment == "development" {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		logger.Logger.Info("Daily check-in scheduler running in development mode with 1m interval")

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				if err := s.ScheduleDailyCheckIns(runCtx); err != nil {
					logger.Logger.Error("Daily check-in scheduler run failed (development interval)", zap.Error(err))
				}
				cancel()
			}
		}
	}

	for {
		// 计算下一次运行时间（今天/明天的 00:05）
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), 0, 5, 0, 0, now.Location())
		if !next.After(now) {
			// 如果已经过了今天 00:05，则设置为明天
			next = next.Add(24 * time.Hour)
		}

		delay := time.Until(next)
		logger.Logger.Info("Scheduled next daily check-in run",
			zap.Time("now", now),
			zap.Time("next_run", next),
			zap.Duration("delay", delay),
		)

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			if err := s.ScheduleDailyCheckIns(runCtx); err != nil {
				logger.Logger.Error("Daily check-in scheduler run failed", zap.Error(err))
			}
			cancel()
		}
	}
}

// runJourneyTimeoutLoop 周期性扫描即将到期的行程并投递超时消息
// 当前实现：每 5 分钟扫描未来 10 分钟内即将到期的行程。
func runJourneyTimeoutLoop(ctx context.Context) {
	js := schedule.GetJourneyScheduler()

	interval := 5 * time.Minute
	if config.Cfg.Environment == "development" {
		interval = 1 * time.Minute
		logger.Logger.Info("Journey timeout loop running in development mode with 1m interval")
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			if err := js.CheckJourneyTimeouts(runCtx, 10*time.Minute); err != nil {
				logger.Logger.Error("Journey timeout check run failed", zap.Error(err))
			}
			cancel()
		}
	}
}

// runOverdueJourneyLoop 周期性执行行程超时补偿任务
// 当前实现：每 1 小时扫描一次已超时但状态仍为 ongoing 的行程。
func runOverdueJourneyLoop(ctx context.Context) {
	js := schedule.GetJourneyScheduler()

	interval := 1 * time.Hour
	if config.Cfg.Environment == "development" {
		interval = 1 * time.Minute
		logger.Logger.Info("Overdue journey loop running in development mode with 1m interval")
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			if err := js.CheckOverdueJourneys(runCtx); err != nil {
				logger.Logger.Error("Overdue journey check run failed", zap.Error(err))
			}
			cancel()
		}
	}
}

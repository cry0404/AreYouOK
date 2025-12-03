package schedule

// 行程调度器：定期扫描即将到期的行程，触发超时检查
// 解决 RabbitMQ 延迟消息最多只能延迟1天的问题

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"AreYouOK/internal/model"
	"AreYouOK/internal/queue"
	"AreYouOK/internal/repository/query"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"
)

var (
	journeySchedulerOnce sync.Once
	journeySchedulerInst *JourneyScheduler
)

// JourneyScheduler 行程调度器
type JourneyScheduler struct {
	logger               *zap.Logger
	timeoutJobRunning    bool
	timeoutJobMu         sync.Mutex
	lastTimeoutCheckTime time.Time
}

// GetJourneyScheduler 获取行程调度器单例
func GetJourneyScheduler() *JourneyScheduler {
	journeySchedulerOnce.Do(func() {
		journeySchedulerInst = &JourneyScheduler{
			logger: logger.Logger,
		}
	})
	return journeySchedulerInst
}

// CheckJourneyTimeouts 检查即将到期的行程（定时任务调用）
// 扫描预计返回时间在未来 timeWindow 内的行程
// timeWindow: 扫描时间窗口，例如 10 分钟
func (s *JourneyScheduler) CheckJourneyTimeouts(ctx context.Context, timeWindow time.Duration) error {
	s.timeoutJobMu.Lock()
	if s.timeoutJobRunning {
		s.timeoutJobMu.Unlock()
		s.logger.Info("Journey timeout check job already running, skipping")
		return nil
	}
	s.timeoutJobRunning = true
	s.timeoutJobMu.Unlock()

	defer func() {
		s.timeoutJobMu.Lock()
		s.timeoutJobRunning = false
		s.timeoutJobMu.Unlock()
	}()

	startTime := time.Now()
	s.lastTimeoutCheckTime = startTime

	s.logger.Info("Starting journey timeout check",
		zap.Time("start_time", startTime),
		zap.Duration("time_window", timeWindow),
	)

	// 计算时间范围：当前时间到当前时间 + timeWindow
	now := time.Now()
	checkUntil := now.Add(timeWindow)

	// 查询预计返回时间在 [now, checkUntil] 范围内的进行中行程
	// 这些行程即将在 timeWindow 内到期
	journeys, err := query.Journey.WithContext(ctx).
		Where(query.Journey.Status.Eq(string(model.JourneyStatusOngoing))).
		Where(query.Journey.ExpectedReturnTime.Gte(now)).
		Where(query.Journey.ExpectedReturnTime.Lte(checkUntil)).
		Find()

	if err != nil {
		s.logger.Error("Failed to query ongoing journeys",
			zap.Error(err),
		)
		return fmt.Errorf("failed to query ongoing journeys: %w", err)
	}

	if len(journeys) == 0 {
		s.logger.Info("No journeys approaching timeout",
			zap.Duration("time_window", timeWindow),
		)
		return nil
	}

	s.logger.Info("Found journeys approaching timeout",
		zap.Int("journey_count", len(journeys)),
		zap.Duration("time_window", timeWindow),
	)

	// 为每个行程生成超时消息
	// 注意：这里不立即触发超时，而是发送延迟消息，延迟到预计返回时间 + 10分钟
	// 这样可以确保在预计返回时间后10分钟才触发超时检查
	var wg sync.WaitGroup
	errors := make([]error, 0)
	errorsMu := sync.Mutex{}

	for _, journey := range journeys {
		wg.Add(1)
		go func(j *model.Journey) {
			defer wg.Done()

			// 计算超时延迟时间：预计返回时间 + 10分钟 - 当前时间（第二阶段：通知紧急联系人）
			timeoutDelay := j.ExpectedReturnTime.Add(10 * time.Minute).Sub(now)
			if timeoutDelay > 24*time.Hour {
				// 极端情况保护，正常不会出现
				s.logger.Warn("Journey delay exceeds 24 hours, skipping",
					zap.Int64("journey_id", j.ID),
					zap.Duration("timeout_delay", timeoutDelay),
				)
				return
			}

			// 查询用户 public_id
			user, err := query.User.WithContext(ctx).
				Where(query.User.ID.Eq(j.UserID)).
				First()
			if err != nil {
				s.logger.Error("Failed to query user",
					zap.Int64("journey_id", j.ID),
					zap.Int64("user_id", j.UserID),
					zap.Error(err),
				)
				errorsMu.Lock()
				errors = append(errors, fmt.Errorf("failed to query user for journey %d: %w", j.ID, err))
				errorsMu.Unlock()
				return
			}

			// 第一阶段：发送提醒消息给用户本人
			// 规则：
			// - 只要还没到 expected_return_time + 10 分钟，并且 ReminderSentAt 为空，就一定要发一次
			// - 如果已经过了预计返回时间，就立刻发（delay=0）
			if j.ReminderSentAt == nil && now.Before(j.ExpectedReturnTime.Add(10*time.Minute)) {
				reminderDelay := j.ExpectedReturnTime.Sub(now)
				if reminderDelay < 0 {
					reminderDelay = 0
				}

				reminderMsgID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
				if err != nil {
					s.logger.Error("Failed to generate reminder message ID",
						zap.Int64("journey_id", j.ID),
						zap.Error(err),
					)
				} else {
					reminderMsg := model.JourneyReminderMessage{
						MessageID:    fmt.Sprintf("journey_reminder_%d", reminderMsgID),
						ScheduledAt:  now.Format(time.RFC3339),
						JourneyID:    j.ID,
						UserID:       user.PublicID,
						DelaySeconds: int(reminderDelay.Seconds()),
					}

					if err := queue.PublishJourneyReminder(reminderMsg); err != nil {
						s.logger.Error("Failed to publish journey reminder message",
							zap.Int64("journey_id", j.ID),
							zap.Int64("user_id", user.PublicID),
							zap.Error(err),
						)
					} else {
						s.logger.Info("Published journey reminder message",
							zap.Int64("journey_id", j.ID),
							zap.Int64("user_id", user.PublicID),
							zap.Duration("delay", reminderDelay),
							zap.Time("expected_return_time", j.ExpectedReturnTime),
						)
					}
				}
			}

			// 第二阶段：发送超时消息（通知紧急联系人）
			// 如果超时延迟时间 <= 0，说明已经超时，立即处理
			if timeoutDelay <= 0 {
				timeoutDelay = 0
			}

			timeoutMsgID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
			if err != nil {
				s.logger.Error("Failed to generate timeout message ID",
					zap.Int64("journey_id", j.ID),
					zap.Error(err),
				)
				errorsMu.Lock()
				errors = append(errors, fmt.Errorf("failed to generate message ID for journey %d: %w", j.ID, err))
				errorsMu.Unlock()
				return
			}

			timeoutMsg := model.JourneyTimeoutMessage{
				MessageID:    fmt.Sprintf("journey_timeout_%d", timeoutMsgID),
				ScheduledAt:  now.Format(time.RFC3339),
				JourneyID:    j.ID,
				UserID:       user.PublicID,
				DelaySeconds: int(timeoutDelay.Seconds()),
			}

			if err := queue.PublishJourneyTimeout(timeoutMsg); err != nil {
				s.logger.Error("Failed to publish journey timeout message",
					zap.Int64("journey_id", j.ID),
					zap.Int64("user_id", user.PublicID),
					zap.Error(err),
				)
				errorsMu.Lock()
				errors = append(errors, fmt.Errorf("failed to publish timeout message for journey %d: %w", j.ID, err))
				errorsMu.Unlock()
				return
			}

			s.logger.Info("Published journey timeout message",
				zap.Int64("journey_id", j.ID),
				zap.Int64("user_id", user.PublicID),
				zap.Duration("timeout_delay", timeoutDelay),
				zap.Time("expected_return_time", j.ExpectedReturnTime),
			)
		}(journey)
	}

	wg.Wait()

	duration := time.Since(startTime)
	s.logger.Info("Journey timeout check completed",
		zap.Duration("duration", duration),
		zap.Int("journey_count", len(journeys)),
		zap.Int("error_count", len(errors)),
	)

	if len(errors) > 0 {
		return fmt.Errorf("journey timeout check completed with %d errors", len(errors))
	}

	return nil
}

// CheckOverdueJourneys 检查已超时的行程（用于补偿机制）
// 查询预计返回时间已过且状态仍为 ongoing 的行程
func (s *JourneyScheduler) CheckOverdueJourneys(ctx context.Context) error {
	s.logger.Info("Starting overdue journey check")

	// 查询已超时的行程（预计返回时间 < 当前时间 - 10分钟）
	journeys, err := query.Journey.WithContext(ctx).
		Where(query.Journey.Status.Eq(string(model.JourneyStatusOngoing))).
		Where(query.Journey.ExpectedReturnTime.Lt(time.Now().Add(-10 * time.Minute))).
		Find()

	if err != nil {
		s.logger.Error("Failed to query overdue journeys",
			zap.Error(err),
		)
		return fmt.Errorf("failed to query overdue journeys: %w", err)
	}

	if len(journeys) == 0 {
		s.logger.Info("No overdue journeys found")
		return nil
	}

	s.logger.Warn("Found overdue journeys",
		zap.Int("journey_count", len(journeys)),
	)

	// 为每个超时行程立即发送超时消息（延迟0秒）
	var wg sync.WaitGroup
	errors := make([]error, 0)
	errorsMu := sync.Mutex{}

	for _, journey := range journeys {
		wg.Add(1)
		go func(j *model.Journey) {
			defer wg.Done()

			// 查询用户 public_id
			user, err := query.User.WithContext(ctx).
				Where(query.User.ID.Eq(j.UserID)).
				First()
			if err != nil {
				s.logger.Error("Failed to query user",
					zap.Int64("journey_id", j.ID),
					zap.Error(err),
				)
				errorsMu.Lock()
				errors = append(errors, fmt.Errorf("failed to query user for journey %d: %w", j.ID, err))
				errorsMu.Unlock()
				return
			}

			// 生成 MessageID
			messageID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
			if err != nil {
				s.logger.Error("Failed to generate message ID",
					zap.Int64("journey_id", j.ID),
					zap.Error(err),
				)
				errorsMu.Lock()
				errors = append(errors, fmt.Errorf("failed to generate message ID for journey %d: %w", j.ID, err))
				errorsMu.Unlock()
				return
			}

			// 构建超时消息（延迟0秒，立即处理）
			timeoutMsg := model.JourneyTimeoutMessage{
				MessageID:    fmt.Sprintf("journey_timeout_%d", messageID),
				ScheduledAt:  time.Now().Format(time.RFC3339),
				JourneyID:    j.ID,
				UserID:       user.PublicID,
				DelaySeconds: 0, // 立即处理
			}

			// 发布消息
			if err := queue.PublishJourneyTimeout(timeoutMsg); err != nil {
				s.logger.Error("Failed to publish overdue journey timeout message",
					zap.Int64("journey_id", j.ID),
					zap.Int64("user_id", user.PublicID),
					zap.Error(err),
				)
				errorsMu.Lock()
				errors = append(errors, fmt.Errorf("failed to publish timeout message for journey %d: %w", j.ID, err))
				errorsMu.Unlock()
				return
			}

			s.logger.Info("Published overdue journey timeout message",
				zap.Int64("journey_id", j.ID),
				zap.Int64("user_id", user.PublicID),
				zap.Time("expected_return_time", j.ExpectedReturnTime),
			)
		}(journey)
	}

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("overdue journey check completed with %d errors", len(errors))
	}

	return nil
}

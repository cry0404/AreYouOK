package schedule

// 打卡调度器：每天 00:00 扫描开启打卡的用户，生成提醒和超时消息

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"AreYouOK/internal/cache"
	"AreYouOK/internal/model"
	"AreYouOK/internal/queue"
	"AreYouOK/internal/repository/query"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"
)

var (
	schedulerOnce sync.Once
	schedulerInst *CheckInScheduler
)

type CheckInScheduler struct {
	logger                   *zap.Logger
	reminderJobRunning       bool
	reminderJobMu            sync.Mutex
	lastReminderJobTime      time.Time
	lastTimeoutCheckJobTime  time.Time
}

func GetScheduler() *CheckInScheduler {
	schedulerOnce.Do(func() {
		schedulerInst = &CheckInScheduler{
			logger: logger.Logger,
		}
	})
	return schedulerInst
}

// ScheduleDailyCheckIns 每天 00:00 调度打卡提醒和超时消息
func (s *CheckInScheduler) ScheduleDailyCheckIns(ctx context.Context) error {
	s.reminderJobMu.Lock()
	if s.reminderJobRunning {
		s.reminderJobMu.Unlock()
		s.logger.Info("Reminder job already running, skipping")
		return nil
	}
	s.reminderJobRunning = true
	s.reminderJobMu.Unlock()

	defer func() {
		s.reminderJobMu.Lock()
		s.reminderJobRunning = false
		s.reminderJobMu.Unlock()
	}()

	startTime := time.Now()
	s.lastReminderJobTime = startTime

	s.logger.Info("Starting daily check-in scheduler",
		zap.Time("start_time", startTime),
	)

	// 获取今天的日期
	today := startTime.Format("2006-01-02")
	batchID, err := snowflake.NextID()
	if err != nil {
		s.logger.Error("Failed to generate batch ID", zap.Error(err))
		return fmt.Errorf("failed to generate batch ID: %w", err)
	}

	// 查询开启打卡且已激活的用户
	users, err := query.User.WithContext(ctx).
		Where(query.User.DailyCheckInEnabled.Is(true)).
		Where(query.User.Status.Eq(string(model.UserStatusActive))).
		Find()

	if err != nil {
		s.logger.Error("Failed to query users", zap.Error(err))
		return fmt.Errorf("failed to query users: %w", err)
	}

	if len(users) == 0 {
		s.logger.Info("No users with daily check-in enabled")
		return nil
	}

	s.logger.Info("Found users to schedule",
		zap.Int("user_count", len(users)),
	)

	// 按提醒时间分组用户
	timeGroups := make(map[string][]*model.User)
	for _, user := range users {
		remindAt := user.DailyCheckInRemindAt
		if remindAt == "" {
			remindAt = "20:00:00" // 默认提醒时间
		}
		timeGroups[remindAt] = append(timeGroups[remindAt], user)
	}

	var wg sync.WaitGroup
	errors := make([]error, 0)
	errorsMu := sync.Mutex{}

	// 为每个时间段生成消息
	for remindAt, groupUsers := range timeGroups {
		// 检查今天是否已经投放过
		shouldSkip := true
		for _, user := range groupUsers {
			scheduled, err := cache.IsCheckinScheduled(ctx, today, user.PublicID)
			if err != nil {
				s.logger.Warn("Failed to check checkin scheduled status",
					zap.Int64("user_id", user.PublicID),
					zap.Error(err),
				)
				// 如果检查失败，继续检查其他用户
				continue
			}
			if !scheduled {
				shouldSkip = false
				break
			}
		}

		if shouldSkip {
			s.logger.Info("All users in group already scheduled, skipping",
				zap.String("remind_at", remindAt),
				zap.Int("user_count", len(groupUsers)),
			)
			continue
		}

		wg.Add(1)
		go func(remindAt string, users []*model.User) {
			defer wg.Done()

			if err := s.scheduleTimeGroup(ctx, today, batchID, remindAt, users); err != nil {
				errorsMu.Lock()
				errors = append(errors, err)
				errorsMu.Unlock()
				s.logger.Error("Failed to schedule time group",
					zap.String("remind_at", remindAt),
					zap.Error(err),
				)
			}
		}(remindAt, groupUsers)
	}

	wg.Wait()

	duration := time.Since(startTime)
	s.logger.Info("Daily check-in scheduler completed",
		zap.Duration("duration", duration),
		zap.Int("error_count", len(errors)),
	)

	if len(errors) > 0 {
		return fmt.Errorf("scheduler completed with %d errors", len(errors))
	}

	return nil
}

// scheduleTimeGroup 为同一提醒时间的用户组生成消息
func (s *CheckInScheduler) scheduleTimeGroup(
	ctx context.Context,
	checkInDate string,
	batchID int64,
	remindAt string,
	users []*model.User,
) error {
	userIDs := make([]int64, len(users))
	snapshots := make(map[string]model.UserSettingSnapshot)

	for i, user := range users {
		userIDs[i] = user.PublicID

		// 构建用户设置快照
		snapshot := model.UserSettingSnapshot{
			RemindAt:   user.DailyCheckInRemindAt,
			Deadline:   user.DailyCheckInDeadline,
			GraceUntil: user.DailyCheckInGraceUntil,
			Timezone:   user.Timezone,
		}
		snapshots[fmt.Sprintf("%d", user.PublicID)] = snapshot

		// 预标记为已投放（即使后续失败也会有幂等性保证）
		cache.MarkCheckinScheduled(ctx, checkInDate, user.PublicID)
	}

	// 解析提醒时间
	reminderTime, err := time.Parse("15:04:05", remindAt)
	if err != nil {
		s.logger.Error("Failed to parse remindAt",
			zap.String("remind_at", remindAt),
			zap.Error(err),
		)
		reminderTime, _ = time.Parse("15:04:05", "20:00:00")
	}

	// 计算延迟时间（从当前时间到提醒时间）
	now := time.Now()
	todayRemindTime := time.Date(now.Year(), now.Month(), now.Day(),
		reminderTime.Hour(), reminderTime.Minute(), reminderTime.Second(), 0, now.Location())

	// 如果提醒时间已过，调整到明天
	if todayRemindTime.Before(now) {
		todayRemindTime = todayRemindTime.Add(24 * time.Hour)
	}

	reminderDelay := time.Until(todayRemindTime)

	// 生成 MessageID
	messageID, err := snowflake.NextID()
	if err != nil {
		s.logger.Error("Failed to generate message ID", zap.Error(err))
		return err
	}

	// 生成提醒消息
	reminderMsg := model.CheckInReminderMessage{
		MessageID:    fmt.Sprintf("ci_reminder_%d", messageID),
		BatchID:      fmt.Sprintf("%d", batchID),
		CheckInDate:  checkInDate,
		ScheduledAt:  now.Format(time.RFC3339),
		UserIDs:      userIDs,
		UserSettings: snapshots,
		DelaySeconds: int(reminderDelay.Seconds()),
	}

	if err := queue.PublishCheckInReminder(reminderMsg); err != nil {
		s.logger.Error("Failed to publish reminder message",
			zap.String("remind_at", remindAt),
			zap.Int("user_count", len(users)),
			zap.Error(err),
		)
		return err
	}

	// 计算超时时间（截止时间 + 1小时）
	deadline := users[0].DailyCheckInDeadline
	if deadline == "" {
		deadline = "21:00:00"
	}

	deadlineTime, err := time.Parse("15:04:05", deadline)
	if err != nil {
		s.logger.Error("Failed to parse deadline",
			zap.String("deadline", deadline),
			zap.Error(err),
		)
		deadlineTime, _ = time.Parse("15:04:05", "21:00:00")
	}

	// 超时时间 = 截止时间 + 1小时宽限期
	todayDeadline := time.Date(now.Year(), now.Month(), now.Day(),
		deadlineTime.Hour(), deadlineTime.Minute(), deadlineTime.Second(), 0, now.Location())
	timeoutTime := todayDeadline.Add(1 * time.Hour)

	// 如果超时时间已过，调整到明天
	if timeoutTime.Before(now) {
		timeoutTime = timeoutTime.Add(24 * time.Hour)
	}

	timeoutDelay := time.Until(timeoutTime)

	// 生成 MessageID for timeout
	timeoutMessageID, err := snowflake.NextID()
	if err != nil {
		s.logger.Error("Failed to generate timeout message ID", zap.Error(err))
		return err
	}

	// 生成超时消息
	timeoutMsg := model.CheckInTimeoutMessage{
		MessageID:    fmt.Sprintf("ci_timeout_%d", timeoutMessageID),
		BatchID:      fmt.Sprintf("%d", batchID),
		CheckInDate:  checkInDate,
		ScheduledAt:  now.Format(time.RFC3339),
		UserIDs:      userIDs,
		DelaySeconds: int(timeoutDelay.Seconds()),
	}

	if err := queue.PublishCheckInTimeout(timeoutMsg); err != nil {
		s.logger.Error("Failed to publish timeout message",
			zap.String("remind_at", remindAt),
			zap.Int("user_count", len(users)),
			zap.Error(err),
		)
		return err
	}

	s.logger.Info("Scheduled time group successfully",
		zap.String("remind_at", remindAt),
		zap.Int("user_count", len(users)),
		zap.Duration("reminder_delay", reminderDelay),
		zap.Duration("timeout_delay", timeoutDelay),
	)

	return nil
}

// MarkCheckinScheduled marks a user's check-in as scheduled for a date
func (s *CheckInScheduler) MarkCheckinScheduled(ctx context.Context, date string, userID int64) error {
	return cache.MarkCheckinScheduled(ctx, date, userID)
}

// CheckIfScheduled checks if a user's check-in has been scheduled for a date
func (s *CheckInScheduler) CheckIfScheduled(ctx context.Context, date string, userID int64) (bool, error) {
	return cache.IsCheckinScheduled(ctx, date, userID)
}

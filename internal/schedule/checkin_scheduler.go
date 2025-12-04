package schedule

// 打卡调度器：每天 00:00 扫描开启打卡的用户，生成提醒和超时消息

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"AreYouOK/config"
	"AreYouOK/internal/cache"
	"AreYouOK/internal/model"
	"AreYouOK/internal/queue"
	"AreYouOK/internal/repository/query"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/storage/database"
)

var (
	schedulerOnce sync.Once
	schedulerInst *CheckInScheduler
)

type CheckInScheduler struct {
	logger              *zap.Logger
	reminderJobRunning  bool
	reminderJobMu       sync.Mutex
	lastReminderJobTime time.Time
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


	today := startTime.Format("2006-01-02")
	batchID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
	if err != nil {
		s.logger.Error("Failed to generate batch ID", zap.Error(err))
		return fmt.Errorf("failed to generate batch ID: %w", err)
	}


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


	for remindAt, groupUsers := range timeGroups {

		shouldSkip := true

		for _, user := range groupUsers {
			reminderScheduled, err := cache.IsReminderScheduled(ctx, today, user.PublicID)
			if err != nil {
				s.logger.Warn("Failed to check reminder scheduled status",
					zap.Int64("user_id", user.PublicID),
					zap.Error(err),
				)

				continue
			}

			timeoutScheduled, err := cache.IsTimeoutScheduled(ctx, today, user.PublicID)
			if err != nil {
				s.logger.Warn("Failed to check timeout scheduled status",
					zap.Int64("user_id", user.PublicID),
					zap.Error(err),
				)
				continue
			}

			// 如果 reminder 或 timeout 任一未发布，则需要调度
			if !reminderScheduled || !timeoutScheduled {
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

	checkInDateParsed, err := time.Parse("2006-01-02", checkInDate)
	if err != nil {
		s.logger.Error("Failed to parse checkInDate", zap.String("check_in_date", checkInDate), zap.Error(err))
		return err
	}


	db := database.DB().WithContext(ctx)
	q := query.Use(db)


	userDBIDs := make([]int64, len(users))
	userMap := make(map[int64]*model.User) // DB ID -> User
	for i, user := range users {
		userDBIDs[i] = user.ID
		userMap[user.ID] = user
	}

	// 打卡就代表有两个任务
	// 查询今日已有 check_in_reminder 或 check_in_timeout 任务的用户，避免重复投递
	// 反正有其中之一就证明今日打卡过了
	existingTasks, err := q.NotificationTask.WithContext(ctx).
		Where(q.NotificationTask.UserID.In(userDBIDs...)).
		Where(q.NotificationTask.Category.In(
			string(model.NotificationCategoryCheckInReminder),
			string(model.NotificationCategoryCheckInTimeout),
		)).
		Where(q.NotificationTask.ScheduledAt.Gte(checkInDateParsed)).
		Where(q.NotificationTask.ScheduledAt.Lt(checkInDateParsed.AddDate(0, 0, 1))).
		Find()

	if err != nil {
		s.logger.Warn("Failed to query existing notification tasks, proceeding with all users",
			zap.Error(err),
		)

	}


	existingUserIDs := make(map[int64]bool)
	for _, task := range existingTasks {
		existingUserIDs[task.UserID] = true
	}

	var filteredUsers []*model.User
	for _, user := range users {
		if existingUserIDs[user.ID] {
			s.logger.Debug("User already has reminder task today, skipping",
				zap.Int64("user_id", user.PublicID),
				zap.String("check_in_date", checkInDate),
			)
			continue
		}
		filteredUsers = append(filteredUsers, user)
	}

	if len(filteredUsers) == 0 {
		s.logger.Info("All users in group already have reminder tasks today, skipping message publish",
			zap.String("remind_at", remindAt),
			zap.Int("original_count", len(users)),
			zap.String("check_in_date", checkInDate),
		)
		return nil
	}

	s.logger.Info("Filtered users for reminder scheduling",
		zap.String("remind_at", remindAt),
		zap.Int("original_count", len(users)),
		zap.Int("filtered_count", len(filteredUsers)),
		zap.Int("skipped_count", len(users)-len(filteredUsers)),
	)


	userIDs := make([]int64, len(filteredUsers))
	snapshots := make(map[string]model.UserSettingSnapshot)

	for i, user := range filteredUsers {
		userIDs[i] = user.PublicID

		// 构建用户设置快照，便于投递时可以查询对比，类似于 cas 的原理
		snapshot := model.UserSettingSnapshot{
			RemindAt:   user.DailyCheckInRemindAt,
			Deadline:   user.DailyCheckInDeadline,
			GraceUntil: user.DailyCheckInGraceUntil,
			Timezone:   user.Timezone,
		}
		snapshots[fmt.Sprintf("%d", user.PublicID)] = snapshot
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

	now := time.Now()
	todayRemindTime := time.Date(now.Year(), now.Month(), now.Day(),
		reminderTime.Hour(), reminderTime.Minute(), reminderTime.Second(), 0, now.Location())

	if todayRemindTime.Before(now) {
		graceUntil := users[0].DailyCheckInGraceUntil
		if graceUntil != "" {
			if graceTime, err := time.Parse("15:04:05", graceUntil); err == nil {
				graceUntilTime := time.Date(now.Year(), now.Month(), now.Day(),
					graceTime.Hour(), graceTime.Minute(), graceTime.Second(), 0, now.Location())

				// 如果还在宽限窗口内
				if now.Before(graceUntilTime) {
					if config.Cfg.Environment == "development" {
						todayRemindTime = now.Add(1 * time.Minute)
					} else {

						todayRemindTime = now
					}
				} else {
					s.logger.Info("Skipped scheduling reminder: remind_at already passed grace window",
						zap.String("remind_at", remindAt),
						zap.String("grace_until", graceUntil),
						zap.Time("now", now),
					)
					return nil
				}
			}
		} else {
			if config.Cfg.Environment == "development" {
				todayRemindTime = now.Add(1 * time.Minute)
			} else {
				todayRemindTime = todayRemindTime.Add(24 * time.Hour)
			}
		}
	}

	reminderDelay := time.Until(todayRemindTime)
	if reminderDelay < 0 {
		reminderDelay = 0
	}


	messageID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
	if err != nil {
		s.logger.Error("Failed to generate message ID", zap.Error(err))
		return err
	}

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
			zap.Int("user_count", len(filteredUsers)),
			zap.Error(err),
		)
		return err
	}


	for _, user := range filteredUsers {
		if err := cache.MarkReminderScheduled(ctx, checkInDate, user.PublicID); err != nil {
			s.logger.Warn("Failed to mark reminder scheduled after publishing message",
				zap.Int64("user_id", user.PublicID),
				zap.Error(err),
			)
		}
	}

	deadline := filteredUsers[0].DailyCheckInDeadline
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

	// 超时时间 = 截止时间 
	todayDeadline := time.Date(now.Year(), now.Month(), now.Day(),
		deadlineTime.Hour(), deadlineTime.Minute(), deadlineTime.Second(), 0, now.Location())
	timeoutTime := todayDeadline

	timeoutDelay := time.Until(timeoutTime)

	//超时时间已过后，就立刻触发打卡机制, 每天只会有 1 次打卡，数据库是根据 date 和人来设置 unique Index 的
	if timeoutDelay < 0 {
		timeoutDelay = 0
		s.logger.Info("Timeout time has passed, will trigger immediately",
			zap.String("check_in_date", checkInDate),
			zap.Time("timeout_time", timeoutTime),
			zap.Time("now", now),
		)
	}


	timeoutMessageID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
	if err != nil {
		s.logger.Error("Failed to generate timeout message ID", zap.Error(err))
		return err
	}

	
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
			zap.Int("user_count", len(filteredUsers)),
			zap.Error(err),
		)
		// Timeout 消息发布失败时，不返回错误，允许下次重试
		// 但 reminder 已经发布成功，所以不会影响 reminder 的标记
	} else {
		// Timeout 消息发送成功后才标记为已投放
		for _, user := range filteredUsers {
			if err := cache.MarkTimeoutScheduled(ctx, checkInDate, user.PublicID); err != nil {
				s.logger.Warn("Failed to mark timeout scheduled after publishing message",
					zap.Int64("user_id", user.PublicID),
					zap.Error(err),
				)
				// 标记失败不影响主流程，因为 MQ 消息已发送
			}
		}
	}

	s.logger.Info("Scheduled time group successfully",
		zap.String("remind_at", remindAt),
		zap.Int("user_count", len(filteredUsers)),
		zap.Duration("reminder_delay", reminderDelay),
		zap.Duration("timeout_delay", timeoutDelay),
	)

	return nil
}

// MarkCheckinScheduled marks a user's check-in as scheduled for a date
// 同时标记 reminder 和 timeout 已调度
func (s *CheckInScheduler) MarkCheckinScheduled(ctx context.Context, date string, userID int64) error {
	return cache.MarkCheckinScheduled(ctx, date, userID)
}

// CheckIfScheduled checks if a user's check-in has been scheduled for a date
// 检查 reminder 和 timeout 是否都已调度
func (s *CheckInScheduler) CheckIfScheduled(ctx context.Context, date string, userID int64) (bool, error) {
	return cache.IsCheckinScheduled(ctx, date, userID)
}

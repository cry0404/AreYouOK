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

	// 获取今天的日期
	today := startTime.Format("2006-01-02")
	batchID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
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
		// 检查今天是否已经投放过（通过 Redis 标记）
		shouldSkip := true

		for _, user := range groupUsers {
			// 分别检查 reminder 和 timeout 是否已发布
			reminderScheduled, err := cache.IsReminderScheduled(ctx, today, user.PublicID)
			if err != nil {
				s.logger.Warn("Failed to check reminder scheduled status",
					zap.Int64("user_id", user.PublicID),
					zap.Error(err),
				)
				// 如果检查失败，继续检查其他用户
				continue
			}

			timeoutScheduled, err := cache.IsTimeoutScheduled(ctx, today, user.PublicID)
			if err != nil {
				s.logger.Warn("Failed to check timeout scheduled status",
					zap.Int64("user_id", user.PublicID),
					zap.Error(err),
				)
				// 如果检查失败，继续检查其他用户
				continue
			}

			// 如果 reminder 或 timeout 任一未发布，则需要调度
			if !reminderScheduled || !timeoutScheduled {
				shouldSkip = false
				break
			}

			// 注意：移除了开发环境清除标记的逻辑
			// 现在统一依赖 Redis 标记 + scheduleTimeGroup 中的数据库检查
			// 这样可以：
			// 1. Redis 标记存在时直接跳过，避免进入 scheduleTimeGroup
			// 2. 只有 Redis 标记不存在时才查数据库
			// 3. 减少数据库查询压力
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
	// 解析打卡日期
	checkInDateParsed, err := time.Parse("2006-01-02", checkInDate)
	if err != nil {
		s.logger.Error("Failed to parse checkInDate", zap.String("check_in_date", checkInDate), zap.Error(err))
		return err
	}

	// 查询数据库，过滤掉今日已有 check_in_reminder 任务的用户
	db := database.DB().WithContext(ctx)
	q := query.Use(db)


	userDBIDs := make([]int64, len(users))
	userMap := make(map[int64]*model.User) // DB ID -> User
	for i, user := range users {
		userDBIDs[i] = user.ID
		userMap[user.ID] = user
	}

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
		// 查询失败时不阻塞，继续处理所有用户
	}

	// 构建已有任务的用户 ID 集合
	existingUserIDs := make(map[int64]bool)
	for _, task := range existingTasks {
		existingUserIDs[task.UserID] = true
	}

	// 过滤掉已有任务的用户
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

	// 如果所有用户都已有任务，跳过投递
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

	// 使用过滤后的用户列表
	userIDs := make([]int64, len(filteredUsers))
	snapshots := make(map[string]model.UserSettingSnapshot)

	for i, user := range filteredUsers {
		userIDs[i] = user.PublicID

		// 构建用户设置快照
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
		// 解析宽限时间（取第一个用户的配置）
		graceUntil := users[0].DailyCheckInGraceUntil
		if graceUntil != "" {
			if graceTime, err := time.Parse("15:04:05", graceUntil); err == nil {
				graceUntilTime := time.Date(now.Year(), now.Month(), now.Day(),
					graceTime.Hour(), graceTime.Minute(), graceTime.Second(), 0, now.Location())

				// 如果还在宽限窗口内
				if now.Before(graceUntilTime) {
					if config.Cfg.Environment == "development" {
						// 开发环境：就近触发，1 分钟后
						todayRemindTime = now.Add(1 * time.Minute)
					} else {
						// 非开发环境：立即触发
						todayRemindTime = now
					}
				} else {
					// 已过宽限时间：今天不再补单，跳过（返回 nil 表示成功但不发消息）
					s.logger.Info("Skipped scheduling reminder: remind_at already passed grace window",
						zap.String("remind_at", remindAt),
						zap.String("grace_until", graceUntil),
						zap.Time("now", now),
					)
					return nil
				}
			}
		} else {
			// 没有配置 grace_until：非开发环境顺延到明天，开发环境就近触发
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

	// 生成 MessageID
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

	// 先发送 MQ 消息，成功后再写入 Redis 标记
	if err := queue.PublishCheckInReminder(reminderMsg); err != nil {
		s.logger.Error("Failed to publish reminder message",
			zap.String("remind_at", remindAt),
			zap.Int("user_count", len(filteredUsers)),
			zap.Error(err),
		)
		return err
	}

	// Reminder 消息发送成功后才标记为已投放
	for _, user := range filteredUsers {
		if err := cache.MarkReminderScheduled(ctx, checkInDate, user.PublicID); err != nil {
			s.logger.Warn("Failed to mark reminder scheduled after publishing message",
				zap.Int64("user_id", user.PublicID),
				zap.Error(err),
			)
			// 标记失败不影响主流程，因为 MQ 消息已发送
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

	// 超时时间 = 截止时间 + 1小时宽限期
	todayDeadline := time.Date(now.Year(), now.Month(), now.Day(),
		deadlineTime.Hour(), deadlineTime.Minute(), deadlineTime.Second(), 0, now.Location())
	timeoutTime := todayDeadline.Add(1 * time.Hour)

	// 计算超时延迟时间
	timeoutDelay := time.Until(timeoutTime)

	//调整为超时时间已过后，就立刻触发打卡机制
	if timeoutDelay < 0 {
		timeoutDelay = 0
		s.logger.Info("Timeout time has passed, will trigger immediately",
			zap.String("check_in_date", checkInDate),
			zap.Time("timeout_time", timeoutTime),
			zap.Time("now", now),
		)
	}

	// 生成 MessageID for timeout
	timeoutMessageID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
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

package service

import (
	"AreYouOK/internal/cache"
	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/queue"
	"AreYouOK/internal/repository/query"
	pkgerrors "AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/storage/database"
	"AreYouOK/utils"
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CheckInService struct{}

var (
	checkInService *CheckInService
	checkInOnce    sync.Once
)

func CheckIn() *CheckInService {
	checkInOnce.Do(func() {
		checkInService = &CheckInService{}
	})

	return checkInService
}

// consumer 中调用对应的用户，然后返回有效的用户 ID，快速过滤今天没有更新的用户，可以优先发送

type ValidateReminderBatchResult struct {
	ProcessNow []int64 // 可以直接处理的用户
	Republish  []int64 // 需要重新投递的用户（设置改变了）
	Skipped    []int64 // 需要跳过的用户（关闭了打卡功能或不在时间段内）
}

// StartCheckInReminderConsumer(ctx) 对应调用的函数，来验证用户设置，然后来处理提醒，最后重新投递
func (s *CheckInService) ValidateReminderBatch(
	ctx context.Context,
	userSettingsSnapshots map[string]queue.UserSettingSnapshot,
	checkInDate string,
	scheduledAt string,
) (*ValidateReminderBatchResult, error) {
	result := &ValidateReminderBatchResult{
		ProcessNow: make([]int64, 0),
		Republish:  make([]int64, 0),
		Skipped:    make([]int64, 0),
	}

	if len(userSettingsSnapshots) == 0 {
		return result, nil
	}

	var msgTime time.Time
	var err error
	if scheduledAt != "" {
		msgTime, err = time.Parse(time.RFC3339, scheduledAt)
		if err != nil {
			logger.Logger.Warn("Failed to parse scheduled_at",
				zap.String("scheduled_at", scheduledAt),
				zap.Error(err),
			)
			// 解析失败，无法进行版本号比较，保守处理：全部跳过
			result.Skipped = make([]int64, 0, len(userSettingsSnapshots))
			for userIDStr := range userSettingsSnapshots {
				userID, _ := strconv.ParseInt(userIDStr, 10, 64)
				if userID > 0 {
					result.Skipped = append(result.Skipped, userID)
				}
			}
			return result, nil
		}
	}

	userIDs := make([]int64, 0, len(userSettingsSnapshots))
	for userIDStr := range userSettingsSnapshots {
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse user ID: %w", err)
		}
		userIDs = append(userIDs, userID)
	}

	cachedSettings, err := cache.BatchGetUserSettings(ctx, userIDs)
	if err != nil {
		logger.Logger.Error("failed to batch get user settings", zap.Error(err))
		//这里是缓存查询失败
	}

	for _, userID := range userIDs {
		userIDStr := strconv.FormatInt(userID, 10)
		snapshot, ok := userSettingsSnapshots[userIDStr]
		if !ok {
			// 快照中没有这个用户，跳过
			logger.Logger.Warn("User snapshot not found",
				zap.Int64("user_id", userID),
			)
			result.Skipped = append(result.Skipped, userID)
			continue
		}

		cached, cacheHit := cachedSettings[userID]

		if !cacheHit {
			// 缓存未命中，说明用户设置没有改变，可以直接处理
			result.ProcessNow = append(result.ProcessNow, userID)
			continue
		}

		// 缓存命中，说明用户设置改变了，需要验证版本号

		if scheduledAt != "" {
			cacheTime := time.Unix(cached.UpdatedAt, 0)
			if cacheTime.After(msgTime) {
				// 缓存的更新时间 > 消息创建时间，说明设置已更新，丢弃消息（不扣款）
				logger.Logger.Info("User settings updated after message created, skipping",
					zap.Int64("user_id", userID),
					zap.Time("message_time", msgTime),
					zap.Time("cache_time", cacheTime),
				)
				result.Skipped = append(result.Skipped, userID)
				continue
			}
		}

		// 检查是否关闭了打卡功能
		if !cached.DailyCheckInEnabled {
			result.Skipped = append(result.Skipped, userID)
			continue
		}

		// 检查提醒时间是否改变
		if cached.DailyCheckInRemindAt != snapshot.RemindAt {
			// 提醒时间改变了，需要重新投递
			result.Republish = append(result.Republish, userID)
			continue
		}
		//  检查截止时间是否改变（可能影响超时消息）
		if cached.DailyCheckInDeadline != snapshot.Deadline {
			// 截止时间改变了，但提醒时间没变，可以正常处理
			// 超时消息会在超时处理时再判断
			result.ProcessNow = append(result.ProcessNow, userID)
			continue
		}

		result.ProcessNow = append(result.ProcessNow, userID)
	}

	return result, nil
	// 然后获取他们对应的设置来重新投递
}

// ProcessReminderBatch 批量发送打卡提醒消息（需要根据额度，或者免费？）
// userIDs: 用户 public_id 列表
func (s *CheckInService) ProcessReminderBatch(
	ctx context.Context,
	userIDs []int64,
	checkInDate string,
) error {
	if len(userIDs) == 0 {
		logger.Logger.Warn("ProcessReminderBatch: empty userIDs list")
		return nil
	}

	// 解析日期
	date, err := time.Parse("2006-01-02", checkInDate)
	if err != nil {
		return fmt.Errorf("invalid check_in_date format: %w", err)
	}

	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	//获取开启了打卡并且已经激活的用户 id

	users, err := q.User.WithContext(ctx).
		Where(q.User.PublicID.In(userIDs...)).
		Where(q.User.Status.Eq(string(model.UserStatusActive))).
		Where(q.User.DailyCheckInEnabled.Is(true)).
		Find()

	if err != nil {
		return fmt.Errorf("failed to query users: %w", err)
	}

	if len(users) == 0 {
		logger.Logger.Info("No valid users to process")
		return nil
	}

	// 构建用户映射（public_id -> User）
	userMap := make(map[int64]*model.User)
	for _, user := range users {
		userMap[user.PublicID] = user
	}

	// 开启事务批量处理
	return db.Transaction(func(tx *gorm.DB) error {
		txQ := query.Use(tx)
		now := time.Now()

		// 为每个用户处理
		for _, publicID := range userIDs {
			func() {
				user, ok := userMap[publicID]
				if !ok {
					logger.Logger.Warn("User not found or invalid",
						zap.Int64("public_id", publicID),
					)
					return
				}

				// 检查是否已创建通知任务（防止重复）
				// 查询今日是否已有 check_in_reminder 类型的通知任务
				nt := txQ.NotificationTask

				existingTasks, err := nt.WithContext(ctx).
					Where(nt.UserID.Eq(user.ID)).
					Where(nt.Category.Eq(string(model.NotificationCategoryCheckInReminder))).
					Where(nt.ScheduledAt.Gte(date)).
					Where(nt.ScheduledAt.Lt(date.AddDate(0, 0, 1))).
					Find()

				if err != nil && err != gorm.ErrRecordNotFound {
					logger.Logger.Error("Failed to query existing tasks",
						zap.Int64("user_id", publicID),
						zap.Error(err),
					)
					return
				}

				if len(existingTasks) > 0 {
					logger.Logger.Info("Notification task already exists, skipping",
						zap.Int64("user_id", publicID),
						zap.Int64("task_id", existingTasks[0].ID),
					)
					return
				}

				// 生成任务 ID 和 TaskCode（都使用 Snowflake）
				taskID, err := snowflake.NextID()
				if err != nil {
					logger.Logger.Error("Failed to generate task ID",
						zap.Int64("user_id", publicID),
						zap.Error(err),
					)
					return
				}

				taskCode, err := snowflake.NextID()
				if err != nil {
					logger.Logger.Error("Failed to generate task code",
						zap.Int64("user_id", publicID),
						zap.Error(err),
					)
					return
				}

				// 构建短信内容
				deadline := user.DailyCheckInDeadline
				if deadline == "" {
					deadline = "21:00:00" // 默认截止时间
				}
				// 这里的 message 内容需要在阿里云处更新
				message := fmt.Sprintf("安否温馨提示您，您开启了平安打卡功能，请于 %s 前进行打卡；如果没有按时打卡，我们将按约定联系您的紧急联系人。", deadline)

				payload := model.JSONB{
					"message": message,
				}

				phoneHash := ""
				if user.PhoneHash != nil {
					phoneHash = *user.PhoneHash
				}

				notificationTask := &model.NotificationTask{
					TaskCode:         taskCode,
					UserID:           user.ID,
					Category:         model.NotificationCategoryCheckInReminder,
					Channel:          model.NotificationChannelSMS,
					Status:           model.NotificationTaskStatusPending,
					Payload:          payload,
					ScheduledAt:      now,
					ContactPriority:  nil, // 提醒消息不需要联系人优先级，消费者也可以根据优先级来进行投递
					ContactPhoneHash: nil,
					RetryCount:       0,
					CostCents:        0,
					Deducted:         false,
				}

				if err := txQ.NotificationTask.WithContext(ctx).Create(notificationTask); err != nil {
					logger.Logger.Error("Failed to create notification task",
						zap.Int64("user_id", publicID),
						zap.Error(err),
					)
					return
				}

				// 发送消息到队列（不扣款，只是投递消息）， 扣款需要在消费者处进行事务处理
				// reminder_sent_at 会在消费者成功发送短信后更新
				notificationMsg := queue.NotificationMessage{
					TaskCode:        taskCode,
					TaskID:          taskID,
					UserID:          publicID, // 消息中使用 public_id
					Category:        "check_in_reminder",
					Channel:         "sms",
					PhoneHash:       phoneHash,
					Payload:         map[string]interface{}{"message": message},
					ContactPriority: 0,
					CheckInDate:     checkInDate, // 添加打卡日期，用于后续更新 reminder_sent_at
				}

				if err := queue.PublishSMSNotification(notificationMsg); err != nil {
					logger.Logger.Error("Failed to publish SMS notification",
						zap.Int64("user_id", publicID),
						zap.Int64("task_id", taskID),
						zap.Error(err),
					)
					// 消息投递失败不影响事务，继续处理其他用户
					return
				}

				logger.Logger.Info("Processed reminder for user",
					zap.Int64("user_id", publicID),
					zap.Int64("task_id", taskID),
					zap.String("check_in_date", checkInDate),
				)
			}()
		}

		return nil
	})
}

// GetTodayCheckIn 查询当天打卡状态
func (s *CheckInService) GetTodayCheckIn(
	ctx context.Context,
	userID string,
) (*dto.CheckInStatusData, error) {
	var userIDInt int64
	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return nil, pkgerrors.InvalidUserID
	}

	user, err := query.User.GetByPublicID(userIDInt)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	if !user.DailyCheckInEnabled {
		return nil, pkgerrors.Definition{
			Code:    "CHECK_IN_DISABLED",
			Message: "Daily check-in is disabled",
		}
	}

	checkIn, err := query.DailyCheckIn.GetTodayCheckInByPublicID(userIDInt)

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to query check-in: %w", err)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	deadline, err := utils.ParseTime(user.DailyCheckInDeadline, today)
	if err != nil {
		return nil, fmt.Errorf("invalid deadline format: %w", err)
	}

	graceUntil, err := utils.ParseTime(user.DailyCheckInGraceUntil, today)
	if err != nil {
		return nil, fmt.Errorf("invalid grace_until format: %w", err)
	}

	if checkIn == nil {
		return &dto.CheckInStatusData{
			Date:             today.Format("2006-01-02"),
			Status:           string(model.CheckInStatusPending),
			Deadline:         deadline,
			GraceUntil:       graceUntil,
			ReminderSentAt:   nil,
			AlertTriggeredAt: nil,
		}, nil
	}

	return &dto.CheckInStatusData{
		Date:             checkIn.CheckInDate.Format("2006-01-02"),
		Status:           string(checkIn.Status),
		Deadline:         deadline,
		GraceUntil:       graceUntil,
		ReminderSentAt:   checkIn.ReminderSentAt,
		AlertTriggeredAt: checkIn.AlertTriggeredAt,
	}, nil
}

// 真正来打卡时再考虑插入，默认时都没有记录的，所以这里需要检查是否存在，不存在则插入，存在则更新，但是消息是需要都发出去的
func (s *CheckInService) CompleteCheckIn(
	ctx context.Context,
	userID string) (*dto.CompleteCheckInResponse, error) {

	var userIDInt int64
	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return nil, pkgerrors.InvalidUserID
	}

	user, err := query.User.GetByPublicID(userIDInt)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	if !user.DailyCheckInEnabled {
		return nil, pkgerrors.Definition{
			Code:    "CHECK_IN_DISABLED",
			Message: "Daily check-in is disabled",
		}
	}

	now := time.Now()

	// 未来考虑支持时区时的补充
	loc := time.FixedZone("CST", 8*3600)
	if user.Timezone != "" {
		if l, err := time.LoadLocation(user.Timezone); err == nil {
			loc = l
		}
	}

	currentDateInUserZone := now.In(loc)

	today := time.Date(currentDateInUserZone.Year(), currentDateInUserZone.Month(), currentDateInUserZone.Day(), 0, 0, 0, 0, currentDateInUserZone.Location())

	db := database.DB().WithContext(ctx)

	checkIn := &model.DailyCheckIn{
		UserID:      user.ID, // 使用数据库主键 id，而不是 public_id
		CheckInDate: today,
		CheckInAt:   &now,
		Status:      model.CheckInStatusDone,
	}

	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "check_in_date"}}, //类似于 postgres 的 onConflict
		DoUpdates: clause.AssignmentColumns([]string{"status", "check_in_at", "updated_at"}),
	}).Create(checkIn).Error

	if err != nil {
		return nil, fmt.Errorf("failed to save check-in: %w", err)
	}

	// TODO: 如果之前触发了超时警报，这里可能需要发送一个 "解除警报" 的事件, 也就是已经 done 了
	// 再投递的时候就不会出现问题

	return &dto.CompleteCheckInResponse{
		Date:        today.Format("2006-01-02"),
		Status:      string(model.CheckInStatusDone),
		CompletedAt: now,
		// StreakDays: 0, // TODO: 计算连续打卡天数
		// RewardPoints: 0, // TODO: 计算积分
	}, nil
}

// // 处理单个用户的打卡信息, 这里需要对接到具体的短信服务需要的 payload 是什么才可以知道如何去做

// func (s *CheckInService) processSingleUserReminder(
// 	ctx context.Context,
// 	q *query.Query,
// 	userID int64,
// 	checkInDate time.Time,
// ) error { //还需要检查用户当天有没有更改

// 	user, err := q.User.GetByID(userID)
// 	if err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			logger.Logger.Warn("User not found",
// 				zap.Int64("user_id", userID),
// 			)
// 			return nil // 用户不存在，跳过
// 		}
// 	}

// 	if user.Status != model.UserStatusActive {
// 		logger.Logger.Info("User is not active, skipping reminder",
// 			zap.Int64("user_id", userID),
// 			zap.String("status", string(user.Status)),
// 		)
// 		return nil
// 	}

// 	if !user.DailyCheckInEnabled {
// 		logger.Logger.Info("User has check-in disabled, skipping reminder",
// 			zap.Int64("user_id", userID),
// 		)
// 		return nil
// 	} // 启用就保证了一定有手机号

// 	//userWithQuotas, err := q.User.GetWithQuotas(userID)
// 	if err != nil {
// 		logger.Logger.Warn("Failed to get user quotas, will continue",
// 			zap.Int64("user_id", userID),
// 			zap.Error(err),
// 		)
// 	} else {
// 		var smsBalance float64

// 		if smsBalance <= 0 {
// 			logger.Logger.Warn("User has insufficient SMS balance, skipping reminder",
// 				zap.Int64("user_id", userID),
// 				zap.Float64("sms_balance", smsBalance),
// 			)
// 			return nil
// 		}
// 	}
// 	return nil
// }

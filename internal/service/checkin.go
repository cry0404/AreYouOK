package service

import (
	"AreYouOK/internal/cache"
	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/repository/query"
	pkgerrors "AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/storage/database"
	"AreYouOK/utils"
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// validateCheckInReminderPayload 验证CheckInReminder消息payload格式， 只针对 checkinreminder， 可以考虑合并到 validate.go 当中去
func validateCheckInReminderPayload(payload model.JSONB) error {

	typeVal, ok := payload["type"]
	if !ok {
		return fmt.Errorf("payload missing 'type' field")
	}
	if typeVal != "checkin_reminder" {
		return fmt.Errorf("payload type must be 'checkin_reminder', got '%v'", typeVal)
	}

	if _, ok := payload["name"]; !ok {
		return fmt.Errorf("payload missing 'name' field")
	}

	timeVal, ok := payload["time"]
	if !ok {
		return fmt.Errorf("payload missing 'time' field")
	}

	timeStr, ok := timeVal.(string)
	if !ok {
		return fmt.Errorf("payload 'time' must be string, got %T", timeVal)
	}

	_, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		return fmt.Errorf("payload 'time' format invalid, expected HH:mm:ss, got '%s'", timeStr)
	}

	return nil
}

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
	Republish  []int64 // 需要重新投递的用户（设置改变了），但是已经在用户更改时就已经投递了，所以就不用在意了
	Skipped    []int64 // 需要跳过的用户（关闭了打卡功能或不在时间段内）
}

// StartCheckInReminderConsumer(ctx) 对应调用的函数，来验证用户设置，然后来处理提醒，最后重新投递
func (s *CheckInService) ValidateReminderBatch(
	ctx context.Context,
	userSettingsSnapshots map[string]model.UserSettingSnapshot,
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

// ProcessReminderBatch 批量发送打卡提醒消息（需要根据额度，或者免费？）， 这里的 Send 也需要重新修改
// userIDs: 用户 public_id 列表
// 返回值: 创建的任务数量, 错误
func (s *CheckInService) ProcessReminderBatch(
	ctx context.Context,
	userIDs []int64,
	checkInDate string,
) (int, error) {
	logger.Logger.Info("ProcessReminderBatch started",
		zap.Int("user_count", len(userIDs)),
		zap.String("check_in_date", checkInDate),
	)
	if len(userIDs) == 0 {
		logger.Logger.Warn("ProcessReminderBatch: empty userIDs list")
		return 0, nil
	}

	// 解析日期
	date, err := time.Parse("2006-01-02", checkInDate)
	if err != nil {
		return 0, fmt.Errorf("invalid check_in_date format: %w", err)
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
		return 0, fmt.Errorf("failed to query users: %w", err)
	}

	if len(users) == 0 {
		logger.Logger.Debug("No valid users to process")
		return 0, nil
	}

	// 构建用户映射（public_id -> User）
	userMap := make(map[int64]*model.User)
	for _, user := range users {
		userMap[user.PublicID] = user
	}

	// 跟踪创建的任务数量
	var createdCount int

	// 开启事务批量处理
	err = db.Transaction(func(tx *gorm.DB) error {
		txQ := query.Use(tx)
		now := time.Now()

		// 为每个用户处理
		for _, publicID := range userIDs {
			user, ok := userMap[publicID]
			if !ok {
				logger.Logger.Warn("User not found or invalid",
					zap.Int64("public_id", publicID),
				)
				continue
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
				return err // 发生错误时回滚事务
			}

			if len(existingTasks) > 0 {
				logger.Logger.Debug("Notification task already exists, skipping",
					zap.Int64("user_id", publicID),
					zap.Int64("task_id", existingTasks[0].ID),
				)
				continue // 继续处理其他用户，但不回滚事务
			}

			// 检查月度提醒限制（每用户每月最多 5 条提醒）
			allowed, count, err := cache.CheckMonthlyReminderLimit(ctx, user.ID)
			if err != nil {
				logger.Logger.Warn("Failed to check monthly reminder limit, allowing send",
					zap.Int64("user_id", publicID),
					zap.Error(err),
				)
				// 出错时降级，允许发送
			} else if !allowed {
				logger.Logger.Info("User exceeded monthly check-in reminder limit, skipping",
					zap.Int64("user_id", publicID),
					zap.Int("count", count),
					zap.Int("limit", cache.MonthlyReminderLimit),
				)
				continue // 超过限制，跳过该用户
			}

			taskID, err := snowflake.NextID(snowflake.GeneratorTypeTask)
			if err != nil {
				logger.Logger.Error("Failed to generate task ID",
					zap.Int64("user_id", publicID),
					zap.Error(err),
				)
				return err // 发生错误时回滚事务
			}

			taskCode, err := snowflake.NextID(snowflake.GeneratorTypeTask)
			if err != nil {
				logger.Logger.Error("Failed to generate task code",
					zap.Int64("user_id", publicID),
					zap.Error(err),
				)
				return err
			}

			// 构建短信内容
			deadline := user.DailyCheckInDeadline
			if deadline == "" {
				deadline = "21:00:00" // 默认截止时间
			}

			phone, _ := utils.DecryptPhone(user.PhoneCipher)

			// 取末尾四位电话作为昵称
			userName := fmt.Sprintf("用户%s", phone[7:])
			if user.Nickname != "" {
				userName = user.Nickname //这里就直接更新了
			}

			payload := model.JSONB{
				"type": "checkin_reminder",
				"name": userName,
				"time": deadline,
			}

			if err := validateCheckInReminderPayload(payload); err != nil {
				logger.Logger.Error("Invalid payload format",
					zap.Int64("user_id", publicID),
					zap.Error(err),
				)
				return err
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
				return err // 创建失败，回滚事务
			}

			createdCount++

			// 增加月度提醒计数
			monthKey := now.Format("2006-01")
			if err := cache.IncrementMonthlyReminderCount(ctx, user.ID, monthKey); err != nil {
				logger.Logger.Warn("Failed to increment monthly reminder count",
					zap.Int64("user_id", publicID),
					zap.Error(err),
				)
				// 不影响主流程，继续处理
			}

			// reminder_sent_at 会在消费者成功发送短信后更新

			logger.Logger.Debug("Processed reminder for user",
				zap.Int64("user_id", publicID),
				zap.Int64("task_id", taskID),
				zap.String("check_in_date", checkInDate),
			)
		}

		return nil
	})
	if err != nil {
		logger.Logger.Error("ProcessReminderBatch failed",
			zap.String("check_in_date", checkInDate),
			zap.Error(err),
		)
		return 0, err
	}

	logger.Logger.Info("ProcessReminderBatch completed",
		zap.String("check_in_date", checkInDate),
		zap.Int("created_count", createdCount),
	)
	return createdCount, nil
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

	if checkIn == nil || checkIn.CheckInDate.IsZero() {
		return &dto.CheckInStatusData{
			Date:             today.Format("2006-01-02"),
			Status:           string(model.CheckInStatusPending),
			Deadline:         deadline,
			GraceUntil:       graceUntil,
			ReminderSentAt:   nil,
			AlertTriggeredAt: nil,
		}, nil
	}

	status := string(checkIn.Status)
	if status == "" {
		status = string(model.CheckInStatusPending)
	}

	return &dto.CheckInStatusData{
		Date:             checkIn.CheckInDate.Format("2006-01-02"),
		Status:           status,
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

// ProcessTimeoutBatch 批量处理打卡超时
// 由超时 consuemer 消费
// 查询超时用户（21:00未打卡且已发送提醒），通知紧急联系人
// 返回创建的通知任务列表（包含超时通知和额度耗尽通知）
func (s *CheckInService) ProcessTimeoutBatch(
	ctx context.Context,
	userIDs []int64,
	checkInDate string,
) ([]*model.NotificationTask, error) {
	db := database.DB().WithContext(ctx)

	q := query.Use(db)

	users, err := q.User.WithContext(ctx).
		Where(q.User.PublicID.In(userIDs...)).
		Find()

	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}

	userMap := make(map[int64]*model.User)

	for _, user := range users {
		userMap[user.PublicID] = user
	}

	// 解析日期
	checkInDateParsed, err := time.Parse("2006-01-02", checkInDate)
	if err != nil {
		return nil, fmt.Errorf("invalid check_in_date format: %w", err)
	}

	// 批量查询打卡记录
	checkIns, err := q.DailyCheckIn.WithContext(ctx).
		Where(q.DailyCheckIn.CheckInDate.Eq(checkInDateParsed)).
		Where(q.DailyCheckIn.UserID.In(func() []int64 {
			ids := make([]int64, 0, len(users))
			for _, u := range users {
				ids = append(ids, u.ID)
			}
			return ids
		}()...)).
		Find()

	if err != nil {
		return nil, fmt.Errorf("failed to query check-ins: %w", err)
	}

	checkInMap := make(map[int64]*model.DailyCheckIn)
	for _, ci := range checkIns {
		checkInMap[ci.UserID] = ci
	}

	now := time.Now()

	// 收集创建的任务
	var createdTasks []*model.NotificationTask

	err = db.Transaction(func(tx *gorm.DB) error {
		txQ := query.Use(tx)

		// 提取所有需要检查额度的用户ID，进行批量查询
		var userIDsForQuota []int64
		for _, publicID := range userIDs {
			if user, ok := userMap[publicID]; ok {
				if checkIn, exists := checkInMap[user.ID]; exists && checkIn.AlertTriggeredAt == nil {
					userIDsForQuota = append(userIDsForQuota, user.ID)
				}
			}
		}

		// 批量查询额度钱包，避免 N+1 查询
		quotaService := Quota()
		walletMap := make(map[int64]*model.QuotaWallet)
		for _, userID := range userIDsForQuota {
			wallet, err := quotaService.GetWallet(ctx, userID, model.QuotaChannelSMS)
			if err != nil {
				logger.Logger.Error("Failed to query SMS quota wallet",
					zap.Int64("user_id", userID),
					zap.Error(err),
				)
				continue // 跳过这个用户
			}
			walletMap[userID] = wallet
		}

		for _, publicID := range userIDs {
			user, ok := userMap[publicID]
			if !ok {
				logger.Logger.Warn("User not found in timeout batch",
					zap.Int64("public_id", publicID),
				)
				continue
			}

			checkIn, ok := checkInMap[user.ID]
			if !ok {
				logger.Logger.Warn("Check-in record not found",
					zap.Int64("user_id", user.ID),
					zap.String("check_in_date", checkInDate),
				)
				continue
			}

			// 检查是否已经触发过告警, 也就是提前发短信那部分
			if checkIn.AlertTriggeredAt != nil {
				logger.Logger.Debug("Alert already triggered for this check-in",
					zap.Int64("user_id", user.ID),
					zap.String("check_in_date", checkInDate),
				)
				continue
			}

			// 使用批量查询的额度信息
			wallet, ok := walletMap[user.ID]
			if !ok {
				logger.Logger.Warn("No quota wallet found for user",
					zap.Int64("user_id", user.ID),
				)
				continue
			}

			smsBalance := wallet.AvailableAmount

			smsUnitPriceCents := 5
			contactCount := len(user.EmergencyContacts)
			totalCost := smsUnitPriceCents * contactCount

			if smsBalance < totalCost {
				logger.Logger.Warn("Insufficient quota for timeout alert",
					zap.Int64("user_id", user.ID),
					zap.Int("balance", smsBalance),
					zap.Int("required", totalCost),
					zap.Int("contact_count", contactCount),
				)

				// 检查是否应该发送额度耗尽提醒
				// 逻辑：每次额度用尽时发送一次，但如果用户充值后再次用尽，应该再次发送
				//  查询最近一次发送额度耗尽提醒的时间
				lastQuotaDepletedTask, err := txQ.NotificationTask.
					Where(txQ.NotificationTask.UserID.Eq(user.ID)).
					Where(txQ.NotificationTask.Category.Eq(string(model.NotificationCategoryQuotaDepleted))).
					Order(txQ.NotificationTask.ScheduledAt.Desc()).
					First()

				shouldSendQuotaDepleted := false
				if err != nil {
					if err == gorm.ErrRecordNotFound {
						// 从未发送过，应该发送
						shouldSendQuotaDepleted = true
					} else {
						logger.Logger.Error("Failed to check quota depleted notification",
							zap.Int64("user_id", user.ID),
							zap.Error(err),
						)

						continue
					}
				} else {
					// 查询从上次发送提醒到现在，是否有充值记录（grant 类型的交易）
					// 如果有充值，说明用户曾经有额度，现在又用尽了，应该再次发送
					rechargeCount, err := txQ.QuotaTransaction.
						Where(txQ.QuotaTransaction.UserID.Eq(user.ID)).
						Where(txQ.QuotaTransaction.Channel.Eq(string(model.QuotaChannelSMS))).
						Where(txQ.QuotaTransaction.TransactionType.Eq(string(model.TransactionTypeGrant))).
						Where(txQ.QuotaTransaction.CreatedAt.Gt(lastQuotaDepletedTask.ScheduledAt)).
						Count()
					if err != nil {
						logger.Logger.Error("Failed to check quota recharge after last depleted notification",
							zap.Int64("user_id", user.ID),
							zap.Error(err),
						)

						continue
					}

					// 如果在上次发送提醒后有充值记录，说明用户曾经有额度，现在又用尽了，应该再次发送
					shouldSendQuotaDepleted = rechargeCount > 0
				}

				// 如果应该发送额度耗尽提醒，则发送
				if shouldSendQuotaDepleted {
					taskCode, err := snowflake.NextID(snowflake.GeneratorTypeTask)
					if err != nil {
						logger.Logger.Error("Failed to generate task code for quota depleted",
							zap.Int64("user_id", user.ID),
							zap.Error(err),
						)
						continue
					}

					// 构建额度耗尽提醒的 payload
					payload := model.JSONB{
						"type": "quota_depleted",
					}

					// 创建额度耗尽提醒任务（发送给用户本人）
					quotaDepletedTask := &model.NotificationTask{
						TaskCode: taskCode,
						UserID:   user.ID,
						Category: model.NotificationCategoryQuotaDepleted,
						Channel:  model.NotificationChannelSMS,
						Status:   model.NotificationTaskStatusPending,
						Payload:  payload,
						// ContactPriority 和 ContactPhoneHash 为空，因为这是发送给用户本人的
						ScheduledAt: now,
					}

					if err := txQ.NotificationTask.Create(quotaDepletedTask); err != nil {
						logger.Logger.Error("Failed to create quota depleted notification task",
							zap.Int64("user_id", user.ID),
							zap.Error(err),
						)
						continue
					}

					createdTasks = append(createdTasks, quotaDepletedTask)

					logger.Logger.Info("Created quota depleted notification task",
						zap.Int64("user_id", user.ID),
						zap.String("check_in_date", checkInDate),
					)
				} else {
					logger.Logger.Debug("Quota depleted notification already sent and no recharge since then, skipping",
						zap.Int64("user_id", user.ID),
						zap.String("check_in_date", checkInDate),
					)
				}

				// 更新打卡状态为超时，但不发送紧急联系人通知
				_, err = txQ.DailyCheckIn.
					Where(txQ.DailyCheckIn.ID.Eq(checkIn.ID)).
					Updates(map[string]interface{}{
						"alert_triggered_at": now,
						"status":             model.CheckInStatusTimeout,
						"updated_at":         now,
					})
				if err != nil {
					logger.Logger.Error("Failed to update check-in status for quota depleted",
						zap.Int64("check_in_id", checkIn.ID),
						zap.Error(err),
					)
					// 更新失败不影响主流程
				}

				continue // 跳过紧急联系人通知
			}

			_, err = txQ.DailyCheckIn.
				Where(txQ.DailyCheckIn.ID.Eq(checkIn.ID)).
				Updates(map[string]interface{}{
					"alert_triggered_at": now,
					"status":             model.CheckInStatusTimeout,
					"updated_at":         now,
				})
			if err != nil {
				logger.Logger.Error("Failed to update check-in alert status",
					zap.Int64("check_in_id", checkIn.ID),
					zap.Error(err),
				)
				return fmt.Errorf("failed to update check-in: %w", err)
			}

			contacts := user.EmergencyContacts
			sort.Slice(contacts, func(i, j int) bool {
				return contacts[i].Priority < contacts[j].Priority
			})

			// 最多通知3位紧急联系人
			maxContacts := 3
			if len(contacts) > maxContacts {
				contacts = contacts[:maxContacts]
			}

			// // 获取用户昵称, 用户昵称应该必须设置才对
			// userName := user.Nickname

			// 获取截止时间
			deadline := user.DailyCheckInDeadline
			if deadline == "" {
				deadline = "21:00:00"
			}

			for _, contact := range contacts {
				// 生成 task_code
				taskCode, err := snowflake.NextID(snowflake.GeneratorTypeTask)
				if err != nil {
					logger.Logger.Error("Failed to generate task code",
						zap.Int64("user_id", user.ID),
						zap.Error(err),
					)
					continue
				}

				payload := model.JSONB{
					"type": "checkin_reminder_contact",
					"name": contact.DisplayName,
				}

				// 创建通知任务
				notificationTask := &model.NotificationTask{
					TaskCode:         taskCode,
					UserID:           user.ID,
					Category:         model.NotificationCategoryCheckInTimeout,
					Channel:          model.NotificationChannelSMS,
					Status:           model.NotificationTaskStatusPending,
					Payload:          payload,
					ContactPriority:  &contact.Priority,
					ContactPhoneHash: &contact.PhoneHash,
					ScheduledAt:      now,
				}

				if err := txQ.NotificationTask.Create(notificationTask); err != nil {
					logger.Logger.Error("Failed to create notification task",
						zap.Int64("user_id", user.ID),
						zap.Int("priority", contact.Priority),
						zap.Error(err),
					)
					return fmt.Errorf("failed to create notification task: %w", err)
				}

				// 添加到创建的任务列表
				createdTasks = append(createdTasks, notificationTask)
			}

			logger.Logger.Info("Processed timeout check-in",
				zap.Int64("user_id", user.ID),
				zap.String("check_in_date", checkInDate),
				zap.Int("contact_count", len(contacts)),
			)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return createdTasks, nil
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

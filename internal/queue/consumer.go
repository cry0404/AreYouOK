package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"AreYouOK/internal/cache"
	"AreYouOK/internal/model"
	"AreYouOK/internal/repository/query"
	"AreYouOK/internal/service"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/storage/database"
	"AreYouOK/storage/mq"
)

type NotificationService interface {
	SendSMS(ctx context.Context, taskCode int64, userID int64, phoneHash string, payload map[string]interface{}) error
}

var notificationService NotificationService

// SetNotificationService 设置通知服务（在 worker 启动时调用）
func SetNotificationService(s NotificationService) {
	notificationService = s
}

// sms 和 voice 都属于消息，由两个部分来投递，task 作为消费
// 打卡对应的是两条消息，所以投递的时候就应该同时投递两条，在更新 userSetting 时也不例外
// StartCheckInReminderConsumer 启动打卡提醒消费者， 有关打卡部分
func StartCheckInReminderConsumer(ctx context.Context) error {
	handler := func(body []byte) error {
		var msg model.CheckInReminderMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal check-in reminder message: %w", err)
		}

		processed, err := cache.TryMarkMessageProcessing(ctx, msg.MessageID, 24*time.Hour)
		if err != nil {
			logger.Logger.Warn("Failed to check message processed status",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
			// 如果检查失败，继续处理（不阻塞业务），但可能重复处理
		} else if !processed {
			logger.Logger.Debug("Message already processed or being processed, skipping",
				zap.String("message_id", msg.MessageID),
				zap.String("batch_id", msg.BatchID),
			)
			return &errors.SkipMessageError{Reason: fmt.Sprintf("Message %s already processed", msg.MessageID)}
		}

		logger.Logger.Debug("Processing check-in reminder batch",
			zap.String("message_id", msg.MessageID),
			zap.String("batch_id", msg.BatchID),
			zap.Int("user_count", len(msg.UserIDs)),
		)

		// 1. 验证用户设置是否变更
		// 对比快照与当前设置，返回三种用户列表
		checkInService := service.CheckIn()
		validationResult, err := checkInService.ValidateReminderBatch(
			ctx,
			msg.UserSettings,
			msg.CheckInDate,
			msg.ScheduledAt,
		)
		if err != nil {
			cache.UnmarkMessageProcessing(ctx, msg.MessageID)
			logger.Logger.Error("Failed to validate reminder batch",
				zap.String("message_id", msg.MessageID),
				zap.String("batch_id", msg.BatchID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to validate reminder batch: %w", err)
		}

		logger.Logger.Debug("Validation result",
			zap.String("message_id", msg.MessageID),
			zap.Int("process_now_count", len(validationResult.ProcessNow)),
			zap.Int("republish_count", len(validationResult.Republish)),
			zap.Int("skipped_count", len(validationResult.Skipped)),
		)

		// 2. 处理 ProcessNow 用户（设置未变更）
		// 这些用户可以直接创建通知任务
		if len(validationResult.ProcessNow) > 0 {
			err = checkInService.ProcessReminderBatch(ctx, validationResult.ProcessNow, msg.CheckInDate)
			if err != nil {
				cache.UnmarkMessageProcessing(ctx, msg.MessageID)
				logger.Logger.Error("Failed to process ProcessNow batch",
					zap.String("message_id", msg.MessageID),
					zap.Int("user_count", len(validationResult.ProcessNow)),
					zap.Error(err),
				)
				return fmt.Errorf("failed to process ProcessNow batch: %w", err)
			}
		}

		// 3. 对于 Republish 用户（设置已变更），不再在消费者侧重新投递
		//    现在由用户设置更新接口在写路径上同步构建并发布新的提醒消息，
		//    这里仅记录日志，原消息视为处理完成。
		if len(validationResult.Republish) > 0 {
			logger.Logger.Info("Users with changed settings will rely on write-path rescheduling, skipping republish in consumer",
				zap.String("message_id", msg.MessageID),
				zap.Int("republish_count", len(validationResult.Republish)),
				zap.String("check_in_date", msg.CheckInDate),
			)
		}

		logger.Logger.Debug("Successfully processed check-in reminder batch",
			zap.String("message_id", msg.MessageID),
			zap.String("batch_id", msg.BatchID),
			zap.Int("user_count", len(msg.UserIDs)),
		)

		// 处理完成后标记消息已处理（延长 TTL）
		if err := cache.MarkMessageProcessed(ctx, msg.MessageID, 48*time.Hour); err != nil {
			logger.Logger.Warn("Failed to mark message as processed",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
			// 记录失败但不影响主流程
		}

		return nil
	}

	return mq.Consume(mq.ConsumeOptions{
		Queue:         "scheduler.check_in.reminder",
		ConsumerTag:   "check_in_reminder_consumer",
		PrefetchCount: 10, // 每次预取10条消息
		Handler:       handler,
		Context:       ctx,
	})
}

// StartCheckInTimeoutConsumer 启动打卡超时消费者
func StartCheckInTimeoutConsumer(ctx context.Context) error {
	handler := func(body []byte) error {
		var msg model.CheckInTimeoutMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal check-in timeout message: %w", err)
		}

		//不重复消费一条消息，这种 checkIn 部分都是一天一次
		processed, err := cache.TryMarkMessageProcessing(ctx, msg.MessageID, 24*time.Hour)
		if err != nil {
			logger.Logger.Warn("Failed to check message processed status",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
		} else if !processed {
			logger.Logger.Debug("Message already processed or being processed, skipping",
				zap.String("message_id", msg.MessageID),
				zap.String("batch_id", msg.BatchID),
			)
			return &errors.SkipMessageError{Reason: fmt.Sprintf("Message %s already processed", msg.MessageID)}
		}

		logger.Logger.Debug("Processing check-in timeout batch",
			zap.String("message_id", msg.MessageID),
			zap.String("batch_id", msg.BatchID),
			zap.Int("user_count", len(msg.UserIDs)),
		)

		// 1. 批量更新 daily_check_ins.alert_triggered_at
		// 2. 批量查询紧急联系人列表
		// 3. 按优先级批量创建通知任务
		// 4. 批量发送短信
		// 5. 批量记录 contact_attempts, 每次尝试的记录记录到 contact_attempts

		checkInService := service.CheckIn()
		err = checkInService.ProcessTimeoutBatch(ctx, msg.UserIDs, msg.CheckInDate)
		if err != nil {
			// 1. 可跳过的错误：标记为已处理，不重试
			if errors.IsSkipMessageError(err) {
				logger.Logger.Info("Skipping check-in timeout batch processing",
					zap.String("message_id", msg.MessageID),
					zap.String("reason", err.(*errors.SkipMessageError).Reason),
					zap.Error(err),
				)
				// 标记为已处理，避免重复处理
				if markErr := cache.MarkMessageProcessed(ctx, msg.MessageID, 48*time.Hour); markErr != nil {
					logger.Logger.Warn("Failed to mark skipped message as processed",
						zap.String("message_id", msg.MessageID),
						zap.Error(markErr),
					)
				}
				return nil // 返回 nil 表示成功处理（跳过）
			}

			// 2. 不可重试错误：记录日志并发送到死信队列
			if errors.IsNonRetryableError(err) {
				logger.Logger.Error("Non-retryable error in check-in timeout batch, message will be sent to DLQ",
					zap.String("message_id", msg.MessageID),
					zap.String("error_code", err.(*errors.NonRetryableError).Code),
					zap.String("reason", err.(*errors.NonRetryableError).Reason),
					zap.Error(err),
				)
				// 取消处理标记，让 RabbitMQ 发送到死信队列
				cache.UnmarkMessageProcessing(ctx, msg.MessageID)
				return fmt.Errorf("non-retryable error in check-in timeout: %w", err)
			}

			// 3. 其他可重试错误：取消标记，允许重试
			logger.Logger.Warn("Retryable error in check-in timeout batch processing, will retry",
				zap.String("message_id", msg.MessageID),
				zap.Int("user_count", len(msg.UserIDs)),
				zap.Error(err),
			)
			cache.UnmarkMessageProcessing(ctx, msg.MessageID)
			return fmt.Errorf("failed to process timeout batch (will retry): %w", err)
		}

		db := database.DB().WithContext(ctx)
		q := query.Use(db)

		today := time.Now().Truncate(24 * time.Hour)
		tasks, err := q.NotificationTask.
			Where(q.NotificationTask.Category.Eq(string(model.NotificationCategoryCheckInTimeout))).
			Where(q.NotificationTask.CreatedAt.Gte(today)).
			Where(q.NotificationTask.Status.Eq(string(model.NotificationTaskStatusPending))).
			Find()

		if err == nil {
			for _, task := range tasks {
				// 查询用户 public_id
				user, err := q.User.GetByID(task.UserID)
				if err != nil {
					logger.Logger.Warn("Failed to query user for notification",
						zap.Int64("task_id", task.ID),
						zap.Error(err),
					)
					continue
				}

				// 构建通知消息
				notificationMsg := model.NotificationMessage{
					MessageID: fmt.Sprintf("notification_%d", task.TaskCode),
					TaskCode:  task.TaskCode,
					UserID:    user.PublicID,
					Category:  string(task.Category),
					Channel:   string(task.Channel),
					PhoneHash: "",
					Payload:   task.Payload,
				}

				// 如果有联系人信息，设置 phone_hash
				if task.ContactPhoneHash != nil {
					notificationMsg.PhoneHash = *task.ContactPhoneHash
				}

				// 发布到队列
				if err := PublishSMSNotification(notificationMsg); err != nil {
					logger.Logger.Error("Failed to publish notification message",
						zap.Int64("task_code", task.TaskCode),
						zap.Error(err),
					)
				}
			}
		}

		if err := cache.MarkMessageProcessed(ctx, msg.MessageID, 48*time.Hour); err != nil {
			logger.Logger.Warn("Failed to mark message as processed",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
		}

		return nil
	}

	return mq.Consume(mq.ConsumeOptions{
		Queue:         "events.check_in.timeout",
		ConsumerTag:   "check_in_timeout_consumer",
		PrefetchCount: 10,
		Handler:       handler,
		Context:       ctx,
	})
}

// 修改 StartJourneyTimeoutConsumer，实现完整的业务逻辑
func StartJourneyTimeoutConsumer(ctx context.Context) error {
	handler := func(body []byte) error {
		var msg model.JourneyTimeoutMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal journey timeout message: %w", err)
		}

		// 幂等性检查
		processed, err := cache.TryMarkMessageProcessing(ctx, msg.MessageID, 24*time.Hour)
		if err != nil {
			logger.Logger.Warn("Failed to check message processed status",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
		} else if !processed {
			logger.Logger.Debug("Message already processed or being processed, skipping",
				zap.String("message_id", msg.MessageID),
				zap.Int64("journey_id", msg.JourneyID),
			)
			return &errors.SkipMessageError{Reason: fmt.Sprintf("Message %s already processed", msg.MessageID)}
		}

		logger.Logger.Debug("Processing journey timeout",
			zap.String("message_id", msg.MessageID),
			zap.Int64("journey_id", msg.JourneyID),
			zap.Int64("user_id", msg.UserID),
		)

		// 调用 service 层处理行程超时逻辑
		journeyService := service.Journey()
		err = journeyService.ProcessTimeout(ctx, msg.JourneyID, msg.UserID)
		if err != nil {
			// 1. 可跳过的错误：标记为已处理，不重试
			if errors.IsSkipMessageError(err) {
				logger.Logger.Info("Skipping journey timeout processing",
					zap.String("message_id", msg.MessageID),
					zap.String("reason", err.(*errors.SkipMessageError).Reason),
					zap.Error(err),
				)
				// 标记为已处理，避免重复处理
				if markErr := cache.MarkMessageProcessed(ctx, msg.MessageID, 48*time.Hour); markErr != nil {
					logger.Logger.Warn("Failed to mark skipped message as processed",
						zap.String("message_id", msg.MessageID),
						zap.Error(markErr),
					)
				}
				return nil // 返回 nil 表示成功处理（跳过）
			}

			// 2. 不可重试错误：记录日志并发送到死信队列
			if errors.IsNonRetryableError(err) {
				logger.Logger.Error("Non-retryable error in journey timeout, message will be sent to DLQ",
					zap.String("message_id", msg.MessageID),
					zap.String("error_code", err.(*errors.NonRetryableError).Code),
					zap.String("reason", err.(*errors.NonRetryableError).Reason),
					zap.Error(err),
				)
				// 取消处理标记，让 RabbitMQ 发送到死信队列
				cache.UnmarkMessageProcessing(ctx, msg.MessageID)
				return fmt.Errorf("non-retryable error in journey timeout: %w", err)
			}

			// 3. 其他可重试错误：取消标记，允许重试
			logger.Logger.Warn("Retryable error in journey timeout processing, will retry",
				zap.String("message_id", msg.MessageID),
				zap.Int64("journey_id", msg.JourneyID),
				zap.Int64("user_id", msg.UserID),
				zap.Error(err),
			)
			cache.UnmarkMessageProcessing(ctx, msg.MessageID)
			return fmt.Errorf("failed to process journey timeout (will retry): %w", err)
		}

		// 发布通知消息到队列（类似打卡超时的逻辑）
		db := database.DB().WithContext(ctx)
		q := query.Use(db)

		// 查询今天创建的行程超时通知任务
		today := time.Now().Truncate(24 * time.Hour)
		tasks, err := q.NotificationTask.
			Where(q.NotificationTask.Category.Eq(string(model.NotificationCategoryJourneyTimeout))).
			Where(q.NotificationTask.CreatedAt.Gte(today)).
			Where(q.NotificationTask.Status.Eq(string(model.NotificationTaskStatusPending))).
			Find()
		if err == nil {
			for _, task := range tasks {
				// 查询用户 public_id
				user, err := q.User.GetByID(task.UserID)
				if err != nil {
					logger.Logger.Warn("Failed to query user for notification",
						zap.Int64("task_id", task.ID),
						zap.Error(err),
					)
					continue
				}

				// 构建通知消息
				notificationMsg := model.NotificationMessage{
					MessageID: fmt.Sprintf("notification_%d", task.TaskCode),
					TaskCode:  task.TaskCode,
					UserID:    user.PublicID,
					Category:  string(task.Category),
					Channel:   string(task.Channel),
					PhoneHash: "",
					Payload:   task.Payload,
				}

				if task.ContactPhoneHash != nil {
					notificationMsg.PhoneHash = *task.ContactPhoneHash
				}

				// 发布到队列
				if err := PublishSMSNotification(notificationMsg); err != nil {
					logger.Logger.Error("Failed to publish notification message",
						zap.Int64("task_code", task.TaskCode),
						zap.Error(err),
					)
				}
			}
		}

		// 标记消息已处理
		if err := cache.MarkMessageProcessed(ctx, msg.MessageID, 48*time.Hour); err != nil {
			logger.Logger.Warn("Failed to mark message as processed",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
		}

		return nil
	}

	return mq.Consume(mq.ConsumeOptions{
		Queue:         "scheduler.journey.timeout",
		ConsumerTag:   "journey_timeout_consumer",
		PrefetchCount: 10,
		Handler:       handler,
		Context:       ctx,
	})
}

//==================== 分界线 ====================

// StartSMSNotificationConsumer 启动短信通知消费者，也就是打卡部分的通知
func StartSMSNotificationConsumer(ctx context.Context) error {
	// 消息队列发过来的消息需要消费
	handler := func(body []byte) error {
		var msg model.NotificationMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal SMS notification message: %w", err)
		}

		//使用 SETNX 原子性地检查并标记消息正在处理
		processed, err := cache.TryMarkMessageProcessing(ctx, msg.MessageID, 24*time.Hour)
		if err != nil {
			logger.Logger.Warn("Failed to check message processed status",
				zap.String("message_id", msg.MessageID),
				zap.Int64("task_code", msg.TaskCode),
				zap.Error(err),
			)

		} else if !processed {
			logger.Logger.Debug("Message already processed or being processed, skipping",
				zap.String("message_id", msg.MessageID),
				zap.Int64("task_code", msg.TaskCode),
			)
			return &errors.SkipMessageError{Reason: fmt.Sprintf("Message %s already processed", msg.MessageID)}
		}

		logger.Logger.Debug("Processing SMS notification",
			zap.String("message_id", msg.MessageID),
			zap.Int64("task_code", msg.TaskCode),
			zap.Int64("user_id", msg.UserID),
		)

		// 调用 service 层处理短信发送逻辑
		if notificationService == nil {
			logger.Logger.Error("NotificationService not initialized",
				zap.String("message_id", msg.MessageID),
			)
			cache.UnmarkMessageProcessing(ctx, msg.MessageID)
			return fmt.Errorf("notification service not initialized")
		}

		if msg.TaskCode == 0 {
			logger.Logger.Error("TaskCode is missing in message",
				zap.String("message_id", msg.MessageID),
			)
			// 标记处理失败，允许重试
			cache.UnmarkMessageProcessing(ctx, msg.MessageID)
			return fmt.Errorf("task_code is required")
		}

		err = notificationService.SendSMS(
			ctx,
			msg.TaskCode,
			msg.UserID,
			msg.PhoneHash,
			msg.Payload,
		)

		// 改进的错误处理逻辑
		if err != nil {
			// 1. 可跳过的错误：标记为已处理，不重试
			if errors.IsSkipMessageError(err) {
				logger.Logger.Info("Skipping message processing",
					zap.String("message_id", msg.MessageID),
					zap.String("reason", err.(*errors.SkipMessageError).Reason),
					zap.Error(err),
				)
				// 标记为已处理，避免重复处理
				if markErr := cache.MarkMessageProcessed(ctx, msg.MessageID, 48*time.Hour); markErr != nil {
					logger.Logger.Warn("Failed to mark skipped message as processed",
						zap.String("message_id", msg.MessageID),
						zap.Error(markErr),
					)
				}
				return nil // 返回 nil 表示成功处理（跳过）
			}

			// 2. 不可重试错误：记录日志并发送到死信队列（通过返回错误让 RabbitMQ 处理）
			if errors.IsNonRetryableError(err) {
				logger.Logger.Error("Non-retryable error occurred, message will be sent to DLQ",
					zap.String("message_id", msg.MessageID),
					zap.String("error_code", err.(*errors.NonRetryableError).Code),
					zap.String("reason", err.(*errors.NonRetryableError).Reason),
					zap.Error(err),
				)
				// 取消处理标记，让 RabbitMQ 发送到死信队列
				cache.UnmarkMessageProcessing(ctx, msg.MessageID)
				return fmt.Errorf("non-retryable error: %w", err)
			}

			// 3. 额度不足错误：特殊处理，发布额度耗尽事件
			if err == errors.QuotaInsufficient {
				logger.Logger.Warn("Quota insufficient for user",
					zap.String("message_id", msg.MessageID),
					zap.Int64("user_id", msg.UserID),
					zap.String("channel", "sms"),
					zap.Error(err),
				)
				// TODO: 发布额度耗尽事件到事件队列，提醒用户充值
				// 目前先标记为跳过，避免无限重试
				if markErr := cache.MarkMessageProcessed(ctx, msg.MessageID, 24*time.Hour); markErr != nil {
					logger.Logger.Warn("Failed to mark quota insufficient message as processed",
						zap.String("message_id", msg.MessageID),
						zap.Error(markErr),
					)
				}
				return nil
			}

			// 4. 其他可重试错误：取消标记，允许重试
			logger.Logger.Warn("Retryable error occurred, will retry",
				zap.String("message_id", msg.MessageID),
				zap.Int64("user_id", msg.UserID),
				zap.Error(err),
			)
			cache.UnmarkMessageProcessing(ctx, msg.MessageID)
			return fmt.Errorf("failed to send SMS (will retry): %w", err)
		}

		// 处理成功，标记为已完成
		if err := cache.MarkMessageProcessed(ctx, msg.MessageID, 48*time.Hour); err != nil {
			logger.Logger.Warn("Failed to mark message as processed",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
			// 不影响主流程，因为已经处理成功了
		}

		// 如果是打卡提醒消息，更新 reminder_sent_at 字段
		if msg.Category == "check_in_reminder" && msg.CheckInDate != "" {
			if err := updateReminderSentAt(ctx, msg.UserID, msg.CheckInDate); err != nil {
				// 记录错误但不影响主流程（短信已发送成功）
				logger.Logger.Warn("Failed to update reminder_sent_at",
					zap.String("message_id", msg.MessageID),
					zap.Int64("user_id", msg.UserID),
					zap.String("check_in_date", msg.CheckInDate),
					zap.Error(err),
				)
			}
		}

		return nil
	}

	return mq.Consume(mq.ConsumeOptions{
		Queue:         "notification.sms",
		ConsumerTag:   "sms_notification_consumer",
		PrefetchCount: 20, // 不足的话届时再说
		Handler:       handler,
		Context:       ctx,
	})
}

// updateReminderSentAt 更新打卡记录的 reminder_sent_at 字段
func updateReminderSentAt(ctx context.Context, publicUserID int64, checkInDateStr string) error {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	user, err := q.User.GetByPublicID(publicUserID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("user not found: public_id=%d", publicUserID)
		}
		return fmt.Errorf("failed to query user: %w", err)
	}

	checkInDate, err := time.Parse("2006-01-02", checkInDateStr)
	if err != nil {
		return fmt.Errorf("failed to parse check_in_date: %w", err)
	}

	now := time.Now()
	checkIn := &model.DailyCheckIn{
		UserID:         user.ID,
		CheckInDate:    checkInDate,
		ReminderSentAt: &now,
		Status:         model.CheckInStatusPending, // 默认状态为 pending
	}

	// 使用 GORM 的 Clauses 实现 Upsert
	// 根据 GORM 文档，当指定 Columns 时，这些列必须有唯一约束
	// 添加了 uniqueIndex:idx_daily_check_ins_user_date
	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "check_in_date"}},
		DoUpdates: clause.AssignmentColumns([]string{"reminder_sent_at", "updated_at"}),
	}).Create(checkIn).Error

	if err != nil {
		return fmt.Errorf("failed to update reminder_sent_at: %w", err)
	}

	logger.Logger.Debug("Updated reminder_sent_at",
		zap.Int64("user_id", user.ID),
		zap.String("check_in_date", checkInDateStr),
		zap.Time("reminder_sent_at", now),
	)

	return nil
}

// StartAllConsumers 启动所有消费者（在服务启动时调用）， 定义程序终对应的消费者
func StartAllConsumers(ctx context.Context) {
	var wg sync.WaitGroup

	consumers := []struct {
		name     string
		consumer func(context.Context) error
	}{
		{"check_in_reminder", StartCheckInReminderConsumer},
		{"check_in_timeout", StartCheckInTimeoutConsumer},
		{"journey_timeout", StartJourneyTimeoutConsumer},
		{"sms_notification", StartSMSNotificationConsumer},
		//{"voice_notification", StartVoiceNotificationConsumer}, 现有全部切换为短信通知
	}

	for _, c := range consumers {
		wg.Add(1)
		go func(name string, consumer func(context.Context) error) {
			defer wg.Done()

			logger.Logger.Info("Starting consumer",
				zap.String("consumer_name", name),
			)

			if err := consumer(ctx); err != nil {
				logger.Logger.Error("Consumer exited with error",
					zap.String("consumer_name", name),
					zap.Error(err),
				)
			}
		}(c.name, c.consumer)
	}

	wg.Wait()

	logger.Logger.Info("All consumers started")
}

// // StartVoiceNotificationConsumer 启动语音通知消费者
// func StartVoiceNotificationConsumer(ctx context.Context) error {
// 	handler := func(body []byte) error {
// 		var msg model.NotificationMessage
// 		if err := json.Unmarshal(body, &msg); err != nil {
// 			return fmt.Errorf("failed to unmarshal voice notification message: %w", err)
// 		}

// 		processed, err := cache.TryMarkMessageProcessing(ctx, msg.MessageID, 24*time.Hour)
// 		if err != nil {
// 			logger.Logger.Warn("Failed to check message processed status",
// 				zap.String("message_id", msg.MessageID),
// 				zap.Int64("task_code", msg.TaskCode),
// 				zap.Error(err),
// 			)
// 		} else if !processed {
// 			logger.Logger.Info("Message already processed or being processed, skipping",
// 				zap.String("message_id", msg.MessageID),
// 				zap.Int64("task_code", msg.TaskCode),
// 			)
// 			return &errors.SkipMessageError{Reason: fmt.Sprintf("Message %s already processed", msg.MessageID)}
// 		}

// 		logger.Logger.Info("Processing voice notification",
// 			zap.String("message_id", msg.MessageID),
// 			zap.Int64("task_code", msg.TaskCode),
// 			zap.Int64("user_id", msg.UserID),
// 		)

// 		// TODO: 调用 service 层处理语音发送逻辑
// 		// 1. 更新 notification_tasks.status = 'processing'
// 		// 2. 调用语音服务发送外呼
// 		// 3. 更新 notification_tasks.status = 'success'/'failed'
// 		// 4. 记录 contact_attempts
// 		// 5. 扣除额度

// 		// notificationService := service.Notification()
// 		// err = notificationService.SendVoice(ctx, msg.TaskCode, msg.UserID, msg.PhoneHash, msg.Payload)
// 		// if err != nil {
// 		// 	// 处理失败，取消标记，允许重试
// 		// 	cache.UnmarkMessageProcessing(ctx, msg.MessageID)
// 		// 	if errors.IsSkipMessageError(err) {
// 		// 		return err
// 		// 	}
// 		// 	return fmt.Errorf("failed to send voice: %w", err)
// 		// }

// 		// 【幂等性标记】处理完成后标记消息已处理（延长 TTL）
// 		if err := cache.MarkMessageProcessed(ctx, msg.MessageID, 48*time.Hour); err != nil {
// 			logger.Logger.Warn("Failed to mark message as processed",
// 				zap.String("message_id", msg.MessageID),
// 				zap.Error(err),
// 			)
// 		}

// 		return nil
// 	}

// 	return mq.Consume(mq.ConsumeOptions{
// 		Queue:         "notification.voice",
// 		ConsumerTag:   "voice_notification_consumer",
// 		PrefetchCount: 10,
// 		Handler:       handler,
// 		Context:       ctx,
// 	})
// }

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

// StartCheckInReminderConsumer 启动打卡提醒消费者
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
			logger.Logger.Info("Message already processed or being processed, skipping",
				zap.String("message_id", msg.MessageID),
				zap.String("batch_id", msg.BatchID),
			)
			return &errors.SkipMessageError{Reason: fmt.Sprintf("Message %s already processed", msg.MessageID)}
		}

		logger.Logger.Info("Processing check-in reminder batch",
			zap.String("message_id", msg.MessageID),
			zap.String("batch_id", msg.BatchID),
			zap.Int("user_count", len(msg.UserIDs)),
		)

		// TODO: 调用 service 层处理打卡提醒逻辑
		// 1. 批量查询用户信息
		// 2. 批量检查额度
		// 3. 批量创建 notification_tasks
		// 4. 批量发送短信
		// 5. 批量更新 daily_check_ins.reminder_sent_at

		// checkInService := service.CheckIn()
		// err = checkInService.ProcessReminderBatch(ctx, msg.UserIDs, msg.CheckInDate)
		// if err != nil {
		// 	// 处理失败，取消标记，允许重试
		// 	cache.UnmarkMessageProcessing(ctx, msg.MessageID)
		// 	return fmt.Errorf("failed to process reminder batch: %w", err)
		// }

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
	})
}

// StartCheckInTimeoutConsumer 启动打卡超时消费者
func StartCheckInTimeoutConsumer(ctx context.Context) error {
	handler := func(body []byte) error {
		var msg model.CheckInTimeoutMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal check-in timeout message: %w", err)
		}

		// 【幂等性检查】使用 SETNX 原子性地检查并标记消息正在处理
		processed, err := cache.TryMarkMessageProcessing(ctx, msg.MessageID, 24*time.Hour)
		if err != nil {
			logger.Logger.Warn("Failed to check message processed status",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
		} else if !processed {
			logger.Logger.Info("Message already processed or being processed, skipping",
				zap.String("message_id", msg.MessageID),
				zap.String("batch_id", msg.BatchID),
			)
			return &errors.SkipMessageError{Reason: fmt.Sprintf("Message %s already processed", msg.MessageID)}
		}

		logger.Logger.Info("Processing check-in timeout batch",
			zap.String("message_id", msg.MessageID),
			zap.String("batch_id", msg.BatchID),
			zap.Int("user_count", len(msg.UserIDs)),
		)

		// TODO: 调用 service 层处理打卡超时逻辑
		// 1. 批量更新 daily_check_ins.alert_triggered_at
		// 2. 批量查询紧急联系人列表
		// 3. 按优先级批量创建通知任务
		// 4. 批量发送外呼/短信
		// 5. 批量记录 contact_attempts

		// checkInService := service.CheckIn()
		// err = checkInService.ProcessTimeoutBatch(ctx, msg.UserIDs, msg.CheckInDate)
		// if err != nil {
		// 	// 处理失败，取消标记，允许重试
		// 	cache.UnmarkMessageProcessing(ctx, msg.MessageID)
		// 	return fmt.Errorf("failed to process timeout batch: %w", err)
		// }

		// 【幂等性标记】处理完成后标记消息已处理（延长 TTL）
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
	})
}

// StartJourneyTimeoutConsumer 启动行程超时消费者
func StartJourneyTimeoutConsumer(ctx context.Context) error {
	handler := func(body []byte) error {
		var msg model.JourneyTimeoutMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal journey timeout message: %w", err)
		}

		// 【幂等性检查】使用 SETNX 原子性地检查并标记消息正在处理
		processed, err := cache.TryMarkMessageProcessing(ctx, msg.MessageID, 24*time.Hour)
		if err != nil {
			logger.Logger.Warn("Failed to check message processed status",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
		} else if !processed {
			logger.Logger.Info("Message already processed or being processed, skipping",
				zap.String("message_id", msg.MessageID),
				zap.Int64("journey_id", msg.JourneyID),
			)
			return &errors.SkipMessageError{Reason: fmt.Sprintf("Message %s already processed", msg.MessageID)}
		}

		logger.Logger.Info("Processing journey timeout",
			zap.String("message_id", msg.MessageID),
			zap.Int64("journey_id", msg.JourneyID),
			zap.Int64("user_id", msg.UserID),
		)

		// TODO: 调用 service 层处理行程超时逻辑
		// 1. 更新 journey.status = 'timeout'
		// 2. 获取紧急联系人列表
		// 3. 按优先级创建通知任务
		// 4. 发送外呼/短信

		// journeyService := service.Journey()
		// err = journeyService.ProcessTimeout(ctx, msg.JourneyID, msg.UserID)
		// if err != nil {
		// 	// 处理失败，取消标记，允许重试
		// 	cache.UnmarkMessageProcessing(ctx, msg.MessageID)
		// 	return fmt.Errorf("failed to process journey timeout: %w", err)
		// }

		// 【幂等性标记】处理完成后标记消息已处理（延长 TTL）
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
	})
}

//==================== 分界线 ====================

// StartSMSNotificationConsumer 启动短信通知消费者
func StartSMSNotificationConsumer(ctx context.Context) error {
	handler := func(body []byte) error {
		var msg model.NotificationMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal SMS notification message: %w", err)
		}

		// 【幂等性检查】使用 SETNX 原子性地检查并标记消息正在处理
		processed, err := cache.TryMarkMessageProcessing(ctx, msg.MessageID, 24*time.Hour)
		if err != nil {
			logger.Logger.Warn("Failed to check message processed status",
				zap.String("message_id", msg.MessageID),
				zap.Int64("task_code", msg.TaskCode),
				zap.Error(err),
			)

		} else if !processed {
			logger.Logger.Info("Message already processed or being processed, skipping",
				zap.String("message_id", msg.MessageID),
				zap.Int64("task_code", msg.TaskCode),
			)
			return &errors.SkipMessageError{Reason: fmt.Sprintf("Message %s already processed", msg.MessageID)}
		}

		logger.Logger.Info("Processing SMS notification",
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

		//这里的逻辑存在问题，应该允许充值，不应该直接上锁从而不再处理
		if err != nil {
			// 如果是 SkipMessageError，标记为已处理并跳过（不重试）
			if errors.IsSkipMessageError(err) {
				// 标记为已处理，避免重复处理
				if markErr := cache.MarkMessageProcessed(ctx, msg.MessageID, 48*time.Hour); markErr != nil {
					logger.Logger.Warn("Failed to mark skipped message as processed",
						zap.String("message_id", msg.MessageID),
						zap.Error(markErr),
					)
				}
				return err
			}

			// 其他错误：取消标记，允许重试
			cache.UnmarkMessageProcessing(ctx, msg.MessageID)
			return fmt.Errorf("failed to send SMS: %w", err)
		}

		// 处理成功，标记为已完成（TTL 延长到 48 小时）
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
	})
}

// updateReminderSentAt 更新打卡记录的 reminder_sent_at 字段
func updateReminderSentAt(ctx context.Context, publicUserID int64, checkInDateStr string) error {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	// 查询用户（使用 public_id）
	user, err := q.User.GetByPublicID(publicUserID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("user not found: public_id=%d", publicUserID)
		}
		return fmt.Errorf("failed to query user: %w", err)
	}

	// 解析打卡日期
	checkInDate, err := time.Parse("2006-01-02", checkInDateStr)
	if err != nil {
		return fmt.Errorf("failed to parse check_in_date: %w", err)
	}

	// 使用 Upsert 更新 reminder_sent_at 字段（如果记录不存在则创建）
	now := time.Now()
	checkIn := &model.DailyCheckIn{
		UserID:         user.ID,
		CheckInDate:    checkInDate,
		ReminderSentAt: &now,
		Status:         model.CheckInStatusPending, // 默认状态为 pending
	}

	// 使用 GORM 的 Clauses 实现 Upsert
	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "check_in_date"}},
		DoUpdates: clause.AssignmentColumns([]string{"reminder_sent_at", "updated_at"}),
	}).Create(checkIn).Error

	if err != nil {
		return fmt.Errorf("failed to update reminder_sent_at: %w", err)
	}

	logger.Logger.Info("Updated reminder_sent_at",
		zap.Int64("user_id", user.ID),
		zap.String("check_in_date", checkInDateStr),
		zap.Time("reminder_sent_at", now),
	)

	return nil
}




// StartVoiceNotificationConsumer 启动语音通知消费者
func StartVoiceNotificationConsumer(ctx context.Context) error {
	handler := func(body []byte) error {
		var msg model.NotificationMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal voice notification message: %w", err)
		}

		// 【幂等性检查】使用 SETNX 原子性地检查并标记消息正在处理
		processed, err := cache.TryMarkMessageProcessing(ctx, msg.MessageID, 24*time.Hour)
		if err != nil {
			logger.Logger.Warn("Failed to check message processed status",
				zap.String("message_id", msg.MessageID),
				zap.Int64("task_code", msg.TaskCode),
				zap.Error(err),
			)
		} else if !processed {
			logger.Logger.Info("Message already processed or being processed, skipping",
				zap.String("message_id", msg.MessageID),
				zap.Int64("task_code", msg.TaskCode),
			)
			return &errors.SkipMessageError{Reason: fmt.Sprintf("Message %s already processed", msg.MessageID)}
		}

		logger.Logger.Info("Processing voice notification",
			zap.String("message_id", msg.MessageID),
			zap.Int64("task_code", msg.TaskCode),
			zap.Int64("user_id", msg.UserID),
		)

		// TODO: 调用 service 层处理语音发送逻辑
		// 1. 更新 notification_tasks.status = 'processing'
		// 2. 调用语音服务发送外呼
		// 3. 更新 notification_tasks.status = 'success'/'failed'
		// 4. 记录 contact_attempts
		// 5. 扣除额度

		// notificationService := service.Notification()
		// err = notificationService.SendVoice(ctx, msg.TaskCode, msg.UserID, msg.PhoneHash, msg.Payload)
		// if err != nil {
		// 	// 处理失败，取消标记，允许重试
		// 	cache.UnmarkMessageProcessing(ctx, msg.MessageID)
		// 	if errors.IsSkipMessageError(err) {
		// 		return err
		// 	}
		// 	return fmt.Errorf("failed to send voice: %w", err)
		// }

		// 【幂等性标记】处理完成后标记消息已处理（延长 TTL）
		if err := cache.MarkMessageProcessed(ctx, msg.MessageID, 48*time.Hour); err != nil {
			logger.Logger.Warn("Failed to mark message as processed",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
		}

		return nil
	}

	return mq.Consume(mq.ConsumeOptions{
		Queue:         "notification.voice",
		ConsumerTag:   "voice_notification_consumer",
		PrefetchCount: 10,
		Handler:       handler,
	})
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
		{"voice_notification", StartVoiceNotificationConsumer},
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

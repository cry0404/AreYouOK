package queue

import (
	// "AreYouOK/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"AreYouOK/pkg/logger"
	"AreYouOK/storage/mq"
)

// StartCheckInReminderConsumer 启动打卡提醒消费者
func StartCheckInReminderConsumer(ctx context.Context) error {
	handler := func(body []byte) error {
		var msg CheckInReminderMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal check-in reminder message: %w", err)
		}

		logger.Logger.Info("Processing check-in reminder batch",
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
		// checkInService.ProcessReminderBatch(ctx, msg.UserIDs, msg.CheckInDate)

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
		var msg CheckInTimeoutMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal check-in timeout message: %w", err)
		}

		logger.Logger.Info("Processing check-in timeout batch",
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
		// checkInService.ProcessTimeoutBatch(ctx, msg.UserIDs, msg.CheckInDate)

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
		var msg JourneyTimeoutMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal journey timeout message: %w", err)
		}

		logger.Logger.Info("Processing journey timeout",
			zap.Int64("journey_id", msg.JourneyID),
			zap.Int64("user_id", msg.UserID),
		)

		// TODO: 调用 service 层处理行程超时逻辑
		// 1. 更新 journey.status = 'timeout'
		// 2. 获取紧急联系人列表
		// 3. 按优先级创建通知任务
		// 4. 发送外呼/短信

		// journeyService := service.Journey()
		// journeyService.ProcessTimeout(ctx, msg.JourneyID, msg.UserID)

		return nil
	}

	return mq.Consume(mq.ConsumeOptions{
		Queue:         "scheduler.journey.timeout",
		ConsumerTag:   "journey_timeout_consumer",
		PrefetchCount: 10,
		Handler:       handler,
	})
}

// StartSMSNotificationConsumer 启动短信通知消费者
func StartSMSNotificationConsumer(ctx context.Context) error {
	handler := func(body []byte) error {
		var msg NotificationMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal SMS notification message: %w", err)
		}

		logger.Logger.Info("Processing SMS notification",
			zap.Int64("task_id", msg.TaskID),
			zap.Int64("user_id", msg.UserID),
		)

		// TODO: 调用 service 层处理短信发送逻辑
		// 1. 更新 notification_tasks.status = 'processing'
		// 2. 调用短信服务发送短信
		// 3. 更新 notification_tasks.status = 'success'/'failed'
		// 4. 记录 contact_attempts
		// 5. 扣除额度

		// notificationService := service.Notification()
		// notificationService.SendSMS(ctx, msg.TaskID, msg.UserID, msg.PhoneHash, msg.Payload)

		return nil
	}

	return mq.Consume(mq.ConsumeOptions{
		Queue:         "notification.sms",
		ConsumerTag:   "sms_notification_consumer",
		PrefetchCount: 20, // 不足的话届时再说
		Handler:       handler,
	})
}

// StartVoiceNotificationConsumer 启动语音通知消费者
func StartVoiceNotificationConsumer(ctx context.Context) error {
	handler := func(body []byte) error {
		var msg NotificationMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			return fmt.Errorf("failed to unmarshal voice notification message: %w", err)
		}

		logger.Logger.Info("Processing voice notification",
			zap.Int64("task_id", msg.TaskID),
			zap.Int64("user_id", msg.UserID),
		)

		// TODO: 调用 service 层处理语音发送逻辑
		// 1. 更新 notification_tasks.status = 'processing'
		// 2. 调用语音服务发送外呼
		// 3. 更新 notification_tasks.status = 'success'/'failed'
		// 4. 记录 contact_attempts
		// 5. 扣除额度

		// notificationService := service.Notification()
		// notificationService.SendVoice(ctx, msg.TaskID, msg.UserID, msg.PhoneHash, msg.Payload)

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

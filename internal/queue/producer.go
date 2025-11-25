package queue

import (
	"fmt"
	"time"

	"AreYouOK/internal/model"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/storage/mq"

	"go.uber.org/zap"
)

// PublishCheckInReminder 发布打卡提醒消息（延迟消息）
func PublishCheckInReminder(msg model.CheckInReminderMessage) error {
	if msg.MessageID == "" {
		id, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
		if err != nil {
			logger.Logger.Error("Failed to generate message ID",
				zap.String("batch_id", msg.BatchID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to generate message ID: %w", err)
		}
		msg.MessageID = fmt.Sprintf("ci_reminder_%d", id)
	}

	delay := time.Duration(msg.DelaySeconds) * time.Second

	err := mq.PublishDelayedMessage(
		"scheduler.delayed",           // exchange
		"scheduler.check_in.reminder", // routing key
		delay,                         // delay
		msg,                           // body
	)

	if err != nil {
		logger.Logger.Error("Failed to publish check-in reminder message",
			zap.String("batch_id", msg.BatchID),
			zap.Int("user_count", len(msg.UserIDs)),
			zap.Error(err),
		)
		return err
	}

	logger.Logger.Info("Published check-in reminder message",
		zap.String("message_id", msg.MessageID),
		zap.String("batch_id", msg.BatchID),
		zap.Int("user_count", len(msg.UserIDs)),
		zap.Duration("delay", delay),
	)

	return nil
}

// PublishCheckInTimeout 发布打卡超时消息（延迟消息）
func PublishCheckInTimeout(msg model.CheckInTimeoutMessage) error {
	// 生成 MessageID 如果为空
	if msg.MessageID == "" {
		id, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
		if err != nil {
			logger.Logger.Error("Failed to generate message ID",
				zap.String("batch_id", msg.BatchID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to generate message ID: %w", err)
		}
		msg.MessageID = fmt.Sprintf("ci_timeout_%d", id)
	}

	delay := time.Duration(msg.DelaySeconds) * time.Second

	err := mq.PublishDelayedMessage(
		"scheduler.delayed",
		"scheduler.check_in.timeout",
		delay,
		msg,
	)

	if err != nil {
		logger.Logger.Error("Failed to publish check-in timeout message",
			zap.String("batch_id", msg.BatchID),
			zap.Int("user_count", len(msg.UserIDs)),
			zap.Error(err),
		)
		return err
	}

	logger.Logger.Info("Published check-in timeout message",
		zap.String("message_id", msg.MessageID),
		zap.String("batch_id", msg.BatchID),
		zap.Int("user_count", len(msg.UserIDs)),
		zap.Duration("delay", delay),
	)

	return nil
}

// PublishJourneyTimeout 发布行程超时消息（延迟消息）
// 注意：如果 delay > 24小时，此函数会返回错误，调用者应该使用定时任务扫描
func PublishJourneyTimeout(msg model.JourneyTimeoutMessage) error {
	// 生成 MessageID 如果为空
	if msg.MessageID == "" {
		id, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
		if err != nil {
			logger.Logger.Error("Failed to generate message ID",
				zap.Int64("journey_id", msg.JourneyID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to generate message ID: %w", err)
		}
		msg.MessageID = fmt.Sprintf("journey_timeout_%d", id)
	}

	delay := time.Duration(msg.DelaySeconds) * time.Second

	// 检查延迟时间是否超过 24 小时（RabbitMQ 延迟消息的限制）
	if delay > 24*time.Hour {
		return fmt.Errorf("delay %v exceeds 24 hours limit, use scheduled task instead", delay)
	}

	err := mq.PublishDelayedMessage(
		"scheduler.delayed",
		"scheduler.journey.timeout",
		delay,
		msg,
	)

	if err != nil {
		logger.Logger.Error("Failed to publish journey timeout message",
			zap.Int64("journey_id", msg.JourneyID),
			zap.Int64("user_id", msg.UserID),
			zap.Error(err),
		)
		return err
	}

	logger.Logger.Info("Published journey timeout message",
		zap.String("message_id", msg.MessageID),
		zap.Int64("journey_id", msg.JourneyID),
		zap.Int64("user_id", msg.UserID),
		zap.Duration("delay", delay),
	)

	return nil
}

// ScheduleJourneyTimeoutIfNeeded 根据预计返回时间决定是否发送延迟消息
// 如果延迟时间 <= 1天，发送延迟消息
// 如果延迟时间 > 1天，不发送消息，等待定时任务扫描
// 返回是否已发送延迟消息
func ScheduleJourneyTimeoutIfNeeded(journeyID, userID int64, expectedReturnTime time.Time) (bool, error) {
	now := time.Now()
	delay := expectedReturnTime.Add(10 * time.Minute).Sub(now) // 预计返回时间 + 10分钟

	// 如果已经超时，立即发送（延迟0秒）
	if delay <= 0 {
		delay = 0
	}

	// 如果延迟时间 > 1天，不发送延迟消息，等待定时任务扫描
	if delay > 24*time.Hour {
		logger.Logger.Info("Journey delay exceeds 24 hours, will be handled by scheduled task",
			zap.Int64("journey_id", journeyID),
			zap.Int64("user_id", userID),
			zap.Duration("delay", delay),
			zap.Time("expected_return_time", expectedReturnTime),
		)
		return false, nil
	}

	messageID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
	if err != nil {
		return false, fmt.Errorf("failed to generate message ID: %w", err)
	}

	// 构建超时消息
	timeoutMsg := model.JourneyTimeoutMessage{
		MessageID:    fmt.Sprintf("journey_timeout_%d", messageID),
		ScheduledAt:  now.Format(time.RFC3339),
		JourneyID:    journeyID,
		UserID:       userID,
		DelaySeconds: int(delay.Seconds()),
	}

	// 发布延迟消息
	if err := PublishJourneyTimeout(timeoutMsg); err != nil {
		return false, fmt.Errorf("failed to publish journey timeout message: %w", err)
	}

	return true, nil
}

// PublishSMSNotification 发布短信通知任务
func PublishSMSNotification(msg model.NotificationMessage) error {

	if msg.MessageID == "" {
		logger.Logger.Warn("MessageID is empty, generating fallback MessageID",
			zap.Int64("task_code", msg.TaskCode),
			zap.Int64("user_id", msg.UserID),
		)
		id, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
		if err != nil {
			logger.Logger.Error("Failed to generate message ID",
				zap.Int64("task_code", msg.TaskCode),
				zap.Error(err),
			)
			return fmt.Errorf("failed to generate message ID: %w", err)
		}
		msg.MessageID = fmt.Sprintf("notification_sms_%d", id)
	}

	// 根据 category 构建 routing key，匹配 notification.sms.* 模式
	routingKey := fmt.Sprintf("notification.sms.%s", msg.Category)

	err := mq.PublishMessage(
		"notification.topic",
		routingKey,
		msg,
	)

	if err != nil {
		logger.Logger.Error("Failed to publish SMS notification",
			zap.String("message_id", msg.MessageID),
			zap.Int64("task_code", msg.TaskCode),
			zap.Int64("user_id", msg.UserID),
			zap.String("routing_key", routingKey),
			zap.Error(err),
		)
		return err
	}

	logger.Logger.Info("Published SMS notification",
		zap.String("message_id", msg.MessageID),
		zap.Int64("task_code", msg.TaskCode),
		zap.Int64("user_id", msg.UserID),
		zap.String("routing_key", routingKey),
	)

	return nil
}

// PublishCheckInTimeoutEvent 发布打卡超时事件
func PublishCheckInTimeoutEvent(userID int64, checkInDate string) error {
	event := model.EventMessage{
		EventKey:   "check_in.timeout",
		EventType:  "check_in_timeout",
		OccurredAt: time.Now().Format(time.RFC3339),
		Payload: map[string]interface{}{
			"user_id":       userID,
			"check_in_date": checkInDate,
		},
	}

	err := mq.PublishMessage(
		"events.topic",
		"check_in.timeout",
		event,
	)

	if err != nil {
		logger.Logger.Error("Failed to publish check-in timeout event",
			zap.Int64("user_id", userID),
			zap.String("check_in_date", checkInDate),
			zap.Error(err),
		)
		return err
	}

	return nil
}

// PublishJourneyTimeoutEvent 发布行程超时事件
func PublishJourneyTimeoutEvent(journeyID, userID int64) error {
	event := model.EventMessage{
		EventKey:   "journey.timeout",
		EventType:  "journey_timeout",
		OccurredAt: time.Now().Format(time.RFC3339),
		Payload: map[string]interface{}{
			"journey_id": journeyID,
			"user_id":    userID,
		},
	}

	err := mq.PublishMessage(
		"events.topic",
		"journey.timeout",
		event,
	)

	if err != nil {
		logger.Logger.Error("Failed to publish journey timeout event",
			zap.Int64("journey_id", journeyID),
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return err
	}

	return nil
}

// PublishQuotaDepletedEvent 发布额度耗尽事件
func PublishQuotaDepletedEvent(userID int64, channel string) error {
	event := model.EventMessage{
		EventKey:   "quota.depleted",
		EventType:  "quota_depleted",
		OccurredAt: time.Now().Format(time.RFC3339),
		Payload: map[string]interface{}{
			"user_id": userID,
			"channel": channel, // sms or voice
		},
	}

	err := mq.PublishMessage(
		"events.topic",
		"quota.depleted",
		event,
	)

	if err != nil {
		logger.Logger.Error("Failed to publish quota depleted event",
			zap.Int64("user_id", userID),
			zap.String("channel", channel),
			zap.Error(err),
		)
		return err
	}

	return nil
}



// // PublishVoiceNotification 发布语音通知任务
// func PublishVoiceNotification(msg model.NotificationMessage) error {
// 	if msg.MessageID == "" {
// 		logger.Logger.Warn("MessageID is empty, generating fallback MessageID",
// 			zap.Int64("task_code", msg.TaskCode),
// 			zap.Int64("user_id", msg.UserID),
// 		)
// 		id, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
// 		if err != nil {
// 			logger.Logger.Error("Failed to generate message ID",
// 				zap.Int64("task_code", msg.TaskCode),
// 				zap.Error(err),
// 			)
// 			return fmt.Errorf("failed to generate message ID: %w", err)
// 		}
// 		msg.MessageID = fmt.Sprintf("notification_voice_%d", id)
// 	}

// 	// 根据 category 构建 routing key
// 	routingKey := fmt.Sprintf("notification.voice.%s", msg.Category)

// 	err := mq.PublishMessage(
// 		"notification.topic",
// 		routingKey,
// 		msg,
// 	)

// 	if err != nil {
// 		logger.Logger.Error("Failed to publish voice notification",
// 			zap.String("message_id", msg.MessageID),
// 			zap.Int64("task_code", msg.TaskCode),
// 			zap.Int64("user_id", msg.UserID),
// 			zap.String("routing_key", routingKey),
// 			zap.Error(err),
// 		)
// 		return err
// 	}

// 	logger.Logger.Info("Published voice notification",
// 		zap.String("message_id", msg.MessageID),
// 		zap.Int64("task_code", msg.TaskCode),
// 		zap.Int64("user_id", msg.UserID),
// 		zap.String("routing_key", routingKey),
// 	)

// 	return nil
// }
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
	// 生成 MessageID 如果为空
	if msg.MessageID == "" {
		id, err := snowflake.NextID()
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
		id, err := snowflake.NextID()
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
func PublishJourneyTimeout(msg model.JourneyTimeoutMessage) error {
	// 生成 MessageID 如果为空
	if msg.MessageID == "" {
		id, err := snowflake.NextID()
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

// PublishSMSNotification 发布短信通知任务
func PublishSMSNotification(msg model.NotificationMessage) error {
	// 生成 MessageID 如果为空（使用 Snowflake 确保唯一性）
	// 注意：正常情况下，调用者应该在创建消息时设置 MessageID
	// 这里的生成逻辑仅作为后备方案
	if msg.MessageID == "" {
		logger.Logger.Warn("MessageID is empty, generating fallback MessageID",
			zap.Int64("task_code", msg.TaskCode),
			zap.Int64("user_id", msg.UserID),
		)
		id, err := snowflake.NextID()
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

// PublishVoiceNotification 发布语音通知任务
func PublishVoiceNotification(msg model.NotificationMessage) error {
	// 生成 MessageID 如果为空（使用 Snowflake 确保唯一性）
	// 注意：正常情况下，调用者应该在创建消息时设置 MessageID
	if msg.MessageID == "" {
		logger.Logger.Warn("MessageID is empty, generating fallback MessageID",
			zap.Int64("task_code", msg.TaskCode),
			zap.Int64("user_id", msg.UserID),
		)
		id, err := snowflake.NextID()
		if err != nil {
			logger.Logger.Error("Failed to generate message ID",
				zap.Int64("task_code", msg.TaskCode),
				zap.Error(err),
			)
			return fmt.Errorf("failed to generate message ID: %w", err)
		}
		msg.MessageID = fmt.Sprintf("notification_voice_%d", id)
	}

	// 根据 category 构建 routing key
	routingKey := fmt.Sprintf("notification.voice.%s", msg.Category)

	err := mq.PublishMessage(
		"notification.topic",
		routingKey,
		msg,
	)

	if err != nil {
		logger.Logger.Error("Failed to publish voice notification",
			zap.String("message_id", msg.MessageID),
			zap.Int64("task_code", msg.TaskCode),
			zap.Int64("user_id", msg.UserID),
			zap.String("routing_key", routingKey),
			zap.Error(err),
		)
		return err
	}

	logger.Logger.Info("Published voice notification",
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

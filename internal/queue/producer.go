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

// PublishJourneyReminder 发布行程提醒消息（发送给用户本人，提醒打卡）
func PublishJourneyReminder(msg model.JourneyReminderMessage) error {
	if msg.MessageID == "" {
		id, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
		if err != nil {
			logger.Logger.Error("Failed to generate message ID",
				zap.Int64("journey_id", msg.JourneyID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to generate message ID: %w", err)
		}
		msg.MessageID = fmt.Sprintf("journey_reminder_%d", id)
	}

	delay := time.Duration(msg.DelaySeconds) * time.Second

	if delay > 24*time.Hour {
		return fmt.Errorf("delay %v exceeds 24 hours limit, use scheduled task instead", delay)
	}

	err := mq.PublishDelayedMessage(
		"scheduler.delayed",
		"scheduler.journey.reminder",
		delay,
		msg,
	)

	if err != nil {
		logger.Logger.Error("Failed to publish journey reminder message",
			zap.Int64("journey_id", msg.JourneyID),
			zap.Int64("user_id", msg.UserID),
			zap.Error(err),
		)
		return err
	}

	logger.Logger.Info("Published journey reminder message",
		zap.String("message_id", msg.MessageID),
		zap.Int64("journey_id", msg.JourneyID),
		zap.Int64("user_id", msg.UserID),
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


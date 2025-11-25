package service

import (
	"AreYouOK/config"
	"AreYouOK/internal/model"
	"AreYouOK/internal/repository/query"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/sms"
	"AreYouOK/storage/database"
	"AreYouOK/utils"
	"context"

	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 基于 task_id 来实现幂等性， message_id 来实现幂等性？或者统一为 message_id
type NotificationService struct{}

var (
	notificationService *NotificationService
	notificationOnce    sync.Once
)

func Notification() *NotificationService {
	notificationOnce.Do(func() {
		notificationService = &NotificationService{}
	})
	return notificationService
}

// taskCode 是一整个事务的任务，这个 notification 的 task， 而 MessageID 只针对这条消息的是否成功
// payload 基于 sms_Message
// 完整流程 -> 定时扫描，发送打卡信息，新注册默认进入消息队列
func (s *NotificationService) SendSMS(
	ctx context.Context,
	taskCode int64, // taskid 的生成
	userID int64,
	phoneHash string,
	payload map[string]interface{}, // 根据不同的 payload 决定是如何去发，以及存储对应的模板, payload 需要包含对应的场景，参数
) error {

	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	// 根据 taskCode 生成任务，在 check_in 中统一生成的，幂等性查询
	task, err := q.NotificationTask.GetByTaskCode(taskCode)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Logger.Warn("Notification task not found, may have been processed",
				zap.Int64("task_code", taskCode),
			)
			// 任务不存在，可能是重复消息，返回 SkipMessageError 跳过
			return &errors.SkipMessageError{Reason: "task not found"}
		}
		return fmt.Errorf("failed to query notification task: %w", err)
	}

	if task.Status == model.NotificationTaskStatusSuccess {
		logger.Logger.Info("Notification task already processed successfully",
			zap.Int64("task_code", taskCode),
			zap.Int64("task_id", task.ID),
		)
		return nil // 已处理，返回成功
	}

	if task.Status == model.NotificationTaskStatusProcessing {
		logger.Logger.Warn("Notification task is being processed by another consumer",
			zap.Int64("task_code", taskCode),
		)
		return fmt.Errorf("task is being processed")
	}

	user, err := q.User.GetByPublicID(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Logger.Warn("User not found",
				zap.Int64("user_id", userID),
			)
			return &errors.SkipMessageError{Reason: "user not found"}
		}
		return fmt.Errorf("failed to query user: %w", err)
	}

	// 解密手机号
	phone, err := utils.DecryptPhone(user.PhoneCipher)
	if err != nil {
		logger.Logger.Error("Failed to decrypt phone",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to decrypt phone: %w", err)
	}

	// 使用 balance_after 字段获取最新余额（更高效，不需要聚合所有历史记录）
	// 查询 SMS 渠道最新的交易记录，获取 balance_after
	smsTransactions, err := q.QuotaTransaction.
		Where(q.QuotaTransaction.UserID.Eq(user.ID)).
		Where(q.QuotaTransaction.Channel.Eq(string(model.QuotaChannelSMS))).
		Order(q.QuotaTransaction.CreatedAt.Desc()).
		Limit(1).
		Find()
	if err != nil {
		logger.Logger.Error("Failed to query SMS quota balance",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to query quota balance: %w", err)
	}

	var smsBalance int
	if len(smsTransactions) > 0 {
		smsBalance = smsTransactions[0].BalanceAfter
	} else {
		// 如果没有交易记录，余额为 0
		smsBalance = 0
	}

	smsUnitPriceCents := 5
	if smsBalance < smsUnitPriceCents {
		logger.Logger.Warn("Insufficient SMS quota",
			zap.Int64("user_id", userID),
			zap.Int("balance", smsBalance),
			zap.Int("required", smsUnitPriceCents),
		)
		// 额度不足，更新任务状态为失败，但不重试（返回 SkipMessageError 避免无限重试）
		now := time.Now()
		_, err = q.NotificationTask.WithContext(ctx).
			Where(q.NotificationTask.ID.Eq(task.ID)).
			Updates(map[string]interface{}{
				"status":       model.NotificationTaskStatusFailed,
				"processed_at": now,
			})
		if err != nil {
			logger.Logger.Error("Failed to update task status",
				zap.Int64("task_id", task.ID),
				zap.Error(err),
			)
		}
		// 返回 SkipMessageError，避免无限重试（额度不足是业务状态问题，重试不会改变结果)
		return &errors.SkipMessageError{Reason: fmt.Sprintf("quota insufficient: balance=%d, required=%d", smsBalance, smsUnitPriceCents)}
	}

	// 解析 payload 为具体的 SMSMessage
	smsMsg, err := model.ParseSMSMessage(payload)
	if err != nil {
		logger.Logger.Error("Failed to parse SMS message",
			zap.Int64("task_code", taskCode),
			zap.Any("payload", payload),
			zap.Error(err),
		)
		// 解析失败，更新任务状态为失败
		now := time.Now()
		_, updateErr := q.NotificationTask.WithContext(ctx).
			Where(q.NotificationTask.ID.Eq(task.ID)).
			Updates(map[string]interface{}{
				"status":       model.NotificationTaskStatusFailed,
				"processed_at": now,
			})
		if updateErr != nil {
			logger.Logger.Error("Failed to update task status",
				zap.Int64("task_id", task.ID),
				zap.Error(updateErr),
			)
		}
		// 解析失败是配置或数据格式错误，不应该重试
		return &errors.SkipMessageError{Reason: fmt.Sprintf("failed to parse SMS message: %v", err)}
	}

	// 打印解析后的消息对象内容（用于调试）
	logFields := []zap.Field{
		zap.Int64("task_code", taskCode),
		zap.String("message_type", smsMsg.GetMessageType()),
		zap.String("phone", smsMsg.GetPhone()),
		zap.String("sign_name", smsMsg.GetSignName()),
		zap.String("template_code", smsMsg.GetTemplateCode()),
	}

	// 根据消息类型打印具体字段值
	switch msg := smsMsg.(type) {
	case *model.CheckInReminder:
		logFields = append(logFields,
			zap.String("name", msg.Name),
			zap.String("deadline", msg.Time),
		)
	case *model.CheckInReminderContactMessage:
		logFields = append(logFields, zap.String("name", msg.Name))
	case *model.JourneyReminderContactMessage:
		logFields = append(logFields,
			zap.String("name", msg.Name),
			zap.String("trip", msg.Trip),
			zap.String("note", msg.Note),
			zap.Time("deadline", msg.Deadline),
		)
	case *model.CheckInTimeOut:
		logFields = append(logFields,
			zap.String("name", msg.Name),
			zap.Time("deadline", msg.Deadline),
		)
	}

	logger.Logger.Info("Parsed SMS message", logFields...)

	if smsMsg.GetPhone() == "" {
		smsMsg.SetPhone(phone)
	}

	cfg := config.Cfg
	if smsMsg.GetSignName() == "" || smsMsg.GetTemplateCode() == "" {
		signName, templateCode, err := cfg.GetSMSTemplateConfig(smsMsg.GetMessageType())
		if err != nil {
			logger.Logger.Error("Failed to get SMS template config",
				zap.Int64("task_code", taskCode),
				zap.String("message_type", smsMsg.GetMessageType()),
				zap.Error(err),
			)
			now := time.Now()
			_, updateErr := q.NotificationTask.WithContext(ctx).
				Where(q.NotificationTask.ID.Eq(task.ID)).
				Updates(map[string]interface{}{
					"status":       model.NotificationTaskStatusFailed,
					"processed_at": now,
				})
			if updateErr != nil {
				logger.Logger.Error("Failed to update task status",
					zap.Int64("task_id", task.ID),
					zap.Error(updateErr),
				)
			}
			return fmt.Errorf("failed to get SMS template config: %w", err)
		}
		if smsMsg.GetSignName() == "" {
			smsMsg.SetSignName(signName)
		}
		if smsMsg.GetTemplateCode() == "" {
			smsMsg.SetTemplateCode(templateCode)
		}
	}

	templateParams, err := smsMsg.GetTemplateParams()
	if err != nil {
		logger.Logger.Error("Failed to get template params",
			zap.Int64("task_code", taskCode),
			zap.String("message_type", smsMsg.GetMessageType()),
			zap.Error(err),
		)
		now := time.Now()
		_, updateErr := q.NotificationTask.WithContext(ctx).
			Where(q.NotificationTask.ID.Eq(task.ID)).
			Updates(map[string]interface{}{
				"status":       model.NotificationTaskStatusFailed,
				"processed_at": now,
			})
		if updateErr != nil {
			logger.Logger.Error("Failed to update task status",
				zap.Int64("task_id", task.ID),
				zap.Error(updateErr),
			)
		}
		return fmt.Errorf("failed to get template params: %w", err)
	}

	// 记录发送的模板参数（用于调试）
	logger.Logger.Info("SMS message details",
		zap.Int64("task_code", taskCode),
		zap.String("message_type", smsMsg.GetMessageType()),
		zap.Any("original_payload", payload),
		zap.String("parsed_phone", smsMsg.GetPhone()),
		zap.String("parsed_sign_name", smsMsg.GetSignName()),
		zap.String("parsed_template_code", smsMsg.GetTemplateCode()),
		zap.String("template_params_json", templateParams),
		zap.String("template_params_length", fmt.Sprintf("%d bytes", len(templateParams))),
	)

	err = sms.SendSingle(
		ctx,
		smsMsg.GetPhone(),
		smsMsg.GetSignName(),
		smsMsg.GetTemplateCode(),
		templateParams,
	)

	if err != nil {
		logger.Logger.Error("Failed to send SMS",
			zap.Int64("task_code", taskCode),
			zap.String("message_type", smsMsg.GetMessageType()),
			zap.String("phone", smsMsg.GetPhone()),
			zap.Error(err),
		)

		now := time.Now()
		_, updateErr := q.NotificationTask.WithContext(ctx).
			Where(q.NotificationTask.ID.Eq(task.ID)).
			Updates(map[string]interface{}{
				"status":       model.NotificationTaskStatusFailed,
				"processed_at": now,
			})
		if updateErr != nil {
			logger.Logger.Error("Failed to update task status",
				zap.Int64("task_id", task.ID),
				zap.Error(updateErr),
			)
		}

		// 如果是不可重试的错误（如配置错误），返回 SkipMessageError 避免重试
		if errors.IsNonRetryableError(err) {
			logger.Logger.Warn("Non-retryable error detected, skipping message",
				zap.Int64("task_code", taskCode),
				zap.Error(err),
			)
			return &errors.SkipMessageError{Reason: fmt.Sprintf("non-retryable error: %v", err)}
		}

		return fmt.Errorf("failed to send SMS: %w", err)
	}

	// 发送成功，使用事务处理额度扣除和任务状态更新
	err = db.Transaction(func(tx *gorm.DB) error {
		txQ := query.Use(tx)

		// 在事务中重新查询最新的余额（使用 balance_after 字段）
		txSMSTransactions, err := txQ.QuotaTransaction.
			Where(txQ.QuotaTransaction.UserID.Eq(user.ID)).
			Where(txQ.QuotaTransaction.Channel.Eq(string(model.QuotaChannelSMS))).
			Order(txQ.QuotaTransaction.CreatedAt.Desc()).
			Limit(1).
			Find()
		if err != nil {
			return fmt.Errorf("failed to query quota balance in transaction: %w", err)
		}

		var currentBalance int
		if len(txSMSTransactions) > 0 {
			currentBalance = txSMSTransactions[0].BalanceAfter
		} else {
			currentBalance = 0
		}

		// 再次检查余额（防止并发扣除导致余额不足）
		if currentBalance < smsUnitPriceCents {
			return fmt.Errorf("insufficient SMS quota: balance=%d, required=%d", currentBalance, smsUnitPriceCents)
		}

		newBalance := currentBalance - smsUnitPriceCents

		quotaTransaction := &model.QuotaTransaction{
			UserID:          user.ID,
			Channel:         model.QuotaChannelSMS,
			TransactionType: model.TransactionTypeDeduct,
			Reason:          "sms_notification", // 扣减原因：短信通知
			Amount:          smsUnitPriceCents,
			BalanceAfter:    newBalance,
		}

		if err := txQ.QuotaTransaction.Create(quotaTransaction); err != nil {
			return fmt.Errorf("failed to create quota transaction: %w", err)
		}

		now := time.Now()
		_, err = txQ.NotificationTask.
			Where(txQ.NotificationTask.ID.Eq(task.ID)).
			Updates(map[string]interface{}{
				"status":       model.NotificationTaskStatusSuccess,
				"processed_at": now,
			})
		if err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		logger.Logger.Info("SMS sent and quota deducted successfully",
			zap.Int64("task_code", taskCode),
			zap.String("message_type", smsMsg.GetMessageType()),
			zap.String("phone", smsMsg.GetPhone()),
			zap.Int("amount_deducted", smsUnitPriceCents),
			zap.Int("balance_before", currentBalance),
			zap.Int("balance_after", newBalance),
		)

		return nil
	})

	// 这里还需要获取发送短信返回的状态码，根据状态码来尝试逻辑
	if err != nil {
		logger.Logger.Error("Failed to process quota deduction and task update",
			zap.Int64("task_code", taskCode),
			zap.Error(err),
		)
		// 短信已发送成功，但额度扣除或状态更新失败
		// 这种情况下需要人工介入处理，因为短信已发送但未扣费
		// 送入日志部分
		return fmt.Errorf("SMS sent but failed to deduct quota or update task: %w", err)
	}

	return nil
}

// 暂不考虑实现
// func (s *NotificationService) SendVoice(ctx context.Context, taskID int64, userID int64, phoneHash string, payload map[string]interface{}) error {
// 	return nil
// }

package service

import (
	"AreYouOK/config"
	"AreYouOK/internal/model"
	"AreYouOK/internal/repository/query"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/metrics"
	"AreYouOK/pkg/sms"
	"AreYouOK/storage/database"
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
// 修改 SendSMS 方法，集成预扣减机制和状态记录

// 由消费者来调用，调用成功生成的 taskid 存储，避免重复调用
func (s *NotificationService) SendSMS(
	ctx context.Context,
	taskCode int64,
	userID int64,
	phoneHash string,
	payload map[string]interface{},
) error {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	// 根据 taskCode 查询任务（幂等性检查）
	task, err := q.NotificationTask.GetByTaskCode(taskCode)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Logger.Warn("Notification task not found, may have been processed",
				zap.Int64("task_code", taskCode),
			)
			return &errors.SkipMessageError{Reason: "task not found"}
		}
		return fmt.Errorf("failed to query notification task: %w", err)
	}

	if task.Status == model.NotificationTaskStatusSuccess {
		logger.Logger.Info("Notification task already processed successfully",
			zap.Int64("task_code", taskCode),
		)
		return nil
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
			return &errors.SkipMessageError{Reason: "user not found"}
		}
		return fmt.Errorf("failed to query user: %w", err)
	}

	smsUnitPriceCents := 5
	quotaService := Quota()
	// 发送成功才扣减
	if err := quotaService.PreDeduct(ctx, user.ID, model.QuotaChannelSMS, smsUnitPriceCents); err != nil {
		if errors.IsQuotaInsufficient(err) {
			// 额度不足，更新任务状态为失败
			now := time.Now()
			_, updateErr := q.NotificationTask.WithContext(ctx).
				Where(q.NotificationTask.ID.Eq(task.ID)).
				Updates(map[string]interface{}{
					"status":            model.NotificationTaskStatusFailed,
					"processed_at":      now,
					"sms_status_code":   "INSUFFICIENT_QUOTA",
					"sms_error_message": "余额不足",
				})
			if updateErr != nil {
				logger.Logger.Error("Failed to update task status", zap.Error(updateErr))
			}
			return &errors.SkipMessageError{Reason: fmt.Sprintf("quota insufficient: %v", err)}
		}
		return fmt.Errorf("failed to pre-deduct quota: %w", err)
	}

	// 解析 payload 为具体的 SMSMessage
	smsMsg, err := model.ParseSMSMessage(payload)
	if err != nil {
		// 解析失败，退款并更新任务状态
		refundErr := quotaService.Refund(ctx, user.ID, model.QuotaChannelSMS, smsUnitPriceCents)
		if refundErr != nil {
			logger.Logger.Error("Failed to refund quota after parse failure",
				zap.Int64("user_id", user.ID),
				zap.Error(refundErr),
			)
		}

		now := time.Now()
		_, updateErr := q.NotificationTask.WithContext(ctx).
			Where(q.NotificationTask.ID.Eq(task.ID)).
			Updates(map[string]interface{}{
				"status":            model.NotificationTaskStatusFailed,
				"processed_at":      now,
				"sms_status_code":   "PARSE_ERROR",
				"sms_error_message": err.Error(),
			})
		if updateErr != nil {
			logger.Logger.Error("Failed to update task status", zap.Error(updateErr))
		}
		return &errors.SkipMessageError{Reason: fmt.Sprintf("failed to parse SMS message: %v", err)}
	}

	// 获取模板配置
	cfg := config.Cfg
	if smsMsg.GetSignName() == "" || smsMsg.GetTemplateCode() == "" {
		signName, templateCode, err := cfg.GetSMSTemplateConfig(smsMsg.GetMessageType())
		if err != nil {
			// 配置错误，退款
			quotaService.Refund(ctx, user.ID, model.QuotaChannelSMS, smsUnitPriceCents)

			now := time.Now()
			_, updateErr := q.NotificationTask.WithContext(ctx).
				Where(q.NotificationTask.ID.Eq(task.ID)).
				Updates(map[string]interface{}{
					"status":            model.NotificationTaskStatusFailed,
					"processed_at":      now,
					"sms_status_code":   "CONFIG_ERROR",
					"sms_error_message": err.Error(),
				})
			// 更新失败，说明数据库出错了
			if updateErr != nil {
				return fmt.Errorf("failed to update notificationTask: %w", err)
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
		// 参数错误，退款
		quotaService.Refund(ctx, user.ID, model.QuotaChannelSMS, smsUnitPriceCents)

		now := time.Now()
		_, updateErr := q.NotificationTask.WithContext(ctx).
			Where(q.NotificationTask.ID.Eq(task.ID)).
			Updates(map[string]interface{}{
				"status":            model.NotificationTaskStatusFailed,
				"processed_at":      now,
				"sms_status_code":   "PARAM_ERROR",
				"sms_error_message": err.Error(),
			})

		if updateErr != nil {
			return fmt.Errorf("failed to update notificationTask: %w", err)
		}

		return fmt.Errorf("failed to get template params: %w", err)
	}

	// 发送短信，开始投递
	smsStart := time.Now()
	sendResp, err := sms.SendSingle(
		ctx,
		smsMsg.GetPhone(),
		smsMsg.GetSignName(),
		smsMsg.GetTemplateCode(),
		templateParams,
	)
	smsDuration := time.Since(smsStart).Seconds()

	if err != nil {
		// 记录短信发送失败的监控指标
		templateCode := smsMsg.GetTemplateCode()
		provider := "aliyun"
		statusCode := "SEND_ERROR"

		if sendResp != nil {
			statusCode = sendResp.StatusCode
			provider = sendResp.Provider
			templateCode = sendResp.Template
		}

		metrics.RecordSMSFailed(templateCode, provider, statusCode, smsDuration)

		// 发送失败，退款
		refundErr := quotaService.Refund(ctx, user.ID, model.QuotaChannelSMS, smsUnitPriceCents)
		if refundErr != nil {
			logger.Logger.Error("Failed to refund quota after SMS send failure",
				zap.Int64("user_id", user.ID),
				zap.Error(refundErr),
			)
		}

		// 记录失败状态
		now := time.Now()
		updateData := map[string]interface{}{
			"status":       model.NotificationTaskStatusFailed,
			"processed_at": now,
		}

		// 如果有响应，记录状态码和错误消息
		if sendResp != nil {
			updateData["sms_status_code"] = sendResp.StatusCode
			updateData["sms_error_message"] = sendResp.Message
		} else {
			updateData["sms_status_code"] = "SEND_ERROR"
			updateData["sms_error_message"] = err.Error()
		}

		_, updateErr := q.NotificationTask.WithContext(ctx).
			Where(q.NotificationTask.ID.Eq(task.ID)).
			Updates(updateData)
		if updateErr != nil {
			logger.Logger.Error("Failed to update task status", zap.Error(updateErr))
		}

		logger.Logger.Error("Failed to send SMS",
			zap.Int64("task_code", taskCode),
			zap.String("message_type", smsMsg.GetMessageType()),
			zap.String("phone", smsMsg.GetPhone()),
			zap.Error(err),
		)

		// 如果是不可重试的错误，返回 SkipMessageError
		if errors.IsNonRetryableError(err) {
			return &errors.SkipMessageError{Reason: fmt.Sprintf("non-retryable error: %v", err)}
		}

		return fmt.Errorf("failed to send SMS: %w", err)
	}

	//  确认扣减，投递成功就确认扣减
	// 发送成功，确认扣减并更新任务状态
	err = db.Transaction(func(tx *gorm.DB) error {
		txQ := query.Use(tx)

		// 确认扣减
		if err := quotaService.ConfirmDeduction(ctx, user.ID, model.QuotaChannelSMS, smsUnitPriceCents); err != nil {
			return fmt.Errorf("failed to confirm deduction: %w", err)
		}

		// 更新任务状态和短信状态信息
		now := time.Now()
		updateData := map[string]interface{}{
			"status":       model.NotificationTaskStatusSuccess,
			"processed_at": now,
		}

		// 记录短信发送状态
		if sendResp != nil {
			updateData["sms_message_id"] = sendResp.MessageID
			updateData["sms_status_code"] = sendResp.StatusCode
			if sendResp.Message != "" {
				updateData["sms_error_message"] = sendResp.Message
			}
		}

		_, err = txQ.NotificationTask.
			Where(txQ.NotificationTask.ID.Eq(task.ID)).
			Updates(updateData)
		if err != nil {
			return fmt.Errorf("failed to update task status: %w", err)
		}

		// 记录短信发送成功的监控指标
		if sendResp != nil {
			metrics.RecordSMSSent(
				sendResp.Template,
				sendResp.Provider,
				smsDuration,
				smsUnitPriceCents,
			)
		}

		logger.Logger.Info("SMS sent and quota confirmed",
			zap.Int64("task_code", taskCode),
			zap.String("message_type", smsMsg.GetMessageType()),
			zap.String("phone", smsMsg.GetPhone()),
			zap.String("message_id", sendResp.MessageID),
			zap.String("status_code", sendResp.StatusCode),
			zap.Float64("duration_seconds", smsDuration),
			zap.Int("cost_cents", smsUnitPriceCents),
		)

		return nil
	})

	if err != nil {
		// 确认扣减失败，但短信已发送成功
		// 这种情况需要人工介入处理， 之后集成到 otel 部分，到时候报警预处理
		logger.Logger.Error("SMS sent but failed to confirm deduction",
			zap.Int64("task_code", taskCode),
			zap.Error(err),
		)
		return fmt.Errorf("SMS sent but failed to confirm deduction: %w", err)
	}

	return nil
}

// 暂不考虑实现
// func (s *NotificationService) SendVoice(ctx context.Context, taskID int64, userID int64, phoneHash string, payload map[string]interface{}) error {
// 	return nil
// }

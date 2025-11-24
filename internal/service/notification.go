package service

import (
//	"AreYouOK/config"
	"AreYouOK/internal/model"
	"AreYouOK/internal/repository/query"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
//	"AreYouOK/pkg/sms"
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
	// 之后再来实现 phone
	_, err = utils.DecryptPhone(user.PhoneCipher)
	if err != nil {
		logger.Logger.Error("Failed to decrypt phone",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to decrypt phone: %w", err)
	}

	balances, err := q.QuotaTransaction.GetBalanceByUserID(user.ID)
	if err != nil {
		logger.Logger.Error("Failed to query quota balance",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to query quota balance: %w", err)
	}

	var smsBalance int
	for _, balance := range balances {
		channel, ok := balance["channel"].(string)
		if !ok {
			continue
		}
		if channel == string(model.QuotaChannelSMS) {
			switch v := balance["balance"].(type) {
			case int:
				smsBalance = v
			case int64:
				smsBalance = int(v)
			case float64:
				smsBalance = int(v)
			}
			break
		}
	}


	const smsUnitPriceCents = 5
	if smsBalance < smsUnitPriceCents {
		logger.Logger.Warn("Insufficient SMS quota",
			zap.Int64("user_id", userID),
			zap.Int("balance", smsBalance),
			zap.Int("required", smsUnitPriceCents),
		)
		// 额度不足，更新任务状态为失败，但不重试, 这里需要考虑发送一条提示信息告知额度不足吗？
		now := time.Now()
		_, err = q.NotificationTask.WithContext(ctx).
			Where(q.NotificationTask.ID.Eq(task.ID)).
			Updates(map[string]interface{}{
				"status":      model.NotificationTaskStatusFailed,
				"processed_at": now,
			})
		if err != nil {
			logger.Logger.Error("Failed to update task status",
				zap.Int64("task_id", task.ID),
				zap.Error(err),
			)
		}
		return errors.QuotaInsufficient
	}
	// 接下来根据 Message 构造场景模板，需要有不同的场景
	// 对你的称呼输入中文或者英文，不可以超过 20 个字符
	// 1、提醒超时，也就是打卡提醒
	// 2、联系紧急联系人，需要姓名
	// 3、journey 部分，超时逻辑，
	// 对应的紧急联系人逻辑
	/*
	- 如果用户没有在行程结束前打卡，则会在行程结束时向用户发送短信：
  - 用户短信：安否温馨提示您，您进行了行程报备功能，请于 10 分钟内进行打卡；如果没有按时打卡，我们将按约定联系您的紧急联系人。
- 如果行程结束后 10 分钟内用户没有打卡，则会向用户本人发送短信，并按规则尝试通知用户的紧急联系人：
  - 用户短信：安否温馨提示您，您进行了行程报备，因为您没有按时打卡，我们按约定开始联系您的紧急联系人，紧急联系次数会进行相应扣除。
  - 紧急联系人联系逻辑1：按照顺位，从第一位紧急联系人开始呼叫，每次呼叫扣除一次紧急联系次数。如果紧急联系人接听，信息播报完成，则视为成功不再执行后续步骤。
    - 电话内容：安否温馨提示，您的联系人【紧急联系人对你的称呼】没有进行行程报备归来打卡，请联系 ta 确认情况。行程信息：【行程名称】/【预计归来时间】/【其他备注】。
  - 紧急联系人联系逻辑2: 如果第一位紧急联系人的呼叫未接听或者信息播报未完成，则向其发送短信，短信内容如下：
    - 短信内容：安否温馨提示，您的联系人【紧急联系人对你的称呼】没有进行行程报备归来打卡，请联系 ta 确认情况。行程信息：【行程名称】/【预计归来时间】/【其他备注】。
  - 紧急联系人联系逻辑3: 如果第一位紧急联系人的呼叫未接听或者信息播报未完成，则向下一位紧急联系人执行「紧急联系人联系逻辑1～2」，直到一位紧急联系人接听、紧急联系人列表穷尽或者用户的紧急联系次数耗尽
  */


	return nil
}

// 暂不考虑实现
// func (s *NotificationService) SendVoice(ctx context.Context, taskID int64, userID int64, phoneHash string, payload map[string]interface{}) error {
// 	return nil
// }

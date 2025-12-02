package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"AreYouOK/internal/cache"
	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/repository/query"
	pkgerrors "AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"

	"AreYouOK/utils"
)

// api 中设计的 user_ID 是 public_id

var (
	userService *UserService
	userOnce    sync.Once
)

func User() *UserService {
	userOnce.Do(func() {
		userService = &UserService{}
	})
	return userService
}

type UserService struct{}

// GetUserStatus 获取用户状态和引导进度
func (s *UserService) GetUserStatus(
	ctx context.Context,
	userID string,
) (*dto.UserStatusData, error) {
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

	result := &dto.UserStatusData{
		Status:        model.StatusToStringMap[user.Status],
		PhoneVerified: user.PhoneHash != nil && *user.PhoneHash != "",
		HasContacts:   len(user.EmergencyContacts) > 0,
	}

	return result, nil
}

// GetUserProfile 获取用户资料（包含额度信息）
func (s *UserService) GetUserProfile(
	ctx context.Context,
	userID string,
) (*dto.UserProfileData, error) {
	var userIDInt int64
	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return nil, pkgerrors.InvalidUserID
	}

	resultMap, err := query.User.GetByPublicIDWithQuotas(userIDInt)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user with quotas: %w", err)
	}

	userIDFromMap, ok := resultMap["public_id"]
	if !ok {
		return nil, fmt.Errorf("public_id not found in result")
	}

	var publicID int64
	switch v := userIDFromMap.(type) {
	case int64:
		publicID = v
	case float64:
		publicID = int64(v)
	default:
		return nil, fmt.Errorf("invalid public_id type: %T", v)
	}

	nickname, _ := resultMap["nickname"].(string)
	statusStr, _ := resultMap["status"].(string)
	timezone, _ := resultMap["timezone"].(string)
	dailyCheckInEnabled, _ := resultMap["daily_check_in_enabled"].(bool)
	journeyAutoNotify, _ := resultMap["journey_auto_notify"].(bool)
	dailyCheckInDeadline, _ := resultMap["daily_check_in_deadline"].(string)
	dailyCheckInGraceUntil, _ := resultMap["daily_check_in_grace_until"].(string)

	phoneCipher, ok := resultMap["phone_cipher"].([]byte)
	var phoneMasked string
	var phoneVerified bool

	if ok && len(phoneCipher) > 0 {
		phone, err := utils.DecryptPhone(phoneCipher)
		if err != nil {
			logger.Logger.Warn("Failed to decrypt phone",
				zap.String("user_id", userID),
				zap.Error(err),
			)
			phoneMasked = ""
			phoneVerified = false
		} else {
			phoneMasked = phone //不再加密
			phoneVerified = true
		}
	} else {
		phoneHash, ok := resultMap["phone_hash"].(*string)
		phoneVerified = ok && phoneHash != nil && *phoneHash != ""
	}

	var smsBalance int
	if smsVal, ok := resultMap["sms_balance"]; ok && smsVal != nil {
		switch v := smsVal.(type) {
		case int:
			smsBalance = v
		case int64:
			smsBalance = int(v)
		case float64:
			smsBalance = int(v)
		}
	}
	

	result := &dto.UserProfileData{
		ID:       strconv.FormatInt(publicID, 10),
		PublicID: strconv.FormatInt(publicID, 10),
		Nickname: nickname,
		Phone: dto.PhoneInfo{
			NumberMasked: phoneMasked,
			Verified:     phoneVerified,
		},
		Status: statusStr,
		Settings: dto.UserSettingsDTO{
			DailyCheckInEnabled:    dailyCheckInEnabled,
			DailyCheckInDeadline:   dailyCheckInDeadline,
			DailyCheckInGraceUntil: dailyCheckInGraceUntil,
			Timezone:               timezone,
			JourneyAutoNotify:      journeyAutoNotify,
		},
		Quotas: dto.QuotaBalance{
			SMSBalance:   smsBalance,
			//VoiceBalance: voiceBalance,
		},
	}

	return result, nil
}

// UpdateUserSettings 更新用户设置， 需要更新 redis 中对应的缓存， 来帮助消息队列确认发送的消息是符合当前用户的预期的
// 以及考虑更新后是否重新发送对应的消息，在这里处理对应的消息投递，改晚了还是改早了，通过更新的部分来检查是否需要重新投递
// 这里最好还得做一个限流的部分
// 考虑是否重新投递消息
// 通过幂等性来过滤

// 返回需要重新投递的消息，由 Handler 层决定是否发布
func (s *UserService) UpdateUserSettings(
	ctx context.Context,
	userID string,
	req dto.UpdateUserSettingsRequest,
) (*model.CheckInReminderMessage, error) {
	var userIDInt int64
	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return nil, pkgerrors.InvalidUserID
	}

	_, err := query.User.GetByPublicID(userIDInt)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	updates := make(map[string]interface{})

	if req.NickName != nil {
		updates["nick_name"] = *req.NickName
	}
	if req.DailyCheckInEnabled != nil {
		updates["daily_check_in_enabled"] = *req.DailyCheckInEnabled
	}
	if req.DailyCheckInRemindAt != nil {
		updates["daily_check_in_remind_at"] = *req.DailyCheckInRemindAt
	}
	if req.DailyCheckInDeadline != nil {
		updates["daily_check_in_deadline"] = *req.DailyCheckInDeadline
	}
	if req.DailyCheckInGraceUntil != nil {
		updates["daily_check_in_grace_until"] = *req.DailyCheckInGraceUntil
	}
	if req.JourneyAutoNotify != nil {
		updates["journey_auto_notify"] = *req.JourneyAutoNotify
	}
	if req.Timezone != nil {
		updates["timezone"] = *req.Timezone
	}

	if len(updates) == 0 {
		return nil, nil
	}

	if _, err := query.User.Where(query.User.PublicID.Eq(userIDInt)).Updates(updates); err != nil {
		return nil, fmt.Errorf("failed to update user settings: %w", err)
	}

	logger.Logger.Info("User settings updated",
		zap.String("user_id", userID),
		zap.Any("updates", updates),
	)
	// 重新查询更新后的用户信息
	updatedUser, err := query.User.GetByPublicID(userIDInt)
	if err != nil {
		return nil, fmt.Errorf("failed to query updated user: %w", err)
	}

	// 更新今日的用户缓存，如果 redis 中没有缓存的话，消息队列就可以直接发送，减少数据库回查, 更新而非失效
	cache.SetUserSettings(ctx, userIDInt, &cache.UserSettingsCache{
		DailyCheckInEnabled:    updatedUser.DailyCheckInEnabled,
		DailyCheckInRemindAt:   updatedUser.DailyCheckInRemindAt,
		DailyCheckInDeadline:   updatedUser.DailyCheckInDeadline,
		DailyCheckInGraceUntil: updatedUser.DailyCheckInGraceUntil,
		UpdatedAt: time.Now().Unix(),
	})

	// 检查是否需要重新投递今日的打卡提醒消息
	// 只有用户状态为 active 且开启了打卡功能才需要重新投递
	var reminderMsg *model.CheckInReminderMessage
	if updatedUser.Status == model.UserStatusActive && updatedUser.DailyCheckInEnabled {
		// 检查设置变更是否影响今日提醒
		affectsReminder := false

		if req.DailyCheckInEnabled != nil {
			affectsReminder = true // 开关变更影响提醒
		}
		if req.DailyCheckInRemindAt != nil {
			affectsReminder = true // 提醒时间变更影响提醒
		}
		if req.Timezone != nil {
			affectsReminder = true // 时区变更影响本地时间计算
		}

		if affectsReminder {
			reminderMsg = s.buildReminderMessage(ctx, updatedUser, userIDInt)
		}
	}

	return reminderMsg, nil
}

// GetUserQuotas 获取用户额度详情
func (s *UserService) GetUserQuotas(
	ctx context.Context,
	userID string,
) (*dto.QuotaBalance, error) {
	var userIDInt int64
	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return nil, pkgerrors.InvalidUserID
	}

	user, err := query.User.GetByPublicID(userIDInt)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// 使用新的 QuotaWallet 系统查询额度
	smsWallet, err := query.QuotaWallet.
		Where(query.QuotaWallet.UserID.Eq(user.ID)).
		Where(query.QuotaWallet.Channel.Eq(string(model.QuotaChannelSMS))).
		First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to query SMS quota wallet: %w", err)
	}

	// 查询 Voice 渠道额度
	// voiceWallet, err := query.QuotaWallet.
	// 	Where(query.QuotaWallet.UserID.Eq(user.ID)).
	// 	Where(query.QuotaWallet.Channel.Eq(string(model.QuotaChannelVoice))).
	// 	First()
	// if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
	// 	return nil, fmt.Errorf("failed to query voice quota wallet: %w", err)
	// }

	var smsBalance int
	if smsWallet != nil {
		smsBalance = smsWallet.AvailableAmount
	}
	// var voiceBalance int
	// if voiceWallet != nil {
	// 	voiceBalance = voiceWallet.AvailableAmount
	// }

	result := &dto.QuotaBalance{
		SMSBalance:     smsBalance,
		//VoiceBalance:   voiceBalance,
		SMSUnitPrice:   0.05,
		//VoiceUnitPrice: 0.1,
	}

	return result, nil
}

// GetWaitlistStatus 获取内测排队状态
func (s *UserService) GetWaitlistStatus(ctx context.Context) (*dto.WaitlistStatusData, error) {
	userCount, err := query.User.CountByStatus()
	if err != nil {
		return nil, fmt.Errorf("failed to count users: %w", err)
	}

	result := &dto.WaitlistStatusData{
		UserCount: len(userCount),
	}

	return result, nil
}

// 分配额度，已废弃
// func (s *UserService) GrantDefaultSMSQuota(ctx context.Context, userID int64) error {
// 	db := database.DB().WithContext(ctx)
// 	q := query.Use(db)

// 	// 检查是否已经分配过额度（防止重复分配）
// 	existingTransactions, err := q.QuotaTransaction.
// 		Where(q.QuotaTransaction.UserID.Eq(userID)).
// 		Where(q.QuotaTransaction.Channel.Eq(string(model.QuotaChannelSMS))).
// 		Where(q.QuotaTransaction.TransactionType.Eq(string(model.TransactionTypeGrant))).
// 		Where(q.QuotaTransaction.Reason.Eq("new_user_bonus")).
// 		Find()

// 	if err != nil {
// 		return fmt.Errorf("failed to check existing quota transactions: %w", err)
// 	}

// 	if len(existingTransactions) > 0 {
// 		logger.Logger.Info("Default SMS quota already granted",
// 			zap.Int64("user_id", userID),
// 		)
// 		return nil // 已经分配过，直接返回成功
// 	}

// 	// 获取默认额度（cents），20 次短信 = 20 * 5 = 100 cents
// 	defaultQuotaCents := config.Cfg.DefaultSMSQuota
// 	if defaultQuotaCents <= 0 {
// 		defaultQuotaCents = 100 // 默认 100 cents = 20 次短信
// 	}

// 	// 创建额度充值记录
// 	quotaTransaction := &model.QuotaTransaction{
// 		UserID:          userID,
// 		Channel:         model.QuotaChannelSMS,
// 		TransactionType: model.TransactionTypeGrant,
// 		Reason:          "new_user_bonus", // 新用户奖励
// 		Amount:          defaultQuotaCents,
// 		BalanceAfter:    defaultQuotaCents, // 首次充值，余额等于充值金额
// 	}

// 	if err := q.QuotaTransaction.Create(quotaTransaction); err != nil {
// 		logger.Logger.Error("Failed to grant default SMS quota",
// 			zap.Int64("user_id", userID),
// 			zap.Int("amount", defaultQuotaCents),
// 			zap.Error(err),
// 		)
// 		return fmt.Errorf("failed to grant default SMS quota: %w", err)
// 	}

// 	logger.Logger.Info("Default SMS quota granted to new user",
// 		zap.Int64("user_id", userID),
// 		zap.Int("amount_cents", defaultQuotaCents),
// 		zap.Int("sms_count", defaultQuotaCents/5), // 每次短信 5 cents
// 	)

// 	return nil
// }

// buildReminderMessage 构建今日打卡提醒消息
func (s *UserService) buildReminderMessage(ctx context.Context, user *model.User, userIDInt int64) *model.CheckInReminderMessage {
	now := time.Now()
	today := now.Format("2006-01-02")

	// 解析提醒时间
	remindAt := user.DailyCheckInRemindAt
	if remindAt == "" {
		remindAt = "20:00:00" // 默认提醒时间
	}

	remindTime, err := time.Parse("15:04:05", remindAt)
	if err != nil {
		logger.Logger.Error("Failed to parse remindAt time",
			zap.String("remindAt", remindAt),
			zap.Int64("user_id", userIDInt),
			zap.Error(err),
		)
		return nil
	}

	// 在今天的日期基础上设置提醒时间
	todayTime := now.Truncate(24 * time.Hour)
	remindDateTime := time.Date(
		todayTime.Year(), todayTime.Month(), todayTime.Day(),
		remindTime.Hour(), remindTime.Minute(), remindTime.Second(),
		0, now.Location(),
	)

	// 如果提醒时间已过，设置为明天
	if remindDateTime.Before(now) {
		remindDateTime = remindDateTime.Add(24 * time.Hour)
	}

	// 计算延迟秒数
	delaySeconds := int(remindDateTime.Sub(now).Seconds())
	if delaySeconds < 0 {
		delaySeconds = 0 // 最小延迟为0（立即发送）
	}

	// 检查是否超过24小时限制
	if delaySeconds > 24*3600 {
		logger.Logger.Info("Reminder delay exceeds 24 hours, skipping message publish",
			zap.Int64("user_id", userIDInt),
			zap.String("remind_at", remindAt),
			zap.Int("delay_seconds", delaySeconds),
		)
		return nil
	}

	// 生成消息ID
	messageID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
	if err != nil {
		logger.Logger.Error("Failed to generate message ID for reminder",
			zap.Int64("user_id", userIDInt),
			zap.Error(err),
		)
		return nil
	}

	// 构建用户设置快照
	userIDStr := fmt.Sprintf("%d", userIDInt)
	userSettings := map[string]model.UserSettingSnapshot{
		userIDStr: {
			RemindAt:   user.DailyCheckInRemindAt,
			Deadline:   user.DailyCheckInDeadline,
			GraceUntil: user.DailyCheckInGraceUntil,
			Timezone:   user.Timezone,
		},
	}

	// 构建消息
	reminderMsg := &model.CheckInReminderMessage{
		MessageID:    fmt.Sprintf("ci_reminder_update_%d", messageID),
		BatchID:      fmt.Sprintf("update_%d_%d", userIDInt, time.Now().Unix()),
		CheckInDate:  today,
		ScheduledAt:  now.Format(time.RFC3339),
		UserIDs:      []int64{userIDInt},
		UserSettings: userSettings,
		DelaySeconds: delaySeconds,
	}

	logger.Logger.Info("Built reminder message for user settings update",
		zap.String("message_id", reminderMsg.MessageID),
		zap.Int64("user_id", userIDInt),
		zap.String("remind_at", remindAt),
		zap.Int("delay_seconds", delaySeconds),
		zap.Time("remind_time", remindDateTime),
	)

	return reminderMsg
}

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"AreYouOK/config"
	"AreYouOK/internal/cache"
	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/repository/query"
	pkgerrors "AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/storage/database"
	"AreYouOK/storage/redis"

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
	dailyCheckInRemindAt, _ := resultMap["daily_check_in_remind_at"].(string)

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
			DailyCheckInRemindAt:   dailyCheckInRemindAt,
			DailyCheckInEnabled:    dailyCheckInEnabled,
			DailyCheckInDeadline:   dailyCheckInDeadline,
			DailyCheckInGraceUntil: dailyCheckInGraceUntil,
			Timezone:               timezone,
			JourneyAutoNotify:      journeyAutoNotify,
		},
		Quotas: dto.QuotaBalance{
			SMSBalance: smsBalance,
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

	now := time.Now()
	remindTime, err := time.Parse("15:04:05", *req.DailyCheckInRemindAt)
	if err != nil {
		return nil, fmt.Errorf("can not parse remindTime: %w", err)
	}

	// 这里可以额外处理
	if remindTime.Sub(now) < 10*time.Minute {
		return nil, pkgerrors.Definition{
			Code:    "TIME_IS_TOO_CLOSE",
			Message: "You can not modify your remind time at close remind time",
		}
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
		UpdatedAt:              time.Now().Unix(),
	})

	// 检查是否需要重新投递今日的打卡提醒消息
	// 只有用户状态为 active 且开启了打卡功能才需要重新投递

	// 如果一天只有一次打卡的话就不需要修改
	var reminderMsg *model.CheckInReminderMessage
	if updatedUser.Status == model.UserStatusActive && updatedUser.DailyCheckInEnabled {
		// 检查设置变更是否影响今日提醒
		affectsReminder := false

		if req.DailyCheckInEnabled != nil {
			affectsReminder = true
		}
		if req.DailyCheckInRemindAt != nil {
			affectsReminder = true // 提醒时间变更影响提醒
		}
		if req.Timezone != nil {
			affectsReminder = true // 时区变更影响本地时间计算
		}

		if affectsReminder {
			reminderMsg = s.buildReminderMessage(ctx, updatedUser, userIDInt)
			// 如果成功构建了提醒消息，清除当天的 Redis 调度标记
			// 这样可以确保新消息能够被正确处理，而旧消息会被幂等性检查跳过
			if reminderMsg != nil {
				today := time.Now().Format("2006-01-02")

				if err := cache.UnmarkCheckinScheduled(ctx, today, userIDInt); err != nil {
					logger.Logger.Warn("Failed to unmark checkin scheduled after settings update",
						zap.Int64("user_id", userIDInt),
						zap.String("date", today),
						zap.Error(err),
					)
				} else {
					logger.Logger.Info("Cleared checkin scheduled mark after settings update",
						zap.Int64("user_id", userIDInt),
						zap.String("date", today),
					)
				}
			}
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
		SMSBalance: smsBalance,
		//VoiceBalance:   voiceBalance,
		SMSUnitPrice: 5,
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

// 删除当前的联系人， 软删除
func (s *UserService) DeleteUser(
	ctx context.Context,
	userID string,
) error {
	var userIDInt int64
	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return pkgerrors.InvalidUserID
	}

	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	user, err := q.User.GetByPublicID(userIDInt)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.ErrUserNotFound
		}
		return fmt.Errorf("failed to query user: %w", err)
	}

	now := time.Now()
	updates := map[string]interface{}{
		"updated_at": now,
		"deleted_at": now,
	}

	if _, err := q.User.
		Where(q.User.ID.Eq(user.ID)).
		Updates(updates); err != nil {
		return fmt.Errorf("failed to soft delete user: %w", err)
	}

	logger.Logger.Info("User soft-deleted",
		zap.Int64("user_id", user.ID),
		zap.Int64("public_id", user.PublicID),
	)

	return nil
}

// GetWaitListInfo 基于 alipay_open_id 查找或创建最小用户，并返回引导步骤
func (s *UserService) GetWaitListInfo(ctx context.Context, alipayID string) (*dto.AuthUserSnapshot, error) {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	redisCli := redis.Client()

	// 简单限流：按 open_id 统计 60 秒内请求次数
	rateKey := redis.Key("waitlist:rate", alipayID)
	if n, err := redisCli.Incr(ctx, rateKey).Result(); err == nil {
		if n == 1 {
			redisCli.Expire(ctx, rateKey, 60*time.Second)
		}
		if n > 30 {
			return nil, fmt.Errorf("too many requests")
		}
	}

	// 读取缓存，避免频繁查库
	cacheKey := redis.Key("waitlist:info", alipayID)
	if cached, err := redisCli.Get(ctx, cacheKey).Result(); err == nil {
		var snap dto.AuthUserSnapshot
		if unmarshalErr := json.Unmarshal([]byte(cached), &snap); unmarshalErr == nil {
			return &snap, nil
		}
	}

	user, err := q.User.Where(q.User.AlipayOpenID.Eq(alipayID)).First()
	isNewUser := false

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("failed to query user: %w", err)
		}

		// 容量检查（按 active 数量）
		count, countErr := q.User.Where(q.User.Status.Eq(string(model.UserStatusActive))).Count()
		if countErr != nil {
			return nil, fmt.Errorf("failed to query User: %w", countErr)
		}
		if count+1 > int64(config.Cfg.WaitlistMaxUsers) {
			return nil, fmt.Errorf("please add to waitlist")
		}

		publicID, idErr := snowflake.NextID(snowflake.GeneratorTypeUser)
		if idErr != nil {
			return nil, fmt.Errorf("failed to generate public_id: %w", idErr)
		}

		newUser := &model.User{
			PublicID:     publicID,
			AlipayOpenID: alipayID,
			Status:       model.UserStatusWaitlisted,
			Timezone:     "Asia/Shanghai",
		}

		if err := q.User.Create(newUser); err != nil {
			return nil, fmt.Errorf("failed to create waitlist user: %w", err)
		}

		user = newUser
		isNewUser = true
	}

	phoneVerified := user.PhoneHash != nil && *user.PhoneHash != ""
	nextStep := resolveNextStep(user.Status, phoneVerified)

	resp := &dto.AuthUserSnapshot{
		ID:            fmt.Sprintf("%d", user.PublicID),
		Nickname:      user.Nickname,
		Status:        model.StatusToStringMap[user.Status],
		NextStep:      nextStep,
		PhoneVerified: phoneVerified,
		IsNewUser:     isNewUser,
	}

	if data, err := json.Marshal(resp); err == nil {
		redisCli.Set(ctx, cacheKey, data, 5*time.Minute)
	}

	return resp, nil
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
// 注意：
// - 正常来说，remind_at 表示「今日打卡前提醒时间」，不应该简单顺延到明天；
// - 这里是「设置更新」时的补单逻辑，只负责尽量让新配置在合理时间窗口内生效。
func (s *UserService) buildReminderMessage(_ context.Context, user *model.User, userIDInt int64) *model.CheckInReminderMessage {
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

	// 解析截止和宽限时间，用于限制「今日补单」的合理窗口
	// 如果解析失败，不终止流程，只用于辅助判断。
	var graceUntilTime *time.Time
	if user.DailyCheckInGraceUntil != "" {
		if t, err := time.Parse("15:04:05", user.DailyCheckInGraceUntil); err == nil {
			gt := time.Date(
				todayTime.Year(), todayTime.Month(), todayTime.Day(),
				t.Hour(), t.Minute(), t.Second(),
				0, now.Location(),
			)
			graceUntilTime = &gt
		}
	}

	// 根据当前环境决定「提醒时间已过」时的处理方式
	if remindDateTime.Before(now) {
		// 如果已经过了今天的提醒时间：
		// - 若还在宽限窗口内：尽量就近触发；
		inGraceWindow := false
		if graceUntilTime != nil && now.Before(*graceUntilTime) {
			inGraceWindow = true
		}

		if inGraceWindow {
			if config.Cfg.Environment == "development" {
				// 开发环境：就近触发，统一用 now + 1 分钟，方便本地调试
				remindDateTime = now.Add(1 * time.Minute)
			} else {
				// 非开发环境：可以直接立即触发，或保持在宽限内的就近时间
				remindDateTime = now
			}
		} else {
			// 已经过了 grace_until：今日提醒已经错过，不再补单
			logger.Logger.Info("Skipped building reminder message: remind_at already passed grace window",
				zap.Int64("user_id", userIDInt),
				zap.String("remind_at", remindAt),
				zap.String("grace_until", user.DailyCheckInGraceUntil),
				zap.Time("now", now),
			)
			return nil
		}
	}

	// 计算延迟秒数
	delaySeconds := int(remindDateTime.Sub(now).Seconds())
	if delaySeconds < 0 {
		delaySeconds = 0 // 最小延迟为0（立即发送）
	}

	// 这里不再做「超过 24 小时直接丢弃」的特殊处理：

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

// resolveNextStep 根据用户状态和手机号验证情况确定前端引导步骤
func resolveNextStep(status model.UserStatus, phoneVerified bool) string {
	switch status {
	case model.UserStatusWaitlisted:
		return "waitlist"
	case model.UserStatusOnboarding:
		if phoneVerified {
			return "fill_contacts"
		}
		return "bind_phone"
	case model.UserStatusContact:
		if phoneVerified {
			return "fill_contacts"
		}
		return "bind_phone"
	case model.UserStatusActive:
		return "home"
	default:
		if phoneVerified {
			return "home"
		}
		return "bind_phone"
	}
}

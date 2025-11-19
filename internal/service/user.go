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
			phoneMasked = utils.MaskPhone(phone)
			phoneVerified = true
		}
	} else {
		phoneHash, ok := resultMap["phone_hash"].(*string)
		phoneVerified = ok && phoneHash != nil && *phoneHash != ""
	}

	var smsBalance, voiceBalance int
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
	if voiceVal, ok := resultMap["voice_balance"]; ok && voiceVal != nil {
		switch v := voiceVal.(type) {
		case int:
			voiceBalance = v
		case int64:
			voiceBalance = int(v)
		case float64:
			voiceBalance = int(v)
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
			VoiceBalance: voiceBalance,
		},
	}

	return result, nil
}

// UpdateUserSettings 更新用户设置， 需要更新 redis 中对应的缓存， 来帮助消息队列确认发送的消息是符合当前用户的预期的
// 以及考虑更新后是否重新发送对应的消息，在这里处理对应的消息投递，改晚了还是改早了
// 通过幂等性来过滤
func (s *UserService) UpdateUserSettings(
	ctx context.Context,
	userID string,
	req dto.UpdateUserSettingsRequest,
) error {
	var userIDInt int64
	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return pkgerrors.InvalidUserID
	}

	_, err := query.User.GetByPublicID(userIDInt)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.ErrUserNotFound
		}
		return fmt.Errorf("failed to query user: %w", err)
	}

	updates := make(map[string]interface{})

	if req.DailyCheckInEnabled != nil {
		updates["daily_check_in_enabled"] = *req.DailyCheckInEnabled
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
		return nil
	}

	if _, err := query.User.Where(query.User.PublicID.Eq(userIDInt)).Updates(updates); err != nil {
		return fmt.Errorf("failed to update user settings: %w", err)
	}

	logger.Logger.Info("User settings updated",
		zap.String("user_id", userID),
		zap.Any("updates", updates),
	)


	// 更新今日的用户缓存，如果 redis 中没有缓存的话，消息队列就可以直接发送，减少数据库回查, 更新而非失效
	user, _ := query.User.GetByPublicID(userIDInt)
	cache.SetUserSettings(ctx, userIDInt, &cache.UserSettingsCache{
		DailyCheckInEnabled:    user.DailyCheckInEnabled,
		DailyCheckInRemindAt:   user.DailyCheckInRemindAt,
		DailyCheckInDeadline:   user.DailyCheckInDeadline,
		DailyCheckInGraceUntil: user.DailyCheckInGraceUntil,
		//Status:                 string(user.Status),
		UpdatedAt:              time.Now().Unix(),
	})

	return nil
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

	balances, err := query.QuotaTransaction.GetBalanceByPublicID(userIDInt)
	if err != nil {
		return nil, fmt.Errorf("failed to query quotas: %w", err)
	}

	var smsBalance, voiceBalance int
	for _, balance := range balances {
		channel, ok := balance["channel"].(string)
		if !ok {
			continue
		}

		var balanceVal int
		switch v := balance["balance"].(type) {
		case int:
			balanceVal = v
		case int64:
			balanceVal = int(v)
		case float64:
			balanceVal = int(v)
		default:
			continue
		}

		switch channel {
		case string(model.QuotaChannelSMS):
			smsBalance = balanceVal
		case string(model.QuotaChannelVoice):
			voiceBalance = balanceVal
		}
	}

	result := &dto.QuotaBalance{
		SMSBalance:     smsBalance,
		VoiceBalance:   voiceBalance,
		SMSUnitPrice:   0.05,
		VoiceUnitPrice: 0.1,
	}

	return result, nil
}

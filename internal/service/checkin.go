package service

import (
	"AreYouOK/internal/cache"
	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/queue"
	"AreYouOK/internal/repository/query"
	pkgerrors "AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/storage/database"
	"AreYouOK/utils"
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CheckInService struct{}

var (
	checkInService *CheckInService
	checkInOnce    sync.Once
)

func CheckIn() *CheckInService {
	checkInOnce.Do(func() {
		checkInService = &CheckInService{}
	})

	return checkInService
}

// ProcessReminderBatch 批量发送打卡提醒消息（需要根据额度，或者免费？）
func (s *CheckInService) ProcessReminderBatch(
	ctx context.Context,
	userIDs []int64,
	checkInDate string,
) error {
	if len(userIDs) == 0 {
		logger.Logger.Warn("ProcessReminderBatch: empty userIDs list")
		return nil
	}
	return nil

	// date, err := time.Parse("2006-01-02", checkInDate)
	// if err != nil {
	// 	return fmt.Errorf("invalid check_in_date format: %w", err)
	// }

	//开启对应事务
	db := database.DB()

	// error 就 rollback
	// TODO: 批量发送如何解决，需要对齐
	return db.Transaction(func(tx *gorm.DB) error {
		// q := query.Use(tx).WithContext(ctx)

		// for _, userID := range userIDs {}

		return nil
	})

}

// 处理单个用户的打卡信息, 这里需要对接到具体的短信服务需要的 payload 是什么才可以知道如何去做

func (s *CheckInService) processSingleUserReminder(
	ctx context.Context,
	q *query.Query,
	userID int64,
	checkInDate time.Time,
) error { //还需要检查用户当天有没有更改

	user, err := q.User.GetByID(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Logger.Warn("User not found",
				zap.Int64("user_id", userID),
			)
			return nil // 用户不存在，跳过
		}
	}

	if user.Status != model.UserStatusActive {
		logger.Logger.Info("User is not active, skipping reminder",
			zap.Int64("user_id", userID),
			zap.String("status", string(user.Status)),
		)
		return nil
	}

	if !user.DailyCheckInEnabled {
		logger.Logger.Info("User has check-in disabled, skipping reminder",
			zap.Int64("user_id", userID),
		)
		return nil
	} // 启用就保证了一定有手机号

	//userWithQuotas, err := q.User.GetWithQuotas(userID)
	if err != nil {
		logger.Logger.Warn("Failed to get user quotas, will continue",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
	} else {
		var smsBalance float64

		if smsBalance <= 0 {
			logger.Logger.Warn("User has insufficient SMS balance, skipping reminder",
				zap.Int64("user_id", userID),
				zap.Float64("sms_balance", smsBalance),
			)
			return nil
		}
	}
	return nil
}

// GetTodayCheckIn 查询当天打卡状态
func (s *CheckInService) GetTodayCheckIn(
	ctx context.Context,
	userID string,
) (*dto.CheckInStatusData, error) {
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

	if !user.DailyCheckInEnabled {
		return nil, pkgerrors.Definition{
			Code:    "CHECK_IN_DISABLED",
			Message: "Daily check-in is disabled",
		}
	}

	checkIn, err := query.DailyCheckIn.GetTodayCheckInByPublicID(userIDInt)

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to query check-in: %w", err)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	deadline, err := utils.ParseTime(user.DailyCheckInDeadline, today)
	if err != nil {
		return nil, fmt.Errorf("invalid deadline format: %w", err)
	}

	graceUntil, err := utils.ParseTime(user.DailyCheckInGraceUntil, today)
	if err != nil {
		return nil, fmt.Errorf("invalid grace_until format: %w", err)
	}

	if checkIn == nil {
		return &dto.CheckInStatusData{
			Date:             today.Format("2006-01-02"),
			Status:           string(model.CheckInStatusPending),
			Deadline:         deadline,
			GraceUntil:       graceUntil,
			ReminderSentAt:   nil,
			AlertTriggeredAt: nil,
		}, nil
	}

	return &dto.CheckInStatusData{
		Date:             checkIn.CheckInDate.Format("2006-01-02"),
		Status:           string(checkIn.Status),
		Deadline:         deadline,
		GraceUntil:       graceUntil,
		ReminderSentAt:   checkIn.ReminderSentAt,
		AlertTriggeredAt: checkIn.AlertTriggeredAt,
	}, nil
}

// consumer 中调用对应的用户，然后返回有效的用户 ID，快速过滤今天没有更新的用户，可以优先发送

type ValidateReminderBatchResult struct {
	ProcessNow []int64 // 可以直接处理的用户
	Republish  []int64 // 需要重新投递的用户（设置改变了）
	Skipped    []int64 // 需要跳过的用户（关闭了打卡功能或不在时间段内）
}

// StartCheckInReminderConsumer(ctx) 对应调用的函数，来验证用户设置，然后来处理提醒，最后重新投递
func (s *CheckInService) ValidateReminderBatch(
	ctx context.Context,
	userSettingsSnapshots map[string]queue.UserSettingSnapshot,
	checkInDate string,
) (*ValidateReminderBatchResult, error) {
	result := &ValidateReminderBatchResult{
		ProcessNow: make([]int64, 0),
		Republish:  make([]int64, 0),
		Skipped:    make([]int64, 0),
	}

	if len(userSettingsSnapshots) == 0 {
		return result, nil
	}

	userIDs := make([]int64, len(userSettingsSnapshots))
	for userIDStr := range userSettingsSnapshots {
		userID, err := strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse user ID: %w", err)
		}
		userIDs = append(userIDs, userID)
	}

	cachedSettings, err := cache.BatchGetUserSettings(ctx, userIDs)
	if err != nil {
		logger.Logger.Error("failed to batch get user settings", zap.Error(err))
	}

	for _, userID := range userIDs {
		settings, ok := cachedSettings[userID]
		if !ok {
			// 缓存未命中，说明用户设置没有改变，可以直接发送
			result.ProcessNow = append(result.ProcessNow, userID)
			continue
		}

		// 接下来说明缓存命中，说明改变了设置
		userStrID := strconv.FormatInt(userID, 10)

		// 时间变化了
		if !settings.DailyCheckInEnabled  { //改成没有开启了，那就直接丢弃
			result.Skipped = append(result.Skipped, userID)
			continue
		}

		if settings.DailyCheckInDeadline != userSettingsSnapshots[string(userStrID)].Deadline {
			result.Republish = append(result.Republish, userID)
			continue
		}

		 if settings.DailyCheckInRemindAt != userSettingsSnapshots[string(userStrID)].RemindAt {
            result.Republish = append(result.Republish, userID)
            continue
        }
	}

	return result, nil
	// 然后获取他们对应的设置来重新投递
}

package service

import (
	"context"
	"fmt"
	"sync"
	"time"
	"errors"

	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/repository/query"
	"AreYouOK/pkg/logger"
	"AreYouOK/utils"
	"AreYouOK/storage/database"
	pkgerrors "AreYouOK/pkg/errors"

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
	}else{
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


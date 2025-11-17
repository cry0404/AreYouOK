package service

import (
	"context"
	"sync"

	"AreYouOK/pkg/logger"
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
}

// GetTodayCheckIn 查询当天打卡状态
func (s *CheckInService) GetTodayCheckIn() {

}

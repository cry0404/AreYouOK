package service

import "sync"

type CheckInService struct{}

var (
	checkInService *CheckInService
	checkInOnce    sync.Once
)

func CheckIn() *CheckInService {
	checkInOnce.Do(func(){
		checkInService = &CheckInService{}
	})

	return checkInService
}

// GetTodayCheckIn 查询当天打卡状态

func (s *CheckInService) GetTodayCheckIn(){

}

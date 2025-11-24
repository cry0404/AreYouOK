package service


import (
	"sync"
	"context"
)
// journey 创建后需要推送 notification 队列，更新时也需要参考对应的用户设置修改的部分，从而重新推送

type JourneyService struct{}

var (
	journeyService *JourneyService
	journeyOnce    sync.Once
)

func Journey() *JourneyService {
	journeyOnce.Do(func() {
		journeyService = &JourneyService{}
	})
	return journeyService
}

func (s *JourneyService) CreateJourney(ctx context.Context, userID int64, journeyID int64) error {
	return nil
}

func (s *JourneyService) UpdateJourney(ctx context.Context, userID int64, journeyID int64) error {
	return nil
}

func (s *JourneyService) GetJourney(ctx context.Context, userID int64, journeyID int64) error {
	return nil
}

func (s *JourneyService) ListJourneys(ctx context.Context, userID int64) error {
	return nil
}
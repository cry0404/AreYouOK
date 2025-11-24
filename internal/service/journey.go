package service

import (
	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/repository/query"
	pkgerrors "AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/storage/database"
	"context"
	"fmt"
	"strconv"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"
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

func (s *JourneyService) ListJourneys(ctx context.Context,
	userID int64,
	status string,
	cursorID int64,
	limit int,
) ([]*dto.JourneyItem, int64, error) {
	db := database.DB()

	q := query.Use(db)
	// 所有的都要基于 user 来联查才对, 先判断是否有 User
	_, err := q.User.GetByPublicID(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, 0, pkgerrors.Definition{
				Code: "USER_NOT_FOUND",
				Message: "User not found",
			}
		}
		return nil, 0, fmt.Errorf("failed to query user: %w", err)
	}

	if status != "" {
		if status != string(model.JourneyStatusOngoing) &&
			status != string(model.JourneyStatusEnded) &&
			status != string(model.JourneyStatusTimeout) {
				return nil, 0, pkgerrors.Definition{
					Code: "INVALID_STATUS",
					Message: "Invalid journey status",
				}
			}
	}

	journeys, err := q.Journey.ListByPublicIDAndStatus(userID, status, cursorID, limit + 1)

	if err != nil {
		logger.Logger.Error("Failed to list journeys",
		zap.Int64("user_id", userID),
		zap.Error(err),
		)

		return nil, 0, fmt.Errorf("failed to list journeys: %w", err)
	}

	var nextCursor int64
	if len(journeys) > limit {
		nextCursor = journeys[limit].ID // 列出返回的模式存储下去，返回给前端表示是否还有下一页
		journeys = journeys[:limit]
	}

	result := make([]*dto.JourneyItem, 0, len(journeys))

	for _, journey := range journeys {
		result = append(result, &dto.JourneyItem{
			ID: 	strconv.FormatInt(journey.ID, 10),
			Title: 	journey.Title,
			Note:	journey.Note,
			Status: string(journey.Status),
			ExpectedReturnTime: journey.ExpectedReturnTime,
			ActualReturnTime:	journey.ActualReturnTime,
			CreatedAt:		journey.CreatedAt,
		})
	}


	return result, nextCursor, nil
}

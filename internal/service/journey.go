package service

import (
	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/repository/query"
	pkgerrors "AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/storage/database"
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

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

func (s *JourneyService) CreateJourney(
	ctx context.Context,
	userID int64,
	req dto.CreateJourneyRequest,
) (*dto.JourneyItem, error) {

	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	user, err := q.User.GetByPublicID(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerrors.Definition{
				Code:    "USER_NOT_FOUND",
				Message: "User not found",
			}
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	if req.ExpectedReturnTime.Before(time.Now()) {
		return nil, pkgerrors.Definition{
			Code:    "INVALID_RETURN_TIME",
			Message: "Expected return time must be in the future",
		}
	}

	journeyID, err := snowflake.NextID(snowflake.GeneratorTypeJourney)

	if err != nil {
		return nil, fmt.Errorf("failed to generate journey ID: %w", err)
	}

	journey := &model.Journey{
		BaseModel: model.BaseModel{
			ID: journeyID,
		},
		UserID:            user.ID,
		Title:             req.Title,
		Note:              req.Note,
		ExpectedReturnTime: req.ExpectedReturnTime,
		Status:            model.JourneyStatusOngoing,
		AlertStatus:       model.AlertStatusPending,
		AlertAttempts:     0,
	}

	if err := q.Journey.Create(journey); err != nil {
		logger.Logger.Error("Failed to create journey",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create journey: %w", err)
	}

	// TODO: 推送 notification 队列
	// 1. 检查用户设置 journey_auto_notify
	// 2. 如果开启，创建 notification_tasks
	// 3. 发布 NotificationMessage 到 RabbitMQ

	logger.Logger.Info("Journey created",
		zap.Int64("journey_id", journeyID),
		zap.Int64("user_id", user.ID),
	)

	return &dto.JourneyItem{
		ID:                 strconv.FormatInt(journey.ID, 10),
		Title:              journey.Title,
		Note:               journey.Note,
		Status:             string(journey.Status),
		ExpectedReturnTime: journey.ExpectedReturnTime,
		ActualReturnTime:   journey.ActualReturnTime,
		CreatedAt:          journey.CreatedAt,
	}, nil
}

func (s *JourneyService) UpdateJourney(
	ctx context.Context,
	userID int64,
	journeyID int64,
	req dto.UpdateJourneyRequest,
) (*dto.JourneyItem, error) {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	journey, err := q.Journey.GetByPublicIDAndJourneyID(userID, journeyID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerrors.Definition{
				Code:    "JOURNEY_NOT_FOUND",
				Message: "Journey not found",
			}
		}
		return nil, fmt.Errorf("failed to query journey: %w", err)
	}

		// 检查行程状态：已结束或超时的行程不可修改
	if journey.Status == model.JourneyStatusEnded || journey.Status == model.JourneyStatusTimeout {
		return nil, pkgerrors.Definition{
			Code:    "JOURNEY_NOT_MODIFIABLE",
			Message: "Journey cannot be modified after it has ended or timed out",
		}
	}

	// 更新字段
	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Note != nil {
		updates["note"] = *req.Note
	}
	if req.ExpectedReturnTime != nil {
		// 验证预计返回时间必须晚于当前时间
		if req.ExpectedReturnTime.Before(time.Now()) {
			return nil, pkgerrors.Definition{
				Code:    "INVALID_RETURN_TIME",
				Message: "Expected return time must be in the future",
			}
		}
		updates["expected_return_time"] = *req.ExpectedReturnTime
	}

		if len(updates) == 0 {
		return &dto.JourneyItem{
			ID:                 strconv.FormatInt(journey.ID, 10),
			Title:              journey.Title,
			Note:               journey.Note,
			Status:             string(journey.Status),
			ExpectedReturnTime: journey.ExpectedReturnTime,
			ActualReturnTime:   journey.ActualReturnTime,
			CreatedAt:          journey.CreatedAt,
		}, nil
	}

	// 执行更新
	updates["updated_at"] = time.Now()
	_, err = q.Journey.
		Where(q.Journey.ID.Eq(journey.ID)).
		Updates(updates)
	if err != nil {
		logger.Logger.Error("Failed to update journey",
			zap.Int64("journey_id", journeyID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to update journey: %w", err)
	}

	// 重新查询更新后的数据
	updatedJourney, err := q.Journey.GetByID(journey.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query updated journey: %w", err)
	}

	// TODO: 如果 expected_return_time 更新了，需要重新推送 notification 队列
	// 1. 检查用户设置 journey_auto_notify
	// 2. 如果开启，更新或重新创建 notification_tasks
	// 3. 发布 NotificationMessage 到 RabbitMQ

	logger.Logger.Info("Journey updated",
		zap.Int64("journey_id", journeyID),
		zap.Int64("user_id", userID),
	)


	return &dto.JourneyItem{
		ID:                 strconv.FormatInt(updatedJourney.ID, 10),
		Title:              updatedJourney.Title,
		Note:               updatedJourney.Note,
		Status:             string(updatedJourney.Status),
		ExpectedReturnTime: updatedJourney.ExpectedReturnTime,
		ActualReturnTime:   updatedJourney.ActualReturnTime,
		CreatedAt:          updatedJourney.CreatedAt,
	}, nil
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
				Code:    "USER_NOT_FOUND",
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
				Code:    "INVALID_STATUS",
				Message: "Invalid journey status",
			}
		}
	}

	journeys, err := q.Journey.ListByPublicIDAndStatus(userID, status, cursorID, limit+1)

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
			ID:                 strconv.FormatInt(journey.ID, 10),
			Title:              journey.Title,
			Note:               journey.Note,
			Status:             string(journey.Status),
			ExpectedReturnTime: journey.ExpectedReturnTime,
			ActualReturnTime:   journey.ActualReturnTime,
			CreatedAt:          journey.CreatedAt,
		})
	}

	return result, nextCursor, nil
}

func (s *JourneyService) GetJourneyDetail(
	ctx context.Context,
	userID int64,
	journeyID int64,
) (*dto.JourneyDetail, error) {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	journey, err := q.Journey.GetByPublicIDAndJourneyID(userID, journeyID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerrors.Definition{
				Code:    "JOURNEY_NOT_FOUND",
				Message: "Journey not found",
			}
		}
		return nil, fmt.Errorf("failed to query journey: %w", err)
	}


	return &dto.JourneyDetail{
		JourneyItem: dto.JourneyItem{
			ID:                 strconv.FormatInt(journey.ID, 10),
			Title:              journey.Title,
			Note:               journey.Note,
			Status:             string(journey.Status),
			ExpectedReturnTime: journey.ExpectedReturnTime,
			ActualReturnTime:   journey.ActualReturnTime,
			CreatedAt:          journey.CreatedAt,
		},
		ReminderSentAt:     journey.ReminderSentAt,
		AlertTriggeredAt:   journey.AlertTriggeredAt,
		AlertLastAttemptAt: journey.AlertLastAttemptAt,
		AlertStatus:        string(journey.AlertStatus),
		AlertAttempts:      journey.AlertAttempts,
	}, nil
}

// CompleteJourney 归来打卡，标记行程结束
func (s *JourneyService) CompleteJourney(
	ctx context.Context,
	userID int64,
	journeyID int64,
) (*dto.JourneyItem, error) {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	// 查询行程（使用 public_id 和 journey_id）
	journey, err := q.Journey.GetByPublicIDAndJourneyID(userID, journeyID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerrors.Definition{
				Code:    "JOURNEY_NOT_FOUND",
				Message: "Journey not found",
			}
		}
		return nil, fmt.Errorf("failed to query journey: %w", err)
	}


	if journey.Status == model.JourneyStatusEnded {
		return nil, pkgerrors.Definition{
			Code:    "JOURNEY_ALREADY_ENDED",
			Message: "Journey has already been completed",
		}
	}

	// 更新行程状态
	now := time.Now()
	_, err = q.Journey.
		Where(q.Journey.ID.Eq(journey.ID)).
		Updates(map[string]interface{}{
			"status":             model.JourneyStatusEnded,
			"actual_return_time": now,
			"updated_at":         now,
		})
	if err != nil {
		logger.Logger.Error("Failed to complete journey",
			zap.Int64("journey_id", journeyID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to complete journey: %w", err)
	}

	completedJourney, err := q.Journey.GetByID(journey.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query completed journey: %w", err)
	}

	// TODO: 取消或标记相关的 notification_tasks 为已取消

	logger.Logger.Info("Journey completed",
		zap.Int64("journey_id", journeyID),
		zap.Int64("user_id", userID),
	)

	return &dto.JourneyItem{
		ID:                 strconv.FormatInt(completedJourney.ID, 10),
		Title:              completedJourney.Title,
		Note:               completedJourney.Note,
		Status:             string(completedJourney.Status),
		ExpectedReturnTime: completedJourney.ExpectedReturnTime,
		ActualReturnTime:   completedJourney.ActualReturnTime,
		CreatedAt:          completedJourney.CreatedAt,
	}, nil
}

// AckJourneyAlert 确认已知晓行程超时提醒
func (s *JourneyService) AckJourneyAlert(
	ctx context.Context,
	userID int64,
	journeyID int64,
) error {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	// 查询行程（使用 public_id 和 journey_id）
	_, err := q.Journey.GetByPublicIDAndJourneyID(userID, journeyID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkgerrors.Definition{
				Code:    "JOURNEY_NOT_FOUND",
				Message: "Journey not found",
			}
		}
		return fmt.Errorf("failed to query journey: %w", err)
	}

	// TODO: 终止后续通知流程
	// 1. 更新 journey.alert_status 为已确认（可能需要新增状态）
	// 2. 取消或标记相关的 notification_tasks 为已取消

	logger.Logger.Info("Journey alert acknowledged",
		zap.Int64("journey_id", journeyID),
		zap.Int64("user_id", userID),
	)

	return nil
}

// GetJourneyAlerts 查询行程提醒执行状态
func (s *JourneyService) GetJourneyAlerts(
	ctx context.Context,
	publicUserID int64,
	journeyID int64,
) (*dto.JourneyAlertData, error) {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	// 查询行程（使用 public_id 和 journey_id）
	journey, err := q.Journey.GetByPublicIDAndJourneyID(publicUserID, journeyID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, pkgerrors.Definition{
				Code:    "JOURNEY_NOT_FOUND",
				Message: "Journey not found",
			}
		}
		return nil, fmt.Errorf("failed to query journey: %w", err)
	}

	return &dto.JourneyAlertData{
		AlertStatus:        string(journey.AlertStatus),
		AlertAttempts:      journey.AlertAttempts,
		AlertLastAttemptAt: journey.AlertLastAttemptAt,
	}, nil
}

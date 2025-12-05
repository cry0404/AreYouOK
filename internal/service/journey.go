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
	"sort"
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
) (*model.Journey, *model.JourneyTimeoutMessage, error) {

	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	user, err := q.User.GetByPublicID(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, pkgerrors.Definition{
				Code:    "USER_NOT_FOUND",
				Message: "User not found",
			}
		}
		return nil, nil, fmt.Errorf("failed to query user: %w", err)
	}

	if req.ExpectedReturnTime.Before(time.Now()) {
		return nil, nil, pkgerrors.Definition{
			Code:    "INVALID_RETURN_TIME",
			Message: "Expected return time must be in the future",
		}
	}
	now := time.Now()
	if req.ExpectedReturnTime.Sub(now) < 10*time.Minute {
		return nil, nil, pkgerrors.Definition{
			Code:    "JOURNEY_RETURN_TIME_TOO_CLOSE",
			Message: "Expected return time must be at least 10 minutes in the future",
		}
	}
	// 这里还需要考虑要不要限定前几分钟不能修改？

	journeyID, err := snowflake.NextID(snowflake.GeneratorTypeJourney)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate journey ID: %w", err)
	}

	journey := &model.Journey{
		BaseModel: model.BaseModel{
			ID: journeyID,
		},
		UserID:             user.ID,
		Title:              req.Title,
		Note:               req.Note,
		ExpectedReturnTime: req.ExpectedReturnTime,
		Status:             model.JourneyStatusOngoing,
		AlertStatus:        model.AlertStatusPending,
		AlertAttempts:      0,
	}

	// journey 的 create 和消息的投放应该是一个事务吗，应该是完整的部分才对
	if err := q.Journey.Create(journey); err != nil {
		logger.Logger.Error("Failed to create journey",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)
		return nil, nil, fmt.Errorf("failed to create journey: %w", err)
	}

	// 开始集成延迟消息的投放，这里的判断消息投递

	// 1. 检查用户设置 journey_auto_notify
	// 2. 如果开启，构建通知消息（但不发布，返回给调用方）

	var timeoutMsg *model.JourneyTimeoutMessage

	if user.JourneyAutoNotify {
		delayTime := journey.ExpectedReturnTime.Add(10 * time.Minute)
		now := time.Now()

		if delayTime.After(now) {
			delay := delayTime.Sub(now)

			// 如果延迟时间 <= 1天，构建消息返回给调用方
			if delay <= 24*time.Hour {
				messageID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
				if err != nil {
					logger.Logger.Error("Failed to generate message ID",
						zap.Int64("journey_id", journeyID),
						zap.Error(err),
					)
					// 不阻塞主流程，继续向下执行
				} else {
					timeoutMsg = &model.JourneyTimeoutMessage{
						MessageID:    fmt.Sprintf("journey_timeout_%d", messageID),
						ScheduledAt:  now.Format(time.RFC3339),
						JourneyID:    journeyID,
						UserID:       userID,
						DelaySeconds: int(delay.Seconds()),
					}
					logger.Logger.Info("Journey timeout message prepared",
						zap.Int64("journey_id", journeyID),
						zap.Int64("user_id", userID),
						zap.Time("expected_return_time", journey.ExpectedReturnTime),
					)
				}
			} else {
				logger.Logger.Info("Journey delay exceeds 24 hours, will be handled by scheduled task",
					zap.Int64("journey_id", journeyID),
					zap.Int64("user_id", userID),
					zap.Duration("delay", delay),
					zap.Time("expected_return_time", journey.ExpectedReturnTime),
				)
			}
		} else {
			logger.Logger.Warn("Journey expected return time is in the past",
				zap.Int64("journey_id", journeyID),
				zap.Time("expected_return_time", journey.ExpectedReturnTime),
			)
		}
	}

	logger.Logger.Info("Journey created",
		zap.Int64("journey_id", journeyID),
		zap.Int64("user_id", user.ID),
	)

	return journey, timeoutMsg, nil
}

func (s *JourneyService) UpdateJourney(
	ctx context.Context,
	userID int64,
	journeyID int64,
	req dto.UpdateJourneyRequest,
) (*model.Journey, *model.JourneyTimeoutMessage, error) {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	journey, err := q.Journey.GetByPublicIDAndJourneyID(userID, journeyID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, pkgerrors.Definition{
				Code:    "JOURNEY_NOT_FOUND",
				Message: "Journey not found",
			}
		}
		return nil, nil, fmt.Errorf("failed to query journey: %w", err)
	}

	// 检查行程状态：已结束或超时的行程不可修改
	if journey.Status == model.JourneyStatusEnded || journey.Status == model.JourneyStatusTimeout {
		return nil, nil, pkgerrors.Definition{
			Code:    "JOURNEY_NOT_MODIFIABLE",
			Message: "Journey cannot be modified after it has ended or timed out",
		}
	}

	now := time.Now()


	// 检查预计返回时间是否太近（至少需要 10 分钟）
	if req.ExpectedReturnTime != nil && req.ExpectedReturnTime.Sub(now) < 10*time.Minute {
		return nil, nil, pkgerrors.Definition{
			Code:    "JOURNEY_RETURN_TIME_TOO_CLOSE",
			Message: "Expected return time must be at least 10 minutes in the future",
		}
	}

	var needReschedule bool
	var oldExpectedReturnTime time.Time

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
			return nil, nil, pkgerrors.Definition{
				Code:    "INVALID_RETURN_TIME",
				Message: "Expected return time must be in the future",
			}
		}
		oldExpectedReturnTime = journey.ExpectedReturnTime
		updates["expected_return_time"] = *req.ExpectedReturnTime
		needReschedule = true // 标记需要重新调度
	}

	if len(updates) == 0 {
		return journey, nil, nil
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
		return nil, nil, fmt.Errorf("failed to update journey: %w", err)
	}

	// 重新查询更新后的数据
	updatedJourney, err := q.Journey.GetByID(journey.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query updated journey: %w", err)
	}

	// 如果 expected_return_time 更新了，重新构建消息对象
	var timeoutMsg *model.JourneyTimeoutMessage
	if needReschedule && req.ExpectedReturnTime != nil {
		// 查询用户设置
		user, err := q.User.GetByID(updatedJourney.UserID)
		if err == nil && user.JourneyAutoNotify {
			// 计算新的延迟时间
			delayTime := updatedJourney.ExpectedReturnTime.Add(10 * time.Minute)
			now := time.Now()

			if delayTime.After(now) {
				delay := delayTime.Sub(now)

				if delay <= 24*time.Hour {
					// 构建新的消息对象（但不发布）
					messageID, err := snowflake.NextID(snowflake.GeneratorTypeMessage)
					if err != nil {
						logger.Logger.Error("Failed to generate message ID for reschedule",
							zap.Int64("journey_id", journeyID),
							zap.Error(err),
						)
					} else {
						timeoutMsg = &model.JourneyTimeoutMessage{
							MessageID:    fmt.Sprintf("journey_timeout_%d", messageID),
							ScheduledAt:  now.Format(time.RFC3339),
							JourneyID:    journeyID,
							UserID:       userID,
							DelaySeconds: int(delay.Seconds()),
							// 可以添加旧时间用于日志
							// OldExpectedReturnTime: oldExpectedReturnTime,
						}
						logger.Logger.Info("Journey timeout message prepared for reschedule",
							zap.Int64("journey_id", journeyID),
							zap.Time("old_time", oldExpectedReturnTime),
							zap.Time("new_time", updatedJourney.ExpectedReturnTime),
						)
					}
				} else {
					logger.Logger.Info("Journey reschedule delay exceeds 24 hours, will be handled by scheduled task",
						zap.Int64("journey_id", journeyID),
						zap.Duration("delay", delay),
						zap.Time("expected_return_time", updatedJourney.ExpectedReturnTime),
					)
				}
			}
		}
	}

	logger.Logger.Info("Journey updated",
		zap.Int64("journey_id", journeyID),
		zap.Int64("user_id", userID),
	)

	return updatedJourney, timeoutMsg, nil
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

	nofilltered, err := q.Journey.ListByPublicIDAndStatus(userID, status, cursorID, limit+1)

	
	var journeys []*model.Journey

	for _, journey := range nofilltered {
		if journey.BaseModel.DeletedAt.Valid  {
			continue //说明不为空，跳过
		} 
		journeys = append(journeys, journey)
	}
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

	// 使用优化查询，避免多次查询
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

	// consumer 会检查行程的状态，如果已经完成了就跳过超时处理

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

// ProcessReminder 处理行程提醒（发送给用户本人）
// 在预计返回时间到了时提醒用户打卡
// 返回创建的通知任务（如果创建了的话），供 consumer 发布到短信队列
func (s *JourneyService) ProcessReminder(
	ctx context.Context,
	journeyID int64,
	userID int64,
) (*model.NotificationTask, error) {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	// 查询行程
	journey, err := q.Journey.GetByPublicIDAndJourneyID(userID, journeyID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 行程不存在：说明已经被删除或从未创建，对当前消息来说是可跳过的
			return nil, &pkgerrors.SkipMessageError{Reason: "Journey not found for reminder"}
		}
		return nil, fmt.Errorf("failed to query journey: %w", err)
	}

	// 检查行程状态，只有进行中的行程才需要提醒
	if journey.Status != model.JourneyStatusOngoing {
		logger.Logger.Debug("Journey is not ongoing, skipping reminder",
			zap.Int64("journey_id", journeyID),
			zap.String("status", string(journey.Status)),
		)
		return nil, nil
	}

	// 检查是否已经发送过提醒（防止重复发送）
	if journey.ReminderSentAt != nil {
		logger.Logger.Debug("Journey reminder already sent, skipping",
			zap.Int64("journey_id", journeyID),
			zap.Time("reminder_sent_at", *journey.ReminderSentAt),
		)
		return nil, nil
	}

	// 查询用户
	user, err := q.User.GetByID(journey.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// 检查用户额度（提醒短信也需要扣费）
	quotaService := Quota()
	wallet, err := quotaService.GetWallet(ctx, user.ID, model.QuotaChannelSMS)
	if err != nil {
		return nil, fmt.Errorf("failed to query SMS quota wallet: %w", err)
	}

	smsUnitPriceCents := 5
	if wallet.AvailableAmount < smsUnitPriceCents {
		logger.Logger.Warn("Insufficient quota for journey reminder",
			zap.Int64("user_id", user.ID),
			zap.Int64("journey_id", journeyID),
			zap.Int("balance", wallet.AvailableAmount),
		)
		// 额度不足，跳过提醒但不报错
		return nil, nil
	}

	// 生成任务代码
	taskCode, err := snowflake.NextID(snowflake.GeneratorTypeTask)
	if err != nil {
		return nil, fmt.Errorf("failed to generate task code: %w", err)
	}

	now := time.Now()

	// 构建 payload（发送给用户本人，使用 journey_timeout 类型）
	payload := model.JSONB{
		"type": "journey_timeout", // 无参数模板，提醒用户打卡
	}

	// 创建通知任务（发送给用户本人，不是紧急联系人）
	notificationTask := &model.NotificationTask{
		TaskCode:    taskCode,
		UserID:      user.ID,
		Category:    model.NotificationCategoryJourneyReminder,
		Channel:     model.NotificationChannelSMS,
		Status:      model.NotificationTaskStatusPending,
		Payload:     payload,
		ScheduledAt: now,
		// 不设置 ContactPriority 和 ContactPhoneHash，表示发送给用户本人
	}

	if err := q.NotificationTask.Create(notificationTask); err != nil {
		return nil, fmt.Errorf("failed to create notification task: %w", err)
	}

	// 更新行程的 reminder_sent_at
	_, err = q.Journey.
		Where(q.Journey.ID.Eq(journey.ID)).
		Updates(map[string]interface{}{
			"reminder_sent_at": now,
			"updated_at":       now,
		})
	if err != nil {
		logger.Logger.Warn("Failed to update journey reminder_sent_at",
			zap.Int64("journey_id", journeyID),
			zap.Error(err),
		)
	}

	logger.Logger.Info("Journey reminder processed",
		zap.Int64("journey_id", journeyID),
		zap.Int64("user_id", user.ID),
		zap.Int64("task_code", taskCode),
	)

	return notificationTask, nil
}

// ProcessTimeout 处理行程超时
// 返回创建的通知任务列表（用于发布到队列）
func (s *JourneyService) ProcessTimeout(
	ctx context.Context,
	journeyID int64,
	userID int64,
) ([]*model.NotificationTask, error) {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	// 查询行程
	journey, err := q.Journey.GetByPublicIDAndJourneyID(userID, journeyID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 行程不存在：说明已经被删除或从未创建，对当前消息来说是可跳过的
			return nil, &pkgerrors.SkipMessageError{Reason: "Journey not found for timeout"}
		}
		return nil, fmt.Errorf("failed to query journey: %w", err)
	}

	// 检查行程状态
	if journey.Status != model.JourneyStatusOngoing {
		logger.Logger.Debug("Journey is not ongoing, skipping timeout processing",
			zap.Int64("journey_id", journeyID),
			zap.String("status", string(journey.Status)),
		)
		return nil, nil // 已处理或已结束，跳过
	}

	// 安全校验：只有在当前时间晚于「预计返回时间 + 10 分钟」时才允许执行超时处理
	now := time.Now()
	timeoutThreshold := journey.ExpectedReturnTime.Add(10 * time.Minute)
	if now.Before(timeoutThreshold) {
		logger.Logger.Info("Journey has not reached timeout threshold, skipping timeout processing",
			zap.Int64("journey_id", journeyID),
			zap.Time("expected_return_time", journey.ExpectedReturnTime),
			zap.Time("timeout_threshold", timeoutThreshold),
			zap.Time("now", now),
		)
		return nil, nil
	}

	// 查询用户
	user, err := q.User.GetByID(journey.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// 检查用户额度 - 使用新的 QuotaService
	quotaService := Quota()
	wallet, err := quotaService.GetWallet(ctx, user.ID, model.QuotaChannelSMS)
	if err != nil {
		return nil, fmt.Errorf("failed to query SMS quota wallet: %w", err)
	}

	smsBalance := wallet.AvailableAmount

	contactCount := len(user.EmergencyContacts)
	smsUnitPriceCents := 5
	totalCost := smsUnitPriceCents * contactCount

	if smsBalance < totalCost {
		logger.Logger.Warn("Insufficient quota for journey timeout alert",
			zap.Int64("user_id", user.ID),
			zap.Int64("journey_id", journeyID),
			zap.Int("balance", smsBalance),
			zap.Int("required", totalCost),
		)
		// 额度不足，更新行程状态但不发送通知
		now := time.Now()
		_, err = q.Journey.
			Where(q.Journey.ID.Eq(journey.ID)).
			Updates(map[string]interface{}{
				"status":             model.JourneyStatusTimeout,
				"alert_status":       model.AlertStatusFailed,
				"alert_triggered_at": now,
				"updated_at":         now,
			})
		return nil, err
	}

	// 收集创建的任务
	var createdTasks []*model.NotificationTask

	// 使用事务处理
	err = db.Transaction(func(tx *gorm.DB) error {
		txQ := query.Use(tx)

		// 更新行程状态
		_, err = txQ.Journey.
			Where(txQ.Journey.ID.Eq(journey.ID)).
			Updates(map[string]interface{}{
				"status":                model.JourneyStatusTimeout,
				"alert_status":          model.AlertStatusTriggered,
				"alert_triggered_at":    now,
				"alert_last_attempt_at": now,
				"alert_attempts":        gorm.Expr("alert_attempts + ?", 1),
				"updated_at":            now,
			})
		if err != nil {
			return fmt.Errorf("failed to update journey: %w", err)
		}

		// 按优先级排序紧急联系人
		contacts := user.EmergencyContacts
		sort.Slice(contacts, func(i, j int) bool {
			return contacts[i].Priority < contacts[j].Priority
		})

		// 最多通知3位
		maxContacts := 3
		if len(contacts) > maxContacts {
			contacts = contacts[:maxContacts]
		}

		// 获取用户昵称
		userName := user.Nickname
		if userName == "" {
			userName = fmt.Sprintf("%d", userID)
		}

		// 为每个紧急联系人创建通知任务
		for _, contact := range contacts {
			taskCode, err := snowflake.NextID(snowflake.GeneratorTypeTask)
			if err != nil {
				logger.Logger.Error("Failed to generate task code",
					zap.Int64("journey_id", journeyID),
					zap.Error(err),
				)
				continue
			}

			// 构建 payload（包含行程信息）
			// 使用 journey_reminder_contact 类型发给紧急联系人
			// 字段需要与阿里云模板变量匹配：name, trip, time, note
			payload := model.JSONB{
				"type": "journey_reminder_contact",
				"name": userName,
				"trip": journey.Title,
				"time": journey.ExpectedReturnTime.Format("2006-01-02 15:04"),
				"note": journey.Note,
			}

			notificationTask := &model.NotificationTask{
				TaskCode:         taskCode,
				UserID:           user.ID,
				Category:         model.NotificationCategoryJourneyTimeout,
				Channel:          model.NotificationChannelSMS,
				Status:           model.NotificationTaskStatusPending,
				Payload:          payload,
				ContactPriority:  &contact.Priority,
				ContactPhoneHash: &contact.PhoneHash,
				ScheduledAt:      now,
			}

			if err := txQ.NotificationTask.Create(notificationTask); err != nil {
				logger.Logger.Error("Failed to create notification task",
					zap.Int64("journey_id", journeyID),
					zap.Error(err),
				)
				return fmt.Errorf("failed to create notification task: %w", err)
			}

			// 收集创建的任务
			createdTasks = append(createdTasks, notificationTask)
		}

		logger.Logger.Info("Journey timeout processed",
			zap.Int64("journey_id", journeyID),
			zap.Int64("user_id", user.ID),
			zap.Int("contact_count", len(contacts)),
		)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return createdTasks, nil
}

// 软删除，避免排查起来痛苦
func (s *JourneyService) DeleteJourney(
	ctx context.Context,
	userPublicID,
	journeyID int64,
) error {
	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	// 根据用户和行程锁定唯一一个部分
	journey, err := q.Journey.GetByPublicIDAndJourneyID(userPublicID, journeyID)

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return pkgerrors.Definition{
				Code:    "JOURNEY_NOT_FOUND",
				Message: "Journey not found",
			}
		}
		return fmt.Errorf("failed to query journey: %w", err)
	}

	// if journey.Status != model.JourneyStatusOngoing {
	// 	return pkgerrors.Definition{
	// 		Code:    "JOURNEY_NOT_DELETABLE",
	// 		Message: "Only ongoing journeys can be deleted",
	// 	}
	// }

	now := time.Now()
	updates := map[string]interface{}{
		"status":     model.JourneyStatusCancelled,
		"updated_at": now,
	}

	updates["deleted_at"] = now

	if _, err := q.Journey.
		Where(q.Journey.ID.Eq(journey.ID)).
		Updates(updates); err != nil {
		return fmt.Errorf("failed to cancel journey: %w", err)
	}

	logger.Logger.Info("Journey cancelled by user",
		zap.Int64("journey_id", journeyID),
		zap.Int64("user_public_id", userPublicID),
	)

	return nil
}

// // AckJourneyAlert 确认已知晓行程超时提醒
// func (s *JourneyService) AckJourneyAlert(
// 	ctx context.Context,
// 	userID int64,
// 	journeyID int64,
// ) error {
// 	db := database.DB().WithContext(ctx)
// 	q := query.Use(db)

// 	// 查询行程（使用 public_id 和 journey_id）
// 	_, err := q.Journey.GetByPublicIDAndJourneyID(userID, journeyID)
// 	if err != nil {
// 		if err == gorm.ErrRecordNotFound {
// 			return pkgerrors.Definition{
// 				Code:    "JOURNEY_NOT_FOUND",
// 				Message: "Journey not found",
// 			}
// 		}
// 		return fmt.Errorf("failed to query journey: %w", err)
// 	}

// 	// TODO: 终止后续通知流程
// 	// 1. 更新 journey.alert_status 为已确认（可能需要新增状态）
// 	// 2. 取消或标记相关的 notification_tasks 为已取消

// 	logger.Logger.Info("Journey alert acknowledged",
// 		zap.Int64("journey_id", journeyID),
// 		zap.Int64("user_id", userID),
// 	)

// 	return nil
// }

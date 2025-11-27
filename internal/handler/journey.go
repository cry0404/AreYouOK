package handler

import (
	"AreYouOK/internal/middleware"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/queue"
	"AreYouOK/internal/service"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/response"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"
)

// 通用的设计模式，解析发送的 param，然后将其作为参数传到 service，由 service 处理即可
// ListJourneys 查询当前与历史行程, 对应 limit 参数默认暂时出对应的条数
// GET /v1/journeys
func ListJourneys(ctx context.Context, c *app.RequestContext) {
	userIDStr, ok := middleware.GetUserID(ctx, c)

	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)

	if err != nil {
		response.Error(ctx, c, errors.Definition{
			Code: "INVALID_USER_ID",
			Message: "Invalid user ID format",
		})
		return
	}

	var query dto.JourneyListQuery

	if err := c.BindAndValidate(&query); err != nil {
		response.BindError(ctx, c, err)
		return
	}

	if query.Limit <= 0 {
		query.Limit = 20
	}

	if query.Limit > 100 {
		query.Limit = 100
	}

	var cursorID int64 // 对应的页指针，防止太远了
	if query.Cursor != "" {
		_, err := strconv.ParseInt(query.Cursor, 10, 64)
		if err != nil {
			response.Error(ctx, c, errors.Definition{
				Code: "INVALID_CURSOR",
				Message: "Invalid cursor format",
			})
			return
		}
	}

	journeyService := service.Journey()

	result, nextCursor, err := journeyService.ListJourneys(ctx, userID, query.Status, cursorID, query.Limit)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	meta := make(map[string]interface{})
	if nextCursor > 0 {
		meta["next_cursor"] = strconv.FormatInt(nextCursor, 10)
	}

	response.SuccessWithMeta(ctx, c, result, meta)
}

// CreateJourney 创建行程报备
// POST /v1/journeys
func CreateJourney(ctx context.Context, c *app.RequestContext) {
	var req dto.CreateJourneyRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.Error(ctx, c, err)
		return
	}

	userIDStr, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)

	if err != nil {
		response.Error(ctx, c, errors.Definition{
			Code: "INVALID_USER_ID",
			Message: "Invalid user ID format",
		})
	}

	journeyService := service.Journey()

	journey, timeoutMsg, err := journeyService.CreateJourney(ctx, userID, req)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	// 如果返回了超时消息，在 Handler 层负责发布到队列
	if timeoutMsg != nil {
		if err := queue.PublishJourneyTimeout(*timeoutMsg); err != nil {
			// 记录错误但不影响主流程
			zap.L().Error("Failed to publish journey timeout message",
				zap.Int64("journey_id", journey.ID),
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
		} else {
			zap.L().Info("Journey timeout message published",
				zap.Int64("journey_id", journey.ID),
				zap.Duration("delay", time.Duration(timeoutMsg.DelaySeconds)*time.Second),
			)
		}
	}

	// 转换为 DTO 返回
	result := &dto.JourneyItem{
		ID:                 strconv.FormatInt(journey.ID, 10),
		Title:              journey.Title,
		Note:               journey.Note,
		Status:             string(journey.Status),
		ExpectedReturnTime: journey.ExpectedReturnTime,
		ActualReturnTime:   journey.ActualReturnTime,
		CreatedAt:          journey.CreatedAt,
	}

	c.JSON(201, response.SuccessResponse{
		Data: result,
	})
}

// GetJourneyDetail 查看行程详情
// GET /v1/journeys/:journey_id
func GetJourneyDetail(ctx context.Context, c *app.RequestContext) {
	journeyIDStr := c.Param("journey_id")
	journeyID, err := strconv.ParseInt(journeyIDStr, 10, 64)
	if err != nil {
		response.Error(ctx, c, errors.Definition{
			Code: "INVALID_JOURNEY_ID",
			Message: "Invalid journey ID format",
		})
		return
	}

	
	userIDStr, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_USER_ID",
			Message: "Invalid user ID format",
		})
		return
	}

	journeyService := service.Journey()
	result, err := journeyService.GetJourneyDetail(ctx, userID, journeyID)

	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}

// UpdateJourney 更新行程
// PATCH /v1/journeys/:journey_id
func UpdateJourney(ctx context.Context, c *app.RequestContext) {
	journeyIDStr := c.Param("journey_id")

	journeyID, err := strconv.ParseInt(journeyIDStr, 10, 64)
	if err != nil {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_JOURNEY_ID",
			Message: "Invalid journey ID format",
		})
		return
	}

	var req dto.UpdateJourneyRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BindError(ctx, c, err)
		return
	}

	userIDStr, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_USER_ID",
			Message: "Invalid user ID format",
		})
		return
	}

	journeyService := service.Journey()
	journey, timeoutMsg, err := journeyService.UpdateJourney(ctx, userID, journeyID, req)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	// 如果返回了超时消息（改期），在 Handler 层负责重新发布到队列
	if timeoutMsg != nil {
		if err := queue.PublishJourneyTimeout(*timeoutMsg); err != nil {
			// 记录错误但不影响主流程
			zap.L().Error("Failed to publish rescheduled journey timeout message",
				zap.Int64("journey_id", journey.ID),
				zap.Int64("user_id", userID),
				zap.Error(err),
			)
		} else {
			zap.L().Info("Journey timeout message republished after reschedule",
				zap.Int64("journey_id", journey.ID),
				zap.Duration("delay", time.Duration(timeoutMsg.DelaySeconds)*time.Second),
			)
		}
	}

	// 转换为 DTO 返回
	result := &dto.JourneyItem{
		ID:                 strconv.FormatInt(journey.ID, 10),
		Title:              journey.Title,
		Note:               journey.Note,
		Status:             string(journey.Status),
		ExpectedReturnTime: journey.ExpectedReturnTime,
		ActualReturnTime:   journey.ActualReturnTime,
		CreatedAt:          journey.CreatedAt,
	}

	response.Success(ctx, c, result)
}

// CompleteJourney 归来打卡，标记行程结束, 点击结束的形式，还可以考虑提供一个手动调用的函数来完成行程
// POST /v1/journeys/:journey_id/complete
func CompleteJourney(ctx context.Context, c *app.RequestContext) {
	journeyIDStr := c.Param("journey_id")
	journeyID, err := strconv.ParseInt(journeyIDStr, 10, 64)
	if err != nil {
		response.Error(ctx, c, errors.Definition{
			Code:  "INVALID_JOURNEY_ID",
			Message: "Invalid journey ID format",
		})
		return
	}

	userIDStr, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_USER_ID",
			Message: "Invalid user ID format",
		})
		return
	}

	journeyService := service.Journey()
	result, err := journeyService.CompleteJourney(ctx, userID, journeyID)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}

// GetJourneyAlerts 查询行程提醒执行状态
// GET /v1/journeys/:journey_id/alerts
func GetJourneyAlerts(ctx context.Context, c *app.RequestContext) {
	journeyIDStr := c.Param("journey_id")
	journeyID, err := strconv.ParseInt(journeyIDStr, 10, 64)
	if err != nil {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_JOURNEY_ID",
			Message: "Invalid journey ID format",
		})
		return
	}

	userIDStr, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_USER_ID",
			Message: "Invalid user ID format",
		})
		return
	}

	journeyService := service.Journey()
	result, err := journeyService.GetJourneyAlerts(ctx, userID, journeyID)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}


// // AckJourneyAlert 确认已知晓行程超时提醒
// // POST /v1/journeys/:journey_id/ack-alert
// func AckJourneyAlert(ctx context.Context, c *app.RequestContext) {
// 		journeyIDStr := c.Param("journey_id")
// 		journeyID, err := strconv.ParseInt(journeyIDStr, 10, 64)
// 		if err != nil {
// 			response.Error(ctx, c, errors.Definition{
// 				Code: "INVALID_JOURNEY_ID",
// 				Message: "Invalid journey ID format",
// 			})
// 			return
// 		}

// 		userIDStr, ok := middleware.GetUserID(ctx, c)
// 	if !ok {
// 		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
// 		return
// 	}

// 	userID, err := strconv.ParseInt(userIDStr, 10, 64)
// 	if err != nil {
// 		response.Error(ctx, c, errors.Definition{
// 			Code:    "INVALID_USER_ID",
// 			Message: "Invalid user ID format",
// 		})
// 		return
// 	}

// 	journeyService := service.Journey()
// 	if err := journeyService.AckJourneyAlert(ctx, userID, journeyID); err != nil {
// 		response.Error(ctx, c, err)
// 		return
// 	}

// 	response.NoContent(ctx, c)
// }

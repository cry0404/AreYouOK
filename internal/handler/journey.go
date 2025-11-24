package handler

import (
	"AreYouOK/internal/middleware"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/service"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/response"
	"context"
	"fmt"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
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
	// TODO: 实现创建行程逻辑
}

// GetJourneyDetail 查看行程详情
// GET /v1/journeys/:journey_id
func GetJourneyDetail(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现获取行程详情逻辑
}

// UpdateJourney 更新行程
// PATCH /v1/journeys/:journey_id
func UpdateJourney(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现更新行程逻辑
}

// CompleteJourney 归来打卡，标记行程结束
// POST /v1/journeys/:journey_id/complete
func CompleteJourney(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现完成行程逻辑
}

// AckJourneyAlert 确认已知晓行程超时提醒
// POST /v1/journeys/:journey_id/ack-alert
func AckJourneyAlert(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现确认提醒逻辑
}

// GetJourneyAlerts 查询行程提醒执行状态
// GET /v1/journeys/:journey_id/alerts
func GetJourneyAlerts(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现获取提醒状态逻辑
}

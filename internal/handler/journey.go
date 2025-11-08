package handler

import (
	"AreYouOK/internal/model"
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// ListJourneys 查询行程列表。
func ListJourneys(ctx context.Context, c *app.RequestContext) {
	var query model.JourneyQuery
	if err := c.Bind(&query); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_QUERY", "Invalid query", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// CreateJourney 创建行程报备。
func CreateJourney(ctx context.Context, c *app.RequestContext) {
	var req model.CreateJourneyRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PAYLOAD", "Invalid payload", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// GetJourney 查询行程详情。
func GetJourney(ctx context.Context, c *app.RequestContext) {
	journeyID := c.Param("journey_id")
	if journeyID == "" {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PATH", "Invalid path parameter", nil))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// UpdateJourney 更新行程信息。
func UpdateJourney(ctx context.Context, c *app.RequestContext) {
	journeyID := c.Param("journey_id")
	if journeyID == "" {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PATH", "Invalid path parameter", nil))
		return
	}
	var req model.UpdateJourneyRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PAYLOAD", "Invalid payload", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// CompleteJourney 标记行程已归来。
func CompleteJourney(ctx context.Context, c *app.RequestContext) {
	journeyID := c.Param("journey_id")
	if journeyID == "" {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PATH", "Invalid path parameter", nil))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// AckJourneyAlert 确认行程超时提醒。
func AckJourneyAlert(ctx context.Context, c *app.RequestContext) {
	journeyID := c.Param("journey_id")
	if journeyID == "" {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PATH", "Invalid path parameter", nil))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// GetJourneyAlerts 查询行程提醒执行状态。
func GetJourneyAlerts(ctx context.Context, c *app.RequestContext) {
	journeyID := c.Param("journey_id")
	if journeyID == "" {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PATH", "Invalid path parameter", nil))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

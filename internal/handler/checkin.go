package handler

import (
	"AreYouOK/internal/model"
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// GetTodayCheckIn 查询当日打卡状态。
func GetTodayCheckIn(ctx context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// CompleteTodayCheckIn 完成当天打卡。
func CompleteTodayCheckIn(ctx context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// GetCheckInHistory 查询打卡历史。
func GetCheckInHistory(ctx context.Context, c *app.RequestContext) {
	var query model.CheckInHistoryQuery
	if err := c.Bind(&query); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_QUERY", "Invalid query", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// AckCheckInReminder 确认收到提醒。
func AckCheckInReminder(ctx context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

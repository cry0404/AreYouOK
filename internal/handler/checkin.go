package handler

import (
	"AreYouOK/internal/middleware"
	"AreYouOK/internal/service"
	"AreYouOK/pkg/response"
	"context"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"
)

// GetTodayCheckIn 查询当天打卡状态, 每次登录加载时需要
// GET /v1/check-ins/today
func GetTodayCheckIn(ctx context.Context, c *app.RequestContext) {
	userID, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
	}

	checkInService := service.CheckIn()
	result, err := checkInService.GetTodayCheckIn(ctx, userID)

	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}

// CompleteTodayCheckIn 完成当日打卡
// POST /v1/check-ins/today/complete
func CompleteTodayCheckIn(ctx context.Context, c *app.RequestContext) {
	userID, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}
	

	checkInService := service.CheckIn()
	result, err := checkInService.CompleteCheckIn(ctx, userID)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}

// GetCheckInHistory 分页查询历史打卡记录
// GET /v1/check-ins/history
func GetCheckInHistory(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现查询打卡历史逻辑
}

// // AckCheckInReminder 确认已知晓打卡提醒
// // POST /v1/check-ins/ack-reminder
// func AckCheckInReminder(ctx context.Context, c *app.RequestContext) {
// 	// TODO: 实现确认提醒逻辑
// }

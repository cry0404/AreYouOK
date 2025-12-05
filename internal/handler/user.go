package handler

import (
	"context"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"AreYouOK/internal/middleware"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/queue"
	"AreYouOK/internal/service"
	"AreYouOK/pkg/response"
)

// GetUserStatus 获取用户状态和引导进度
// GET /v1/users/me/status
func GetUserStatus(ctx context.Context, c *app.RequestContext) {
	userID, ok := middleware.GetUserID(ctx, c)

	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	userService := service.User()

	result, err := userService.GetUserStatus(ctx, userID)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}

// GetUserProfile 获取用户资料
// GET /v1/users/me
func GetUserProfile(ctx context.Context, c *app.RequestContext) {
	userID, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	userService := service.User()
	result, err := userService.GetUserProfile(ctx, userID)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}

// UpdateUserSettings 更新用户设置
// PUT /v1/users/me/settings
func UpdateUserSettings(ctx context.Context, c *app.RequestContext) {
	var req dto.UpdateUserSettingsRequest

	if err := c.BindAndValidate(&req); err != nil {
		response.BindError(ctx, c, err)
		return
	}

	userID, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	userService := service.User()
	reminderMsg, err := userService.UpdateUserSettings(ctx, userID, req)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	// 如果返回了提醒消息，在 Handler 层负责发布到队列
	if reminderMsg != nil {
		if err := queue.PublishCheckInReminder(*reminderMsg); err != nil {
			// 记录错误但不影响主流程
			zap.L().Error("Failed to publish check-in reminder message after settings update",
				zap.String("user_id", userID),
				zap.String("message_id", reminderMsg.MessageID),
				zap.Int("delay_seconds", reminderMsg.DelaySeconds),
				zap.Error(err),
			)
		} else {
			zap.L().Info("Check-in reminder message published after settings update",
				zap.String("user_id", userID),
				zap.String("message_id", reminderMsg.MessageID),
				zap.Int("delay_seconds", reminderMsg.DelaySeconds),
			)
		}
	}

	response.Success(ctx, c, nil)
}

// GetUserQuotas 获取用户额度详情
// GET /v1/users/me/quotas
func GetUserQuotas(ctx context.Context, c *app.RequestContext) {
	userID, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	userService := service.User()
	result, err := userService.GetUserQuotas(ctx, userID)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}
	response.Success(ctx, c, result)
}

// DeleteUser 软删除 User 部分
// DELETE /v1/users/me

func DeleteUserProfile(ctx context.Context, c *app.RequestContext) {
	userIDStr, ok := middleware.GetUserID(ctx, c)

	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	userService := service.User()
	if err := userService.DeleteUser(ctx, userIDStr); err != nil {
		response.Error(ctx, c, err)
		return
	}

	c.Status(204) // 这里需要前端清楚 session 和 token
}

// GetWaitListInfo 获取排队/引导信息（通过 auth_code -> open_id）
// POST /v1/users/waitlist
func GetWaitListInfo(ctx context.Context, c *app.RequestContext) {
	var req dto.WaitlistRequest
	if err := c.BindAndValidate(&req); err != nil {
		response.BindError(ctx, c, err)
		return
	}

	userService := service.User()

	result, err := userService.GetWaitListInfo(ctx, req.AuthCode, req.AlipayOpenID)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, result)
}

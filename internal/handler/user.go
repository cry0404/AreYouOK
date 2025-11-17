package handler

import (
	"context"
	"fmt"

	"github.com/cloudwego/hertz/pkg/app"

	"AreYouOK/internal/middleware"
	"AreYouOK/internal/model/dto"
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
	if err := userService.UpdateUserSettings(ctx, userID, req); err != nil {
		response.Error(ctx, c, err)
		return
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

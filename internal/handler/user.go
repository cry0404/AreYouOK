package handler

import (
	"AreYouOK/internal/model"
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// GetCurrentUser 获取当前用户资料。
func GetCurrentUser(ctx context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// UpdateUserSettings 更新用户个性化设置。
func UpdateUserSettings(ctx context.Context, c *app.RequestContext) {
	var req model.UpdateUserSettingsRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PAYLOAD", "Invalid payload", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// GetUserQuotas 查询用户额度。
func GetUserQuotas(ctx context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

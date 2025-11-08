package handler

import (
	"AreYouOK/internal/model"
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// GetOnboardingProgress 获取当前用户的引导进度。
func GetOnboardingProgress(ctx context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

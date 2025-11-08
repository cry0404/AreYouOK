package handler

import (
	"AreYouOK/internal/model"
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// ListNotificationTasks 查询通知任务列表。
func ListNotificationTasks(ctx context.Context, c *app.RequestContext) {
	var query model.NotificationTaskQuery
	if err := c.Bind(&query); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_QUERY", "Invalid query", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// GetNotificationTask 获取通知任务详情。
func GetNotificationTask(ctx context.Context, c *app.RequestContext) {
	taskID := c.Param("task_id")
	if taskID == "" {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PATH", "Invalid path parameter", nil))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// AckNotification 处理通知确认请求。
func AckNotification(ctx context.Context, c *app.RequestContext) {
	var req model.NotificationAckRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PAYLOAD", "Invalid payload", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// ListNotificationTemplates 查询通知模板列表。
func ListNotificationTemplates(ctx context.Context, c *app.RequestContext) {
	var query model.NotificationTemplateQuery
	if err := c.Bind(&query); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_QUERY", "Invalid query", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

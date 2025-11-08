package handler

import (
	"AreYouOK/internal/model"
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// ListContacts 列出当前用户的紧急联系人。
func ListContacts(ctx context.Context, c *app.RequestContext) {
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// CreateContact 新增紧急联系人。
func CreateContact(ctx context.Context, c *app.RequestContext) {
	var req model.CreateContactRequest
	if err := c.Bind(&req); err != nil {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PAYLOAD", "Invalid payload", model.ErrorDetail{"error": err.Error()}))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

// DeleteContact 删除紧急联系人。
func DeleteContact(ctx context.Context, c *app.RequestContext) {
	contactID := c.Param("contact_id")
	if contactID == "" {
		c.JSON(consts.StatusBadRequest, model.NewErrorResponse("INVALID_PATH", "Invalid path parameter", nil))
		return
	}
	c.JSON(consts.StatusNotImplemented, model.NewNotImplementedResponse())
}

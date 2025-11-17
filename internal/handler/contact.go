package handler

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"

	"AreYouOK/internal/middleware"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/service"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/response"
	"AreYouOK/utils"
)

// ListContacts 列表紧急联系人
// GET /v1/contacts
func ListContacts(ctx context.Context, c *app.RequestContext) {
	userID, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("userID not found in context"))
		return
	}

	contactService := service.Contact()
	contacts, err := contactService.ListContacts(ctx, userID)

	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.Success(ctx, c, contacts)
}

// CreateContact 新增紧急联系人
// POST /v1/contacts
func CreateContact(ctx context.Context, c *app.RequestContext) {
	var req dto.CreateContactRequest

	if err := c.BindAndValidate(&req); err != nil {
		response.BindError(ctx, c, err)
		return
	}

	if !utils.ValidatePhone(req.Phone) {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_PHONE",
			Message: "Invalid phone number format",
		})
		return
	}

	// 验证优先级范围
	if req.Priority < 1 || req.Priority > 3 {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_PRIORITY",
			Message: "Priority must be between 1 and 3",
		})
		return
	}

	userID, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	contactService := service.Contact()
	result, err := contactService.CreateContact(ctx, userID, req)
	if err != nil {
		response.Error(ctx, c, err)
		return
	}

	c.JSON(201, response.SuccessResponse{
		Data: result,
	})
}

// DeleteContact 删除紧急联系人
// DELETE /v1/contacts/:priority
func DeleteContact(ctx context.Context, c *app.RequestContext) {
	priorityStr := c.Param("priority")
	priority, err := strconv.Atoi(priorityStr)
	if err != nil {
		response.Error(ctx, c, errors.Definition{
			Code:    "INVALID_PRIORITY",
			Message: "Invalid priority format",
		})
		return
	}

	userID, ok := middleware.GetUserID(ctx, c)
	if !ok {
		response.Error(ctx, c, fmt.Errorf("user ID not found in context"))
		return
	}

	contactService := service.Contact()
	if err := contactService.DeleteContact(ctx, userID, priority); err != nil {
		response.Error(ctx, c, err)
		return
	}

	response.NoContent(ctx, c)
}

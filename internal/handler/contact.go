package handler

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

// ListContacts 列表紧急联系人
// GET /v1/contacts
func ListContacts(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现列表联系人逻辑
}

// CreateContact 新增紧急联系人
// POST /v1/contacts
func CreateContact(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现创建联系人逻辑
}

// DeleteContact 删除紧急联系人
// DELETE /v1/contacts/:priority
func DeleteContact(ctx context.Context, c *app.RequestContext) {
	// TODO: 实现删除联系人逻辑
}



package dto

import "time"

// ========== Contact 相关 DTO ==========
// 注意：根据 schema.sql，紧急联系人存储在 users.emergency_contacts JSONB 字段中，不是独立表

// ContactItem 紧急联系人项
type ContactItem struct {
	DisplayName  string    `json:"display_name"`
	Relationship string    `json:"relationship"`
	PhoneMasked  string    `json:"phone_masked"`
	Priority     int       `json:"priority"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateContactRequest 创建联系人请求
type CreateContactRequest struct {
	DisplayName  string `json:"display_name" binding:"required"`
	Relationship string `json:"relationship" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Priority     int    `json:"priority" binding:"required"`
}

// CreateContactResponse 创建联系人响应
type CreateContactResponse struct {
	DisplayName  string `json:"display_name"`
	Relationship string `json:"relationship"`
	PhoneMasked  string `json:"phone_masked"`
	Priority     int    `json:"priority"`
}

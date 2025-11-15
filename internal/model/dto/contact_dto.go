package dto

import "time"

// ========== Contact 相关 DTO ==========
// 存储在 user 中的部分，得先确定是哪个用户，根据索引

// ContactItem 紧急联系人项
type ContactItem struct {
	CreatedAt    time.Time `json:"created_at"`
	DisplayName  string    `json:"display_name"`
	Relationship string    `json:"relationship"`
	PhoneMasked  string    `json:"phone_masked"`
	Priority     int       `json:"priority"`
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

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

type UpdateContactRequest struct {
	DisplayName  string `json:"display_name,omitempty"`
	Relationship string `json:"relationship,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Priority     int    `json:"priority,omitempty"`
}

// CreateContactResponse 创建联系人响应
type CreateContactResponse struct {
	DisplayName  string `json:"display_name"`
	Relationship string `json:"relationship"`
	PhoneMasked  string `json:"phone_masked"`
	Priority     int    `json:"priority"`
}


type UpdateContactResponse struct {
	DisplayName  string `json:"display_name"`
	Relationship string `json:"relationship"`
	PhoneMasked  string `json:"phone_masked"`
	Priority     int    `json:"priority"`
}

// ReplaceContactItem 批量替换联系人时的单个联系人项
// 不包含 created_at，由后端自动生成
type ReplaceContactItem struct {
	DisplayName  string `json:"display_name" binding:"required"`
	Relationship string `json:"relationship" binding:"required"`
	Phone        string `json:"phone" binding:"required"`
	Priority     int    `json:"priority" binding:"required,min=1,max=3"`
}

// ReplaceContactsRequest 批量替换联系人请求
type ReplaceContactsRequest struct {
	Contacts []ReplaceContactItem `json:"contacts" binding:"required,min=1,max=3,dive"`
}
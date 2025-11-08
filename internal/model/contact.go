package model

// ContactItem 表示单个紧急联系人的信息。
type ContactItem struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	Relationship string `json:"relationship"`
	PhoneMasked  string `json:"phone_masked"`
	Priority     int    `json:"priority"`
	CreatedAt    string `json:"created_at"`
}

// CreateContactRequest 表示新增联系人的请求体。
type CreateContactRequest struct {
	DisplayName  string `json:"display_name"`
	Relationship string `json:"relationship"`
	Phone        string `json:"phone"`
	Priority     int    `json:"priority"`
}

package model

// Meta 表示统一响应中的元数据。
type Meta struct {
	RequestID       string `json:"request_id,omitempty"`
	NextCursor      string `json:"next_cursor,omitempty"`
	CompatibleSince string `json:"compatible_since,omitempty"`
}

// SuccessResponse 表示统一的成功响应结构。
type SuccessResponse struct {
	Data interface{} `json:"data"`
	Meta Meta        `json:"meta"`
}

// ErrorDetail 表示错误详情。
type ErrorDetail map[string]interface{}

// ErrorBody 表示错误主体。
type ErrorBody struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details ErrorDetail `json:"details,omitempty"`
}

// ErrorResponse 表示统一的错误响应结构。
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
	Meta  Meta      `json:"meta,omitempty"`
}

// NewSuccessResponse 创建成功响应。
func NewSuccessResponse(data interface{}) SuccessResponse {
	return SuccessResponse{
		Data: data,
		Meta: Meta{CompatibleSince: "v1"},
	}
}

// NewErrorResponse 创建错误响应。
func NewErrorResponse(code, message string, details ErrorDetail) ErrorResponse {
	return ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
		Meta: Meta{CompatibleSince: "v1"},
	}
}

// NewNotImplementedResponse 创建未实现的通用错误响应。
func NewNotImplementedResponse() ErrorResponse {
	return NewErrorResponse("NOT_IMPLEMENTED", "Not Implemented", nil)
}


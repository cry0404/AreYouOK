package response

import (
	"context"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"

	"AreYouOK/pkg/errors"
)

// ErrorResponse 统一的错误响应格式
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Details map[string]interface{} `json:"details,omitempty"`
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
}

// SuccessResponse 统一的成功响应格式
type SuccessResponse struct {
	Data interface{}            `json:"data"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

func errorToHTTPStatus(err error) int {
	// 检查是否是 Definition 类型
	def, ok := err.(errors.Definition)
	if !ok {
		if errMsg := err.Error(); errMsg != "" {
			return http.StatusInternalServerError
		}
		return http.StatusInternalServerError
	}

	// 根据错误码映射 HTTP 状态码
	switch def.Code {
	case "CAPTCHA_RATE_LIMITED", "VERIFICATION_SLIDER_REQUIRED":
		return http.StatusTooManyRequests // 429
	case "AUTH_CODE_INVALID", "VERIFICATION_CODE_EXPIRED",
		"VERIFICATION_CODE_INVALID", "VERIFICATION_SLIDER_FAILED",
		"INVALID_REQUEST", "INVALID_PHONE",
		"CONTACT_LIMIT_REACHED", "CONTACT_PRIORITY_CONFLICT",
		"JOURNEY_OVERLAP", "JOURNEY_NOT_MODIFIABLE",
		"NOTIFY_ACK_INVALID", "QUOTA_CHANNEL_INVALID":
		return http.StatusBadRequest // 400
	case "USER_STATUS_INVALID":
		return http.StatusForbidden // 403
	default:
		return http.StatusInternalServerError // 500
	}
}

// Error 返回错误响应
func Error(ctx context.Context, c *app.RequestContext, err error) {
	statusCode := errorToHTTPStatus(err)

	var code, message string
	var details map[string]interface{}

	if def, ok := err.(errors.Definition); ok {
		code = def.Code
		message = def.Message
	} else {
		code = "INTERNAL_ERROR"
		message = err.Error()
	}

	c.JSON(statusCode, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func ErrorWithDetails(ctx context.Context, c *app.RequestContext, err error, details map[string]interface{}) {
	statusCode := errorToHTTPStatus(err)

	var code, message string
	if def, ok := err.(errors.Definition); ok {
		code = def.Code
		message = def.Message
	} else {
		code = "INTERNAL_ERROR"
		message = err.Error()
	}

	c.JSON(statusCode, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func Success(ctx context.Context, c *app.RequestContext, data interface{}) {
	c.JSON(http.StatusOK, SuccessResponse{
		Data: data,
	})
}

func SuccessWithMeta(ctx context.Context, c *app.RequestContext, data interface{}, meta map[string]interface{}) {
	c.JSON(http.StatusOK, SuccessResponse{
		Data: data,
		Meta: meta,
	})
}

func BindError(ctx context.Context, c *app.RequestContext, err error) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error: ErrorDetail{
			Code:    "INVALID_REQUEST",
			Message: err.Error(),
		},
	})
}

// NoContent 返回 204 No Content（用于 DELETE 等操作）
func NoContent(ctx context.Context, c *app.RequestContext) {
	c.Status(http.StatusNoContent)
}

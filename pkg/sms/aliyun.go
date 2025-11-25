package sms

import (
	
	"encoding/json"
	"fmt"
	"strconv"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	
	"github.com/alibabacloud-go/tea/tea"
	credential "github.com/aliyun/credentials-go/credentials"
	
)

type AliyunClient struct {
	client *openapi.Client
}

// isNonRetryableError 判断是否为不可重试的错误
// 这些错误通常是配置错误、参数错误等，重试不会改变结果
func isNonRetryableError(code string) bool {
	nonRetryableCodes := []string{
		"isv.SMS_TEMPLATE_ILLEGAL",        // 模板内容与参数不匹配
		"isv.INVALID_PARAMETERS",          // 参数错误
		"isv.BUSINESS_LIMIT_CONTROL",      // 业务限流（需要人工处理）
		"isv.MOBILE_NUMBER_ILLEGAL",       // 手机号格式错误
		"isv.SMS_SIGNATURE_ILLEGAL",       // 签名不合法
		"isv.INVALID_JSON_PARAM",          // JSON 参数格式错误
		"isv.TEMPLATE_MISSING_PARAMETERS", // 模板缺少参数
		"isv.TEMPLATE_PARAMS_ILLEGAL",     // 模板参数不合法
	}

	for _, nonRetryableCode := range nonRetryableCodes {
		if code == nonRetryableCode {
			return true
		}
	}
	return false
}

// NewAliyunClient 创建阿里云 SMS 客户端
// 使用环境变量自动获取 AccessKey 和 SecretKey
// 需要设置环境变量：ALIBABA_CLOUD_ACCESS_KEY_ID 和 ALIBABA_CLOUD_ACCESS_KEY_SECRET
func NewAliyunClient() (*AliyunClient, error) {
	// 使用环境变量或配置文件自动获取凭据（推荐方式）
	// 工程代码建议使用更安全的无AK方式，凭据配置方式请参见：
	// https://help.aliyun.com/document_detail/378661.html
	cred, err := credential.NewCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create aliyun credential: %w", err)
	}

	// 创建 OpenAPI 客户端
	openapiConfig := &openapi.Config{
		Credential: cred,
		Endpoint:   tea.String("dysmsapi.aliyuncs.com"),
	}

	client, err := openapi.NewClient(openapiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create aliyun client: %w", err)
	}

	return &AliyunClient{
		client: client,
	}, nil
}

func (c *AliyunClient) createApiInfo(action string) *openapi.Params {
	return &openapi.Params{
		Action:      tea.String(action),
		Version:     tea.String("2017-05-25"),
		Protocol:    tea.String("HTTPS"),
		Method:      tea.String("POST"),
		AuthType:    tea.String("AK"),
		Style:       tea.String("RPC"),
		Pathname:    tea.String("/"),
		ReqBodyType: tea.String("json"),
		BodyType:    tea.String("json"),
	}
}

// 在文件顶部添加辅助函数
func parseStatusCode(v interface{}) (int, error) {
	switch val := v.(type) {
	case int:
		return val, nil
	case int64:
		return int(val), nil
	case float64:
		return int(val), nil
	case json.Number:
		code, err := val.Int64()
		if err != nil {
			return 0, fmt.Errorf("invalid statusCode value: %w", err)
		}
		return int(code), nil
	case string:
		code, err := strconv.Atoi(val)
		if err != nil {
			return 0, fmt.Errorf("invalid statusCode value: %w", err)
		}
		return code, nil
	default:
		return 0, fmt.Errorf("invalid statusCode type: %T", v)
	}
}


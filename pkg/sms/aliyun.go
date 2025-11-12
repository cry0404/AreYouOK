package sms

import (
	"context"
	"encoding/json"
	"fmt"

	"AreYouOK/pkg/logger"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	openapiutil "github.com/alibabacloud-go/openapi-util/service"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	credential "github.com/aliyun/credentials-go/credentials"
	"go.uber.org/zap"
)


type AliyunClient struct {
	client *openapi.Client
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

// createApiInfo 创建 API 信息
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

// SendSingle 发送单条短信
func (c *AliyunClient) SendSingle(ctx context.Context, phone, signName, templateCode, templateParam string) error {

	if signName == "" {
		return fmt.Errorf("signName is required")
	}
	if templateCode == "" {
		return fmt.Errorf("templateCode is required")
	}

	params := c.createApiInfo("SendSms")

	queries := map[string]interface{}{
		"PhoneNumbers":  tea.String(phone),
		"SignName":      tea.String(signName),
		"TemplateCode":  tea.String(templateCode),
		"TemplateParam": tea.String(templateParam),
	}

	runtime := &util.RuntimeOptions{}
	request := &openapi.OpenApiRequest{
		Query: openapiutil.Query(queries),
	}

	resp, err := c.client.CallApi(params, request, runtime)
	if err != nil {
		logger.Logger.Error("Failed to send SMS",
			zap.String("phone", phone),
			zap.String("template", templateCode),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send SMS: %w", err)
	}


	if resp["statusCode"] != nil {
		statusCode := resp["statusCode"].(int)
		if statusCode != 200 {
			body := resp["body"]
			logger.Logger.Error("SMS API returned error",
				zap.Int("statusCode", statusCode),
				zap.Any("body", body),
			)
			return fmt.Errorf("SMS API error: statusCode=%d", statusCode)
		}
	}


	if resp["body"] != nil {
		bodyBytes, _ := json.Marshal(resp["body"])
		var bodyMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
			if code, ok := bodyMap["Code"].(string); ok && code != "OK" {
				message := ""
				if msg, ok := bodyMap["Message"].(string); ok {
					message = msg
				}
				logger.Logger.Error("SMS send failed",
					zap.String("code", code),
					zap.String("message", message),
				)
				return fmt.Errorf("SMS send failed: %s - %s", code, message)
			}
		}
	}

	logger.Logger.Info("SMS sent successfully",
		zap.String("phone", phone),
		zap.String("template", templateCode),
	)

	return nil
}

// SendBatch 批量发送短信
// 根据阿里云 SendBatchSms API 文档：
// - PhoneNumberJson: JSON 数组格式的手机号列表，如 ["13800138000","13800138001"]
// - SignNameJson: JSON 数组格式的签名列表，每个手机号对应一个签名（通常使用相同签名）
// - TemplateCode: 模板代码
// - TemplateParamJson: JSON 数组格式的模板参数列表，每个元素对应一个手机号的参数
func (c *AliyunClient) SendBatch(ctx context.Context, phones []string, signName, templateCode string, templateParams []string) error {
	// 参数校验
	if signName == "" {
		return fmt.Errorf("signName is required")
	}
	if templateCode == "" {
		return fmt.Errorf("templateCode is required")
	}
	if len(phones) == 0 {
		return fmt.Errorf("phones list is empty")
	}

	if len(templateParams) != len(phones) {
		return fmt.Errorf("templateParams count (%d) must match phones count (%d)", len(templateParams), len(phones))
	}

	// 构建 PhoneNumberJson: ["13800138000","13800138001",...]
	phoneNumbersJSON, err := json.Marshal(phones)
	if err != nil {
		return fmt.Errorf("failed to marshal phone numbers: %w", err)
	}

	// 构建 SignNameJson: 所有手机号使用相同的签名
	signNames := make([]string, len(phones))
	for i := range signNames {
		signNames[i] = signName
	}
	signNamesJSON, err := json.Marshal(signNames)
	if err != nil {
		return fmt.Errorf("failed to marshal sign names: %w", err)
	}

	// 构建 TemplateParamJson: ["{\"code\":\"1234\"}","{\"code\":\"5678\"}",...]
	// 注意：templateParams 已经是 JSON 字符串，需要作为字符串数组的元素
	templateParamsArray := make([]string, len(templateParams))
	for i, param := range templateParams {
		// 如果参数不是有效的 JSON，尝试包装它
		var testJSON interface{}
		if err := json.Unmarshal([]byte(param), &testJSON); err != nil {
			// 如果不是 JSON，包装成 JSON 对象
			templateParamsArray[i] = fmt.Sprintf(`{"param":"%s"}`, param)
		} else {
			// 已经是有效的 JSON 字符串，直接使用
			templateParamsArray[i] = param
		}
	}
	templateParamsJSON, err := json.Marshal(templateParamsArray)
	if err != nil {
		return fmt.Errorf("failed to marshal template params: %w", err)
	}

	params := c.createApiInfo("SendBatchSms")

	queries := map[string]interface{}{
		"PhoneNumberJson":   tea.String(string(phoneNumbersJSON)),
		"SignNameJson":      tea.String(string(signNamesJSON)),
		"TemplateCode":      tea.String(templateCode),
		"TemplateParamJson": tea.String(string(templateParamsJSON)),
	}

	runtime := &util.RuntimeOptions{}
	request := &openapi.OpenApiRequest{
		Query: openapiutil.Query(queries),
	}

	resp, err := c.client.CallApi(params, request, runtime)
	if err != nil {
		logger.Logger.Error("Failed to send batch SMS",
			zap.Int("count", len(phones)),
			zap.String("template", templateCode),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send batch SMS: %w", err)
	}

	// 检查响应状态
	if resp["statusCode"] != nil {
		statusCode := resp["statusCode"].(int)
		if statusCode != 200 {
			body := resp["body"]
			logger.Logger.Error("SMS API returned error",
				zap.Int("statusCode", statusCode),
				zap.Any("body", body),
			)
			return fmt.Errorf("SMS API error: statusCode=%d", statusCode)
		}
	}

	// 解析响应体
	if resp["body"] != nil {
		bodyBytes, _ := json.Marshal(resp["body"])
		var bodyMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &bodyMap); err == nil {
			if code, ok := bodyMap["Code"].(string); ok && code != "OK" {
				message := ""
				if msg, ok := bodyMap["Message"].(string); ok {
					message = msg
				}
				logger.Logger.Error("Batch SMS send failed",
					zap.String("code", code),
					zap.String("message", message),
				)
				return fmt.Errorf("batch SMS send failed: %s - %s", code, message)
			}
		}
	}

	logger.Logger.Info("Batch SMS sent successfully",
		zap.Int("count", len(phones)),
		zap.String("template", templateCode),
	)

	return nil
}

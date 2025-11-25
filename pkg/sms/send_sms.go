package sms

import (
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	openapiutil "github.com/alibabacloud-go/openapi-util/service"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"go.uber.org/zap"

	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
)

// SendSingle 发送单条短信
func (c *AliyunClient) SendSingle(ctx context.Context, phone, signName, templateCode, templateParam string) error {
	if signName == "" {
		return errors.ErrSignNameRequired
	}
	if templateCode == "" {
		return errors.ErrTemplateCodeRequired
	}

	params := c.createApiInfo("SendSms")

	queries := map[string]interface{}{
		"PhoneNumbers":  tea.String(phone),
		"SignName":      tea.String(signName),
		"TemplateCode":  tea.String(templateCode),
		"TemplateParam": tea.String(templateParam),
	}

	// 打印实际发送给阿里云的参数（用于调试）
	logger.Logger.Info("Sending SMS to Aliyun",
		zap.String("phone", phone),
		zap.String("sign_name", signName),
		zap.String("template_code", templateCode),
		zap.String("template_param", templateParam),
		zap.String("template_param_escaped", fmt.Sprintf("%q", templateParam)),
		zap.Int("template_param_length", len(templateParam)),
	)

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
		statusCode, err := parseStatusCode(resp["statusCode"])
		if err != nil {
			return err
		}
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
		bodyBytes, err := json.Marshal(resp["body"])
		if err != nil {
			return fmt.Errorf("failed to marshal response body: %w", err)
		}
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

				// 识别不可重试的错误（配置错误、参数错误等）
				if isNonRetryableError(code) {
					return errors.NewNonRetryableError(code, message, "SMS template or parameter configuration error")
				}

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
		return errors.ErrSignNameRequired
	}
	if templateCode == "" {
		return errors.ErrTemplateCodeRequired
	}
	if len(phones) == 0 {
		return errors.ErrPhonesListEmpty
	}

	if len(templateParams) != len(phones) {
		return fmt.Errorf("%s: %d vs %d", errors.ErrTemplateParamsMismatch.Message, len(templateParams), len(phones))
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
		if jsonErr := json.Unmarshal([]byte(param), &testJSON); jsonErr != nil {
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
		statusCode, err := parseStatusCode(resp["statusCode"])
		if err != nil {
			return err
		}
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
		bodyBytes, err := json.Marshal(resp["body"])
		if err != nil {
			return fmt.Errorf("failed to marshal response body: %w", err)
		}
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

				// 识别不可重试的错误（配置错误、参数错误等）
				if isNonRetryableError(code) {
					return errors.NewNonRetryableError(code, message, "SMS template or parameter configuration error")
				}

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

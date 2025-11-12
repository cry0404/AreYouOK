package sms

import (
	"context"
	"encoding/json"
	"fmt"

	"AreYouOK/config"
)

// SendBatchCaptchaSMS 批量发送验证码短信
// phones: 手机号列表
// codes: 验证码列表（与手机号一一对应）
// scene: 场景（register/login），用于选择不同的模板
func SendBatchCaptchaSMS(ctx context.Context, phones []string, codes []string, scene string) error {
	if len(phones) != len(codes) {
		return fmt.Errorf("phones and codes count mismatch")
	}

	if len(phones) == 0 {
		return fmt.Errorf("phones list is empty")
	}

	cfg := config.Cfg
	signName := cfg.SMSSignName
	templateCode := cfg.SMSTemplateCode

	// 为每个手机号构建模板参数
	templateParams := make([]string, len(phones))
	for i, code := range codes {
		param := map[string]string{
			"code": code,
		}
		paramJSON, err := json.Marshal(param)
		if err != nil {
			return fmt.Errorf("failed to marshal template param for phone %s: %w", phones[i], err)
		}
		templateParams[i] = string(paramJSON)
	}

	// 使用批量发送接口
	return SendBatch(ctx, phones, signName, templateCode, templateParams)
}

// SendBatchCheckInReminder 批量发送打卡提醒短信（用于每日打卡场景）
// phones: 手机号列表
// reminderMessage: 提醒消息内容（可选，如果模板需要）
func SendBatchCheckInReminder(ctx context.Context, phones []string, reminderMessage string) error {
	if len(phones) == 0 {
		return fmt.Errorf("phones list is empty")
	}

	cfg := config.Cfg
	signName := cfg.SMSSignName
	templateCode := cfg.SMSTemplateCode

	// 为每个手机号构建模板参数（打卡提醒通常使用相同的消息）
	templateParams := make([]string, len(phones))
	param := map[string]string{}
	if reminderMessage != "" {
		param["message"] = reminderMessage
	}
	paramJSON, err := json.Marshal(param)
	if err != nil {
		return fmt.Errorf("failed to marshal template param: %w", err)
	}

	// 所有手机号使用相同的模板参数
	for i := range templateParams {
		templateParams[i] = string(paramJSON)
	}

	// 使用批量发送接口
	return SendBatch(ctx, phones, signName, templateCode, templateParams)
}

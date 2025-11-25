package sms

import (
	"context"
	"encoding/json"
	"fmt"

	"AreYouOK/config"
)

// SendCaptchaSMS 发送验证码短信
// phone: 手机号
// code: 验证码

func SendCaptchaSMS(ctx context.Context, phone, code string) error {
	cfg := config.Cfg

	//不再区分对应模板了，统一发送验证码
	signName := cfg.SMSSignName
	templateCode := cfg.SMSTemplateCode // 暂时先默认使用配置的模板

	// 构建模板参数
	templateParam := map[string]string{
		"code": code,
	}
	paramJSON, err := json.Marshal(templateParam)
	if err != nil {
		return fmt.Errorf("failed to marshal template param: %w", err)
	}

	return SendSingle(ctx, phone, signName, templateCode, string(paramJSON))
}

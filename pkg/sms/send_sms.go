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
// scene: 场景（register/login），用于选择不同的模板
func SendCaptchaSMS(ctx context.Context, phone, code, scene string) error {
	cfg := config.Cfg

	// 根据 scene 选择模板代码和签名
	// 具体届时配置 login 和 register 的模板即可
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

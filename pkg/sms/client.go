package sms

import (
	"context"
	"fmt"
	"sync"

	"AreYouOK/config"
	"AreYouOK/pkg/logger"

	"go.uber.org/zap"
)

// Client SMS 客户端接口
type Client interface {
	// SendSingle 发送单条短信
	// phone: 手机号
	// signName: 短信签名名称
	// templateCode: 模板代码
	// templateParam: 模板参数（JSON 字符串）
	SendSingle(ctx context.Context, phone, signName, templateCode, templateParam string) error

	// SendBatch 批量发送短信
	// phones: 手机号列表
	// signName: 短信签名名称（所有手机号使用相同签名）
	// templateCode: 模板代码
	// templateParams: 模板参数列表（JSON 字符串数组），每个元素对应一个手机号的参数
	// 如果所有手机号使用相同的模板参数，可以传入相同长度的数组，每个元素相同
	SendBatch(ctx context.Context, phones []string, signName, templateCode string, templateParams []string) error
}

var (
	smsClient Client
	smsOnce   sync.Once
	smsErr    error
)

// Init 初始化 SMS 客户端
func Init() error {
	smsOnce.Do(func() {
		cfg := config.Cfg

		switch cfg.SMSProvider {
		case "aliyun":
			smsClient, smsErr = NewAliyunClient()
		case "tencent":
			// TODO: 实现腾讯云 SMS 客户端，不打算实现了
			smsErr = fmt.Errorf("tencent SMS provider not implemented yet")
		default:
			smsErr = fmt.Errorf("unsupported SMS provider: %s", cfg.SMSProvider)
		}

		if smsErr != nil {
			logger.Logger.Error("Failed to initialize SMS client", zap.Error(smsErr))
			return
		}

		logger.Logger.Info("SMS client initialized successfully",
			zap.String("provider", cfg.SMSProvider),
		)
	})

	return smsErr
}

func GetClient() Client {
	if smsClient == nil {
		panic("SMS client not initialized, call sms.Init() first")
	}
	return smsClient
}

func SendSingle(ctx context.Context, phone, signName, templateCode, templateParam string) error {
	return GetClient().SendSingle(ctx, phone, signName, templateCode, templateParam)
}

func SendBatch(ctx context.Context, phones []string, signName, templateCode string, templateParams []string) error {
	return GetClient().SendBatch(ctx, phones, signName, templateCode, templateParams)
}

package slider

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"AreYouOK/config"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
)

// Client 滑块验证客户端接口
type Client interface {
	// Verify 验证滑块 token
	// captchaVerifyToken: 前端滑块组件返回的验证 token

	// sceneid: 对应的业务场景
	Verify(ctx context.Context, captchaVerifyParam,  sceneid string) (bool, error)
}

var (
	sliderClient Client
	sliderOnce   sync.Once
	sliderErr    error
)

// Init 初始化滑块验证客户端
func Init() error {
	sliderOnce.Do(func() {
		cfg := config.Cfg

		switch cfg.CaptchaProvider {
		case "aliyun":
			sliderClient, sliderErr = NewAliyunClient()
		case "none":
			sliderClient = &MockClient{}
			sliderErr = nil
		default:
			sliderErr = fmt.Errorf("%w: %s", errors.ErrUnsupportedCaptchaProvider, cfg.CaptchaProvider)
		}

		if sliderErr != nil {
			logger.Logger.Error("Failed to initialize slider client", zap.Error(sliderErr))
			return
		}

		logger.Logger.Info("Slider client initialized successfully",
			zap.String("provider", cfg.CaptchaProvider),
		)
	})

	return sliderErr
}

func GetClient() Client {
	if sliderClient == nil {
		panic("Slider client not initialized, call slider.Init() first")
	}
	return sliderClient
}

func Verify(ctx context.Context, captchaVerifyParam, sceneid string) (bool, error) {
	return GetClient().Verify(ctx, captchaVerifyParam,  sceneid)
}

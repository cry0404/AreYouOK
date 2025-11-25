package middleware

// TODO: 统一的关闭逻辑
// 事后看似乎不需要关闭
import (
	"go.uber.org/zap"

	"AreYouOK/pkg/logger"
)

// 或者在导入包的时候直接默认 init 初始化？
// Init 初始化所有中间件
func Init() error {
	if err := initAuthMiddleware(); err != nil {
		logger.Logger.Error("Failed to initialize auth middleware", zap.Error(err))
		return err
	}

	logger.Logger.Info("All middlewares initialized successfully")
	return nil
}

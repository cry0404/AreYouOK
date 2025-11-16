package slider

import (
	"context"

	"AreYouOK/pkg/errors"
)

// MockClient 开发环境使用的 Mock 客户端

type MockClient struct{}

// Verify Mock 验证方法
func (m *MockClient) Verify(ctx context.Context, captchaVerifyToken, remoteIp, scene string) (bool, error) {
	if captchaVerifyToken == "" {
		return false, errors.ErrCaptchaTokenRequired
	}

	// Mock: 只要 token 不为空就认为验证通过
	return true, nil
}

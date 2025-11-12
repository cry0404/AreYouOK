package slider

import (
	"context"
	"fmt"
)

// MockClient 开发环境使用的 Mock 客户端
// 用于测试，不进行真实的验证
type MockClient struct{}

// Verify Mock 验证方法
// 在开发环境中，如果 captchaVerifyToken 不为空，则返回成功
func (m *MockClient) Verify(ctx context.Context, captchaVerifyToken, remoteIp, scene string) (bool, error) {
	if captchaVerifyToken == "" {
		return false, fmt.Errorf("captchaVerifyToken is required")
	}

	// Mock: 只要 token 不为空就认为验证通过
	return true, nil
}


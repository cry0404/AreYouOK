package sms

import (
	"context"
	"errors"
	"sync"
)

type MockCall struct {
	Phone         string
	SignName      string
	TemplateCode  string
	TemplateParam string
}

// MockClient 可配置的短信客户端 mock，实现 Client 接口
type MockClient struct {
	mu    sync.Mutex
	Calls []MockCall

	// FailNext 置为 true 时，下一次调用返回 mock 错误并自动复位
	FailNext bool
}


func NewMockClient() *MockClient {
	return &MockClient{
		Calls: make([]MockCall, 0),
	}
}

func (m *MockClient) SendSingle(ctx context.Context, phone, signName, templateCode, templateParam string) (*SendResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, MockCall{
		Phone:         phone,
		SignName:      signName,
		TemplateCode:  templateCode,
		TemplateParam: templateParam,
	})

	if m.FailNext {
		m.FailNext = false
		return nil, errors.New("mock sms send failure")
	}

	return &SendResponse{
		MessageID:  "mock-message-id",
		StatusCode: "OK",
		Code:       "OK",
		Message:    "mock send success",
		RequestID:  "mock-request-id",
		Provider:   "mock",
		Template:   templateCode,
	}, nil
}

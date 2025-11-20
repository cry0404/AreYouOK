package service

import (
	"sync"
	"context"
)


// 基于 task_id 来实现幂等性
type NotificationService struct{}

var (
	notificationService *NotificationService
	notificationOnce    sync.Once
)

func Notification() *NotificationService {
	notificationOnce.Do(func() {
		notificationService = &NotificationService{}
	})
	return notificationService
}


func (s *NotificationService) SendSMS(ctx context.Context, taskID int64, userID int64, phoneHash string, payload map[string]interface{}) error {
	return nil
}

func (s *NotificationService) SendVoice(ctx context.Context, taskID int64, userID int64, phoneHash string, payload map[string]interface{}) error {
	return nil
}
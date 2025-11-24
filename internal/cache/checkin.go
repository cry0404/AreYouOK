package cache

import (
	"context"
	"fmt"
	"time"

	"AreYouOK/storage/redis"
)

const (
	// 用于存储打卡状态，帮助消息队列中消息查询时快速了解
	checkinScheduledPrefix = "ayok:checkin:scheduled"
	messageProcessedPrefix = "ayok:message:processed"

	// TTL 设置
	scheduledTTL = 24 * time.Hour // 打卡投放标记保留 24 小时
	processedTTL = 48 * time.Hour // 消息处理标记保留 48 小时
)

// IsCheckinScheduled 检查指定日期的打卡是否已投放
func IsCheckinScheduled(ctx context.Context, date string, userID int64) (bool, error) {
	key := redis.Key(checkinScheduledPrefix, date, fmt.Sprintf("%d", userID))
	result, err := redis.Client().Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check checkin scheduled status: %w", err)
	}
	return result > 0, nil
}

// MarkCheckinScheduled 标记指定日期的打卡已投放
func MarkCheckinScheduled(ctx context.Context, date string, userID int64) error {
	key := redis.Key(checkinScheduledPrefix, date, fmt.Sprintf("%d", userID))
	return redis.Client().Set(ctx, key, "1", scheduledTTL).Err()
}

// TryMarkMessageProcessing 尝试原子性地标记消息正在处理（使用 SETNX）
// 返回 true 表示成功标记（首次处理），false 表示已被标记（重复消息或正在处理）
func TryMarkMessageProcessing(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
	key := redis.Key(messageProcessedPrefix, messageID)
	if ttl <= 0 {
		ttl = processedTTL
	}

	// SETNX：如果 key 不存在则设置，返回 true；如果已存在则返回 false
	result, err := redis.Client().SetNX(ctx, key, "processing", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to mark message as processing: %w", err)
	}
	return result, nil
}

// UnmarkMessageProcessing 取消消息处理标记（处理失败时调用，允许重试）
func UnmarkMessageProcessing(ctx context.Context, messageID string) error {
	key := redis.Key(messageProcessedPrefix, messageID)
	return redis.Client().Del(ctx, key).Err()
}

// MarkMessageProcessed 标记消息已处理（处理成功时调用，延长 TTL）
func MarkMessageProcessed(ctx context.Context, messageID string, ttl time.Duration) error {
	key := redis.Key(messageProcessedPrefix, messageID)
	if ttl <= 0 {
		ttl = processedTTL
	}
	// 更新值为 "completed"，并延长 TTL
	return redis.Client().Set(ctx, key, "completed", ttl).Err()
}

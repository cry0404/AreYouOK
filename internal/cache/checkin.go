package cache

import (
	"context"
	"fmt"
	"time"

	"AreYouOK/storage/redis"
)

const (
	// 用于存储打卡状态，帮助消息队列中消息查询时快速了解，更新时方便跳过
	checkinScheduledPrefix = "checkin:scheduled"
	checkinReminderPrefix  = "checkin:reminder:scheduled"
	checkinTimeoutPrefix   = "checkin:timeout:scheduled"
	messageProcessedPrefix = "message:processed"

	scheduledTTL = 24 * time.Hour 
	processedTTL = 48 * time.Hour 
)

// IsCheckinScheduled 检查指定日期的打卡是否已投放（reminder 和 timeout 都已发布）
// 为了向后兼容，保留此函数，但实际检查 reminder 和 timeout 是否都已调度
func IsCheckinScheduled(ctx context.Context, date string, userID int64) (bool, error) {
	reminderScheduled, err := IsReminderScheduled(ctx, date, userID)
	if err != nil {
		return false, err
	}

	timeoutScheduled, err := IsTimeoutScheduled(ctx, date, userID)
	if err != nil {
		return false, err
	}

	// 只有当 reminder 和 timeout 都已调度时，才返回 true
	return reminderScheduled && timeoutScheduled, nil
}

// MarkCheckinScheduled 标记指定日期的打卡已投放（同时标记 reminder 和 timeout）
// 为了向后兼容，保留此函数，但实际会同时标记 reminder 和 timeout
func MarkCheckinScheduled(ctx context.Context, date string, userID int64) error {
	// 同时标记 reminder 和 timeout
	if err := MarkReminderScheduled(ctx, date, userID); err != nil {
		return err
	}
	return MarkTimeoutScheduled(ctx, date, userID)
}

// UnmarkCheckinScheduled 清除指定日期的打卡已投放标记（用于设置更新或重试）
// 需要同时删除 legacy 标记（checkin:scheduled）以及新的 reminder/timeout 标记
func UnmarkCheckinScheduled(ctx context.Context, date string, userID int64) error {
	keys := []string{
		redis.Key(checkinScheduledPrefix, date, fmt.Sprintf("%d", userID)), // 兼容旧逻辑
		redis.Key(checkinReminderPrefix, date, fmt.Sprintf("%d", userID)),
		redis.Key(checkinTimeoutPrefix, date, fmt.Sprintf("%d", userID)),
	}

	if err := redis.Client().Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to unmark checkin scheduled: %w", err)
	}
	return nil
}

// IsReminderScheduled 检查指定日期的提醒消息是否已投放
func IsReminderScheduled(ctx context.Context, date string, userID int64) (bool, error) {
	key := redis.Key(checkinReminderPrefix, date, fmt.Sprintf("%d", userID))
	result, err := redis.Client().Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check reminder scheduled status: %w", err)
	}
	return result > 0, nil
}

// MarkReminderScheduled 标记指定日期的提醒消息已投放
func MarkReminderScheduled(ctx context.Context, date string, userID int64) error {
	key := redis.Key(checkinReminderPrefix, date, fmt.Sprintf("%d", userID))
	return redis.Client().Set(ctx, key, "1", scheduledTTL).Err()
}

// IsTimeoutScheduled 检查指定日期的超时消息是否已投放
func IsTimeoutScheduled(ctx context.Context, date string, userID int64) (bool, error) {
	key := redis.Key(checkinTimeoutPrefix, date, fmt.Sprintf("%d", userID))
	result, err := redis.Client().Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check timeout scheduled status: %w", err)
	}
	return result > 0, nil
}

// MarkTimeoutScheduled 标记指定日期的超时消息已投放
func MarkTimeoutScheduled(ctx context.Context, date string, userID int64) error {
	key := redis.Key(checkinTimeoutPrefix, date, fmt.Sprintf("%d", userID))
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

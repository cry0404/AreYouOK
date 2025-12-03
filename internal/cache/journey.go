package cache

import (
	"context"
	"fmt"
	"time"

	"AreYouOK/storage/redis"
)

const (
	// 行程防止重发，避免对应的 worker 争抢
	journeyReminderScheduledPrefix = "journey:reminder:scheduled"
	journeyTimeoutScheduledPrefix  = "journey:timeout:scheduled"


	journeyScheduledTTL = 24 * time.Hour
)

// IsJourneyReminderScheduled 检查行程提醒消息是否已投放
func IsJourneyReminderScheduled(ctx context.Context, journeyID int64) (bool, error) {
	key := redis.Key(journeyReminderScheduledPrefix, fmt.Sprintf("%d", journeyID))
	result, err := redis.Client().Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check journey reminder scheduled status: %w", err)
	}
	return result > 0, nil
}

// MarkJourneyReminderScheduled 标记行程提醒消息已投放
func MarkJourneyReminderScheduled(ctx context.Context, journeyID int64) error {
	key := redis.Key(journeyReminderScheduledPrefix, fmt.Sprintf("%d", journeyID))
	return redis.Client().Set(ctx, key, "1", journeyScheduledTTL).Err()
}

// IsJourneyTimeoutScheduled 检查行程超时消息是否已投放
func IsJourneyTimeoutScheduled(ctx context.Context, journeyID int64) (bool, error) {
	key := redis.Key(journeyTimeoutScheduledPrefix, fmt.Sprintf("%d", journeyID))
	result, err := redis.Client().Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check journey timeout scheduled status: %w", err)
	}
	return result > 0, nil
}

// MarkJourneyTimeoutScheduled 标记行程超时消息已投放
func MarkJourneyTimeoutScheduled(ctx context.Context, journeyID int64) error {
	key := redis.Key(journeyTimeoutScheduledPrefix, fmt.Sprintf("%d", journeyID))
	return redis.Client().Set(ctx, key, "1", journeyScheduledTTL).Err()
}

// UnmarkJourneyScheduled 清除行程调度标记（用于行程取消或重试）
func UnmarkJourneyScheduled(ctx context.Context, journeyID int64) error {
	keys := []string{
		redis.Key(journeyReminderScheduledPrefix, fmt.Sprintf("%d", journeyID)),
		redis.Key(journeyTimeoutScheduledPrefix, fmt.Sprintf("%d", journeyID)),
	}

	if err := redis.Client().Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("failed to unmark journey scheduled: %w", err)
	}
	return nil
}

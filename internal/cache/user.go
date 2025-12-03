package cache

import (
	"AreYouOK/pkg/logger"
	"AreYouOK/storage/redis"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"
)

// 用于缓存 user 部分的设置，避免设置更改后频繁去查数据库，消息队列查询时更新

const (
	userSettingsPrefix = "user:settings"
	userSettingsTTL    = 24 * time.Hour
)

type TimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// UserSettingsCache 用户设置缓存结构
type UserSettingsCache struct {
	DailyCheckInEnabled    bool   `json:"daily_check_in_enabled"`
	DailyCheckInRemindAt   string `json:"daily_check_in_remind_at"`
	DailyCheckInDeadline   string `json:"daily_check_in_deadline"`
	DailyCheckInGraceUntil string `json:"daily_check_in_grace_until"`

	// Status                 string `json:"status"`   用户状态用户无法改变
	DailyCheckInTimeRange *TimeRange `json:"daily_check_in_time_range,omitempty"` // {"start": "08:00:00", "end": "20:00:00"}

	UpdatedAt int64 `json:"updated_at"` //这里本身就是一个版本号
}

//如果更新了，在缓存中就能获取，如果没更新，直接就可以发送，所以发送前只需要检查缓存

func SetUserSettings(ctx context.Context, userID int64, settings *UserSettingsCache) error {
	key := strconv.FormatInt(userID, 10)
	cache := UserSettingsProtectedCache

	return cache.Set(ctx, key, settings)
}

// 获取用户缓存设置, 去除 userSettingCache（带空值保护）

func GetUserSettings(ctx context.Context, userID int64) (*UserSettingsCache, error) {
	key := strconv.FormatInt(userID, 10)
	var settings UserSettingsCache

	cache := UserSettingsProtectedCache
	hit, err := cache.Get(ctx, key, &settings)
	if err != nil {
		return nil, err
	}

	if hit {
		if IsEmptyValue(&settings) {
			return nil, nil // 空值命中
		}
		return &settings, nil
	}

	return nil, nil // 缓存未命中
}

// BatchGetUserSettings 批量获取用户设置缓存（带空值保护和防雪崩）

func BatchGetUserSettings(ctx context.Context, userIDs []int64) (map[int64]*UserSettingsCache, error) {
	if len(userIDs) == 0 {
		return make(map[int64]*UserSettingsCache), nil
	}

	logger.Logger.Info("BatchGetUserSettings start",
		zap.Int("user_count", len(userIDs)),
	)

	// 转换为字符串键
	keys := make([]string, len(userIDs))
	for i, userID := range userIDs {
		keys[i] = strconv.FormatInt(userID, 10)
	}

	cache := UserSettingsProtectedCache

	// 批量获取缓存（自带防雪崩随机延迟）
	result, err := cache.BatchGet(ctx, keys, func(key string) interface{} {
		return &UserSettingsCache{}
	})

	if err != nil {
		logger.Logger.Error("BatchGetUserSettings cache error", zap.Error(err))
		return nil, fmt.Errorf("failed to batch get user settings: %w", err)
	}

	logger.Logger.Info("BatchGetUserSettings cache hit summary",
		zap.Int("requested", len(keys)),
		zap.Int("hit", len(result)),
	)

	// 转换结果为正确的类型
	settingsMap := make(map[int64]*UserSettingsCache)
	for keyStr, value := range result {
		if IsEmptyValue(value) {
			// 空值，跳过
			continue
		}

		if settings, ok := value.(*UserSettingsCache); ok {
			if userID, err := strconv.ParseInt(keyStr, 10, 64); err == nil {
				settingsMap[userID] = settings
			}
		}
	}

	return settingsMap, nil
} //批量获取用户缓存，没有获取到的，就说明没有改变，可以直接发送

func BatchSetUserSettings(ctx context.Context, settingsMap map[int64]*UserSettingsCache) error {
	if len(settingsMap) == 0 {
		return nil
	}

	pipe := redis.Client().Pipeline()

	for userID, settings := range settingsMap {
		key := redis.Key(userSettingsPrefix, strconv.FormatInt(userID, 10))
		data, err := json.Marshal(settings)
		if err != nil {
			continue
		}
		pipe.Set(ctx, key, data, userSettingsTTL)
	}

	_, err := pipe.Exec(ctx)

	return err
}

// //更新设置时删除缓存
// // 失效时的用户缓存设置
// func InvalidateUserSettings(ctx context.Context, userID int64) error {
// 	key := redis.Key(userSettingsPrefix, strconv.FormatInt(userID, 10))

// 	return redis.Client().Del(ctx, key).Err()
// }

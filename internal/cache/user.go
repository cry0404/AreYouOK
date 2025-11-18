package cache

import (
	"AreYouOK/storage/redis"
	"context"
	"encoding/json"
	"fmt"
	ri "github.com/redis/go-redis/v9"
	"strconv"
	"time"
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

type UserSettingsCache struct {
	DailyCheckInEnabled    bool   `json:"daily_check_in_enabled"`
	DailyCheckInRemindAt   string `json:"daily_check_in_remind_at"`
	DailyCheckInDeadline   string `json:"daily_check_in_deadline"`
	DailyCheckInGraceUntil string `json:"daily_check_in_grace_until"`
	
	// Status                 string `json:"status"`   用户状态用户无法改变
	DailyCheckInTimeRange  *TimeRange `json:"daily_check_in_time_range,omitempty"` // {"start": "08:00:00", "end": "20:00:00"}
    
    UpdatedAt              int64  `json:"updated_at"`
}

//如果更新了，在缓存中就能获取，如果没更新，直接就可以发送，所以发送前只需要检查缓存

func SetUserSettings(ctx context.Context, userID int64, settings *UserSettingsCache) error {
	key := redis.Key(userSettingsPrefix, strconv.FormatInt(userID, 10))
	data, _ := json.Marshal(settings)

	return redis.Client().Set(ctx, key, data, userSettingsTTL).Err()
}

// 获取用户缓存设置, 去除 userSettingCache

func GetUserSettings(ctx context.Context, userID int64) (*UserSettingsCache, error) {
	key := redis.Key(userSettingsPrefix, strconv.FormatInt(userID, 10))

	data, err := redis.Client().Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var settings UserSettingsCache
	if err := json.Unmarshal([]byte(data), &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user settings: %w", err)
	}

	return &settings, nil
}

// BatchGetUserSettings 批量获取用户设置缓存 （使用 pipeline ）

func BatchGetUserSettings(ctx context.Context, userIDs []int64) (map[int64]*UserSettingsCache, error) {
	if len(userIDs) == 0 {
		return make(map[int64]*UserSettingsCache), nil
	}

	pipe := redis.Client().Pipeline()
	// 开启一个 pipeline ，通过 pipeline 快速获取 redis 中的内容
	cmds := make(map[int64]*ri.StringCmd)

	for _, userID := range userIDs {
		key := redis.Key(userSettingsPrefix, strconv.FormatInt(userID, 10))
		cmds[userID] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)

	if err != nil && err != ri.Nil {
		return nil, fmt.Errorf("failed to batch get user settings: %w", err)
	}

	result := make(map[int64]*UserSettingsCache)
	for userID, cmd := range cmds {
		data, err := cmd.Result()
		if err != nil {
			// 缓存未命中，跳过
			continue
		}

		var settings UserSettingsCache
		if err := json.Unmarshal([]byte(data), &settings); err != nil {
			// 反序列化失败，跳过
			continue
		}

		result[userID] = &settings
	}

	return result, nil
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

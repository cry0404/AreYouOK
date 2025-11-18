package cache

import (
	"AreYouOK/storage/redis"
	"context"
	"encoding/json"
	"strconv"
	"time"
	"fmt"
)

// 用于缓存 user 部分的设置，避免设置更改后频繁去查数据库

const (
	userSettingsPrefix = "user:settings"
	userSettingsTTL    = 24 * time.Hour
)

type UserSettingsCache struct {
	DailyCheckInEnabled    bool   `json:"daily_check_in_enabled"`
	DailyCheckInRemindAt   string `json:"daily_check_in_remind_at"`
	DailyCheckInDeadline   string `json:"daily_check_in_deadline"`
	DailyCheckInGraceUntil string `json:"daily_check_in_grace_until"`
	Status                 string `json:"status"`
	UpdatedAt              int64  `json:"updated_at"`
}

//如果更新了，在缓存中就能获取，如果没更新，直接就可以发送，所以发送前只需要检查缓存

func SetUserSettings(ctx context.Context, userID int64, settings *UserSettingsCache) error {
	key := redis.Key(userSettingsPrefix, strconv.FormatInt(userID, 10))
	data, _ := json.Marshal(settings)

	return redis.Client().Set(ctx, key, data, userSettingsTTL).Err()
}

// 获取用户缓存设置

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
package cache

import (
	"context"
	"time"
	"AreYouOK/storage/redis"

)

// 实现分布式锁，防止重发，通过 SetNx 即可实现一个分布式锁，来为多个消费者来定义
const (
	lockPrefix = "ayok:lock"
)


// TODO：每一个的 taskid 是独特的，发送完后第二天才结束？ 将 ttl 设置成 tomorrow ？
func TryLock(ctx context.Context, key string, ttl time.Duration) (bool, error) {

	fullkey := redis.Key(lockPrefix, key)

	result, err := redis.Client().SetNX(ctx, fullkey, 1, ttl).Result()

	if err != nil {
		return false, err
	}

	return result, err
}

func Unlock(ctx context.Context, key string) error {
	fullkey := redis.Key(lockPrefix, key)

	return redis.Client().Del(ctx, fullkey).Err()
}
package redis

// 	"context"
// "github.com/redis/go-redis/extra/redisotel/v9" 这部分可以支撑 OpenTelemetry 的检测来监控对应的性能

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"

	"AreYouOK/config"
	pkredis "AreYouOK/pkg/redis"
)

var (
	client redis.UniversalClient // 使用 UniversalClient 接口，支持普通模式和 Sentinel 模式
	once   sync.Once
	err    error
)

func Init() error {
	once.Do(func() {
		cfg := config.Cfg

		// 根据配置选择连接模式
		if cfg.RedisSentinelEnabled && cfg.RedisSentinelAddrs != "" {
			// Sentinel 模式
			sentinelAddrs := strings.Split(cfg.RedisSentinelAddrs, ",")
			client = redis.NewFailoverClient(&redis.FailoverOptions{
				MasterName:       cfg.RedisSentinelMaster,
				SentinelAddrs:    sentinelAddrs,
				Password:         cfg.RedisPassword,
				DB:               cfg.RedisDB,
				DialTimeout:      5 * time.Second,
				ReadTimeout:      3 * time.Second,
				WriteTimeout:     3 * time.Second,
				MinIdleConns:     5,
				MaxRetries:       3,
				SentinelPassword: "", // 如果 Sentinel 需要密码，可以添加配置
			})
		} else {
			// 普通模式
			client = redis.NewClient(&redis.Options{
				Addr:         cfg.RedisAddr,
				Password:     cfg.RedisPassword,
				DB:           cfg.RedisDB,
				DialTimeout:  5 * time.Second,
				ReadTimeout:  3 * time.Second,
				WriteTimeout: 3 * time.Second,
				MinIdleConns: 5,
				MaxRetries:   3,
			})
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err = client.Ping(ctx).Err(); err != nil {
			return
		}

		// 集成 OpenTelemetry Redis 追踪
		serviceName := "areyouok-api"

		// 处理对应来自不同服务的 name
		if os.Getenv("SERVICE_NAME") == "worker" {
			serviceName = "areyouok-worker"
		}

		// UniversalClient 兼容 InstrumentRedisClient
		if c, ok := client.(*redis.Client); ok {
			pkredis.InstrumentRedisClient(c, serviceName, cfg.RedisDB)
		}

		// 初始化 Redis 指标
		meter := otel.Meter(serviceName)
		if err := pkredis.InitRedisMetrics(meter); err != nil {
			
		}
	})

	return err
}

// Client 返回 Redis UniversalClient
func Client() redis.UniversalClient {
	if client == nil {
		panic("Redis client not init")
	}
	return client
}

// Close 关闭 Redis 连接
func Close(ctx context.Context) error {
	if client == nil {
		return nil
	}
	return client.Close()
}

func Key(parts ...string) string {
	prefix := config.Cfg.RedisPrefix
	if prefix == "" {
		prefix = "ayok"
	}

	var sb strings.Builder
	sb.WriteString(prefix)
	for _, part := range parts {
		if part != "" {
			sb.WriteString(":")
			sb.WriteString(part)
		}
	}

	return sb.String()
}

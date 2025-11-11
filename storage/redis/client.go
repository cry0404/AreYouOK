package redis

// 	"context"
//"github.com/redis/go-redis/extra/redisotel/v9" 这部分可以支撑 OpenTelemetry 的检测来监控对应的性能

import (
	"AreYouOK/config"
	"context"
	//"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	//"go.uber.org/zap"
)

var (
	client *redis.Client
	once   sync.Once
	err    error

)

func Init() error {
	once.Do(func() {
		cfg := config.Cfg

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

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		defer cancel()

		if err = client.Ping(ctx).Err(); err != nil {
			return
		}

		// 可选：集成 OpenTelemetry（后续）
		// if err := redisotel.InstrumentTracing(client); err != nil {
		//     logger.Logger.Warn("Failed to instrument Redis tracing", zap.Error(err))
		// }
	})

	return err
}

func Client() *redis.Client {
	if client == nil {
		panic("Redis client not init")
	}
	return client
}

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

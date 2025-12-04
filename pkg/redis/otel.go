package redis

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Redis 相关指标
	redisCommandsTotal     metric.Int64Counter
	redisCommandDuration   metric.Float64Histogram
	//redisConnectionsActive metric.Int64UpDownCounter
	redisCacheHits         metric.Int64Counter
	redisCacheMisses       metric.Int64Counter
)

// InitRedisMetrics 初始化 Redis 指标
func InitRedisMetrics(meter metric.Meter) error {
	var err error

	// Redis 命令总数
	redisCommandsTotal, err = meter.Int64Counter(
		"redis.commands.total",
		metric.WithDescription("Total number of Redis commands"),
		metric.WithUnit("{command}"),
	)
	if err != nil {
		return err
	}

	// Redis 命令耗时
	redisCommandDuration, err = meter.Float64Histogram(
		"redis.command.duration",
		metric.WithDescription("Redis command duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5),
	)
	if err != nil {
		return err
	}

	// // Redis 活跃连接数
	// redisConnectionsActive, err = meter.Int64UpDownCounter(
	// 	"redis.connections.active",
	// 	metric.WithDescription("Number of active Redis connections"),
	// 	metric.WithUnit("{connection}"),
	// )
	// if err != nil {
	// 	return err
	// }

	// Redis 缓存命中
	redisCacheHits, err = meter.Int64Counter(
		"redis.cache.hits",
		metric.WithDescription("Number of cache hits"),
		metric.WithUnit("{hit}"),
	)
	if err != nil {
		return err
	}

	// Redis 缓存未命中
	redisCacheMisses, err = meter.Int64Counter(
		"redis.cache.misses",
		metric.WithDescription("Number of cache misses"),
		metric.WithUnit("{miss}"),
	)
	if err != nil {
		return err
	}

	return nil
}

// TracingHook Redis 追踪 Hook
type TracingHook struct {
	tracer trace.Tracer
	attrs  []attribute.KeyValue
}

// NewTracingHook 创建追踪 Hook
func NewTracingHook(serviceName string, db int) *TracingHook {
	return &TracingHook{
		tracer: otel.Tracer(serviceName + ".redis"),
		attrs: []attribute.KeyValue{
			semconv.DBSystemRedis,
			semconv.DBRedisDBIndex(db),
			attribute.String("service.name", serviceName),
		},
	}
}

// DialHook 实现 redis.Hook 接口
func (th *TracingHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

// ProcessHook 实现 redis.Hook 接口
func (th *TracingHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		spanName := cmd.Name()
		if len(cmd.Args()) > 0 {
			if name, ok := cmd.Args()[0].(string); ok {
				spanName = name
			}
		}

		ctx, span := th.tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(th.attrs...),
		)
		defer span.End()

		// 设置命令属性
		span.SetAttributes(
			semconv.DBOperation(cmd.Name()),
			attribute.String("redis.command", cmd.String()),
		)

		// 提取键名（避免记录敏感数据）
		if cmd.Args() != nil {
			keys := extractKeys(cmd.Args())
			if len(keys) > 0 {
				span.SetAttributes(
					attribute.StringSlice("redis.keys", keys),
				)
			}
		}

		startTime := time.Now()
		err := next(ctx, cmd)
		duration := time.Since(startTime).Seconds()

		// 记录命令执行结果
		status := "success"
		if err != nil {
			if err != redis.Nil {
				status = "error"
				span.SetStatus(codes.Error, err.Error())
				span.RecordError(err)
			} else {
				status = "not_found"
				span.SetStatus(codes.Ok, "Key not found")
			}
		} else {
			span.SetStatus(codes.Ok, "Success")
		}

		// 记录指标
		labels := []attribute.KeyValue{
			attribute.String("redis.command", cmd.Name()),
			attribute.String("redis.status", status),
		}

		redisCommandsTotal.Add(ctx, 1, metric.WithAttributes(labels...))
		redisCommandDuration.Record(ctx, duration, metric.WithAttributes(labels...))

		// 记录缓存命中/未命中
		if cmd.Name() == "GET" || cmd.Name() == "MGET" {
			if err == redis.Nil {
				redisCacheMisses.Add(ctx, 1)
			} else if err == nil {
				redisCacheHits.Add(ctx, 1)
			}
		}

		return err
	}
}

// ProcessPipelineHook 实现 redis.Hook 接口
func (th *TracingHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		ctx, span := th.tracer.Start(ctx, "redis.pipeline",
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(th.attrs...),
		)
		defer span.End()

		commands := make([]string, 0, len(cmds))
		for _, cmd := range cmds {
			commands = append(commands, cmd.String())
		}

		span.SetAttributes(
			attribute.Int("redis.pipeline.count", len(cmds)),
			attribute.String("redis.pipeline.commands", strings.Join(commands, ";")),
		)

		err := next(ctx, cmds)

		// 记录 Pipeline 执行指标
		labels := []attribute.KeyValue{
			attribute.String("redis.operation", "pipeline"),
		}

		successCount := 0
		for _, cmd := range cmds {
			if cmd.Err() == nil {
				successCount++
			}
		}

		span.SetAttributes(
			attribute.Int("redis.pipeline.success_count", successCount),
			attribute.Int("redis.pipeline.error_count", len(cmds)-successCount),
		)

		redisCommandsTotal.Add(ctx, 1, metric.WithAttributes(labels...))

		return err
	}
}

// extractKeys 提取 Redis 命令中的键名（避免记录敏感值）
func extractKeys(args []interface{}) []string {
	keys := make([]string, 0, len(args)-1)

	// 第一个参数通常是命令名，跳过
	for i := 1; i < len(args) && len(keys) < 5; i++ {
		if key, ok := args[i].(string); ok {
			keys = append(keys, sanitizeKey(key))
		}
	}

	return keys
}

// sanitizeKey 清理键名，移除敏感信息
func sanitizeKey(key string) string {
	// 移除常见的敏感模式
	if strings.Contains(key, "token") ||
		strings.Contains(key, "password") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "session") {
		// 返回键名的第一部分，隐藏敏感信息
		parts := strings.Split(key, ":")
		if len(parts) > 1 {
			return parts[0] + ":***"
		}
		return "***"
	}

	// 限制键名长度
	if len(key) > 100 {
		return key[:100] + "..."
	}

	return key
}

// InstrumentRedisClient 为 Redis 客户端添加 OpenTelemetry 支持
func InstrumentRedisClient(client redis.Cmdable, serviceName string, db int) redis.Cmdable {
	hook := NewTracingHook(serviceName, db)
	if cli, ok := client.(*redis.Client); ok {
		cli.AddHook(hook)
		return cli
	}
	return client
}

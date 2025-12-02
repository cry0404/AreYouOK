package middleware

import (
	"context"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/config"
	hertztracing "github.com/hertz-contrib/obs-opentelemetry/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	// HTTP 相关指标
	httpServerRequestTotal   metric.Int64Counter
	httpServerDuration       metric.Float64Histogram
	httpServerRequestSize    metric.Int64Histogram
	httpServerResponseSize   metric.Int64Histogram
	httpServerActiveRequests metric.Int64UpDownCounter
)

// InitMetrics 初始化指标
func InitMetrics(meter metric.Meter) error {
	var err error

	// HTTP 请求总数
	httpServerRequestTotal, err = meter.Int64Counter(
		"http.server.requests.total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	// HTTP 请求耗时
	httpServerDuration, err = meter.Float64Histogram(
		"http.server.duration",
		metric.WithDescription("HTTP request duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0),
	)
	if err != nil {
		return err
	}

	// HTTP 请求大小
	httpServerRequestSize, err = meter.Int64Histogram(
		"http.server.request.size",
		metric.WithDescription("HTTP request size"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	// HTTP 响应大小
	httpServerResponseSize, err = meter.Int64Histogram(
		"http.server.response.size",
		metric.WithDescription("HTTP response size"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	// 活跃请求数
	httpServerActiveRequests, err = meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	return nil
}

// OpenTelemetryMiddleware 创建 OpenTelemetry 中间件
func OpenTelemetryMiddleware() app.HandlerFunc {
	tracer := otel.Tracer("hertz-server")

	return func(ctx context.Context, c *app.RequestContext) {
		startTime := time.Now()

		// 增加活跃请求计数
		httpServerActiveRequests.Add(ctx, 1)

		// 创建 Span
		spanName := string(c.Method()) + " " + string(c.Path())
		spanCtx, span := tracer.Start(ctx, spanName, trace.WithAttributes(
			semconv.HTTPMethod(string(c.Method())),
			semconv.HTTPRoute(string(c.Path())),
			semconv.HTTPURL(c.Request.URI().String()),
			semconv.HTTPScheme(string(c.Request.URI().Scheme())),
			attribute.String("http.host", string(c.Host())),
			attribute.String("http.user_agent", string(c.UserAgent())),
		))
		defer span.End()

		// 添加用户 ID (如果已认证)
		if userID := c.GetString("user_id"); userID != "" {
			span.SetAttributes(attribute.String("enduser.id", userID))
		}

		// 添加请求 ID, 用于 tracing 对应的请求
		if requestID := c.GetHeader("X-Request-Id"); len(requestID) > 0 {
			span.SetAttributes(attribute.String("http.request_id", string(requestID)))
		}

		// 更新上下文
		c.Next(spanCtx)

		// 记录请求指标
		duration := time.Since(startTime).Seconds()
		statusCode := int(c.Response.StatusCode())
		method := string(c.Method())
		path := string(c.Path())

		// 设置 Span 属性
		span.SetAttributes(
			semconv.HTTPStatusCode(statusCode),
			attribute.Float64("http.duration", duration),
		)

		// 根据状态码设置 Span 状态
		if statusCode >= 400 {
			span.SetStatus(codes.Error, "HTTP error")
			if statusCode >= 500 {
				span.RecordError(c.Errors.Last())
			}
		} else {
			span.SetStatus(codes.Ok, "HTTP success")
		}

		// 记录指标
		labels := []attribute.KeyValue{
			semconv.HTTPMethod(method),
			semconv.HTTPRoute(path),
			semconv.HTTPStatusCode(statusCode),
		}

		httpServerRequestTotal.Add(ctx, 1, metric.WithAttributes(labels...))
		httpServerDuration.Record(ctx, duration, metric.WithAttributes(labels...))

		// 记录请求和响应大小
		if requestSize := int64(c.Request.Header.ContentLength()); requestSize > 0 {
			httpServerRequestSize.Record(ctx, requestSize, metric.WithAttributes(labels...))
		}
		if responseSize := int64(len(c.Response.Body())); responseSize > 0 {
			httpServerResponseSize.Record(ctx, responseSize, metric.WithAttributes(labels...))
		}

		// 减少活跃请求计数
		httpServerActiveRequests.Add(ctx, -1)
	}
}

// NewServerTracerConfig 创建 Hertz Server 的追踪配置
// 返回用于初始化 Hertz server 的配置选项和追踪中间件
func NewServerTracerConfig(opts ...hertztracing.Option) (config.Option, app.HandlerFunc) {
	tracer, cfg := hertztracing.NewServerTracer(opts...)
	return tracer, hertztracing.ServerMiddleware(cfg)
}

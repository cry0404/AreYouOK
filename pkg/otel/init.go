package otel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// Config OpenTelemetry 配置
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	OTLPEndpoint   string
	SampleRatio    float64
}

// InitOpenTelemetry 初始化 OpenTelemetry
func InitOpenTelemetry(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.SampleRatio == 0 {
		cfg.SampleRatio = 0.1 // 默认 10% 采样
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}

	if cfg.Environment == "development" {
		cfg.SampleRatio = 1 // 测试时百分百采样率用于监控
	}
	// 1. 创建 Resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
			semconv.ServiceNamespace("areyouok"),
			semconv.TelemetrySDKLanguageGo,
		),
		resource.WithHost(),
		resource.WithOSType(),
		resource.WithOSDescription(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 2. 初始化 TracerProvider
	tracerProvider, err := initTracerProvider(ctx, res, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tracer provider: %w", err)
	}

	// 3. 初始化 MetricProvider
	metricProvider, err := initMeterProvider(ctx, res, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize meter provider: %w", err)
	}

	// 4. 设置全局 Provider
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(metricProvider)

	// 5. 设置 Propagator
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	// 6. 返回清理函数
	return func(c context.Context) error {
		ctx, cancel := context.WithTimeout(c, 5*time.Second)
		defer cancel()

		var errs []error
		if err := tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer shutdown error: %w", err))
		}
		if err := metricProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter shutdown error: %w", err))
		}

		if len(errs) > 0 {
			return fmt.Errorf("shutdown errors: %v", errs)
		}
		return nil
	}, nil
}

// initTracerProvider 初始化 TracerProvider
func initTracerProvider(ctx context.Context, res *resource.Resource, cfg Config) (*sdktrace.TracerProvider, error) {
	// 确保 endpoint 格式正确（移除 http:// 前缀如果存在）
	endpoint := cfg.OTLPEndpoint
	if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
	} else if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
	}

	// 创建 OTLP exporter
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// 创建 TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(
			traceExporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithExportTimeout(10*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
			sdktrace.WithMaxExportBatchSize(1024),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(cfg.SampleRatio),
		)),
	)

	return tp, nil
}

// initMeterProvider 初始化 MeterProvider
func initMeterProvider(ctx context.Context, res *resource.Resource, cfg Config) (*sdkmetric.MeterProvider, error) {
	// 确保 endpoint 格式正确（移除 http:// 前缀如果存在）
	endpoint := cfg.OTLPEndpoint
	if strings.HasPrefix(endpoint, "http://") {
		endpoint = strings.TrimPrefix(endpoint, "http://")
	} else if strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimPrefix(endpoint, "https://")
	}

	// 创建 OTLP exporter
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// 创建 MeterProvider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(
				metricExporter,
				sdkmetric.WithInterval(15*time.Second),
				sdkmetric.WithTimeout(5*time.Second),
			),
		),
		sdkmetric.WithResource(res),
	)

	return mp, nil
}

// GetServiceAttributes 获取服务属性
func GetServiceAttributes(serviceName, serviceVersion, environment string) []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(serviceVersion),
		semconv.DeploymentEnvironment(environment),
		semconv.ServiceNamespace("areyouok"),
	}
}

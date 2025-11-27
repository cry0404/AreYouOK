package otel
// otel 是一组工具集
// 可观测性的三个关键 ： Metrics、 Logs 、 Traces
// otel 用统一的 api 来生成三种数据，确认其可以关联起来，并且这些数据都是以标准化的格式进行存储的
// 这样就可以方便的进行数据分析和监控
// 
// otel 的三个关键概念：
// 1. Resource：资源，用于描述一个实体，如一个服务、一个容器、一个虚拟机等
// 2. Span：跨度，用于描述一个操作，如一个函数、一个方法、一个请求等
// 3. Metric：指标，用于描述一个度量，如一个计数、一个速率、一个百分比等
// 4. Log：日志，用于描述一个事件，如一个错误、一个警告、一个信息等
// 5. Trace：追踪，用于描述一个操作的轨迹，如一个请求的完整路径、一个操作的耗时等
// 6. Context：上下文，用于描述一个操作的上下文，如一个请求的上下文、一个操作的上下文等
// 7. Propagation：传播，用于描述一个操作的传播，如一个请求的传播、一个操作的传播等



// Resource 是 OpenTelemetry 的核心概念之一
// 它描述了产生 telemetry 数据的实体（服务、容器、主机等）
// 这些信息会被附加到所有的 spans 和 metrics 上，用于标识数据的来源

/*
Resource (资源描述)
    ↓
TracerProvider (追踪提供者)
    ├── Sampler (采样器) - 决定是否采样
    ├── Resource (资源) - 服务元信息
    ├── BatchSpanProcessor (批处理器) - 批量处理 spans
    └── Exporter (导出器) - 发送数据到 Collector
         ↓
    OpenTelemetry Collector
         ↓
    Jaeger/Tempo/Grafana
*/
import (
	// "os"

	// "go.opentelemetry.io/otel/attribute"
	// "go.opentelemetry.io/otel/sdk/resource"
	// semconv "go.opentelemetry.io/otel/semconv/v1.9.0"
)


package database

import (
	"context"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

var (
	// 数据库相关指标
	dbQueriesTotal     metric.Int64Counter
	dbQueryDuration    metric.Float64Histogram
	dbConnectionsTotal metric.Int64Counter
	dbConnectionsActive metric.Int64UpDownCounter
	dbTransactionTotal metric.Int64Counter
)

// InitDatabaseMetrics 初始化数据库指标
func InitDatabaseMetrics(meter metric.Meter) error {
	var err error

	// 数据库查询总数
	dbQueriesTotal, err = meter.Int64Counter(
		"db.queries.total",
		metric.WithDescription("Total number of database queries"),
		metric.WithUnit("{query}"),
	)
	if err != nil {
		return err
	}

	// 数据库查询耗时
	dbQueryDuration, err = meter.Float64Histogram(
		"db.query.duration",
		metric.WithDescription("Database query duration"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0),
	)
	if err != nil {
		return err
	}

	// 数据库连接总数
	dbConnectionsTotal, err = meter.Int64Counter(
		"db.connections.total",
		metric.WithDescription("Total number of database connections"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return err
	}

	// 数据库活跃连接数
	dbConnectionsActive, err = meter.Int64UpDownCounter(
		"db.connections.active",
		metric.WithDescription("Number of active database connections"),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		return err
	}

	// 数据库事务总数
	dbTransactionTotal, err = meter.Int64Counter(
		"db.transactions.total",
		metric.WithDescription("Total number of database transactions"),
		metric.WithUnit("{transaction}"),
	)
	if err != nil {
		return err
	}

	return nil
}

// OTELPlugin GORM OpenTelemetry 插件
type OTELPlugin struct {
	tracer trace.Tracer
	config PluginConfig
}

// PluginConfig 插件配置
type PluginConfig struct {
	ServiceName     string
	EnableSQLParams bool
	EnableMetrics   bool
	MaxSQLLength    int
}

// DefaultPluginConfig 默认插件配置
func DefaultPluginConfig() PluginConfig {
	return PluginConfig{
		ServiceName:     "areyouok",
		EnableSQLParams: false, // 默认不记录 SQL 参数，避免敏感信息泄露
		EnableMetrics:   true,
		MaxSQLLength:    500, // 最大 SQL 长度
	}
}

// NewOTELPlugin 创建插件实例
func NewOTELPlugin(config PluginConfig) *OTELPlugin {
	if config.ServiceName == "" {
		config.ServiceName = "areyouok"
	}

	return &OTELPlugin{
		tracer: otel.Tracer(config.ServiceName + ".gorm"),
		config: config,
	}
}

// Name 实现 gorm.Plugin 接口
func (p *OTELPlugin) Name() string {
	return "otel_plugin"
}

// Initialize 注册回调
func (p *OTELPlugin) Initialize(db *gorm.DB) error {
	// 注册各种操作的回调
	callbacks := db.Callback()

	// 查询操作
	callbacks.Query().Before("gorm:query").Register("otel:before_query", p.beforeCallback)
	callbacks.Query().After("gorm:query").Register("otel:after_query", p.afterCallback)

	// 创建操作
	callbacks.Create().Before("gorm:create").Register("otel:before_create", p.beforeCallback)
	callbacks.Create().After("gorm:create").Register("otel:after_create", p.afterCallback)

	// 更新操作
	callbacks.Update().Before("gorm:update").Register("otel:before_update", p.beforeCallback)
	callbacks.Update().After("gorm:update").Register("otel:after_update", p.afterCallback)

	// 删除操作
	callbacks.Delete().Before("gorm:delete").Register("otel:before_delete", p.beforeCallback)
	callbacks.Delete().After("gorm:delete").Register("otel:after_delete", p.afterCallback)

	// Row 操作
	callbacks.Row().Before("gorm:row").Register("otel:before_row", p.beforeCallback)
	callbacks.Row().After("gorm:row").Register("otel:after_row", p.afterCallback)

	// Raw 操作
	callbacks.Raw().Before("gorm:raw").Register("otel:before_raw", p.beforeCallback)
	callbacks.Raw().After("gorm:raw").Register("otel:after_raw", p.afterCallback)

	return nil
}

// beforeCallback 操作前回调
func (p *OTELPlugin) beforeCallback(db *gorm.DB) {
	ctx := db.Statement.Context
	operation := p.getOperationName(db)

	// 创建 Span
	ctx, span := p.tracer.Start(ctx, operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(p.getDBAttributes(db)...),
	)

	// 存储开始时间
	db.InstanceSet("otel:start_time", time.Now())
	db.InstanceSet("otel:span", span)

	// 更新上下文
	db.Statement.Context = ctx
}

// afterCallback 操作后回调
func (p *OTELPlugin) afterCallback(db *gorm.DB) {
	// 获取 Span 和开始时间
	span, exists := db.InstanceGet("otel:span")
	if !exists {
		return
	}

	startTimeI, exists := db.InstanceGet("otel:start_time")
	if !exists {
		return
	}

	startTime, ok := startTimeI.(time.Time)
	if !ok {
		return
	}

	otelSpan, ok := span.(trace.Span)
	if !ok {
		return
	}
	defer otelSpan.End()

	// 计算耗时
	duration := time.Since(startTime).Seconds()

	// 设置属性和状态
	p.setSpanAttributes(otelSpan, db)
	p.setSpanStatus(otelSpan, db)

	// 记录指标
	if p.config.EnableMetrics {
		p.recordMetrics(db.Statement.Context, db, duration)
	}
}

// getOperationName 获取操作名称
func (p *OTELPlugin) getOperationName(db *gorm.DB) string {
	table := db.Statement.Table
	if table == "" {
		table = "unknown"
	}

	switch db.Statement.SQL.String() {
	case "":
		return "db.unknown"
	default:
		// 从 SQL 中提取操作类型
		sql := strings.ToUpper(strings.TrimSpace(db.Statement.SQL.String()))
		if strings.HasPrefix(sql, "SELECT") {
			return "db.select"
		} else if strings.HasPrefix(sql, "INSERT") {
			return "db.insert"
		} else if strings.HasPrefix(sql, "UPDATE") {
			return "db.update"
		} else if strings.HasPrefix(sql, "DELETE") {
			return "db.delete"
		} else {
			return "db.query"
		}
	}
}

// getDBAttributes 获取数据库属性
func (p *OTELPlugin) getDBAttributes(db *gorm.DB) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		semconv.DBSystemPostgreSQL,
		attribute.String("db.name", p.config.ServiceName),
		attribute.String("service.name", p.config.ServiceName),
	}

	// 添加表名
	if table := db.Statement.Table; table != "" {
		attrs = append(attrs, attribute.String("db.table", table))
	}

	// 添加 SQL 语句（截断长 SQL）
	sql := db.Statement.SQL.String()
	if len(sql) > p.config.MaxSQLLength {
		sql = sql[:p.config.MaxSQLLength] + "..."
	}

	// 清理 SQL 语句，移除敏感信息
	cleanSQL := p.sanitizeSQL(sql)
	attrs = append(attrs, semconv.DBStatement(cleanSQL))

	// 添加 SQL 参数（如果启用）
	if p.config.EnableSQLParams && len(db.Statement.Vars) > 0 {
		// 这里只记录参数数量，避免记录敏感值
		attrs = append(attrs, attribute.Int("db.parameter_count", len(db.Statement.Vars)))
	}

	return attrs
}

// sanitizeSQL 清理 SQL 语句，移除敏感信息
func (p *OTELPlugin) sanitizeSQL(sql string) string {
	// 移除常见的敏感参数
	sql = strings.ToLower(sql)

	// 移除密码字段
	re := regexp.MustCompile(`password\s*=\s*'[^\']*'`)
	sql = re.ReplaceAllString(sql, "password='***'")

	// 移除 token 字段
	re = regexp.MustCompile(`token\s*=\s*'[^\']*'`)
	sql = re.ReplaceAllString(sql, "token='***'")

	// 移除 secret 字段
	re = regexp.MustCompile(`secret\s*=\s*'[^\']*'`)
	sql = re.ReplaceAllString(sql, "secret='***'")

	return sql
}

// setSpanAttributes 设置 Span 属性
func (p *OTELPlugin) setSpanAttributes(span trace.Span, db *gorm.DB) {
	// 添加影响的行数
	if db.Statement.RowsAffected >= 0 {
		span.SetAttributes(attribute.Int64("db.rows_affected", db.Statement.RowsAffected))
	}

	// 添加错误信息
	if db.Error != nil && db.Error != gorm.ErrRecordNotFound {
		span.SetAttributes(attribute.String("db.error", db.Error.Error()))
	}
}

// setSpanStatus 设置 Span 状态
func (p *OTELPlugin) setSpanStatus(span trace.Span, db *gorm.DB) {
	if db.Error != nil {
		if db.Error == gorm.ErrRecordNotFound {
			span.SetStatus(codes.Ok, "Record not found")
		} else {
			span.SetStatus(codes.Error, db.Error.Error())
			span.RecordError(db.Error)
		}
	} else {
		span.SetStatus(codes.Ok, "Success")
	}
}

// recordMetrics 记录指标
func (p *OTELPlugin) recordMetrics(ctx context.Context, db *gorm.DB, duration float64) {
	operation := p.getOperationName(db)
	status := "success"
	if db.Error != nil {
		status = "error"
	}

	labels := []attribute.KeyValue{
		attribute.String("db.operation", operation),
		attribute.String("db.status", status),
	}

	dbQueriesTotal.Add(ctx, 1, metric.WithAttributes(labels...))
	dbQueryDuration.Record(ctx, duration, metric.WithAttributes(labels...))
}

// WithOTELPlugin 为 GORM 添加 OpenTelemetry 插件
func WithOTELPlugin(db *gorm.DB, config PluginConfig) error {
	plugin := NewOTELPlugin(config)
	return db.Use(plugin)
}

// WithDefaultOTELPlugin 使用默认配置添加 OpenTelemetry 插件
func WithDefaultOTELPlugin(db *gorm.DB, serviceName string) error {
	config := DefaultPluginConfig()
	config.ServiceName = serviceName
	return WithOTELPlugin(db, config)
}
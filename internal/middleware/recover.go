package middleware

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"go.uber.org/zap"

	"AreYouOK/config"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/response"
)

// RecoverConfig recover 中间件配置
type RecoverConfig struct {
	// 是否启用堆栈追踪
	EnableStackTrace bool
	// 堆栈追踪级别（full, simple, none）
	StackTraceLevel string
	// 生产环境是否返回详细错误
	ExposeDetailsInProduction bool
	// 日志级别（debug, info, warn, error）
	LogLevel string
	// 是否记录请求详情
	LogRequestDetails bool
	// 是否在 span 中记录异常（OpenTelemetry）
	RecordInSpan bool
	// 严重错误回调函数（可用于发送告警）
	OnSevereError func(ctx context.Context, c *app.RequestContext, err interface{}, stack []byte)
	// 是否是生产环境
	IsProduction bool
}

// NewRecoverConfig 创建 recover 配置
func NewRecoverConfig() RecoverConfig {
	return RecoverConfig{
		EnableStackTrace:          true,
		StackTraceLevel:           "simple",
		ExposeDetailsInProduction: false,
		LogLevel:                  "error",
		LogRequestDetails:         true,
		RecordInSpan:              true,
		OnSevereError:             nil,
		IsProduction:              config.Cfg.IsProduction(),
	}
}

// DefaultRecoverConfig 默认配置
var DefaultRecoverConfig = NewRecoverConfig()

// RecoverMiddleware 创建 recover 中间件
func RecoverMiddleware() app.HandlerFunc {
	return RecoverMiddlewareWithConfig(DefaultRecoverConfig)
}

// RecoverMiddlewareWithConfig 带配置的 recover 中间件
func RecoverMiddlewareWithConfig(config RecoverConfig) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		defer func() {
			if err := recover(); err != nil {
				// 处理 panic
				handlePanic(ctx, c, err, config)
			}
		}()

		// 继续处理请求
		c.Next(ctx)
	}
}

// handlePanic 处理 panic 并记录日志
func handlePanic(ctx context.Context, c *app.RequestContext, err interface{}, config RecoverConfig) {
	// 获取堆栈信息
	var stack []byte
	if config.EnableStackTrace {
		stack = getStackTrace(config.StackTraceLevel)
	}

	

	// 记录日志
	logPanic(ctx, c, err, stack, config)

	// 调用严重错误回调（如果配置）
	if config.OnSevereError != nil {
		config.OnSevereError(ctx, c, err, stack)
	}

	// 返回响应
	writeErrorResponse(c, err, stack, config)
}

// logPanic 记录 panic 日志
func logPanic(ctx context.Context, c *app.RequestContext, err interface{}, stack []byte, config RecoverConfig) {
	logPanicWithRequest(ctx, c, err, stack, config)
}

// writeErrorResponse 返回错误响应
func writeErrorResponse(c *app.RequestContext, err interface{}, stack []byte, config RecoverConfig) {
	// 创建错误响应
	var errDef errors.Definition
	if config.IsProduction && !config.ExposeDetailsInProduction {
		// 生产环境返回友好提示
		errDef = errors.Definition{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: "服务器内部错误，请稍后重试",
		}
	} else {
		// 开发环境返回详细错误
		errDef = errors.Definition{
			Code:    "INTERNAL_SERVER_ERROR",
			Message: fmt.Sprintf("Internal error: %v", err),
		}
	}

	// 添加详情
	var details map[string]interface{}
	if !config.IsProduction || config.ExposeDetailsInProduction {
		details = map[string]interface{}{
			"panic":     fmt.Sprintf("%v", err),
			"timestamp": time.Now().Format(time.RFC3339),
		}

		if config.EnableStackTrace {
			details["stack"] = string(stack)
		}
	}

	// 返回响应
	if details != nil {
		response.ErrorWithDetails(context.Background(), c, errDef, details)
	} else {
		response.Error(context.Background(), c, errDef)
	}
}

// getStackTrace 获取堆栈追踪
func getStackTrace(level string) []byte {
	var buf bytes.Buffer

	switch level {
	case "full":
		// 完整的堆栈信息（所有 goroutine）
		buf.Write(debug.Stack())
	case "simple":
		// 简化的堆栈信息（当前 goroutine 的调用栈）
		buf.WriteString("goroutine panic:\n")
		skip := 3 // 跳过 runtime 和 recover 相关的函数
		for i := skip; ; i++ {
			pc, file, line, ok := runtime.Caller(i)
			if !ok {
				break
			}
			fn := runtime.FuncForPC(pc)
			if fn == nil {
				continue
			}
			buf.WriteString(fmt.Sprintf("  %s:%d\n    %s\n", file, line, fn.Name()))
		}
	}

	return buf.Bytes()
}

// getFormattedStack 格式化堆栈信息（移除冗余信息）
func getFormattedStack(stack []byte) []byte {
	if len(stack) == 0 {
		return nil
	}

	// 移除 runtime 相关的冗余堆栈
	lines := strings.Split(string(stack), "\n")
	var filtered []string

	for i, line := range lines {
		if strings.Contains(line, "runtime/panic.go") ||
			strings.Contains(line, "runtime/defer.go") ||
			strings.Contains(line, "signal_unix.go") {
			continue
		}
		// 保留非 runtime 的堆栈行
		if !strings.Contains(line, "/runtime/") && !strings.Contains(line, "src/runtime/") {
			if i < len(lines)-1 && strings.Contains(lines[i+1], "\tsrc/runtime/") {
				continue
			}
			filtered = append(filtered, line)
		}
	}

	return []byte(strings.Join(filtered, "\n"))
}

// logPanicWithRequest 记录 panic 日志（包含请求详情）
func logPanicWithRequest(ctx context.Context, c *app.RequestContext, err interface{}, stack []byte, config RecoverConfig) {
	// 创建日志字段
	fields := []zap.Field{
		zap.String("panic", fmt.Sprintf("%v", err)),
		zap.String("path", string(c.Path())),
		zap.String("method", string(c.Method())),
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", string(c.UserAgent())),
	}

	// 请求ID
	requestID := string(c.GetHeader("X-Request-ID"))
	if requestID == "" {
		requestID = string(c.GetHeader("X-Trace-ID"))
	}
	fields = append(fields, zap.String("request_id", requestID))

	// 用户ID
	if userID, exists := GetUserID(ctx, c); exists {
		fields = append(fields, zap.String("user_id", userID))
	}

	// 如果启用详细日志
	if config.LogRequestDetails {
		// 请求头
		headers := make(map[string]string)
		c.Request.Header.VisitAll(func(key, value []byte) {
			headers[string(key)] = string(value)
		})
		fields = append(fields, zap.Any("headers", headers))

		// 请求体（谨慎记录）
		body := c.Request.Body()
		if len(body) > 0 && len(body) < 1024 {
			contentType := string(c.ContentType())
			if !strings.Contains(contentType, "multipart") &&
				!strings.Contains(contentType, "image") &&
				!strings.Contains(contentType, "video") {
				fields = append(fields, zap.String("body", string(body)))
			}
		}
	}

	// 堆栈信息
	if config.EnableStackTrace {
		fields = append(fields, zap.ByteString("stack", getFormattedStack(stack)))
	}

	// 记录到 span（OpenTelemetry）
	if config.RecordInSpan {
		// TODO: 集成 OpenTelemetry span 记录
	}

	// 记录日志
	switch config.LogLevel {
	case "debug":
		logger.Logger.Debug("[PANIC RECOVERED]", fields...)
	case "info":
		logger.Logger.Info("[PANIC RECOVERED]", fields...)
	case "warn":
		logger.Logger.Warn("[PANIC RECOVERED]", fields...)
	default:
		logger.Logger.Error("[PANIC RECOVERED]", fields...)
	}

	// 严重错误
	if isSeverePanic(err) {
		logger.Logger.Error("[SEVERE PANIC DETECTED]", fields...)
	}
}

// isSeverePanic 判断是否为严重错误
func isSeverePanic(err interface{}) bool {
	if err == nil {
		return false
	}

	errStr := fmt.Sprintf("%v", err)

	// 检查是否为严重错误
	severePatterns := []string{
		"runtime: out of memory",
		"fatal error:",
		"concurrent map writes",
		"concurrent map read and map write",
		"runtime error: makeslice:", // OOM
		"all goroutines are asleep - deadlock!",
		"index out of range",          // 可能严重
		"slice bounds out of range",   // 可能严重
		"unexpected signal",           // 系统信号
	}

	for _, pattern := range severePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// SendAlertOnSeverePanic 严重错误时发送告警的示例实现
func SendAlertOnSeverePanic(ctx context.Context, c *app.RequestContext, err interface{}, stack []byte) {
	// 发送钉钉告警
	// sendDingTalkAlert(errorMsg, stack)

	// 发送邮件告警
	// sendEmailAlert(errorMsg, stack)

	// 记录到专门的错误日志
	// logger.SevereLogger.Error("[SEVERE PANIC]", fields...)

	// TODO: 集成实际的通知服务
	logger.Logger.Error("[ALERT TRIGGERED] Severity panic detected", zap.String("panic", fmt.Sprintf("%v", err)))
}

// RecoverMiddlewareWithAlert 带告警功能的 recover 中间件
func RecoverMiddlewareWithAlert(webhookURL string, mentionList []string) app.HandlerFunc {
	config := DefaultRecoverConfig
	config.OnSevereError = func(ctx context.Context, c *app.RequestContext, err interface{}, stack []byte) {
		// 构建告警消息
		msg := fmt.Sprintf("🚨 **严重错误告警**\n\n"+
			"**错误**: %v\n"+
			"**路径**: %s %s\n"+
			"**用户**: %s\n"+
			"**时间**: %s\n"+
			"**堆栈**: ```\n%s\n```",
			err,
			string(c.Method()), string(c.Path()),
			getUserInfo(ctx, c),
			time.Now().Format("2006-01-02 15:04:05"),
			getShortStack(stack),
		)

		// TODO: 调用钉钉/飞书/Slack webhook
		// sendToWebhook(webhookURL, msg, mentionList)
		logger.Logger.Error("[ALERT] Send to webhook", zap.String("message", msg))
	}

	return RecoverMiddlewareWithConfig(config)
}

// getUserInfo 获取用户信息
func getUserInfo(ctx context.Context, c *app.RequestContext) string {
	var info strings.Builder

	if userID, exists := GetUserID(ctx, c); exists {
		info.WriteString(fmt.Sprintf("UserID: %s", userID))
	}

	info.WriteString(fmt.Sprintf(", IP: %s", c.ClientIP()))
	info.WriteString(fmt.Sprintf(", UA: %s", string(c.UserAgent())))

	return info.String()
}

// getShortStack 获取简化的堆栈（只显示关键行）
func getShortStack(stack []byte) string {
	if len(stack) == 0 {
		return ""
	}

	lines := strings.Split(string(stack), "\n")
	if len(lines) > 20 {
		// 只保留前20行和后10行
		short := append(lines[:20], "...\n(middle part omitted)\n...")
		short = append(short, lines[len(lines)-10:]...)
		return strings.Join(short, "\n")
	}

	return string(stack)
}
